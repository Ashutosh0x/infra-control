package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// Prior state extraction.
//
// A `terraform plan -refresh-only` run contacts every provider and reads the
// real attributes of every managed resource. `terraform show -json` on the
// resulting plan file exposes those refreshed attributes under prior_state.
//
// That is a live read of the infrastructure, produced by Terraform itself,
// using credentials the user has already configured. Extracting it removes the
// hardest part of adopting this tool: without it, drift detection requires the
// user to build a live snapshot before they can run a single scan.
//
// The limitation is inherent and must be stated wherever this is offered:
// Terraform refreshes only what it manages, so a snapshot taken this way can
// never contain an unmanaged resource. Detecting those needs a real inventory
// read.

// planPriorState is the subset of `terraform show -json` needed to recover the
// refreshed attribute values.
type planPriorState struct {
	FormatVersion string `json:"format_version"`
	PriorState    struct {
		Values struct {
			RootModule planModule `json:"root_module"`
		} `json:"values"`
	} `json:"prior_state"`

	// ResourceDrift is Terraform's own account of what changed since the last
	// apply. It is read to report how many resources the refresh found moved,
	// which tells the user whether the snapshot is worth scanning.
	ResourceDrift []struct {
		Address string `json:"address"`
	} `json:"resource_drift"`
}

// planModule is a module in the prior-state tree. Modules nest recursively.
type planModule struct {
	Address      string           `json:"address"`
	Resources    []planStateValue `json:"resources"`
	ChildModules []planModule     `json:"child_modules"`
}

// planStateValue is one resource instance with its refreshed attributes.
type planStateValue struct {
	Address      string         `json:"address"`
	Mode         string         `json:"mode"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Index        any            `json:"index,omitempty"`
	ProviderName string         `json:"provider_name"`
	Values       map[string]any `json:"values"`
}

// PriorStateSnapshot is the result of extracting live values from a plan.
type PriorStateSnapshot struct {
	// Resources maps Terraform address to the refreshed attribute values.
	Resources map[string]map[string]any
	// Providers lists the distinct providers seen.
	Providers []string
	// DriftedAddresses are the resources Terraform's own refresh flagged as
	// changed. Reported so the user knows whether the refresh found anything
	// before they scan.
	DriftedAddresses []string
}

// ExtractPriorStateFile reads refreshed values from a JSON plan on disk.
func ExtractPriorStateFile(path string) (*PriorStateSnapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open plan file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	snapshot, err := ExtractPriorState(f)
	if err != nil {
		return nil, fmt.Errorf("read plan file %s: %w", path, err)
	}
	return snapshot, nil
}

// ExtractPriorState reads refreshed values from a JSON plan.
func ExtractPriorState(r io.Reader) (*PriorStateSnapshot, error) {
	var plan planPriorState

	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&plan); err != nil {
		return nil, fmt.Errorf("decode plan json: %w", err)
	}

	if plan.FormatVersion == "" {
		return nil, fmt.Errorf(
			"missing format_version: this does not look like `terraform show -json` output " +
				"(a binary plan file cannot be read directly)")
	}
	if !supportedPlanVersion(plan.FormatVersion) {
		return nil, fmt.Errorf(
			"unsupported plan format version %s (this build reads %v)",
			plan.FormatVersion, PlanFormatVersions)
	}

	snapshot := &PriorStateSnapshot{Resources: map[string]map[string]any{}}
	providers := map[string]struct{}{}

	collectModule(plan.PriorState.Values.RootModule, snapshot, providers)

	if len(snapshot.Resources) == 0 {
		return nil, fmt.Errorf(
			"the plan contains no prior state.\n" +
				"  This happens when the plan was generated with -refresh=false, or against\n" +
				"  an empty state. Re-run: terraform plan -refresh-only -out=tfplan")
	}

	for p := range providers {
		snapshot.Providers = append(snapshot.Providers, p)
	}
	sort.Strings(snapshot.Providers)

	for _, drift := range plan.ResourceDrift {
		snapshot.DriftedAddresses = append(snapshot.DriftedAddresses, drift.Address)
	}
	sort.Strings(snapshot.DriftedAddresses)

	return snapshot, nil
}

// collectModule walks a module and its children, gathering managed resources.
func collectModule(module planModule, snapshot *PriorStateSnapshot, providers map[string]struct{}) {
	for _, resource := range module.Resources {
		// Data sources are read by Terraform but not owned by it. Including
		// them would make every upstream change look like unauthorised drift.
		if resource.Mode != "managed" {
			continue
		}
		if resource.Values == nil {
			continue
		}

		if p := normaliseProvider(resource.ProviderName); p != "" {
			providers[p] = struct{}{}
		}

		// The address in prior_state already carries the module path and the
		// count or for_each index, so it matches the address the state parser
		// builds and the snapshot can be joined against state directly.
		snapshot.Resources[resource.Address] = normaliseAttributes(resource.Values)
	}

	for _, child := range module.ChildModules {
		collectModule(child, snapshot, providers)
	}
}
