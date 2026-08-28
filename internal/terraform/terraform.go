// Package terraform provides integration with Terraform and OpenTofu for
// state parsing, plan analysis, execution, and HCL configuration parsing.
package terraform

import (
	"context"
	"fmt"
	"sort"

	"go.uber.org/zap"
)

// Service orchestrates all Terraform/OpenTofu operations including
// state management, plan generation, and execution.
type Service struct {
	binaryPath   string
	workspaceDir string
	parallelism  int
	logger       *zap.Logger
}

// Config holds Terraform service configuration.
type Config struct {
	BinaryPath   string // Path to terraform or tofu binary
	WorkspaceDir string // Directory for Terraform workspaces
	Parallelism  int    // Max parallel operations
	PlanTimeout  int    // Plan timeout in seconds
	ApplyTimeout int    // Apply timeout in seconds
}

// NewService creates a new Terraform service.
func NewService(cfg Config, logger *zap.Logger) (*Service, error) {
	if cfg.BinaryPath == "" {
		cfg.BinaryPath = "terraform"
	}
	if cfg.Parallelism == 0 {
		cfg.Parallelism = 10
	}

	return &Service{
		binaryPath:   cfg.BinaryPath,
		workspaceDir: cfg.WorkspaceDir,
		parallelism:  cfg.Parallelism,
		logger:       logger,
	}, nil
}

// Workspace represents a Terraform workspace with its state and configuration.
type Workspace struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Backend   BackendConfig     `json:"backend"`
	Variables map[string]string `json:"variables,omitempty"`
}

// BackendConfig describes the Terraform state backend.
type BackendConfig struct {
	Type   string         `json:"type"` // "local", "s3", "gcs", "azurerm", "remote"
	Config map[string]any `json:"config"`
}

// StateResource represents a resource from Terraform state.
type StateResource struct {
	Address      string         `json:"address"`    // e.g., "aws_s3_bucket.my_bucket"
	Mode         string         `json:"mode"`       // "managed" or "data"
	Type         string         `json:"type"`       // e.g., "aws_s3_bucket"
	Name         string         `json:"name"`       // e.g., "my_bucket"
	Provider     string         `json:"provider"`   // e.g., "registry.terraform.io/hashicorp/aws"
	Module       string         `json:"module"`     // e.g., "module.vpc"
	Attributes   map[string]any `json:"attributes"` // Full attribute map
	Dependencies []string       `json:"dependencies"`
}

// PlanChange represents a single resource change in a Terraform plan.
type PlanChange struct {
	Address      string         `json:"address"`
	Type         string         `json:"type"`
	Name         string         `json:"name"`
	Provider     string         `json:"provider"`
	Action       PlanAction     `json:"action"`
	Before       map[string]any `json:"before,omitempty"`
	After        map[string]any `json:"after,omitempty"`
	AfterUnknown map[string]any `json:"after_unknown,omitempty"`
}

// PlanAction represents the action Terraform will take on a resource.
type PlanAction string

const (
	PlanActionCreate  PlanAction = "create"
	PlanActionUpdate  PlanAction = "update"
	PlanActionDelete  PlanAction = "delete"
	PlanActionReplace PlanAction = "replace"
	PlanActionRead    PlanAction = "read"
	PlanActionNoop    PlanAction = "no-op"
)

// PlanSummary provides a high-level overview of a Terraform plan.
type PlanSummary struct {
	TotalChanges int            `json:"total_changes"`
	Creates      int            `json:"creates"`
	Updates      int            `json:"updates"`
	Deletes      int            `json:"deletes"`
	Replaces     int            `json:"replaces"`
	Changes      []PlanChange   `json:"changes"`
	Outputs      map[string]any `json:"outputs,omitempty"`
}

