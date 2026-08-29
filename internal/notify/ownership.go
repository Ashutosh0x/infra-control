package notify

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Ownership resolution.
//
// An alert with no owner has no responder, so this runs before routing and its
// failure mode is counted rather than hidden: an unowned finding still goes
// somewhere, and a high unowned rate is itself a finding worth surfacing.

// Owner is who should act on a finding.
type Owner struct {
	// Name is a routable handle: a Slack user group, a GitHub team, an email.
	Name string `json:"name"`
	// Source records how the owner was determined, so a wrong route can be
	// traced to the rule that produced it rather than guessed at.
	Source string `json:"source"`
}

// Unowned is the zero owner, used when nothing matched.
var Unowned = Owner{Name: "", Source: "none"}

// Known returns whether an owner was resolved.
func (o Owner) Known() bool { return o.Name != "" }

// OwnershipConfig declares how owners are found.
type OwnershipConfig struct {
	// TagKeys are resource tag names checked first, in order.
	TagKeys []string `yaml:"tag_keys" json:"tag_keys"`
	// CodeownersPath is a CODEOWNERS file matched against module source paths.
	CodeownersPath string `yaml:"codeowners" json:"codeowners"`
	// Fallback maps an address prefix to an owner. A trailing * is a prefix
	// match, consistent with the ignore-rule syntax.
	Fallback map[string]string `yaml:"fallback" json:"fallback"`
	// Default receives everything unmatched. Findings routed here are counted
	// separately so the gap is visible.
	Default string `yaml:"default" json:"default"`
}

// SourceResolver maps a Terraform address to the configuration that declared it.
//
// Mapping an address to a file and line is the enrichment that turns
// "aws_s3_bucket.assets" into "modules/storage/main.tf:47, owned by
// @data-platform". It is also genuinely fiddly: state records a module
// address, not a source path, and recovering the path needs the configuration
// block from a plan plus module-source resolution.
//
// The interface exists so that work can land later without disturbing
// routing. Until then the module half is implemented and line numbers are
// reported as unknown, because a wrong line number sends someone to the wrong
// code, which is worse than sending them to the right module with none.
type SourceResolver interface {
	// Resolve returns the declaring file and line. found is false when the
	// position cannot be determined; callers must degrade rather than display
	// a guess.
	Resolve(address string) (file string, line int, found bool)
}

// ModuleResolver derives the module path from a Terraform address.
//
// This is the half that can be done reliably from an address alone: the module
// prefix is structural, so `module.vpc.aws_subnet.private[0]` is declared
// somewhere under the vpc module regardless of where that module's source
// lives.
type ModuleResolver struct{}

// Resolve returns the module path for an address. The line is always unknown.
func (ModuleResolver) Resolve(address string) (string, int, bool) {
	module := ModulePath(address)
	if module == "" {
		return "", 0, false
	}
	return module, 0, true
}

// ModulePath extracts the module prefix from a Terraform address, or an empty
// string for a root-module resource.
func ModulePath(address string) string {
	if !strings.HasPrefix(address, "module.") {
		return ""
	}

	// A module address is module.NAME[.module.NAME...].TYPE.NAME, so the module
	// path is everything before the final two dot-separated segments.
	parts := strings.Split(address, ".")
	if len(parts) < 4 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], ".")
}

// Resolver assigns owners to findings.
type Resolver struct {
	config     OwnershipConfig
	codeowners []codeownersRule
	tags       map[string]map[string]string
}

// codeownersRule is one CODEOWNERS line.
type codeownersRule struct {
	pattern string
	owners  []string
}

// NewResolver builds an ownership resolver, loading CODEOWNERS if configured.
func NewResolver(config OwnershipConfig, tags map[string]map[string]string) (*Resolver, error) {
	resolver := &Resolver{config: config, tags: tags}

	if config.CodeownersPath != "" {
		rules, err := parseCodeowners(config.CodeownersPath)
		if err != nil {
			return nil, err
		}
		resolver.codeowners = rules
	}
	return resolver, nil
}

// Resolve finds the owner of a finding, first match wins.
func (r *Resolver) Resolve(finding Finding) Owner {
	// 1. A tag on the resource is the most specific signal and the one the
	//    owning team controls directly.
	if attrs, ok := r.tags[finding.Address]; ok {
		for _, key := range r.config.TagKeys {
			if value, present := attrs[strings.ToLower(key)]; present && value != "" {
				return Owner{Name: value, Source: "tag:" + key}
			}
		}
	}

	// 2. CODEOWNERS, matched against the module path.
	if module := ModulePath(finding.Address); module != "" && len(r.codeowners) > 0 {
		if owners := matchCodeowners(r.codeowners, module); len(owners) > 0 {
			return Owner{Name: owners[0], Source: "codeowners"}
		}
	}

	// 3. An explicit fallback map, longest prefix first so a specific rule
	//    beats a general one regardless of map ordering.
	if owner, source, ok := matchFallback(r.config.Fallback, finding.Address); ok {
		return Owner{Name: owner, Source: "fallback:" + source}
	}

	// 4. The default, which is a destination rather than an owner.
	if r.config.Default != "" {
		return Owner{Name: r.config.Default, Source: "default"}
	}
	return Unowned
}

// matchFallback finds the most specific fallback rule matching an address.
func matchFallback(fallback map[string]string, address string) (owner, pattern string, ok bool) {
	best := -1
	for candidate, candidateOwner := range fallback {
		prefix := strings.TrimSuffix(candidate, "*")

		matched := candidate == address
		if strings.HasSuffix(candidate, "*") {
			matched = strings.HasPrefix(address, prefix)
		}
		if !matched {
			continue
		}
		// Longest pattern wins, so module.payments.* beats module.*
		if len(prefix) > best {
			best, owner, pattern, ok = len(prefix), candidateOwner, candidate, true
		}
	}
	return owner, pattern, ok
}

// parseCodeowners reads a CODEOWNERS file.
func parseCodeowners(path string) ([]codeownersRule, error) {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		// A configured-but-absent CODEOWNERS is worth saying, because the user
		// asked for it and would otherwise wonder why nothing is owned.
		return nil, fmt.Errorf("codeowners file not found: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("open codeowners %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	var rules []codeownersRule
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		rules = append(rules, codeownersRule{pattern: fields[0], owners: fields[1:]})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read codeowners %s: %w", path, err)
	}
	return rules, nil
}

// matchCodeowners applies CODEOWNERS precedence: the last matching rule wins,
// which is how git itself resolves them.
func matchCodeowners(rules []codeownersRule, path string) []string {
	var owners []string
	for _, rule := range rules {
		if codeownersMatch(rule.pattern, path) {
			owners = rule.owners
		}
	}
	return owners
}

// codeownersMatch reports whether a CODEOWNERS pattern covers a path.
//
// This implements the subset of the syntax that matters here: a bare `*`
// matches everything, a trailing slash or `/**` matches a directory subtree,
// and anything else is a prefix or substring match. Full gitignore-style
// globbing is deliberately not attempted, since over-matching would assign an
// owner that is wrong rather than absent.
func codeownersMatch(pattern, path string) bool {
	if pattern == "*" {
		return true
	}

	pattern = strings.TrimPrefix(pattern, "/")
	pattern = strings.TrimSuffix(pattern, "/**")
	pattern = strings.TrimSuffix(pattern, "/")

	if pattern == "" {
		return true
	}
	return strings.Contains(path, pattern)
}
