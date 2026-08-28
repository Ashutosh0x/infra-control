package terraform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// PlanFormatVersions lists the `terraform show -json` plan format versions this
// parser accepts. Terraform has emitted 1.x for the whole 0.12+ line, adding
// fields without breaking existing readers, so the major version is what matters.
var PlanFormatVersions = []string{"1.0", "1.1", "1.2"}

// planFile is the raw shape of `terraform show -json <planfile>` output.
type planFile struct {
	FormatVersion    string `json:"format_version"`
	TerraformVersion string `json:"terraform_version"`

	ResourceChanges []planResourceChange  `json:"resource_changes"`
	OutputChanges   map[string]planChange `json:"output_changes"`
}

// planResourceChange is one resource entry in the plan.
type planResourceChange struct {
	Address      string     `json:"address"`
	ModuleAddr   string     `json:"module_address"`
	Mode         string     `json:"mode"`
	Type         string     `json:"type"`
	Name         string     `json:"name"`
	ProviderName string     `json:"provider_name"`
	Change       planChange `json:"change"`
}

// planChange carries the before/after values and the action list.
type planChange struct {
	// Actions is Terraform's action list. A replace is encoded as a two-element
	// list, either ["delete","create"] or ["create","delete"] depending on
	// whether the resource is created before destroy.
	Actions      []string       `json:"actions"`
	Before       map[string]any `json:"before"`
	After        map[string]any `json:"after"`
	AfterUnknown map[string]any `json:"after_unknown"`

	// BeforeSensitive and AfterSensitive mirror the value shape with booleans
	// marking which leaves the provider declared sensitive.
	BeforeSensitive any `json:"before_sensitive"`
	AfterSensitive  any `json:"after_sensitive"`
}

// ParsePlanFile reads and parses a JSON plan from disk.
//
// The input must be the output of `terraform show -json <planfile>`, not the
// opaque binary plan file itself, which has no public format.
func ParsePlanFile(path string) (*PlanSummary, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open plan file %s: %w", path, err)
	}
	defer f.Close()

	plan, err := ParsePlan(f)
	if err != nil {
		return nil, fmt.Errorf("parse plan file %s: %w", path, err)
	}
	return plan, nil
}

// ParsePlan parses a JSON plan from a reader.
func ParsePlan(r io.Reader) (*PlanSummary, error) {
	var raw planFile

	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode plan json: %w", err)
	}

	if raw.FormatVersion == "" {
		return nil, fmt.Errorf(
			"missing format_version: this does not look like `terraform show -json` output " +
				"(a binary plan file cannot be read directly)",
		)
	}
	if !supportedPlanVersion(raw.FormatVersion) {
		return nil, fmt.Errorf(
			"unsupported plan format version %s (this build reads %v)",
			raw.FormatVersion, PlanFormatVersions,
		)
	}

	summary := &PlanSummary{Outputs: map[string]any{}}

	for _, rc := range raw.ResourceChanges {
		// Data source reads are not infrastructure changes and would inflate the
		// change count that approval policies gate on.
		if rc.Mode != "managed" {
			continue
		}

		action := classifyActions(rc.Change.Actions)
		if action == PlanActionNoop {
			continue
		}

		summary.Changes = append(summary.Changes, PlanChange{
			Address:      rc.Address,
			Type:         rc.Type,
			Name:         rc.Name,
			Provider:     normaliseProvider(rc.ProviderName),
			Action:       action,
			Before:       normaliseAttributes(rc.Change.Before),
			After:        normaliseAttributes(rc.Change.After),
			AfterUnknown: rc.Change.AfterUnknown,
		})

		switch action {
		case PlanActionCreate:
			summary.Creates++
		case PlanActionUpdate:
			summary.Updates++
		case PlanActionDelete:
			summary.Deletes++
		case PlanActionReplace:
			summary.Replaces++
		}
	}

	summary.TotalChanges = len(summary.Changes)

	// Sort by address so two runs over the same plan produce identical output.
	sort.Slice(summary.Changes, func(i, j int) bool {
		return summary.Changes[i].Address < summary.Changes[j].Address
	})

	for name, change := range raw.OutputChanges {
		summary.Outputs[name] = normaliseValue(change.After)
	}

	return summary, nil
}

// supportedPlanVersion reports whether the format version is one we read.
func supportedPlanVersion(version string) bool {
	for _, v := range PlanFormatVersions {
		if v == version {
			return true
		}
	}
	return false
}

// classifyActions collapses Terraform's action list into a single action.
//
// Terraform encodes a replacement as a delete plus a create; treating that as
// two independent changes would understate the risk, since a replacement
// destroys a live resource and everything depending on it.
func classifyActions(actions []string) PlanAction {
	switch len(actions) {
	case 0:
		return PlanActionNoop

	case 1:
		switch actions[0] {
		case "create":
			return PlanActionCreate
		case "update":
			return PlanActionUpdate
		case "delete":
			return PlanActionDelete
		case "read":
			return PlanActionRead
		case "no-op":
			return PlanActionNoop
		}

	case 2:
		hasCreate, hasDelete := false, false
		for _, a := range actions {
			switch a {
			case "create":
				hasCreate = true
			case "delete":
				hasDelete = true
			}
		}
		if hasCreate && hasDelete {
			return PlanActionReplace
		}
	}

	return PlanActionNoop
}

// IsDestructive reports whether the action removes or recreates live
// infrastructure. This is the predicate approval policies gate on.
func (a PlanAction) IsDestructive() bool {
	return a == PlanActionDelete || a == PlanActionReplace
}

// Destructive returns only the changes that destroy or replace resources.
func (p *PlanSummary) Destructive() []PlanChange {
	var destructive []PlanChange
	for _, change := range p.Changes {
		if change.Action.IsDestructive() {
			destructive = append(destructive, change)
		}
	}
	return destructive
}

// HasChanges reports whether the plan would modify anything.
func (p *PlanSummary) HasChanges() bool { return p.TotalChanges > 0 }
