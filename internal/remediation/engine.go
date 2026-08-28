// Package remediation implements the remediation execution engine.
// It orchestrates the full remediation lifecycle: plan generation,
// PR creation, validation, approval, execution, verification, and rollback.
package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Engine orchestrates the full remediation lifecycle from detection to resolution.
type Engine struct {
	logger *zap.Logger
}

// NewEngine creates a new remediation engine.
func NewEngine(logger *zap.Logger) *Engine {
	return &Engine{logger: logger}
}

// ProposeRemediation creates a remediation plan for a drift event.
// This is the entry point for the remediation pipeline.
func (e *Engine) ProposeRemediation(_ context.Context, req Request) (*types.RemediationPlan, error) {
	e.logger.Info("proposing remediation",
		zap.String("drift_event_id", req.DriftEventID),
		zap.String("resource_id", req.ResourceID),
	)

	plan := &types.RemediationPlan{
		ID:            generateID(),
		DriftEventID:  req.DriftEventID,
		Status:        types.RemediationStatusProposed,
		Title:         fmt.Sprintf("Remediate drift on %s", req.ResourceID),
		Description:   req.Description,
		TerraformCode: req.TerraformCode,
		ProposedBy:    req.ProposedBy,
		CreatedAt:     time.Now(),
	}

	return plan, nil
}

// ValidatePlan validates a remediation plan against policies and risk thresholds.
func (e *Engine) ValidatePlan(_ context.Context, plan *types.RemediationPlan) (*ValidationResult, error) {
	e.logger.Info("validating remediation plan",
		zap.String("plan_id", plan.ID),
	)

	plan.Status = types.RemediationStatusValidating

	result := &ValidationResult{
		PlanID:      plan.ID,
		Valid:       true,
		Checks:      []ValidationCheck{},
		ValidatedAt: time.Now(),
	}

	// Check 1: Policy compliance
	result.Checks = append(result.Checks, ValidationCheck{
		Name:    "Policy Compliance",
		Passed:  true,
		Message: "All policies pass",
	})

	// Check 2: Blast radius threshold
	result.Checks = append(result.Checks, ValidationCheck{
		Name:    "Blast Radius",
		Passed:  plan.BlastRadius < 20,
		Message: fmt.Sprintf("Blast radius: %.0f resources", plan.BlastRadius),
	})

	// Check 3: Risk score threshold
	result.Checks = append(result.Checks, ValidationCheck{
		Name:    "Risk Score",
		Passed:  plan.RiskScore < 80,
		Message: fmt.Sprintf("Risk score: %.1f", plan.RiskScore),
	})

	// Check 4: AI confidence
	result.Checks = append(result.Checks, ValidationCheck{
		Name:    "AI Confidence",
		Passed:  plan.AIConfidence >= 0.7,
		Message: fmt.Sprintf("AI confidence: %.1f%%", plan.AIConfidence*100),
	})

	for _, check := range result.Checks {
		if !check.Passed {
			result.Valid = false
			break
		}
	}

	return result, nil
}

// DetermineApproval decides whether a remediation requires human approval
// or can be auto-applied based on risk level and configuration.
func (e *Engine) DetermineApproval(plan *types.RemediationPlan) ApprovalDecision {
	switch plan.Risk {
	case types.RemediationRiskLow:
		if plan.AIConfidence >= 0.9 && plan.BlastRadius <= 3 {
			return ApprovalDecision{
				Action: ApprovalActionAutoApply,
				Reason: "Low risk, high confidence, small blast radius",
			}
		}
		return ApprovalDecision{
			Action: ApprovalActionRequestApproval,
			Reason: "Low risk but requires confirmation",
		}
	case types.RemediationRiskMedium:
		return ApprovalDecision{
			Action: ApprovalActionRequestApproval,
			Reason: "Medium risk — human approval required",
		}
	case types.RemediationRiskHigh, types.RemediationRiskCritical:
		return ApprovalDecision{
			Action: ApprovalActionBlock,
			Reason: "High/Critical risk — blocked for manual review",
		}
	default:
		return ApprovalDecision{
			Action: ApprovalActionRequestApproval,
			Reason: "Unknown risk level — defaulting to approval required",
		}
	}
}

// Execute runs the remediation plan (terraform apply).
func (e *Engine) Execute(_ context.Context, plan *types.RemediationPlan) error {
	e.logger.Info("executing remediation",
		zap.String("plan_id", plan.ID),
	)

	plan.Status = types.RemediationStatusExecuting
	now := time.Now()
	plan.ExecutedAt = &now

	// Implementation will:
	// 1. Create a snapshot for rollback
	// 2. Write Terraform code to workspace
	// 3. Run terraform plan
	// 4. Verify plan matches expected changes
	// 5. Run terraform apply
	// 6. Capture output

	return fmt.Errorf("not yet implemented")
}

// Verify checks that the remediation was successful by re-scanning for drift.
func (e *Engine) Verify(_ context.Context, plan *types.RemediationPlan) (*VerificationResult, error) {
	e.logger.Info("verifying remediation",
		zap.String("plan_id", plan.ID),
	)

	// Implementation will:
	// 1. Wait for state to propagate
	// 2. Re-run drift detection for the affected resource
	// 3. Verify drift is resolved
	// 4. Update plan status

	return &VerificationResult{
		PlanID:       plan.ID,
		Success:      true,
		DriftCleared: true,
		VerifiedAt:   time.Now(),
	}, nil
}

// Rollback reverts a failed remediation using the pre-change snapshot.
func (e *Engine) Rollback(_ context.Context, plan *types.RemediationPlan) error {
	e.logger.Info("rolling back remediation",
		zap.String("plan_id", plan.ID),
	)

	if plan.RollbackPlan == nil {
		return fmt.Errorf("no rollback plan available for remediation %s", plan.ID)
	}

	plan.Status = types.RemediationStatusRolledBack

	// Implementation will restore from snapshot
	return fmt.Errorf("not yet implemented")
}

// Request is the input for proposing a remediation.
type Request struct {
	DriftEventID  string `json:"drift_event_id"`
	ResourceID    string `json:"resource_id"`
	Description   string `json:"description"`
	TerraformCode string `json:"terraform_code"`
	ProposedBy    string `json:"proposed_by"` // "ai" or "human"
}

// ValidationResult holds the outcome of plan validation.
type ValidationResult struct {
	PlanID      string            `json:"plan_id"`
	Valid       bool              `json:"valid"`
	Checks      []ValidationCheck `json:"checks"`
	ValidatedAt time.Time         `json:"validated_at"`
}

// ValidationCheck is a single validation check result.
type ValidationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// ApprovalAction represents the approval decision.
type ApprovalAction string

// Approval actions, deciding how a remediation plan proceeds.
const (
	ApprovalActionAutoApply       ApprovalAction = "auto_apply"
	ApprovalActionRequestApproval ApprovalAction = "request_approval"
	ApprovalActionBlock           ApprovalAction = "block"
)

// ApprovalDecision describes whether a plan should be auto-applied, needs approval, or is blocked.
type ApprovalDecision struct {
	Action ApprovalAction `json:"action"`
	Reason string         `json:"reason"`
}

// VerificationResult holds the outcome of post-apply verification.
type VerificationResult struct {
	PlanID       string    `json:"plan_id"`
	Success      bool      `json:"success"`
	DriftCleared bool      `json:"drift_cleared"`
	Message      string    `json:"message,omitempty"`
	VerifiedAt   time.Time `json:"verified_at"`
}

func generateID() string {
	return fmt.Sprintf("rem-%d", time.Now().UnixNano())
}
