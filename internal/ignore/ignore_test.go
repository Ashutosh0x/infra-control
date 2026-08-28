package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeIgnore writes an ignore file into a temp dir and returns its path.
func writeIgnore(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), DefaultFilename)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write ignore file: %v", err)
	}
	return path
}

func TestLoadRequiresAReason(t *testing.T) {
	// A rule with no reason is rejected rather than defaulted. The whole point
	// of the file is that a reviewer can tell why each entry is there; a
	// reasonless rule is how suppression quietly becomes permanent.
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for a rule with no reason")
	}
	if !strings.Contains(err.Error(), "no reason") {
		t.Errorf("error should name the missing reason, got: %v", err)
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	path := writeIgnore(t, "version: 99\nrules: []\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected an error for an unsupported version")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error should name the version it rejected, got: %v", err)
	}
}

func TestMissingDefaultFileIsNotAnError(t *testing.T) {
	// Most projects have no ignore file, and that is the correct state for them.
	set, err := Load("")
	if err != nil {
		t.Fatalf("absent default file should not error: %v", err)
	}
	if set.Len() != 0 {
		t.Errorf("expected an empty ruleset, got %d rules", set.Len())
	}
}

func TestMissingExplicitFileIsAnError(t *testing.T) {
	// The user named a file. If it is not there, that is a mistake worth saying.
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Error("an explicitly named missing file should error")
	}
}

func TestMatchWholeResource(t *testing.T) {
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
    reason: decommissioning
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// With no attribute restriction the rule covers every finding on the
	// resource, including one with no attributes such as a deletion.
	if _, ok := set.Match("aws_s3_bucket.assets", []string{"acl"}); !ok {
		t.Error("expected an attribute change to be suppressed")
	}
	if _, ok := set.Match("aws_s3_bucket.assets", nil); !ok {
		t.Error("expected a non-attribute finding to be suppressed")
	}
	if _, ok := set.Match("aws_s3_bucket.other", []string{"acl"}); ok {
		t.Error("a different resource must not be suppressed")
	}
}

func TestMatchAttributeScoped(t *testing.T) {
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_autoscaling_group.web
    attributes: [desired_capacity, min_size]
    reason: managed by the autoscaling policy
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := set.Match("aws_autoscaling_group.web", []string{"desired_capacity"}); !ok {
		t.Error("a listed attribute should be suppressed")
	}
	if _, ok := set.Match("aws_autoscaling_group.web", []string{"desired_capacity", "min_size"}); !ok {
		t.Error("all-listed attributes should be suppressed")
	}

	// The important case: an expected attribute moving alongside an unexpected
	// one is still a finding, because the unexpected one is the whole point.
	if _, ok := set.Match("aws_autoscaling_group.web", []string{"desired_capacity", "iam_role"}); ok {
		t.Error("a rule must not suppress a finding it only partly covers")
	}

	// An attribute-scoped rule says nothing about a deletion.
	if _, ok := set.Match("aws_autoscaling_group.web", nil); ok {
		t.Error("an attribute-scoped rule must not suppress a non-attribute finding")
	}
}

func TestMatchTrailingWildcard(t *testing.T) {
	path := writeIgnore(t, `
version: 1
rules:
  - address: "module.legacy.*"
    reason: legacy module pending removal
  - address: "aws_instance.worker"
    attributes: ["tags.*"]
    reason: tags are managed by the fleet controller
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if _, ok := set.Match("module.legacy.aws_vpc.main", []string{"cidr_block"}); !ok {
		t.Error("address wildcard should match a nested address")
	}
	if _, ok := set.Match("module.current.aws_vpc.main", []string{"cidr_block"}); ok {
		t.Error("address wildcard must not match a different module")
	}
	if _, ok := set.Match("aws_instance.worker", []string{"tags.team", "tags.env"}); !ok {
		t.Error("attribute wildcard should match a subtree")
	}
	if _, ok := set.Match("aws_instance.worker", []string{"instance_type"}); ok {
		t.Error("attribute wildcard must not match outside its subtree")
	}
}

func TestExpiredRuleStopsSuppressing(t *testing.T) {
	// An exception that outlives its reason is how suppression rots. Past the
	// expiry the rule stops applying and is surfaced for renewal or removal.
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
    reason: temporary exception
    expires: "2020-01-01"
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if set.Len() != 0 {
		t.Errorf("an expired rule must not be active, got %d active rules", set.Len())
	}
	if _, ok := set.Match("aws_s3_bucket.assets", []string{"acl"}); ok {
		t.Error("an expired rule must not suppress")
	}
	if len(set.Expired()) != 1 {
		t.Fatalf("expected the expired rule to be reported, got %d", len(set.Expired()))
	}
}

func TestUnexpiredRuleStillApplies(t *testing.T) {
	future := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
    reason: migration in progress
    expires: "`+future+`"
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if set.Len() != 1 {
		t.Fatalf("expected 1 active rule, got %d", set.Len())
	}
	if _, ok := set.Match("aws_s3_bucket.assets", []string{"acl"}); !ok {
		t.Error("an unexpired rule should suppress")
	}
}

func TestExpiryIsInclusiveOfItsLastDay(t *testing.T) {
	// A rule expiring today is still active for the whole of today; expiring at
	// midnight would surprise anyone who wrote today's date meaning "through
	// today".
	today := time.Now().UTC().Format("2006-01-02")
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
    reason: expires at end of today
    expires: "`+today+`"
`)
	set, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if set.Len() != 1 {
		t.Errorf("a rule expiring today should still be active, got %d active", set.Len())
	}
}

func TestInvalidExpiryIsRejected(t *testing.T) {
	path := writeIgnore(t, `
version: 1
rules:
  - address: aws_s3_bucket.assets
    reason: bad date
    expires: "next tuesday"
`)
	if _, err := Load(path); err == nil {
		t.Error("expected an error for an unparseable expiry")
	}
}

func TestFindDefaultWalksUpwards(t *testing.T) {
	// A scan run from a subdirectory should still pick up rules committed at
	// the repository root.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, DefaultFilename), []byte("version: 1\nrules: []\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	nested := filepath.Join(root, "envs", "prod")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	found := FindDefault(nested)
	if found == "" {
		t.Fatal("expected to find the ignore file in a parent directory")
	}
	if filepath.Base(found) != DefaultFilename {
		t.Errorf("found the wrong file: %s", found)
	}
}

func TestNilRulesetIsSafe(t *testing.T) {
	// A nil ruleset stands in for "no suppression configured", so every method
	// has to tolerate it rather than the caller branching.
	var set *Ruleset
	if _, ok := set.Match("anything", nil); ok {
		t.Error("a nil ruleset must not suppress")
	}
	if set.Len() != 0 || set.Expired() != nil || set.Path() != "" {
		t.Error("a nil ruleset should report as empty")
	}
}
