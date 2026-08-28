package terraform

import (
	"strings"
	"testing"
)

// minimalState is a valid format-4 state with one resource, used as the base
// for tests that vary one thing at a time.
const minimalState = `{
  "version": 4,
  "terraform_version": "1.9.5",
  "serial": 1,
  "lineage": "abc",
  "resources": [
    {
      "mode": "managed",
      "type": "aws_s3_bucket",
      "name": "assets",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{"schema_version": 0, "attributes": {"bucket": "prod-assets"}}]
    }
  ]
}`

func TestParseStateRejectsUnknownFormatVersion(t *testing.T) {
	// A future state format must be rejected rather than parsed on a best-effort
	// basis: a partially understood state yields a resource list that is wrong in
	// ways drift detection cannot see.
	input := strings.Replace(minimalState, `"version": 4`, `"version": 5`, 1)

	_, err := ParseState(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected an error for state format version 5, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported state format version 5") {
		t.Errorf("error should name the version it rejected, got: %v", err)
	}
}

func TestParseStateReadsMetadata(t *testing.T) {
	state, err := ParseState(strings.NewReader(minimalState))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}

	if state.TerraformVersion != "1.9.5" {
		t.Errorf("TerraformVersion = %q, want 1.9.5", state.TerraformVersion)
	}
	if state.Serial != 1 {
		t.Errorf("Serial = %d, want 1", state.Serial)
	}
}

func TestManagedResourcesExcludesDataSources(t *testing.T) {
	// Data sources are read, not owned. Reporting drift on them would flag every
	// upstream change as an unauthorised edit.
	input := `{
      "version": 4,
      "resources": [
        {"mode": "managed", "type": "aws_vpc", "name": "main", "provider": "p",
         "instances": [{"attributes": {}}]},
        {"mode": "data", "type": "aws_caller_identity", "name": "current", "provider": "p",
         "instances": [{"attributes": {}}]}
      ]
    }`

	state, err := ParseState(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}

	managed := state.ManagedResources()
	if len(managed) != 1 {
		t.Fatalf("got %d managed resources, want 1 (the data source must be excluded)", len(managed))
	}
	if managed[0].Type != "aws_vpc" {
		t.Errorf("kept the wrong resource: %s", managed[0].Type)
	}
}

func TestBuildAddressHandlesCountAndForEach(t *testing.T) {
	// The address must round-trip to what Terraform itself prints, because it is
	// the key the live snapshot is matched against.
	input := `{
      "version": 4,
      "resources": [
        {"mode": "managed", "type": "aws_subnet", "name": "private", "provider": "p",
         "instances": [
           {"index_key": 0, "attributes": {}},
           {"index_key": 1, "attributes": {}}
         ]},
        {"mode": "managed", "type": "aws_instance", "name": "web", "provider": "p",
         "instances": [{"index_key": "primary", "attributes": {}}]},
        {"mode": "managed", "module": "module.vpc", "type": "aws_route_table", "name": "rt",
         "provider": "p", "instances": [{"attributes": {}}]}
      ]
    }`

	state, err := ParseState(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}

	got := map[string]bool{}
	for _, r := range state.ManagedResources() {
		got[r.Address] = true
	}

	for _, want := range []string{
		"aws_subnet.private[0]",
		"aws_subnet.private[1]",
		`aws_instance.web["primary"]`,
		"module.vpc.aws_route_table.rt",
	} {
		if !got[want] {
			t.Errorf("missing expected address %q; got %v", want, keysOf(got))
		}
	}
}

func TestNormaliseProvider(t *testing.T) {
	cases := map[string]string{
		`provider["registry.terraform.io/hashicorp/aws"]`: "aws",
		`provider["registry.opentofu.org/hashicorp/gcp"]`: "gcp",
		"registry.terraform.io/hashicorp/azurerm":         "azurerm",
		"aws": "aws",
		"":    "",
	}

	for input, want := range cases {
		if got := normaliseProvider(input); got != want {
			t.Errorf("normaliseProvider(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLargeIntegersSurviveParsing(t *testing.T) {
	// Decoding through float64 would silently corrupt IDs past 2^53. Cloud
	// account and snapshot IDs routinely exceed that, and a corrupted ID would
	// show up as permanent phantom drift.
	const bigID = 9007199254740993 // 2^53 + 1

	input := `{
      "version": 4,
      "resources": [{"mode": "managed", "type": "t", "name": "n", "provider": "p",
        "instances": [{"attributes": {"numeric_id": 9007199254740993}}]}]
    }`

	state, err := ParseState(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}

	got := state.ManagedResources()[0].Attributes["numeric_id"]
	value, ok := got.(int64)
	if !ok {
		t.Fatalf("numeric_id decoded as %T, want int64", got)
	}
	if value != bigID {
		t.Errorf("numeric_id = %d, want %d (precision was lost)", value, bigID)
	}
}

func TestResourceTypesCountsInstances(t *testing.T) {
	state, err := ParseState(strings.NewReader(`{
      "version": 4,
      "resources": [
        {"mode": "managed", "type": "aws_subnet", "name": "a", "provider": "p",
         "instances": [{"index_key": 0, "attributes": {}}, {"index_key": 1, "attributes": {}}]},
        {"mode": "managed", "type": "aws_vpc", "name": "main", "provider": "p",
         "instances": [{"attributes": {}}]}
      ]
    }`))
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}

	counts := state.ResourceTypes()
	if counts["aws_subnet"] != 2 {
		t.Errorf("aws_subnet count = %d, want 2 (both instances)", counts["aws_subnet"])
	}
	if state.ResourceCount() != 3 {
		t.Errorf("ResourceCount = %d, want 3", state.ResourceCount())
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
