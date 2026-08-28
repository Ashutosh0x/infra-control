package types

import (
	"time"
)

// PolicyType defines the nature of the policy.
type PolicyType string

// Policy types, grouping policies by the concern they cover.
const (
	PolicyTypeSecurity    PolicyType = "security"
	PolicyTypeCost        PolicyType = "cost"
	PolicyTypeReliability PolicyType = "reliability"
	PolicyTypeCompliance  PolicyType = "compliance"
	PolicyTypeCustom      PolicyType = "custom"
)

// String implements fmt.Stringer
func (p PolicyType) String() string { return string(p) }

// PolicySeverity indicates the critical level of the policy.
type PolicySeverity string

// Policy severities, describing how a violation should be treated.
const (
	PolicySeverityError   PolicySeverity = "error"
	PolicySeverityWarning PolicySeverity = "warning"
	PolicySeverityInfo    PolicySeverity = "info"
)

// String implements fmt.Stringer
func (p PolicySeverity) String() string { return string(p) }

// PolicyEnforcement defines how the policy should be enforced.
type PolicyEnforcement string

// Policy enforcement modes, from reporting only through to blocking.
const (
	PolicyEnforcementAdvisory      PolicyEnforcement = "advisory"
	PolicyEnforcementSoftMandatory PolicyEnforcement = "soft_mandatory"
	PolicyEnforcementHardMandatory PolicyEnforcement = "hard_mandatory"
)

// String implements fmt.Stringer
func (p PolicyEnforcement) String() string { return string(p) }

// PolicyRule defines a single condition to evaluate.
type PolicyRule struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Condition    string `json:"condition"`
	ErrorMessage string `json:"error_message"`
}

// Policy is a set of rules for compliance or governance.
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        PolicyType        `json:"type"`
	Severity    PolicySeverity    `json:"severity"`
	Enforcement PolicyEnforcement `json:"enforcement"`
	Provider    string            `json:"provider"` // e.g., "all", "aws", "gcp", "azure", "kubernetes"
	Rules       []PolicyRule      `json:"rules"`
	RegoCode    string            `json:"rego_code"`
	Version     int               `json:"version"`
	Enabled     bool              `json:"enabled"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolicyViolation details an instance where a resource failed a policy rule.
type PolicyViolation struct {
	RuleID       string         `json:"rule_id"`
	ResourceID   string         `json:"resource_id"`
	ResourceType string         `json:"resource_type"`
	Message      string         `json:"message"`
	Severity     PolicySeverity `json:"severity"`
	Remediation  string         `json:"remediation"`
}

// PolicyResult holds the outcome of evaluating a policy.
type PolicyResult struct {
	PolicyID    string            `json:"policy_id"`
	PolicyName  string            `json:"policy_name"`
	Passed      bool              `json:"passed"`
	Violations  []PolicyViolation `json:"violations"`
	EvaluatedAt time.Time         `json:"evaluated_at"`
}

// PolicySummary aggregates the evaluation results.
type PolicySummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
}

// PolicyEvaluationRequest represents a request to evaluate policies.
type PolicyEvaluationRequest struct {
	Resources []Resource `json:"resources"`
	PlanJSON  []byte     `json:"plan_json"`
	Policies  []Policy   `json:"policies"`
}

// PolicyEvaluationResponse contains the results of evaluating policies.
type PolicyEvaluationResponse struct {
	Results   []PolicyResult `json:"results"`
	Summary   PolicySummary  `json:"summary"`
	PassedAll bool           `json:"passed_all"`
}