// PlanValidation holds the results of validating a Terraform plan.
type PlanValidation struct {
	Valid         bool           `json:"valid"`
	Errors        []string       `json:"errors,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
	RiskScore     float64        `json:"risk_score"`
	BlastRadius   int            `json:"blast_radius"`
	PolicyResults []PolicyResult `json:"policy_results,omitempty"`
}

// PolicyResult represents a single policy evaluation result for a plan.
type PolicyResult struct {
	PolicyName string `json:"policy_name"`
	Passed     bool   `json:"passed"`
	Message    string `json:"message,omitempty"`
}

// ParseState reads a Terraform state file and returns all managed resources.
func (s *Service) ParseState(ctx context.Context, statePath string) ([]StateResource, error) {
	s.logger.Info("parsing terraform state", zap.String("path", statePath))

	state, err := ParseStateFile(statePath)
	if err != nil {
		return nil, err
	}

	resources := state.ManagedResources()
	s.logger.Info("parsed terraform state",
		zap.String("terraform_version", state.TerraformVersion),
		zap.Uint64("serial", state.Serial),
		zap.Int("managed_resources", len(resources)),
	)
	return resources, nil
}

// GeneratePlan creates a Terraform plan for the given workspace.
func (s *Service) GeneratePlan(ctx context.Context, workspace Workspace) (*PlanSummary, error) {
	s.logger.Info("generating terraform plan",
		zap.String("workspace", workspace.Name),
		zap.String("path", workspace.Path),
	)
	// Implementation will use hashicorp/terraform-exec (tfexec)
	return nil, fmt.Errorf("not yet implemented")
}

// ValidatePlan validates a Terraform plan against policies and risk thresholds.
func (s *Service) ValidatePlan(ctx context.Context, plan *PlanSummary) (*PlanValidation, error) {
	s.logger.Info("validating terraform plan", zap.Int("changes", plan.TotalChanges))
	// Implementation will evaluate policies and calculate blast radius
	return nil, fmt.Errorf("not yet implemented")
}

// Apply executes a Terraform apply for the given workspace.
func (s *Service) Apply(ctx context.Context, workspace Workspace) error {
	s.logger.Info("applying terraform changes",
		zap.String("workspace", workspace.Name),
	)
	// Implementation will use terraform-exec with approval checks
	return fmt.Errorf("not yet implemented")
}

// Refresh performs a Terraform refresh to sync state with real infrastructure.
func (s *Service) Refresh(ctx context.Context, workspace Workspace) error {
	s.logger.Info("refreshing terraform state",
		zap.String("workspace", workspace.Name),
	)
	return fmt.Errorf("not yet implemented")
}

// ListWorkspaces returns all configured Terraform workspaces.
func (s *Service) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	return nil, fmt.Errorf("not yet implemented")
}

// GetStateResources retrieves all resources from a workspace's Terraform state.
func (s *Service) GetStateResources(ctx context.Context, workspace Workspace) ([]StateResource, error) {
	return nil, fmt.Errorf("not yet implemented")
}

// CompareStateToLive compares Terraform state resources against live cloud
// resources and returns the differences. This is the foundation of drift
// detection.
//
// liveResources is keyed by Terraform address. Three outcomes are reported:
// an address present in both with differing attributes is "modified"; an
// address in state but absent live is "missing_in_live" (the resource was
// deleted outside Terraform); an address live but absent from state is
// "missing_in_state" (an unmanaged resource).
func (s *Service) CompareStateToLive(ctx context.Context, stateResources []StateResource, liveResources map[string]map[string]any) ([]StateDiff, error) {
	s.logger.Info("comparing state to live infrastructure",
		zap.Int("state_resources", len(stateResources)),
		zap.Int("live_resources", len(liveResources)),
	)

	var diffs []StateDiff
	inState := make(map[string]struct{}, len(stateResources))

	for _, resource := range stateResources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		inState[resource.Address] = struct{}{}

		live, found := liveResources[resource.Address]
		if !found {
			diffs = append(diffs, StateDiff{
				Address:    resource.Address,
				Type:       resource.Type,
				DiffType:   DiffTypeMissingInLive,
				StateValue: resource.Attributes,
			})
			continue
		}

		changes := CompareAttributes(resource.Attributes, live)
		if len(changes) == 0 {
			continue
		}

		diffs = append(diffs, StateDiff{
			Address:    resource.Address,
			Type:       resource.Type,
			DiffType:   DiffTypeModified,
			StateValue: resource.Attributes,
			LiveValue:  live,
			Changes:    changes,
		})
	}

	for address, live := range liveResources {
		if _, managed := inState[address]; managed {
			continue
		}
		diffs = append(diffs, StateDiff{
			Address:   address,
			Type:      typeFromAddress(address),
			DiffType:  DiffTypeMissingInState,
			LiveValue: live,
		})
	}

	// Sort so repeated runs over unchanged inputs produce identical output.
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Address < diffs[j].Address })

	s.logger.Info("state comparison complete", zap.Int("differences", len(diffs)))
	return diffs, nil
}

// StateDiff represents a difference between Terraform state and live infrastructure.
type StateDiff struct {
	Address    string         `json:"address"`
	Type       string         `json:"type"`
	DiffType   string         `json:"diff_type"` // "modified", "missing_in_state", "missing_in_live"
	StateValue map[string]any `json:"state_value,omitempty"`
	LiveValue  map[string]any `json:"live_value,omitempty"`
	Changes    []FieldDiff    `json:"changes,omitempty"`
}

// FieldDiff represents a difference in a single field.
type FieldDiff struct {
	Path     string `json:"path"`
	Expected any    `json:"expected"`
	Actual   any    `json:"actual"`
}
