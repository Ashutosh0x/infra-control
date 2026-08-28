package terraform

import (
	"strings"
	"testing"
)

func planJSON(changes string) string {
	return `{"format_version":"1.2","terraform_version":"1.9.5","resource_changes":[` + changes + `]}`
}

func TestParsePlanRejectsBinaryPlanFile(t *testing.T) {
	// A binary plan file has no public format. The error must say so, because
	// passing the binary file rather than the JSON is the most common mistake.
	_, err := ParsePlan(strings.NewReader(`{"resource_changes":[]}`))
	if err == nil {
		t.Fatal("expected an error when format_version is missing")
	}
	if !strings.Contains(err.Error(), "terraform show -json") {
		t.Errorf("error should point at the right command, got: %v", err)
	}
}

func TestParsePlanClassifiesReplaceAsSingleChange(t *testing.T) {
	// Terraform encodes a replacement as delete plus create. Counting those as
	// two independent changes would understate the risk, since the delete
	// destroys a live resource.
	plan, err := ParsePlan(strings.NewReader(planJSON(
		`{"address":"aws_db_instance.main","mode":"managed","type":"aws_db_instance",
          "name":"main","provider_name":"registry.terraform.io/hashicorp/aws",
          "change":{"actions":["delete","create"]}}`)))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if plan.TotalChanges != 1 {
		t.Errorf("TotalChanges = %d, want 1", plan.TotalChanges)
	}
	if plan.Replaces != 1 {
		t.Errorf("Replaces = %d, want 1", plan.Replaces)
	}
	if plan.Creates != 0 || plan.Deletes != 0 {
		t.Errorf("a replace must not also count as a create or delete: creates=%d deletes=%d",
			plan.Creates, plan.Deletes)
	}
	if plan.Changes[0].Action != PlanActionReplace {
		t.Errorf("Action = %q, want replace", plan.Changes[0].Action)
	}
}

func TestParsePlanCreateBeforeDestroyIsStillReplace(t *testing.T) {
	// The action list order depends on the lifecycle setting; both orders mean
	// the same thing.
	plan, err := ParsePlan(strings.NewReader(planJSON(
		`{"address":"a.b","mode":"managed","type":"a","name":"b","provider_name":"p",
          "change":{"actions":["create","delete"]}}`)))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if plan.Replaces != 1 {
		t.Errorf("create-before-destroy should classify as replace, got %+v", plan.Changes)
	}
}

func TestParsePlanSkipsNoopsAndDataSources(t *testing.T) {
	plan, err := ParsePlan(strings.NewReader(planJSON(
		`{"address":"a.noop","mode":"managed","type":"a","name":"noop","provider_name":"p",
          "change":{"actions":["no-op"]}},
         {"address":"data.b.read","mode":"data","type":"b","name":"read","provider_name":"p",
          "change":{"actions":["read"]}},
         {"address":"c.real","mode":"managed","type":"c","name":"real","provider_name":"p",
          "change":{"actions":["update"]}}`)))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if plan.TotalChanges != 1 {
		t.Fatalf("TotalChanges = %d, want 1 (no-ops and data reads are not changes)", plan.TotalChanges)
	}
	if plan.Changes[0].Address != "c.real" {
		t.Errorf("kept the wrong change: %s", plan.Changes[0].Address)
	}
}

func TestDestructiveReturnsOnlyDeletesAndReplaces(t *testing.T) {
	plan, err := ParsePlan(strings.NewReader(planJSON(
		`{"address":"a.create","mode":"managed","type":"a","name":"create","provider_name":"p",
          "change":{"actions":["create"]}},
         {"address":"b.update","mode":"managed","type":"b","name":"update","provider_name":"p",
          "change":{"actions":["update"]}},
         {"address":"c.delete","mode":"managed","type":"c","name":"delete","provider_name":"p",
          "change":{"actions":["delete"]}},
         {"address":"d.replace","mode":"managed","type":"d","name":"replace","provider_name":"p",
          "change":{"actions":["delete","create"]}}`)))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	destructive := plan.Destructive()
	if len(destructive) != 2 {
		t.Fatalf("got %d destructive changes, want 2: %+v", len(destructive), destructive)
	}
	for _, change := range destructive {
		if !change.Action.IsDestructive() {
			t.Errorf("%s (%s) should not be in the destructive set", change.Address, change.Action)
		}
	}
}

func TestParsePlanRejectsUnsupportedFormatVersion(t *testing.T) {
	_, err := ParsePlan(strings.NewReader(
		`{"format_version":"9.9","terraform_version":"9.0","resource_changes":[]}`))
	if err == nil {
		t.Fatal("expected an error for an unsupported plan format version")
	}
	if !strings.Contains(err.Error(), "9.9") {
		t.Errorf("error should name the version it rejected, got: %v", err)
	}
}

func TestParsePlanOrdersChangesDeterministically(t *testing.T) {
	plan, err := ParsePlan(strings.NewReader(planJSON(
		`{"address":"z.last","mode":"managed","type":"z","name":"last","provider_name":"p",
          "change":{"actions":["create"]}},
         {"address":"a.first","mode":"managed","type":"a","name":"first","provider_name":"p",
          "change":{"actions":["create"]}}`)))
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}

	if plan.Changes[0].Address != "a.first" {
		t.Errorf("changes must be sorted by address; got %s first", plan.Changes[0].Address)
	}
}
