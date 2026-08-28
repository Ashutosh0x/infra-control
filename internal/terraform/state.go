package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// StateFormatVersion is the only state format this parser accepts.
//
// Terraform has used format 4 for every 0.12 and later release, and OpenTofu
// inherited it. Refusing an unknown version is deliberate: silently parsing a
// format we do not understand would produce a resource list that is wrong in
// ways the drift engine cannot detect.
const StateFormatVersion = 4

// State is a parsed Terraform or OpenTofu state file.
type State struct {
	Version          int                    `json:"version"`
	TerraformVersion string                 `json:"terraform_version"`
	Serial           uint64                 `json:"serial"`
	Lineage          string                 `json:"lineage"`
	Outputs          map[string]StateOutput `json:"outputs"`
	Resources        []StateResourceBlock   `json:"resources"`
}

// StateOutput is a root module output value.
type StateOutput struct {
	Value     any  `json:"value"`
	Sensitive bool `json:"sensitive"`
}

// StateResourceBlock is one resource block in state, which may hold several
// instances when the resource uses count or for_each.
type StateResourceBlock struct {
	Module    string          `json:"module"`
	Mode      string          `json:"mode"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider"`
	Instances []StateInstance `json:"instances"`
}

// StateInstance is a single instantiation of a resource block.
type StateInstance struct {
	// IndexKey is the count index or for_each key. It is absent for a plain
	// singleton resource, an int for count, and a string for for_each.
	IndexKey any `json:"index_key,omitempty"`

	SchemaVersion int            `json:"schema_version"`
	Attributes    map[string]any `json:"attributes"`
	Dependencies  []string       `json:"dependencies,omitempty"`

	// SensitiveAttributes lists attribute paths the provider marked sensitive.
	// Terraform encodes these as cty path steps rather than dotted strings.
	SensitiveAttributes []json.RawMessage `json:"sensitive_attributes,omitempty"`
}

// ParseStateFile reads and parses a state file from disk.
func ParseStateFile(path string) (*State, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open state file %s: %w", path, err)
	}
	defer f.Close()

	state, err := ParseState(f)
	if err != nil {
		return nil, fmt.Errorf("parse state file %s: %w", path, err)
	}
	return state, nil
}

// ParseState parses state from a reader.
func ParseState(r io.Reader) (*State, error) {
	var state State

	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&state); err != nil {
		return nil, fmt.Errorf("decode state json: %w", err)
	}

	if state.Version != StateFormatVersion {
		return nil, fmt.Errorf(
			"unsupported state format version %d (this build reads version %d, written by Terraform 0.12+ and all OpenTofu releases)",
			state.Version, StateFormatVersion,
		)
	}

	return &state, nil
}

// ManagedResources flattens the state into one StateResource per instance,
// skipping data sources.
//
// Data sources are excluded because they describe things Terraform reads but
// does not own; reporting drift on them would flag every upstream change as an
// unauthorised edit.
func (s *State) ManagedResources() []StateResource {
	var resources []StateResource

	for _, block := range s.Resources {
		if block.Mode != "managed" {
			continue
		}

		for _, instance := range block.Instances {
			resources = append(resources, StateResource{
				Address:      buildAddress(block, instance),
				Mode:         block.Mode,
				Type:         block.Type,
				Name:         block.Name,
				Provider:     normaliseProvider(block.Provider),
				Module:       block.Module,
				Attributes:   normaliseAttributes(instance.Attributes),
				Dependencies: instance.Dependencies,
			})
		}
	}

	// A stable order makes scan output reproducible across runs, which matters
	// when the result is committed or diffed in CI.
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].Address < resources[j].Address
	})
	return resources
}

// buildAddress reconstructs the canonical Terraform address for an instance,
// for example `module.vpc.aws_subnet.private[2]`.
func buildAddress(block StateResourceBlock, instance StateInstance) string {
	var b strings.Builder

	if block.Module != "" {
		b.WriteString(block.Module)
		b.WriteString(".")
	}
	b.WriteString(block.Type)
	b.WriteString(".")
	b.WriteString(block.Name)

	switch key := instance.IndexKey.(type) {
	case nil:
		// Singleton resource: no index suffix.
	case string:
		fmt.Fprintf(&b, "[%q]", key)
	case json.Number:
		fmt.Fprintf(&b, "[%s]", key.String())
	case float64:
		fmt.Fprintf(&b, "[%d]", int64(key))
	default:
		fmt.Fprintf(&b, "[%v]", key)
	}

	return b.String()
}

// normaliseProvider strips the registry host and bracket syntax from a provider
// reference, turning `provider["registry.terraform.io/hashicorp/aws"]` into `aws`.
func normaliseProvider(provider string) string {
	if provider == "" {
		return ""
	}

	// Trim the provider[...] wrapper if present.
	if start := strings.Index(provider, `["`); start >= 0 {
		if end := strings.LastIndex(provider, `"]`); end > start {
			provider = provider[start+2 : end]
		}
	}

	// Keep only the final path segment: hashicorp/aws becomes aws.
	if idx := strings.LastIndex(provider, "/"); idx >= 0 {
		provider = provider[idx+1:]
	}
	return provider
}

// normaliseAttributes converts json.Number values back into plain Go numbers.
//
// The decoder is put in UseNumber mode so that large integer IDs survive the
// round trip without float64 precision loss, but the comparison layer expects
// ordinary types, so numbers are converted here where the choice is explicit.
func normaliseAttributes(attrs map[string]any) map[string]any {
	if attrs == nil {
		return nil
	}
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		out[k] = normaliseValue(v)
	}
	return out
}

// normaliseValue recursively converts json.Number values within a decoded tree.
func normaliseValue(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()

	case map[string]any:
		out := make(map[string]any, len(t))
		for k, inner := range t {
			out[k] = normaliseValue(inner)
		}
		return out

	case []any:
		out := make([]any, len(t))
		for i, inner := range t {
			out[i] = normaliseValue(inner)
		}
		return out

	default:
		return v
	}
}

// ResourceCount returns the number of managed resource instances in the state.
func (s *State) ResourceCount() int {
	count := 0
	for _, block := range s.Resources {
		if block.Mode == "managed" {
			count += len(block.Instances)
		}
	}
	return count
}

// Providers returns the distinct providers referenced by managed resources,
// in sorted order.
func (s *State) Providers() []string {
	seen := make(map[string]struct{})
	for _, block := range s.Resources {
		if block.Mode != "managed" {
			continue
		}
		if p := normaliseProvider(block.Provider); p != "" {
			seen[p] = struct{}{}
		}
	}

	providers := make([]string, 0, len(seen))
	for p := range seen {
		providers = append(providers, p)
	}
	sort.Strings(providers)
	return providers
}

// ResourceTypes returns a count of managed instances per resource type.
func (s *State) ResourceTypes() map[string]int {
	counts := make(map[string]int)
	for _, block := range s.Resources {
		if block.Mode != "managed" {
			continue
		}
		counts[block.Type] += len(block.Instances)
	}
	return counts
}

// FindByAddress returns the managed resource at the given Terraform address.
func (s *State) FindByAddress(address string) (*StateResource, bool) {
	for _, resource := range s.ManagedResources() {
		if resource.Address == address {
			found := resource
			return &found, true
		}
	}
	return nil, false
}
