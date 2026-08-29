package terraform

import (
	"strings"
	"testing"
)

// refreshPlan is a `terraform show -json` refresh-only plan: prior_state holds
// the values Terraform read back from the providers.
const refreshPlan = `{
  "format_version": "1.2",
  "terraform_version": "1.9.5",
  "prior_state": {
    "values": {
      "root_module": {
        "resources": [
          {"address":"aws_vpc.main","mode":"managed","type":"aws_vpc","name":"main",
           "provider_name":"registry.terraform.io/hashicorp/aws",
           "values":{"id":"vpc-1","cidr_block":"10.0.0.0/16"}},
          {"address":"aws_subnet.private[0]","mode":"managed","type":"aws_subnet","name":"private","index":0,
           "provider_name":"registry.terraform.io/hashicorp/aws",
           "values":{"id":"subnet-1"}},
          {"address":"data.aws_ami.ubuntu","mode":"data","type":"aws_ami","name":"ubuntu",
           "provider_name":"registry.terraform.io/hashicorp/aws",
           "values":{"id":"ami-1"}}
        ],
        "child_modules": [
          {"address":"module.vpc",
           "resources":[
             {"address":"module.vpc.aws_route_table.rt","mode":"managed","type":"aws_route_table","name":"rt",
              "provider_name":"registry.terraform.io/hashicorp/aws","values":{"id":"rtb-1"}}],
           "child_modules": [
             {"address":"module.vpc.module.inner",
              "resources":[
                {"address":"module.vpc.module.inner.aws_eip.nat","mode":"managed","type":"aws_eip","name":"nat",
                 "provider_name":"registry.terraform.io/hashicorp/aws","values":{"id":"eip-1"}}]}
           ]}
        ]
      }
    }
  },
  "resource_drift": [{"address":"aws_vpc.main"}]
}`

func TestExtractPriorStateCollectsNestedModules(t *testing.T) {
	// Modules nest arbitrarily deep. Stopping at the first level would silently
	// omit most resources in any real configuration.
	snapshot, err := ExtractPriorState(strings.NewReader(refreshPlan))
	if err != nil {
		t.Fatalf("ExtractPriorState: %v", err)
	}

	for _, want := range []string{
		"aws_vpc.main",
		"aws_subnet.private[0]",
		"module.vpc.aws_route_table.rt",
		"module.vpc.module.inner.aws_eip.nat",
	} {
		if _, ok := snapshot.Resources[want]; !ok {
			t.Errorf("missing %s from the snapshot", want)
		}
	}
}

func TestExtractPriorStateExcludesDataSources(t *testing.T) {
	snapshot, err := ExtractPriorState(strings.NewReader(refreshPlan))
	if err != nil {
		t.Fatalf("ExtractPriorState: %v", err)
	}

	if _, present := snapshot.Resources["data.aws_ami.ubuntu"]; present {
		t.Error("a data source must not appear in a live snapshot; Terraform reads it but does not own it")
	}
	if len(snapshot.Resources) != 4 {
		t.Errorf("got %d resources, want 4", len(snapshot.Resources))
	}
}

func TestExtractPriorStateReportsTerraformsOwnDrift(t *testing.T) {
	snapshot, err := ExtractPriorState(strings.NewReader(refreshPlan))
	if err != nil {
		t.Fatalf("ExtractPriorState: %v", err)
	}
	if len(snapshot.DriftedAddresses) != 1 || snapshot.DriftedAddresses[0] != "aws_vpc.main" {
		t.Errorf("DriftedAddresses = %v, want [aws_vpc.main]", snapshot.DriftedAddresses)
	}
}

func TestExtractPriorStateNormalisesProviders(t *testing.T) {
	snapshot, err := ExtractPriorState(strings.NewReader(refreshPlan))
	if err != nil {
		t.Fatalf("ExtractPriorState: %v", err)
	}
	if len(snapshot.Providers) != 1 || snapshot.Providers[0] != "aws" {
		t.Errorf("Providers = %v, want [aws]", snapshot.Providers)
	}
}

func TestExtractPriorStateRejectsBinaryPlan(t *testing.T) {
	_, err := ExtractPriorState(strings.NewReader(`{"prior_state":{}}`))
	if err == nil {
		t.Fatal("expected an error when format_version is missing")
	}
	if !strings.Contains(err.Error(), "terraform show -json") {
		t.Errorf("error should point at the right command, got: %v", err)
	}
}

func TestExtractPriorStateRejectsEmptyPriorState(t *testing.T) {
	// A plan produced with -refresh=false carries no refreshed values. Silently
	// writing an empty snapshot would make the next scan report every managed
	// resource as deleted.
	_, err := ExtractPriorState(strings.NewReader(
		`{"format_version":"1.2","prior_state":{"values":{"root_module":{}}}}`))
	if err == nil {
		t.Fatal("expected an error for a plan with no prior state")
	}
	if !strings.Contains(err.Error(), "refresh") {
		t.Errorf("error should explain how to produce a usable plan, got: %v", err)
	}
}

func TestExtractPriorStatePreservesLargeIntegers(t *testing.T) {
	snapshot, err := ExtractPriorState(strings.NewReader(`{
      "format_version":"1.2",
      "prior_state":{"values":{"root_module":{"resources":[
        {"address":"a.b","mode":"managed","type":"a","name":"b","provider_name":"p",
         "values":{"numeric_id":9007199254740993}}]}}}}`))
	if err != nil {
		t.Fatalf("ExtractPriorState: %v", err)
	}

	got := snapshot.Resources["a.b"]["numeric_id"]
	if value, ok := got.(int64); !ok || value != 9007199254740993 {
		t.Errorf("numeric_id = %v (%T), want int64 9007199254740993", got, got)
	}
}
