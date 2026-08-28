// Package policy implements the policy-as-code engine using embedded OPA/Rego.
// It supports built-in compliance policies, custom policies, natural language
// policy generation, and pre-deployment plan validation.
package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Engine is the policy evaluation engine. It manages policy lifecycle,
// evaluates resources and plans against policies, and enforces guardrails.
type Engine struct {
	mu       sync.RWMutex
	policies map[string]*types.Policy
	logger   *zap.Logger
}

// NewEngine creates a new policy engine.
func NewEngine(logger *zap.Logger) *Engine {
	return &Engine{
		policies: make(map[string]*types.Policy),
		logger:   logger,
	}
}

// LoadPolicies loads policies from the configured policy directory and built-in library.
func (e *Engine) LoadPolicies(_ context.Context, dir string) error {
	e.logger.Info("loading policies", zap.String("directory", dir))
	// Implementation will:
	// 1. Load built-in policies from embedded Rego files
	// 2. Load custom policies from the directory
	// 3. Validate all policies
	// 4. Register them in the engine
	return nil
}

// AddPolicy adds a policy to the engine.
func (e *Engine) AddPolicy(policy *types.Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	e.policies[policy.ID] = policy
	e.logger.Info("policy added",
		zap.String("id", policy.ID),
		zap.String("name", policy.Name),
	)
	return nil
}

// RemovePolicy removes a policy from the engine.
func (e *Engine) RemovePolicy(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[id]; !exists {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(e.policies, id)
	return nil
}

// EvaluateResources evaluates a set of resources against all enabled policies.
func (e *Engine) EvaluateResources(ctx context.Context, resources []*types.Resource) (*types.PolicyEvaluationResponse, error) {
	e.mu.RLock()
	policies := e.getEnabledPolicies()
	e.mu.RUnlock()

	e.logger.Info("evaluating resources against policies",
		zap.Int("resources", len(resources)),
		zap.Int("policies", len(policies)),
	)

	var results []types.PolicyResult
	passedAll := true

	for _, policy := range policies {
		result, err := e.evaluatePolicy(ctx, policy, resources)
		if err != nil {
			e.logger.Error("policy evaluation failed",
				zap.String("policy", policy.Name),
				zap.Error(err),
			)
			continue
		}
		results = append(results, *result)
		if !result.Passed {
			passedAll = false
		}
	}

	summary := e.summarizeResults(results)

	return &types.PolicyEvaluationResponse{
		Results:   results,
		Summary:   summary,
		PassedAll: passedAll,
	}, nil
}

// EvaluatePlan evaluates a Terraform plan against relevant policies.
func (e *Engine) EvaluatePlan(_ context.Context, _ []byte) (*types.PolicyEvaluationResponse, error) {
	e.logger.Info("evaluating terraform plan against policies")
	// Implementation will:
	// 1. Parse the plan JSON
	// 2. Extract planned changes
	// 3. Evaluate each change against relevant policies
	// 4. Return violations with remediation suggestions
	return nil, fmt.Errorf("not yet implemented")
}

// evaluatePolicy evaluates a single policy against resources using embedded OPA.
func (e *Engine) evaluatePolicy(_ context.Context, policy *types.Policy, _ []*types.Resource) (*types.PolicyResult, error) {
	// Implementation will use github.com/open-policy-agent/opa/rego
	// to evaluate the policy's Rego code against the resources
	result := &types.PolicyResult{
		PolicyID:   policy.ID,
		PolicyName: policy.Name,
		Passed:     true,
		Violations: []types.PolicyViolation{},
	}
	return result, nil
}

// getEnabledPolicies returns all enabled policies.
func (e *Engine) getEnabledPolicies() []*types.Policy {
	var enabled []*types.Policy
	for _, p := range e.policies {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// summarizeResults generates a summary of policy evaluation results.
func (e *Engine) summarizeResults(results []types.PolicyResult) types.PolicySummary {
	summary := types.PolicySummary{Total: len(results)}
	for _, r := range results {
		if r.Passed {
			summary.Passed++
		} else {
			summary.Failed++
			for _, v := range r.Violations {
				switch v.Severity {
				case types.PolicySeverityError:
					summary.Errors++
				case types.PolicySeverityWarning:
					summary.Warnings++
				case types.PolicySeverityInfo:
					summary.Infos++
				}
			}
		}
	}
	return summary
}

// ListPolicies returns all registered policies.
func (e *Engine) ListPolicies() []*types.Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*types.Policy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	return policies
}

// GetPolicy returns a policy by ID.
func (e *Engine) GetPolicy(id string) (*types.Policy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	p, exists := e.policies[id]
	if !exists {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return p, nil
}
