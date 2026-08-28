package ui

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSeverityToSarifLevel(t *testing.T) {
	// SARIF has four levels where this tool has five severities. Critical and
	// high both map to error; the original severity survives in properties.
	cases := map[string]string{
		"critical":   "error",
		"high":       "error",
		"medium":     "warning",
		"low":        "note",
		"info":       "note",
		"negligible": "note",
		"nonsense":   "none",
	}
	for severity, want := range cases {
		if got := SeverityToSarifLevel(severity); got != want {
			t.Errorf("SeverityToSarifLevel(%q) = %q, want %q", severity, got, want)
		}
	}
}

func TestBuildSarifDeclaresEachRuleOnce(t *testing.T) {
	log := BuildSarif("infractl", "1.0.0", "https://example.test", []SarifFinding{
		{RuleID: "drift-modified", RuleName: "Modified", Level: "error", Message: "a", File: "s.tfstate"},
		{RuleID: "drift-modified", RuleName: "Modified", Level: "error", Message: "b", File: "s.tfstate"},
		{RuleID: "drift-unmanaged", RuleName: "Unmanaged", Level: "warning", Message: "c", File: "s.tfstate"},
	})

	rules := log.Runs[0].Tool.Driver.Rules
	if len(rules) != 2 {
		t.Fatalf("got %d rules, want 2 (duplicates must collapse): %+v", len(rules), rules)
	}
	if len(log.Runs[0].Results) != 3 {
		t.Errorf("got %d results, want 3", len(log.Runs[0].Results))
	}
}

func TestBuildSarifEmptyScanIsStillValid(t *testing.T) {
	// A clean scan must produce a valid document rather than nothing: it is how
	// GitHub learns the previous findings are fixed. A null rules array is
	// rejected on upload, so it has to be an empty array.
	log := BuildSarif("infractl", "1.0.0", "https://example.test", nil)

	encoded, err := json.Marshal(log)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"rules":[]`)) {
		t.Errorf("empty scan must emit an empty rules array, got: %s", encoded)
	}
	if log.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", log.Version)
	}
}

func TestSarifResultsCarryFingerprints(t *testing.T) {
	// Fingerprints are what let GitHub mark a finding fixed rather than merely
	// absent from the latest run.
	log := BuildSarif("infractl", "1.0.0", "", []SarifFinding{
		{RuleID: "drift-modified", Level: "error", Message: "m", File: "s.tfstate",
			Fingerprint: "modified:aws_s3_bucket.assets"},
	})

	fp := log.Runs[0].Results[0].PartialFingerprints
	if fp["infractlFingerprint/v1"] != "modified:aws_s3_bucket.assets" {
		t.Errorf("fingerprint not carried through: %+v", fp)
	}
}

func TestSarifResultsCarryALocation(t *testing.T) {
	// GitHub needs an artifact and a region to anchor an alert.
	log := BuildSarif("infractl", "1.0.0", "", []SarifFinding{
		{RuleID: "r", Level: "note", Message: "m", File: "terraform.tfstate"},
	})

	loc := log.Runs[0].Results[0].Locations
	if len(loc) != 1 {
		t.Fatalf("got %d locations, want 1", len(loc))
	}
	if loc[0].PhysicalLocation.ArtifactLocation.URI != "terraform.tfstate" {
		t.Errorf("artifact URI = %q", loc[0].PhysicalLocation.ArtifactLocation.URI)
	}
	if loc[0].PhysicalLocation.Region == nil || loc[0].PhysicalLocation.Region.StartLine < 1 {
		t.Error("a region with a positive start line is required")
	}
}

func TestSarifFormatIsMachineReadable(t *testing.T) {
	if !FormatSARIF.IsMachine() {
		t.Error("sarif must be treated as a machine format so progress output is suppressed")
	}
	format, _, err := ParseFormat("sarif")
	if err != nil || format != FormatSARIF {
		t.Errorf("ParseFormat(sarif) = %q/%v", format, err)
	}
}

func TestWriteSarifRejectsCommandsWithoutIt(t *testing.T) {
	// Asking a command with no SARIF form for SARIF is an error, not an empty
	// document that would tell GitHub every previous finding was fixed.
	r, _, _ := newTestRenderer(false)
	if err := r.Write(View{Data: map[string]string{"a": "b"}}, FormatSARIF, ""); err == nil {
		t.Error("expected an error when the view carries no SARIF findings")
	}
}
