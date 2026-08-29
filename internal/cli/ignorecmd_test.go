package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ashutosh0x/infra-control/internal/ignore"
)

// inTempDir runs the test in a scratch directory and restores the previous one.
func inTempDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	return dir
}

func TestAppendRuleCreatesAParseableFile(t *testing.T) {
	// The written file must be readable by the loader that scans consume, or a
	// rule added today breaks every scan tomorrow.
	dir := inTempDir(t)
	path := filepath.Join(dir, ignore.DefaultFilename)

	rule := ignore.Rule{Address: "aws_s3_bucket.assets", Reason: "migration in progress"}
	if err := appendRule(path, rule); err != nil {
		t.Fatalf("appendRule: %v", err)
	}

	rules, err := ignore.Load(path)
	if err != nil {
		t.Fatalf("the file this command wrote does not load: %v", err)
	}
	if rules.Len() != 1 {
		t.Fatalf("got %d rules, want 1", rules.Len())
	}
	if _, matched := rules.Match("aws_s3_bucket.assets", []string{"acl"}); !matched {
		t.Error("the written rule does not suppress what it names")
	}
}

func TestAppendRulePreservesExistingRules(t *testing.T) {
	dir := inTempDir(t)
	path := filepath.Join(dir, ignore.DefaultFilename)

	first := ignore.Rule{Address: "a.b", Reason: "first"}
	second := ignore.Rule{Address: "c.d", Reason: "second"}

	if err := appendRule(path, first); err != nil {
		t.Fatalf("appendRule first: %v", err)
	}
	if err := appendRule(path, second); err != nil {
		t.Fatalf("appendRule second: %v", err)
	}

	rules, err := ignore.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if rules.Len() != 2 {
		t.Errorf("got %d rules, want 2; the first was lost", rules.Len())
	}
}

func TestAppendRuleRefusesDuplicates(t *testing.T) {
	// Silently accepting a duplicate leaves two rules with different reasons
	// suppressing the same thing, and no way to tell which one is current.
	dir := inTempDir(t)
	path := filepath.Join(dir, ignore.DefaultFilename)

	rule := ignore.Rule{Address: "a.b", Reason: "first"}
	if err := appendRule(path, rule); err != nil {
		t.Fatalf("appendRule: %v", err)
	}

	err := appendRule(path, ignore.Rule{Address: "a.b", Reason: "second"})
	if err == nil {
		t.Fatal("expected a duplicate rule to be refused")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should say the rule exists, got: %v", err)
	}
}

func TestAppendRuleDistinguishesAttributeScope(t *testing.T) {
	// Same address, different attribute scope, is a different rule: one covers
	// the whole resource and one covers two attributes.
	dir := inTempDir(t)
	path := filepath.Join(dir, ignore.DefaultFilename)

	if err := appendRule(path, ignore.Rule{Address: "a.b", Reason: "whole resource"}); err != nil {
		t.Fatalf("appendRule: %v", err)
	}
	if err := appendRule(path, ignore.Rule{
		Address: "a.b", Attributes: []string{"tags.team"}, Reason: "just the tag",
	}); err != nil {
		t.Errorf("an attribute-scoped rule should not collide with a whole-resource one: %v", err)
	}
}

func TestAppendRuleRefusesToTouchAMalformedFile(t *testing.T) {
	// Appending to a file that does not parse would either destroy its contents
	// or produce something equally broken.
	dir := inTempDir(t)
	path := filepath.Join(dir, ignore.DefaultFilename)

	if err := os.WriteFile(path, []byte("rules: [unclosed\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := appendRule(path, ignore.Rule{Address: "a.b", Reason: "r"}); err == nil {
		t.Error("expected an error rather than overwriting an unparseable file")
	}
}

func TestSameAttributes(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "a"}, []string{"a", "b"}, false},
		{nil, []string{"a"}, false},
	}
	for _, tc := range cases {
		if got := sameAttributes(tc.a, tc.b); got != tc.want {
			t.Errorf("sameAttributes(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestRenderRuleRoundTrips(t *testing.T) {
	rule := ignore.Rule{
		Address:    "aws_autoscaling_group.web",
		Attributes: []string{"desired_capacity"},
		Reason:     "managed by the autoscaling policy",
		Expires:    "2026-12-31",
	}

	rendered, err := renderRule(rule)
	if err != nil {
		t.Fatalf("renderRule: %v", err)
	}
	for _, want := range []string{rule.Address, rule.Attributes[0], rule.Reason, rule.Expires} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered rule omitted %q:\n%s", want, rendered)
		}
	}
}
