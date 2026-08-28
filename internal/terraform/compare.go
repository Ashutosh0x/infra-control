package terraform

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Diff types reported by CompareStateToLive.
const (
	// DiffTypeModified means the resource exists in both state and live
	// infrastructure but their attributes disagree.
	DiffTypeModified = "modified"
	// DiffTypeMissingInLive means state records a resource that no longer
	// exists, which usually indicates an out-of-band deletion.
	DiffTypeMissingInLive = "missing_in_live"
	// DiffTypeMissingInState means a live resource is not tracked by Terraform.
	DiffTypeMissingInState = "missing_in_state"
)

// ignoredAttributes are attributes that differ between state and a live read
// without representing real drift.
//
// Terraform records bookkeeping fields in state that no cloud API returns, and
// providers write timestamps that change on every read. Comparing them would
// make every scan report drift on every resource, which trains users to ignore
// the tool.
var ignoredAttributes = map[string]struct{}{
	"id":                     {},
	"arn":                    {},
	"timeouts":               {},
	"tags_all":               {},
	"last_modified":          {},
	"last_updated":           {},
	"creation_date":          {},
	"created_at":             {},
	"updated_at":             {},
	"etag":                   {},
	"version_id":             {},
	"self_link":              {},
	"fingerprint":            {},
	"label_fingerprint":      {},
	"metadata_fingerprint":   {},
	"resource_guid":          {},
	"provisioner_connection": {},
	"terraform_labels":       {},
	"effective_labels":       {},
}

// sensitiveAttributePatterns identify attributes whose values must never be
// printed. Matching is done on the lowercased final path segment.
var sensitiveAttributePatterns = []string{
	"password", "secret", "token", "private_key", "certificate_body",
	"credentials", "access_key", "secret_key", "passphrase", "auth",
	"client_secret", "connection_string", "sas_token", "shared_key",
}

// CompareAttributes returns the field-level differences between the attributes
// recorded in Terraform state and those observed live.
//
// Only attributes present in state are compared. A live-only attribute is not
// drift: cloud APIs return many server-assigned fields that Terraform never
// tracks, and reporting them would bury the real changes.
func CompareAttributes(state, live map[string]any) []FieldDiff {
	var diffs []FieldDiff
	compareInto("", state, live, &diffs)

	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Path < diffs[j].Path })
	return diffs
}

// compareInto walks the state tree, recording differences against live.
func compareInto(prefix string, state, live map[string]any, diffs *[]FieldDiff) {
	for key, stateVal := range state {
		if _, ignored := ignoredAttributes[key]; ignored {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		liveVal, present := live[key]
		if !present {
			// An attribute that state records as null and live omits entirely is
			// the same absence expressed two ways, not a change.
			if stateVal == nil {
				continue
			}
			*diffs = append(*diffs, FieldDiff{Path: path, Expected: stateVal, Actual: nil})
			continue
		}

		stateMap, stateIsMap := stateVal.(map[string]any)
		liveMap, liveIsMap := liveVal.(map[string]any)
		if stateIsMap && liveIsMap {
			compareInto(path, stateMap, liveMap, diffs)
			continue
		}

		if equalValues(stateVal, liveVal) {
			continue
		}

		*diffs = append(*diffs, FieldDiff{Path: path, Expected: stateVal, Actual: liveVal})
	}
}

// equalValues reports whether two decoded JSON values are equivalent.
//
// Comparison goes through canonical JSON rather than reflect.DeepEqual because
// the two sides arrive from different decoders: state is read with UseNumber
// and normalised to int64, while a live cloud read may yield float64 for the
// same field. DeepEqual would call int64(3) and float64(3) different.
func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if an, aok := numericValue(a); aok {
		if bn, bok := numericValue(b); bok {
			return an == bn
		}
	}

	if as, aok := a.(string); aok {
		if bs, bok := b.(string); bok {
			return as == bs
		}
	}

	aJSON, aErr := json.Marshal(a)
	bJSON, bErr := json.Marshal(b)
	if aErr != nil || bErr != nil {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
	return string(aJSON) == string(bJSON)
}

// numericValue coerces any numeric representation to float64 for comparison.
func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// IsSensitivePath reports whether an attribute path names a secret whose value
// must be masked before display.
func IsSensitivePath(path string) bool {
	segment := path
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		segment = path[idx+1:]
	}
	segment = strings.ToLower(segment)

	for _, pattern := range sensitiveAttributePatterns {
		if strings.Contains(segment, pattern) {
			return true
		}
	}
	return false
}

// typeFromAddress extracts the resource type from a Terraform address,
// for example `module.vpc.aws_subnet.private[0]` yields `aws_subnet`.
func typeFromAddress(address string) string {
	// Strip any index suffix.
	if idx := strings.Index(address, "["); idx >= 0 {
		address = address[:idx]
	}

	parts := strings.Split(address, ".")
	// A bare address is type.name; a module address ends with the same pair.
	if len(parts) < 2 {
		return address
	}
	return parts[len(parts)-2]
}
