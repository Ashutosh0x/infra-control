// Package risk implements the multi-dimensional risk scoring engine.
// It assesses security, reliability, cost, and compliance risk for
// infrastructure resources and generates composite risk scores.
package risk

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Engine is the risk scoring engine that evaluates resources across
// multiple risk dimensions and produces composite scores.
type Engine struct {
	weights RiskWeights
	logger  *zap.Logger
}

// RiskWeights defines the relative importance of each risk category.
type RiskWeights struct {
	Security    float64 `json:"security"    yaml:"security"`
	Reliability float64 `json:"reliability" yaml:"reliability"`
	Cost        float64 `json:"cost"        yaml:"cost"`
	Compliance  float64 `json:"compliance"  yaml:"compliance"`
}

// DefaultWeights returns the default risk scoring weights.
func DefaultWeights() RiskWeights {
	return RiskWeights{
		Security:    0.35,
		Reliability: 0.30,
		Cost:        0.15,
		Compliance:  0.20,
	}
}

// NewEngine creates a new risk scoring engine with the given weights.
func NewEngine(weights RiskWeights, logger *zap.Logger) *Engine {
	return &Engine{
		weights: weights,
		logger:  logger,
	}
}

// Assess performs a full risk assessment on a resource and returns a composite score.
func (e *Engine) Assess(ctx context.Context, resource *types.Resource) (*types.RiskScore, error) {
	e.logger.Debug("assessing risk",
		zap.String("resource_id", resource.ID),
		zap.String("type", resource.Type),
	)

	var factors []types.RiskFactor

	// Security assessment
	secScore, secFactors := e.assessSecurity(resource)
	factors = append(factors, secFactors...)

	// Reliability assessment
	relScore, relFactors := e.assessReliability(resource)
	factors = append(factors, relFactors...)

	// Cost assessment
	costScore, costFactors := e.assessCost(resource)
	factors = append(factors, costFactors...)

	// Compliance assessment
	compScore, compFactors := e.assessCompliance(resource)
	factors = append(factors, compFactors...)

	// Calculate composite score
	overall := secScore*e.weights.Security +
		relScore*e.weights.Reliability +
		costScore*e.weights.Cost +
		compScore*e.weights.Compliance

	// Normalize to 0-100
	overall = math.Min(100, math.Max(0, overall))

	score := &types.RiskScore{
		Overall:     overall,
		Security:    secScore,
		Reliability: relScore,
		Cost:        costScore,
		Compliance:  compScore,
		Level:       e.scoreToLevel(overall),
		Factors:     factors,
		AssessedAt:  time.Now(),
	}

	e.logger.Info("risk assessment complete",
		zap.String("resource_id", resource.ID),
		zap.Float64("overall", overall),
		zap.String("level", string(score.Level)),
	)

	return score, nil
}

// AssessBatch performs risk assessment on multiple resources concurrently.
func (e *Engine) AssessBatch(ctx context.Context, resources []*types.Resource) (map[string]*types.RiskScore, error) {
	scores := make(map[string]*types.RiskScore, len(resources))
	for _, r := range resources {
		score, err := e.Assess(ctx, r)
		if err != nil {
			return nil, fmt.Errorf("assessing resource %s: %w", r.ID, err)
		}
		scores[r.ID] = score
	}
	return scores, nil
}

// assessSecurity evaluates security risk factors for a resource.
func (e *Engine) assessSecurity(resource *types.Resource) (float64, []types.RiskFactor) {
	var factors []types.RiskFactor
	score := 0.0

	// Check public exposure
	if resource.Metadata.IsPublic {
		score += 30
		factors = append(factors, types.RiskFactor{
			Category:    types.RiskCategorySecurity,
			Name:        "Public Exposure",
			Description: "Resource is publicly accessible",
			Score:       30,
			Weight:      1.0,
		})
	}

	// Check encryption
	if !resource.Metadata.IsEncrypted {
		score += 25
		factors = append(factors, types.RiskFactor{
			Category:    types.RiskCategorySecurity,
			Name:        "Missing Encryption",
			Description: "Resource is not encrypted at rest",
			Score:       25,
			Weight:      1.0,
		})
	}

	// Check if managed by IaC
	if resource.Metadata.ManagedBy == "" || resource.Metadata.ManagedBy == "manual" {
		score += 15
		factors = append(factors, types.RiskFactor{
			Category:    types.RiskCategorySecurity,
			Name:        "Not IaC Managed",
			Description: "Resource is not managed by Infrastructure as Code",
			Score:       15,
			Weight:      1.0,
		})
	}

	return math.Min(100, score), factors
}

