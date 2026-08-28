// Package ignore implements drift suppression rules.
//
// Some infrastructure drifts by design. An autoscaling group's desired capacity
// moves on its own, a load balancer's IP is assigned by the provider, and a
// resource being decommissioned will disagree with state until it is removed.
// Reporting these on every scan is the failure mode that gets drift tooling
// switched off: the real findings get lost among the ones nobody can act on.
//
// Three properties keep suppression from becoming a way to hide problems:
//
//   - Every rule must state a reason. A rule with no reason is a config error,
//     not a silent default, so the file stays reviewable.
//   - A rule may carry an expiry. Past it the rule stops suppressing and the
//     scan says so, which stops a temporary exception becoming permanent.
//   - Suppressed findings are always counted and reported. The scan says how
//     many it hid and which rules hid them, so nothing disappears silently.
package ignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultFilename is the file looked for when no path is given.
const DefaultFilename = ".infractl-ignore.yaml"

// SupportedVersion is the only schema version this build reads.
const SupportedVersion = 1

// File is the parsed contents of an ignore file.
type File struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

// Rule suppresses drift on the resources and attributes it matches.
type Rule struct {
	// Address matches Terraform addresses. It supports a trailing * wildcard,
	// as in "aws_autoscaling_group.*", and matches exactly otherwise.
	Address string `yaml:"address"`

	// Attributes limits the rule to specific attribute paths. Each entry
	// supports the same trailing wildcard. When empty the rule suppresses every
	// finding on a matching resource, including its deletion.
	Attributes []string `yaml:"attributes,omitempty"`

	// Reason is why this drift is expected. Required.
	Reason string `yaml:"reason"`

	// Expires is an optional RFC 3339 date after which the rule stops applying.
	Expires string `yaml:"expires,omitempty"`

	// expiresAt is the parsed Expires value.
	expiresAt *time.Time
}

// Ruleset is a validated collection of rules ready to match against.
type Ruleset struct {
	rules []Rule
	// path records where the rules came from, for error messages.
	path string
	// expired holds rules that are past their expiry date. They no longer
	// suppress anything, but they are reported so the user knows to remove or
	// renew them.
	expired []Rule
}

// Load reads and validates an ignore file.
//
// An explicit path that does not exist is an error, because the user asked for
// a file that is not there. The default path being absent is not: most projects
// have no ignore file, and that is the correct state for them.
func Load(path string) (*Ruleset, error) {
	explicit := path != ""
	if !explicit {
		path = DefaultFilename
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if explicit {
			return nil, fmt.Errorf("ignore file not found: %s", path)
		}
		return &Ruleset{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ignore file %s: %w", path, err)
	}

	var file File
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse ignore file %s: %w", path, err)
	}

	if file.Version != SupportedVersion {
		return nil, fmt.Errorf(
			"ignore file %s has version %d; this build reads version %d",
			path, file.Version, SupportedVersion)
	}

	set := &Ruleset{path: path}
	now := time.Now().UTC()

	for i, rule := range file.Rules {
		if strings.TrimSpace(rule.Address) == "" {
			return nil, fmt.Errorf("%s: rule %d has no address", path, i+1)
		}
		// A rule without a reason is rejected rather than defaulted. The whole
		// point of the file is that a reader can tell why each entry is there.
		if strings.TrimSpace(rule.Reason) == "" {
			return nil, fmt.Errorf(
				"%s: rule %d (%s) has no reason.\n"+
					"  Every ignore rule must say why the drift is expected, so that the file\n"+
					"  stays reviewable and an exception can be re-evaluated later",
				path, i+1, rule.Address)
		}

		if rule.Expires != "" {
			expiry, err := parseExpiry(rule.Expires)
			if err != nil {
				return nil, fmt.Errorf("%s: rule %d (%s): %w", path, i+1, rule.Address, err)
			}
			rule.expiresAt = &expiry

			if now.After(expiry) {
				set.expired = append(set.expired, rule)
				continue
			}
		}

		set.rules = append(set.rules, rule)
	}

	return set, nil
}

// parseExpiry accepts a plain date or a full RFC 3339 timestamp.
func parseExpiry(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			// A plain date means end of that day, so a rule expiring today is
			// still active for the whole of today.
			if layout == "2006-01-02" {
				t = t.Add(24*time.Hour - time.Nanosecond)
			}
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf(
		"invalid expires %q (want YYYY-MM-DD or an RFC 3339 timestamp)", value)
}

// Match reports whether a finding is suppressed, and by which rule.
//
// attributes lists the attribute paths that changed. An empty slice means the
// finding is not an attribute change, such as a resource missing from live
// infrastructure or an unmanaged resource; only a rule with no Attributes
// restriction suppresses those.
//
// When a rule restricts attributes, it suppresses the finding only if it covers
// every changed attribute. A resource where one expected attribute and one
// unexpected attribute both moved is still reported, because the unexpected one
// is the finding.
func (s *Ruleset) Match(address string, attributes []string) (Rule, bool) {
	if s == nil {
		return Rule{}, false
	}

	for _, rule := range s.rules {
		if !matchPattern(rule.Address, address) {
			continue
		}

		if len(rule.Attributes) == 0 {
			return rule, true
		}
		if len(attributes) == 0 {
			// The rule is attribute-scoped but the finding is not an attribute
			// change, so the rule does not speak to it.
			continue
		}

		covered := true
		for _, attr := range attributes {
			if !matchesAny(rule.Attributes, attr) {
				covered = false
				break
			}
		}
		if covered {
			return rule, true
		}
	}

	return Rule{}, false
}

// matchesAny reports whether any pattern matches the value.
func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, value) {
			return true
		}
	}
	return false
}

// matchPattern matches a value against a pattern supporting a trailing
// wildcard.
//
// Only a trailing * is supported rather than full globbing. Terraform addresses
// contain dots and brackets that a glob library would treat as syntax, and a
// prefix match covers what an ignore file actually needs: a whole resource type,
// a module, or one attribute subtree.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}

// Len returns the number of active rules.
func (s *Ruleset) Len() int {
	if s == nil {
		return 0
	}
	return len(s.rules)
}

// Expired returns the rules that are past their expiry date and so no longer
// suppress anything.
func (s *Ruleset) Expired() []Rule {
	if s == nil {
		return nil
	}
	return s.expired
}

// Path returns where the rules were loaded from.
func (s *Ruleset) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Describe renders a rule for display.
func (r Rule) Describe() string {
	var b strings.Builder
	b.WriteString(r.Address)
	if len(r.Attributes) > 0 {
		fmt.Fprintf(&b, " [%s]", strings.Join(r.Attributes, ", "))
	}
	fmt.Fprintf(&b, ": %s", r.Reason)
	if r.Expires != "" {
		fmt.Fprintf(&b, " (expires %s)", r.Expires)
	}
	return b.String()
}

// FindDefault looks for an ignore file in dir and its parents, stopping at the
// filesystem root. Walking up means a scan run from a subdirectory of a repo
// still picks up the rules committed at the repo root.
func FindDefault(dir string) string {
	for {
		candidate := filepath.Join(dir, DefaultFilename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
