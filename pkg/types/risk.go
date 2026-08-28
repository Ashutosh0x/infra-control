package types

import (
	"time"
)

// RiskCategory represents the domain of a given risk.
type RiskCategory string

// Risk categories, one per scored dimension.
const (
	RiskCategorySecurity    RiskCategory = "security"
	RiskCategoryReliability RiskCategory = "reliability"
	RiskCategoryCost        RiskCategory = "cost"
	RiskCategoryCompliance  RiskCategory = "compliance"
)

// String implements fmt.Stringer
func (r RiskCategory) String() string { return string(r) }

// RiskLevel categorizes the severity of a risk.
type RiskLevel string

// Risk levels, ordered from most to least severe.
const (
	RiskLevelCritical   RiskLevel = "critical"
	RiskLevelHigh       RiskLevel = "high"
	RiskLevelMedium     RiskLevel = "medium"
	RiskLevelLow        RiskLevel = "low"
	RiskLevelNegligible RiskLevel = "negligible"
)

// String implements fmt.Stringer
func (r RiskLevel) String() string { return string(r) }

// Marker returns a short text marker for the risk level.
//
// Text rather than an emoji: emoji render inconsistently across terminals,
// carry no meaning for a screen reader, and cannot be styled by the colour
// layer. Callers that want colour apply it to this marker themselves.
func (r RiskLevel) Marker() string {
	switch r {
	case RiskLevelCritical:
		return "CRIT"
	case RiskLevelHigh:
		return "HIGH"
	case RiskLevelMedium:
		return "MED"
	case RiskLevelLow:
		return "LOW"
	case RiskLevelNegligible:
		return "NEG"
	default:
		return "UNKN"
	}
}

// RiskFactor represents a specific condition contributing to the overall risk score.
type RiskFactor struct {
	Category    RiskCategory   `json:"category"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Score       float64        `json:"score"`
	Weight      float64        `json:"weight"`
	Details     map[string]any `json:"details"`
}

// RiskScore aggregates risk data into overall and category-specific scores.
type RiskScore struct {
	Overall     float64      `json:"overall"` // 0-100
	Security    float64      `json:"security"`
	Reliability float64      `json:"reliability"`
	Cost        float64      `json:"cost"`
	Compliance  float64      `json:"compliance"`
	Level       RiskLevel    `json:"level"`
	Factors     []RiskFactor `json:"factors"`
	AssessedAt  time.Time    `json:"assessed_at"`
}

// RiskScoreSnapshot captures a score at a specific point in time.
type RiskScoreSnapshot struct {
	Score     RiskScore `json:"score"`
	Timestamp time.Time `json:"timestamp"`
}

// RiskTrend tracks how risk is changing over time for a specific resource.
type RiskTrend struct {
	ResourceID string              `json:"resource_id"`
	Scores     []RiskScoreSnapshot `json:"scores"`
	Direction  string              `json:"direction"` // e.g., "improving", "degrading", "stable"
}
