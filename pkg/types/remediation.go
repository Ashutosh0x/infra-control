package types

import (
	"time"
)

// RemediationStatus reflects the lifecycle state of a remediation plan.
type RemediationStatus string

// Remediation statuses, tracking a plan through its lifecycle.
const (
	RemediationStatusProposed         RemediationStatus = "proposed"
	RemediationStatusValidating       RemediationStatus = "validating"
	RemediationStatusAwaitingApproval RemediationStatus = "awaiting_approval"
	RemediationStatusApproved         RemediationStatus = "approved"
	RemediationStatusRejected         RemediationStatus = "rejected"
	RemediationStatusExecuting        RemediationStatus = "executing"
	RemediationStatusCompleted        RemediationStatus = "completed"
	RemediationStatusFailed           RemediationStatus = "failed"
	RemediationStatusRolledBack       RemediationStatus = "rolled_back"
)

// String implements fmt.Stringer
func (r RemediationStatus) String() string { return string(r) }

// RemediationRisk categorizes the risk level associated with applying a remediation.
type RemediationRisk string

// Remediation risk bands, gating which plans may auto-apply.
const (
	RemediationRiskLow      RemediationRisk = "low"
	RemediationRiskMedium   RemediationRisk = "medium"
	RemediationRiskHigh     RemediationRisk = "high"
	RemediationRiskCritical RemediationRisk = "critical"
)

// String implements fmt.Stringer
func (r RemediationRisk) String() string { return string(r) }

// RemediationStep details a single action in the plan.
type RemediationStep struct {
	Order           int    `json:"order"`
	Action          string `json:"action"`
	ResourceID      string `json:"resource_id"`
	Description     string `json:"description"`
	TerraformChange string `json:"terraform_change"`
	Risk            string `json:"risk"`
	Status          string `json:"status"`
}

// RollbackStep details an operation to revert a change.
type RollbackStep struct {
	Order         int    `json:"order"`
	Action        string `json:"action"`
	ResourceID    string `json:"resource_id"`
	Description   string `json:"description"`
	PreviousState any    `json:"previous_state"`
}

// RollbackPlan provides instructions to reverse a remediation.
type RollbackPlan struct {
	ID            string         `json:"id"`
	RemediationID string         `json:"remediation_id"`
	SnapshotID    string         `json:"snapshot_id"`
	Steps         []RollbackStep `json:"steps"`
	CreatedAt     time.Time      `json:"created_at"`
}

// RemediationPlan represents a proposed set of changes to resolve an issue.
type RemediationPlan struct {
	ID            string            `json:"id"`
	DriftEventID  string            `json:"drift_event_id"`
	Status        RemediationStatus `json:"status"`
	Risk          RemediationRisk   `json:"risk"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Steps         []RemediationStep `json:"steps"`
	TerraformCode string            `json:"terraform_code"`
	PRUrl         string            `json:"pr_url"`
	BlastRadius   float64           `json:"blast_radius"`
	RiskScore     float64           `json:"risk_score"`
	AIConfidence  float64           `json:"ai_confidence"`
	ProposedBy    string            `json:"proposed_by"` // "ai", "human"
	ApprovedBy    string            `json:"approved_by"`
	CreatedAt     time.Time         `json:"created_at"`
	ExecutedAt    *time.Time        `json:"executed_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	RollbackPlan  *RollbackPlan     `json:"rollback_plan,omitempty"`
}
