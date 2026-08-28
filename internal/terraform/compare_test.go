package terraform

import (
	"encoding/json"
	"testing"
)

func TestCompareAttributesIgnoresProviderBookkeeping(t *testing.T) {
	// id, arn, and tags_all are written by the provider and are not returned by
	// a live read in the same form. Comparing them would report drift on every
	// resource on every run.
	state := map[string]any{
		"id":       "bucket-1",
		"arn":      "arn:aws:s3:::bucket-1",
		"tags_all": map[string]any{"a": "b"},
		"bucket":   "prod-assets",
	}
	live := map[string]any{
		"bucket": "prod-assets",
	}

	if diffs := CompareAttributes(state, live); len(diffs) != 0 {
		t.Errorf("expected no drift from bookkeeping fields, got %d: %+v", len(diffs), diffs)
	}
}

func TestCompareAttributesDetectsRealChange(t *testing.T) {
	state := map[string]any{"acl": "private"}
	live := map[string]any{"acl": "public-read"}

	diffs := CompareAttributes(state, live)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1: %+v", len(diffs), diffs)
	}
	if diffs[0].Path != "acl" {
		t.Errorf("Path = %q, want acl", diffs[0].Path)
	}
	if diffs[0].Expected != "private" || diffs[0].Actual != "public-read" {
		t.Errorf("diff = %+v, want private -> public-read", diffs[0])
	}
}

func TestCompareAttributesNumericTypesAreEquivalent(t *testing.T) {
	// State is decoded with UseNumber and normalised to int64; a live cloud read
	// commonly yields float64 for the same field. Treating those as different
	// would report drift on every numeric attribute.
	state := map[string]any{"port": int64(443), "ratio": int64(1)}
	live := map[string]any{"port": float64(443), "ratio": json.Number("1")}

	if diffs := CompareAttributes(state, live); len(diffs) != 0 {
		t.Errorf("numeric representations should compare equal, got %+v", diffs)
	}
}

func TestCompareAttributesNestedMaps(t *testing.T) {
	state := map[string]any{
		"encryption": map[string]any{
			"enabled":   true,
			"algorithm": "AES256",
		},
	}
	live := map[string]any{
		"encryption": map[string]any{
			"enabled":   true,
			"algorithm": "aws:kms",
		},
	}

	diffs := CompareAttributes(state, live)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1: %+v", len(diffs), diffs)
	}
	if diffs[0].Path != "encryption.algorithm" {
		t.Errorf("Path = %q, want encryption.algorithm (nested paths must be dotted)", diffs[0].Path)
	}
}

func TestCompareAttributesIgnoresLiveOnlyFields(t *testing.T) {
	// Cloud APIs return many server-assigned fields Terraform never tracks.
	// Reporting them would bury the real changes.
	state := map[string]any{"bucket": "assets"}
	live := map[string]any{
		"bucket":              "assets",
		"server_side_thing":   "computed",
		"region_replica_role": "primary",
	}

	if diffs := CompareAttributes(state, live); len(diffs) != 0 {
		t.Errorf("live-only fields are not drift, got %+v", diffs)
	}
}

func TestCompareAttributesNullVersusAbsent(t *testing.T) {
	// An attribute state records as null and live omits entirely is the same
	// absence expressed two ways.
	state := map[string]any{"kms_key_id": nil, "bucket": "assets"}
	live := map[string]any{"bucket": "assets"}

	if diffs := CompareAttributes(state, live); len(diffs) != 0 {
		t.Errorf("null in state and absent live are equivalent, got %+v", diffs)
	}
}

func TestCompareAttributesDetectsRemovedValue(t *testing.T) {
	state := map[string]any{"kms_key_id": "arn:aws:kms:key/abc"}
	live := map[string]any{}

	diffs := CompareAttributes(state, live)
	if len(diffs) != 1 {
		t.Fatalf("got %d diffs, want 1", len(diffs))
	}
	if diffs[0].Actual != nil {
		t.Errorf("Actual = %v, want nil for a removed attribute", diffs[0].Actual)
	}
}

func TestIsSensitivePath(t *testing.T) {
	sensitive := []string{
		"password", "master_password", "db.password",
		"secret_key", "client_secret", "private_key",
		"connection_string", "config.access_key",
	}
	for _, path := range sensitive {
		if !IsSensitivePath(path) {
			t.Errorf("IsSensitivePath(%q) = false, want true", path)
		}
	}

	safe := []string{"bucket", "cidr_block", "instance_type", "tags.environment", "name"}
	for _, path := range safe {
		if IsSensitivePath(path) {
			t.Errorf("IsSensitivePath(%q) = true, want false", path)
		}
	}
}

func TestTypeFromAddress(t *testing.T) {
	cases := map[string]string{
		"aws_s3_bucket.assets":                "aws_s3_bucket",
		"aws_subnet.private[0]":               "aws_subnet",
		"module.vpc.aws_route_table.rt":       "aws_route_table",
		`module.a.module.b.aws_instance.x[2]`: "aws_instance",
	}
	for address, want := range cases {
		if got := typeFromAddress(address); got != want {
			t.Errorf("typeFromAddress(%q) = %q, want %q", address, got, want)
		}
	}
}

func TestCompareAttributesIsDeterministic(t *testing.T) {
	// Map iteration order is randomised in Go. Without the sort, two runs over
	// identical input would emit differently ordered diffs, breaking any CI step
	// that diffs or checksums the output.
	state := map[string]any{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	live := map[string]any{"a": 9, "b": 9, "c": 9, "d": 9, "e": 9}

	first := CompareAttributes(state, live)
	for i := 0; i < 20; i++ {
		next := CompareAttributes(state, live)
		if len(next) != len(first) {
			t.Fatalf("diff count varied between runs: %d then %d", len(first), len(next))
		}
		for j := range first {
			if first[j].Path != next[j].Path {
				t.Fatalf("diff order varied between runs at %d: %q then %q",
					j, first[j].Path, next[j].Path)
			}
		}
	}
}
