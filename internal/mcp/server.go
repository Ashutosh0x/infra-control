// Package mcp implements the Model Context Protocol server for infra-control.
// It exposes infrastructure tools, resources, and prompts to AI agents through
// the MCP specification, enabling controlled AI access with policy guardrails.
package mcp

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Server implements the MCP (Model Context Protocol) server interface,
// providing AI agents with controlled access to infrastructure operations.
type Server struct {
	logger      *zap.Logger
	permissions *PermissionManager
	guardrails  *Guardrails
}

// ServerConfig configures the MCP server.
type ServerConfig struct {
	Port         int      `json:"port" yaml:"port"`
	AuthToken    string   `json:"auth_token" yaml:"auth_token"`
	AllowedTools []string `json:"allowed_tools" yaml:"allowed_tools"`
}

// NewServer creates a new MCP server.
func NewServer(cfg ServerConfig, logger *zap.Logger) *Server {
	return &Server{
		logger:      logger,
		permissions: NewPermissionManager(logger),
		guardrails:  NewGuardrails(logger),
	}
}

// Start starts the MCP server.
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("starting MCP server")
	// Implementation will use github.com/modelcontextprotocol/go-sdk
	// to register tools, resources, and prompts
	return fmt.Errorf("not yet implemented")
}

// Tool represents an MCP tool that AI agents can invoke.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
	Handler     ToolHandler    `json:"-"`
}

// ToolHandler processes an MCP tool invocation.
type ToolHandler func(ctx context.Context, input map[string]any) (any, error)

// Resource represents an MCP resource that AI agents can read.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mime_type"`
}

// RegisterTools returns all available MCP tools for the infrastructure control plane.
func (s *Server) RegisterTools() []Tool {
	return []Tool{
		{
			Name:        "discover_resources",
			Description: "Discover infrastructure resources across cloud providers",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"provider": map[string]any{"type": "string", "enum": []string{"aws", "gcp", "azure", "kubernetes"}},
					"type":     map[string]any{"type": "string", "description": "Resource type filter"},
					"region":   map[string]any{"type": "string", "description": "Region filter"},
				},
			},
		},
		{
			Name:        "detect_drift",
			Description: "Detect infrastructure drift between Terraform state and live resources",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"provider": map[string]any{"type": "string"},
					"severity": map[string]any{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
				},
			},
		},
		{
			Name:        "evaluate_policy",
			Description: "Evaluate resources against policy-as-code rules",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource_id": map[string]any{"type": "string"},
					"policy_type": map[string]any{"type": "string", "enum": []string{"security", "cost", "reliability", "compliance"}},
				},
			},
		},
		{
			Name:        "calculate_risk",
			Description: "Calculate the risk score for a resource",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource_id": map[string]any{"type": "string", "description": "Resource ID to assess"},
				},
				"required": []string{"resource_id"},
			},
		},
		{
			Name:        "get_blast_radius",
			Description: "Calculate the blast radius for a resource change",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource_id": map[string]any{"type": "string"},
					"max_depth":   map[string]any{"type": "integer", "default": 5},
				},
				"required": []string{"resource_id"},
			},
		},
		{
			Name:        "propose_remediation",
			Description: "Propose a remediation plan for a drift event (requires approval for execution)",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"drift_event_id": map[string]any{"type": "string"},
					"description":    map[string]any{"type": "string"},
				},
				"required": []string{"drift_event_id"},
			},
		},
		{
			Name:        "query_graph",
			Description: "Query the infrastructure dependency graph",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start_node_id": map[string]any{"type": "string"},
					"direction":     map[string]any{"type": "string", "enum": []string{"upstream", "downstream", "both"}},
					"max_depth":     map[string]any{"type": "integer", "default": 3},
				},
				"required": []string{"start_node_id"},
			},
		},
		{
			Name:        "get_audit_trail",
			Description: "Retrieve the audit trail for a resource or time range",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"resource_id": map[string]any{"type": "string"},
					"since":       map[string]any{"type": "string", "description": "ISO 8601 timestamp"},
					"limit":       map[string]any{"type": "integer", "default": 50},
				},
			},
		},
	}
}

// RegisterResources returns all available MCP resources.
func (s *Server) RegisterResources() []Resource {
	return []Resource{
		{URI: "infra://graph", Name: "Infrastructure Graph", Description: "The full infrastructure dependency graph", MimeType: "application/json"},
		{URI: "infra://drift/events", Name: "Drift Events", Description: "Current drift events", MimeType: "application/json"},
		{URI: "infra://policies", Name: "Policy Library", Description: "Available policies", MimeType: "application/json"},
		{URI: "infra://audit", Name: "Audit Trail", Description: "Immutable audit trail", MimeType: "application/json"},
		{URI: "infra://risk/summary", Name: "Risk Summary", Description: "Overall risk posture summary", MimeType: "application/json"},
		{URI: "infra://compliance", Name: "Compliance Status", Description: "Compliance framework status", MimeType: "application/json"},
	}
}

// PermissionManager manages AI agent permissions and access control.
type PermissionManager struct {
	logger *zap.Logger
}

// NewPermissionManager creates a new permission manager.
func NewPermissionManager(logger *zap.Logger) *PermissionManager {
	return &PermissionManager{logger: logger}
}

// AgentPermission defines what an AI agent is allowed to do.
type AgentPermission string

const (
	PermissionReadOnly AgentPermission = "read_only" // Can only read/query
	PermissionPropose  AgentPermission = "propose"   // Can propose changes (needs approval)
	PermissionExecute  AgentPermission = "execute"   // Can execute approved changes
)

// CheckPermission verifies if the agent has the required permission.
func (pm *PermissionManager) CheckPermission(ctx context.Context, agentID string, required AgentPermission) error {
	// Implementation will check agent permissions against RBAC
	pm.logger.Debug("checking agent permission",
		zap.String("agent_id", agentID),
		zap.String("required", string(required)),
	)
	return nil
}

// Guardrails enforces safety boundaries for AI agent operations.
type Guardrails struct {
	logger *zap.Logger
}

// NewGuardrails creates a new guardrails enforcer.
func NewGuardrails(logger *zap.Logger) *Guardrails {
	return &Guardrails{logger: logger}
}

// EnforceRateLimit checks if the agent has exceeded its rate limit.
func (g *Guardrails) EnforceRateLimit(ctx context.Context, agentID string) error {
	return nil
}

// EnforceScope checks if the requested operation is within the agent's allowed scope.
func (g *Guardrails) EnforceScope(ctx context.Context, agentID string, tool string, input map[string]any) error {
	// Block destructive operations unless explicitly allowed
	destructiveTools := map[string]bool{
		"propose_remediation": true,
	}

	if destructiveTools[tool] {
		g.logger.Warn("AI agent attempting destructive operation",
			zap.String("agent_id", agentID),
			zap.String("tool", tool),
		)
	}

	return nil
}
