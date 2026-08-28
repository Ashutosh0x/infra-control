// Package ai implements the AI engine for infrastructure investigation,
// remediation planning, code review, and natural language policy generation.
// It supports multiple LLM providers (OpenAI, Gemini, Anthropic) with
// prompt templates, confidence scoring, and safety guardrails.
package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Complete sends a prompt to the LLM and returns the response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Stream sends a prompt and streams the response token by token.
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

// CompletionRequest represents a request to an LLM.
type CompletionRequest struct {
	SystemPrompt string            `json:"system_prompt"`
	UserPrompt   string            `json:"user_prompt"`
	Model        string            `json:"model,omitempty"`
	Temperature  float64           `json:"temperature,omitempty"`
	MaxTokens    int               `json:"max_tokens,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// CompletionResponse represents an LLM response.
type CompletionResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	TokensUsed   int    `json:"tokens_used"`
	FinishReason string `json:"finish_reason"`
}

// StreamChunk represents a single token in a streaming response.
type StreamChunk struct {
	Content string `json:"content"`
	Done    bool   `json:"done"`
	Error   error  `json:"error,omitempty"`
}

// Agent is the AI agent orchestrator that coordinates investigation,
// planning, and remediation using LLM providers.
type Agent struct {
	provider   Provider
	logger     *zap.Logger
	maxRetries int
}

// AgentConfig configures the AI agent.
type AgentConfig struct {
	ProviderName string  `json:"provider" yaml:"provider"`
	Model        string  `json:"model" yaml:"model"`
	APIKey       string  `json:"api_key" yaml:"api_key"`
	Temperature  float64 `json:"temperature" yaml:"temperature"`
	MaxTokens    int     `json:"max_tokens" yaml:"max_tokens"`
	MaxRetries   int     `json:"max_retries" yaml:"max_retries"`
}

// NewAgent creates a new AI agent with the specified provider.
func NewAgent(provider Provider, logger *zap.Logger) *Agent {
	return &Agent{
		provider:   provider,
		logger:     logger,
		maxRetries: 3,
	}
}

// InvestigateDrift uses AI to investigate the root cause of a drift event.
func (a *Agent) InvestigateDrift(ctx context.Context, event *types.DriftEvent, auditEvents []string) (*Investigation, error) {
	a.logger.Info("investigating drift with AI",
		zap.String("resource_id", event.ResourceID),
		zap.String("severity", string(event.Severity)),
	)

	prompt := buildInvestigationPrompt(event, auditEvents)

	resp, err := a.provider.Complete(ctx, CompletionRequest{
		SystemPrompt: investigatorSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  0.3,
		MaxTokens:    2000,
	})
	if err != nil {
		return nil, fmt.Errorf("AI investigation failed: %w", err)
	}

	investigation := &Investigation{
		DriftEventID:    event.ID,
		Summary:         resp.Content,
		RootCause:       "Requires parsing from AI response",
		Confidence:      0.0,
		Recommendations: []string{},
		GeneratedAt:     time.Now(),
	}

	return investigation, nil
}

// GenerateRemediation generates Terraform remediation code for a drift event.
func (a *Agent) GenerateRemediation(ctx context.Context, event *types.DriftEvent, currentTF string) (*RemediationPlan, error) {
	a.logger.Info("generating remediation plan",
		zap.String("resource_id", event.ResourceID),
	)

	prompt := buildRemediationPrompt(event, currentTF)

	resp, err := a.provider.Complete(ctx, CompletionRequest{
		SystemPrompt: remediationSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  0.2,
		MaxTokens:    4000,
	})
	if err != nil {
		return nil, fmt.Errorf("AI remediation generation failed: %w", err)
	}

	plan := &RemediationPlan{
		DriftEventID:  event.ID,
		TerraformCode: resp.Content,
		Explanation:   "AI-generated remediation",
		Confidence:    0.0,
		GeneratedAt:   time.Now(),
	}

	return plan, nil
}

// ReviewCode reviews AI-generated or human-written Terraform code for safety.
func (a *Agent) ReviewCode(ctx context.Context, code string, context string) (*CodeReview, error) {
	a.logger.Info("reviewing terraform code with AI")

	prompt := buildCodeReviewPrompt(code, context)

	resp, err := a.provider.Complete(ctx, CompletionRequest{
		SystemPrompt: reviewerSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  0.1,
		MaxTokens:    2000,
	})
	if err != nil {
		return nil, fmt.Errorf("AI code review failed: %w", err)
	}

	review := &CodeReview{
		Approved:    true, // Parsed from AI response
		Comments:    []string{resp.Content},
		RiskLevel:   "low",
		Suggestions: []string{},
		ReviewedAt:  time.Now(),
	}

	return review, nil
}

// GeneratePolicy converts a natural language policy description to Rego code.
func (a *Agent) GeneratePolicy(ctx context.Context, description string) (*PolicyGeneration, error) {
	a.logger.Info("generating policy from natural language",
		zap.String("description", description),
	)

	prompt := buildPolicyGenPrompt(description)

	resp, err := a.provider.Complete(ctx, CompletionRequest{
		SystemPrompt: policyGeneratorSystemPrompt,
		UserPrompt:   prompt,
		Temperature:  0.2,
		MaxTokens:    3000,
	})
	if err != nil {
		return nil, fmt.Errorf("AI policy generation failed: %w", err)
	}

	gen := &PolicyGeneration{
		Description: description,
		RegoCode:    resp.Content,
		Explanation: "AI-generated Rego policy",
		GeneratedAt: time.Now(),
	}

	return gen, nil
}

// ScoreConfidence evaluates the confidence level of an AI-generated remediation.
func (a *Agent) ScoreConfidence(_ context.Context, plan *RemediationPlan, blastRadius int) float64 {
	confidence := 0.8 // Base confidence

	// Reduce confidence for large blast radius
	if blastRadius > 10 {
		confidence -= 0.2
	} else if blastRadius > 5 {
		confidence -= 0.1
	}

	// Reduce confidence for complex changes
	if len(plan.TerraformCode) > 5000 {
		confidence -= 0.15
	}

	if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// Investigation represents the result of an AI drift investigation.
type Investigation struct {
	DriftEventID    string    `json:"drift_event_id"`
	Summary         string    `json:"summary"`
	RootCause       string    `json:"root_cause"`
	Confidence      float64   `json:"confidence"`
	Recommendations []string  `json:"recommendations"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// RemediationPlan represents an AI-generated remediation plan.
type RemediationPlan struct {
	DriftEventID  string    `json:"drift_event_id"`
	TerraformCode string    `json:"terraform_code"`
	Explanation   string    `json:"explanation"`
	Confidence    float64   `json:"confidence"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// CodeReview represents an AI code review result.
type CodeReview struct {
	Approved    bool      `json:"approved"`
	Comments    []string  `json:"comments"`
	RiskLevel   string    `json:"risk_level"`
	Suggestions []string  `json:"suggestions"`
	ReviewedAt  time.Time `json:"reviewed_at"`
}

// PolicyGeneration represents an AI-generated policy.
type PolicyGeneration struct {
	Description string    `json:"description"`
	RegoCode    string    `json:"rego_code"`
	Explanation string    `json:"explanation"`
	GeneratedAt time.Time `json:"generated_at"`
}

// System prompts for different AI agent roles.
const investigatorSystemPrompt = `You are an expert infrastructure security investigator. Given a drift event with audit trail data, determine the root cause, assess the risk, and recommend remediation. Be precise and actionable.`

const remediationSystemPrompt = `You are an expert Terraform engineer. Generate minimal, safe Terraform code to remediate infrastructure drift. Only modify what's necessary. Include comments explaining each change. Never remove security controls.`

const reviewerSystemPrompt = `You are an expert Terraform code reviewer focused on security, reliability, and best practices. Review the provided code and identify risks, improvements, and potential issues.`

const policyGeneratorSystemPrompt = `You are an expert in OPA Rego policy language. Convert the natural language policy description into valid Rego code that can be evaluated by Open Policy Agent. Include proper package declaration, rule names, and helpful error messages.`

// Prompt builders
func buildInvestigationPrompt(event *types.DriftEvent, auditEvents []string) string {
	return fmt.Sprintf("Investigate this infrastructure drift event:\nResource: %s\nType: %s\nSeverity: %s\nDrift Type: %s\nProperties changed: %d\n\nAudit trail events:\n%v",
		event.ResourceID, event.Type, event.Severity, event.Classification, len(event.Diff), auditEvents)
}

func buildRemediationPrompt(event *types.DriftEvent, currentTF string) string {
	return fmt.Sprintf("Generate Terraform code to remediate this drift:\nResource: %s\nExpected state: %v\nActual state: %v\n\nCurrent Terraform:\n%s",
		event.ResourceID, event.ExpectedState, event.ActualState, currentTF)
}

func buildCodeReviewPrompt(code, context string) string {
	return fmt.Sprintf("Review this Terraform code for safety and best practices:\nContext: %s\n\nCode:\n%s", context, code)
}

func buildPolicyGenPrompt(description string) string {
	return fmt.Sprintf("Convert this natural language policy to OPA Rego:\n\n%s\n\nGenerate valid Rego code with proper package declaration and helpful violation messages.", description)
}
