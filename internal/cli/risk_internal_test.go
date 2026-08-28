package cli

import (
	"testing"

	"github.com/ashutosh0x/infra-control/internal/terraform"
)

func TestInferPublicDetectsExposure(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		want  bool
	}{
		{"public read ACL", map[string]any{"acl": "public-read"}, true},
		{"public write ACL", map[string]any{"acl": "public-read-write"}, true},
		{"private ACL", map[string]any{"acl": "private"}, false},
		{"publicly accessible flag", map[string]any{"publicly_accessible": true}, true},
		{"not publicly accessible", map[string]any{"publicly_accessible": false}, false},
		{"open to the world", map[string]any{"cidr_blocks": []any{"0.0.0.0/0"}}, true},
		{"restricted cidr", map[string]any{"cidr_blocks": []any{"10.0.0.0/8"}}, false},
		{"nothing relevant", map[string]any{"bucket": "assets"}, false},
	}

	for _, tc := range cases {
		if got := inferPublic(tc.attrs); got != tc.want {
			t.Errorf("%s: inferPublic = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestInferEncrypted(t *testing.T) {
	cases := []struct {
		name  string
		attrs map[string]any
		want  bool
	}{
		{"boolean true", map[string]any{"storage_encrypted": true}, true},
		{"boolean false", map[string]any{"storage_encrypted": false}, false},
		{"kms key reference", map[string]any{"kms_key_id": "arn:aws:kms:key/abc"}, true},
		{"empty kms key", map[string]any{"kms_key_id": ""}, false},
		{"encryption block", map[string]any{
			"server_side_encryption_configuration": map[string]any{"rule": "AES256"}}, true},
		{"no encryption attribute", map[string]any{"bucket": "assets"}, false},
	}

	for _, tc := range cases {
		if got := inferEncrypted(tc.attrs); got != tc.want {
			t.Errorf("%s: inferEncrypted = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestApplicabilityGatesInapplicableChecks(t *testing.T) {
	// A VPC has no encryption-at-rest setting and a subnet exists in exactly one
	// availability zone by definition. Scoring either against those checks
	// produces a finding nobody can act on, which is how a tool teaches its
	// users to ignore it.
	if supportsEncryption("aws_vpc") {
		t.Error("aws_vpc has no encryption-at-rest setting")
	}
	if supportsEncryption("aws_subnet") {
		t.Error("aws_subnet has no encryption-at-rest setting")
	}
	if !supportsEncryption("aws_s3_bucket") {
		t.Error("aws_s3_bucket does support encryption at rest")
	}
	if !supportsEncryption("google_storage_bucket") {
		t.Error("applicability must be provider-agnostic")
	}

	if supportsMultiAZ("aws_subnet") {
		t.Error("a subnet cannot span availability zones")
	}
	if !supportsMultiAZ("aws_db_instance") {
		t.Error("an RDS instance can be multi-AZ")
	}
}

func TestReliabilityConfigStripsInapplicableKeys(t *testing.T) {
	subnet := terraform.StateResource{
		Type: "aws_subnet",
		Attributes: map[string]any{
			"availability_zone": "us-east-1a",
			"cidr_block":        "10.0.1.0/24",
		},
	}

	config := reliabilityConfig(subnet)
	if _, present := config["availability_zone"]; present {
		t.Error("availability_zone must be withheld for a subnet, which is single-AZ by definition")
	}
	if _, present := config["cidr_block"]; !present {
		t.Error("unrelated attributes must be preserved")
	}

	// A database keeps the key, because single-AZ is a real finding there.
	db := terraform.StateResource{
		Type:       "aws_db_instance",
		Attributes: map[string]any{"availability_zone": "us-east-1a"},
	}
	if _, present := reliabilityConfig(db)["availability_zone"]; !present {
		t.Error("availability_zone must be kept for a database")
	}
}

func TestScoreChangesWeightsSecurityHighest(t *testing.T) {
	securityChange := scoreChanges([]terraform.FieldDiff{{Path: "acl"}})
	tagChange := scoreChanges([]terraform.FieldDiff{{Path: "tags.team"}})

	if securityChange <= tagChange {
		t.Errorf("a security change (%v) must outscore a tag change (%v)", securityChange, tagChange)
	}
	if severityForScore(securityChange) != "medium" && severityForScore(securityChange) != "high" {
		t.Errorf("a single ACL change scored %v -> %s", securityChange, severityForScore(securityChange))
	}
}

func TestSeverityForScoreBanding(t *testing.T) {
	cases := map[float64]string{
		100: "critical",
		80:  "critical",
		79:  "high",
		50:  "high",
		49:  "medium",
		25:  "medium",
		24:  "low",
		10:  "low",
		9:   "info",
		0:   "info",
	}
	for score, want := range cases {
		if got := string(severityForScore(score)); got != want {
			t.Errorf("severityForScore(%v) = %s, want %s", score, got, want)
		}
	}
}

func TestParseFailOnRejectsNonsense(t *testing.T) {
	if _, _, err := parseFailOn("sometimes"); err == nil {
		t.Error("parseFailOn should reject an unknown threshold")
	}

	severity, set, err := parseFailOn("none")
	if err != nil || set {
		t.Errorf("parseFailOn(none) = %v/%v/%v, want unset", severity, set, err)
	}

	severity, set, err = parseFailOn("high")
	if err != nil || !set || string(severity) != "high" {
		t.Errorf("parseFailOn(high) = %v/%v/%v", severity, set, err)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" aws_s3_bucket , aws_vpc ,, ")
	if len(got) != 2 || got[0] != "aws_s3_bucket" || got[1] != "aws_vpc" {
		t.Errorf("splitList = %v, want [aws_s3_bucket aws_vpc]", got)
	}
	if splitList("") != nil {
		t.Error("splitList of an empty string should be nil")
	}
}

func TestExtractTagsLowercasesKeys(t *testing.T) {
	tags := extractTags(map[string]any{
		"tags": map[string]any{"Environment": "prod", "Owner": "platform"},
	})
	if tags["environment"] != "prod" {
		t.Errorf("tag lookup must be case-insensitive, got %v", tags)
	}
}