// assessReliability evaluates reliability risk factors for a resource.
func (e *Engine) assessReliability(resource *types.Resource) (float64, []types.RiskFactor) {
	var factors []types.RiskFactor
	score := 0.0

	cfg := resource.Configuration

	// Check for single AZ deployment
	if _, hasAZ := cfg["availability_zone"]; hasAZ {
		if _, hasMultiAZ := cfg["multi_az"]; !hasMultiAZ {
			score += 20
			factors = append(factors, types.RiskFactor{
				Category:    types.RiskCategoryReliability,
				Name:        "Single AZ",
				Description: "Resource is deployed in a single availability zone",
				Score:       20,
				Weight:      1.0,
			})
		}
	}

	// Check for backups
	if backupEnabled, ok := cfg["backup_retention_period"]; ok {
		if period, isFloat := backupEnabled.(float64); isFloat && period == 0 {
			score += 25
			factors = append(factors, types.RiskFactor{
				Category:    types.RiskCategoryReliability,
				Name:        "No Backups",
				Description: "Resource does not have backups configured",
				Score:       25,
				Weight:      1.0,
			})
		}
	}

	return math.Min(100, score), factors
}

// assessCost evaluates cost risk factors for a resource.
func (e *Engine) assessCost(resource *types.Resource) (float64, []types.RiskFactor) {
	var factors []types.RiskFactor
	score := 0.0

	// Check for potential overprovisioning indicators
	cfg := resource.Configuration
	if instanceType, ok := cfg["instance_type"].(string); ok {
		if isLargeInstance(instanceType) {
			score += 20
			factors = append(factors, types.RiskFactor{
				Category:    types.RiskCategoryCost,
				Name:        "Large Instance",
				Description: fmt.Sprintf("Instance type %s may be overprovisioned", instanceType),
				Score:       20,
				Weight:      1.0,
			})
		}
	}

	return math.Min(100, score), factors
}

// assessCompliance evaluates compliance risk factors for a resource.
func (e *Engine) assessCompliance(resource *types.Resource) (float64, []types.RiskFactor) {
	var factors []types.RiskFactor
	score := 0.0

	// Check for required compliance tags
	requiredTags := []string{"environment", "owner", "cost-center"}
	for _, tag := range requiredTags {
		if _, exists := resource.Tags[tag]; !exists {
			score += 10
			factors = append(factors, types.RiskFactor{
				Category:    types.RiskCategoryCompliance,
				Name:        "Missing Tag",
				Description: fmt.Sprintf("Required tag '%s' is missing", tag),
				Score:       10,
				Weight:      0.5,
			})
		}
	}

	return math.Min(100, score), factors
}

// scoreToLevel converts a numeric risk score to a risk level.
func (e *Engine) scoreToLevel(score float64) types.RiskLevel {
	switch {
	case score >= 80:
		return types.RiskLevelCritical
	case score >= 60:
		return types.RiskLevelHigh
	case score >= 40:
		return types.RiskLevelMedium
	case score >= 20:
		return types.RiskLevelLow
	default:
		return types.RiskLevelNegligible
	}
}

// isLargeInstance checks if an instance type is considered large (potential overprovisioning).
func isLargeInstance(instanceType string) bool {
	largeTypes := []string{
		"xlarge", "2xlarge", "4xlarge", "8xlarge", "12xlarge",
		"16xlarge", "24xlarge", "metal",
		"n2-standard-16", "n2-standard-32", "n2-standard-64",
		"Standard_D16", "Standard_D32", "Standard_D64",
	}
	for _, lt := range largeTypes {
		if containsStr(instanceType, lt) {
			return true
		}
	}
	return false
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
