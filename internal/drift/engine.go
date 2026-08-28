// Package drift implements the infrastructure drift detection engine.
// It compares live cloud resource state against Terraform/OpenTofu state
// to identify unauthorized changes, misconfigurations, and configuration drift.
package drift

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// Engine is the core drift detection orchestrator. It coordinates resource
// discovery, state comparison, classification, and event generation.
type Engine struct {
	detector   *Detector
	classifier *Classifier
	attributor *Attributor
	scheduler  *Scheduler
	logger     *zap.Logger
	mu         sync.RWMutex
	running    bool
}

// EngineConfig configures the drift detection engine.
type EngineConfig struct {
	ScanInterval       time.Duration `json:"scan_interval"`
	EnableRealtime     bool          `json:"enable_realtime"`
	AutoRemediate      bool          `json:"auto_remediate"`
	MaxRemediationRisk string        `json:"max_remediation_risk"`
}

// NewEngine creates a new drift detection engine.
func NewEngine(cfg EngineConfig, logger *zap.Logger) *Engine {
	return &Engine{
		detector:   NewDetector(logger),
		classifier: NewClassifier(logger),
		attributor: NewAttributor(logger),
		scheduler:  NewScheduler(cfg.ScanInterval, logger),
		logger:     logger,
	}
}

// Start begins continuous drift detection.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("drift engine is already running")
	}
	e.running = true
	e.mu.Unlock()

	e.logger.Info("starting drift detection engine")
	return e.scheduler.Start(ctx, e.runScan)
}

// Stop halts the drift detection engine.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = false
	e.logger.Info("stopping drift detection engine")
	return e.scheduler.Stop()
}

// RunScan performs a single drift detection scan.
func (e *Engine) runScan(_ context.Context) error {
	e.logger.Info("running drift detection scan")
	// 1. Get all Terraform state resources
	// 2. Get all live cloud resources
	// 3. Compare and detect differences
	// 4. Classify each drift event
	// 5. Attribute to identity
	// 6. Score severity
	// 7. Emit events
	return nil
}

// Scan performs an on-demand drift scan and returns detected events.
func (e *Engine) Scan(_ context.Context) ([]types.DriftEvent, error) {
	e.logger.Info("performing on-demand drift scan")
	return nil, fmt.Errorf("not yet implemented")
}

// Detector performs the core comparison between expected and actual resource state.
type Detector struct {
	logger *zap.Logger
}

// NewDetector creates a new drift detector.
func NewDetector(logger *zap.Logger) *Detector {
	return &Detector{logger: logger}
}

// Detect compares expected state with actual live state and returns property differences.
func (d *Detector) Detect(expected, actual map[string]any) []types.PropertyDiff {
	var diffs []types.PropertyDiff
	d.compareRecursive("", expected, actual, &diffs)
	return diffs
}

// compareRecursive recursively compares two maps and collects differences.
func (d *Detector) compareRecursive(prefix string, expected, actual map[string]any, diffs *[]types.PropertyDiff) {
	for key, expVal := range expected {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		actVal, exists := actual[key]
		if !exists {
			*diffs = append(*diffs, types.PropertyDiff{
				Path:     path,
				Expected: expVal,
				Actual:   nil,
			})
			continue
		}

		// Recursively compare nested maps
		expMap, expIsMap := expVal.(map[string]any)
		actMap, actIsMap := actVal.(map[string]any)
		if expIsMap && actIsMap {
			d.compareRecursive(path, expMap, actMap, diffs)
			continue
		}

		// Compare values using JSON serialization for deep equality
		expJSON, _ := json.Marshal(expVal)
		actJSON, _ := json.Marshal(actVal)
		if string(expJSON) != string(actJSON) {
			*diffs = append(*diffs, types.PropertyDiff{
				Path:     path,
				Expected: expVal,
				Actual:   actVal,
			})
		}
	}

	// Check for keys in actual that are not in expected
	for key, actVal := range actual {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if _, exists := expected[key]; !exists {
			*diffs = append(*diffs, types.PropertyDiff{
				Path:     path,
				Expected: nil,
				Actual:   actVal,
			})
		}
	}
}

// Classifier categorizes drift events by their likely cause and severity.
type Classifier struct {
	logger *zap.Logger
}

// NewClassifier creates a new drift classifier.
func NewClassifier(logger *zap.Logger) *Classifier {
	return &Classifier{logger: logger}
}

// Classify determines the classification and severity of a drift event.
func (c *Classifier) Classify(event *types.DriftEvent) {
	// Classification logic based on:
	// 1. Resource type (security-critical resources get higher severity)
	// 2. Changed properties (encryption, public access = critical)
	// 3. Attribution source (console = likely intentional, unknown = suspicious)
	// 4. Time patterns (off-hours changes = suspicious)

	event.Classification = c.classifyType(event)
	event.Severity = c.scoreSeverity(event)
}

// classifyType determines whether drift was intentional, accidental, or malicious.
func (c *Classifier) classifyType(event *types.DriftEvent) types.DriftClassification {
	if event.Attribution != nil {
		switch event.Attribution.Source {
		case "console":
			return types.DriftClassificationIntentional
		case "sdk", "cli":
			return types.DriftClassificationIntentional
		case "unknown":
			return types.DriftClassificationUnknown
		}
	}
	return types.DriftClassificationUnknown
}

// scoreSeverity calculates the severity of a drift event based on multiple factors.
func (c *Classifier) scoreSeverity(event *types.DriftEvent) types.DriftSeverity {
	score := 0.0

	for _, diff := range event.Diff {
		switch {
		case isCriticalProperty(diff.Path):
			score += 40
		case isHighProperty(diff.Path):
			score += 25
		case isMediumProperty(diff.Path):
			score += 10
		default:
			score += 2
		}
	}

	switch {
	case score >= 80:
		return types.DriftSeverityCritical
	case score >= 50:
		return types.DriftSeverityHigh
	case score >= 25:
		return types.DriftSeverityMedium
	case score >= 10:
		return types.DriftSeverityLow
	default:
		return types.DriftSeverityInfo
	}
}

// isCriticalProperty returns true if the property path indicates a security-critical change.
func isCriticalProperty(path string) bool {
	criticalPatterns := []string{
		"public_access", "acl", "encryption", "kms_key",
		"iam_policy", "security_group", "network_acl",
		"password", "secret", "credentials", "auth",
	}
	for _, pattern := range criticalPatterns {
		if pathContains(path, pattern) {
			return true
		}
	}
	return false
}

// isHighProperty returns true if the property indicates a high-impact change.
func isHighProperty(path string) bool {
	highPatterns := []string{
		"instance_type", "machine_type", "size",
		"availability_zone", "region", "vpc",
		"subnet", "route", "firewall",
	}
	for _, pattern := range highPatterns {
		if pathContains(path, pattern) {
			return true
		}
	}
	return false
}

// isMediumProperty returns true if the property indicates a medium-impact change.
func isMediumProperty(path string) bool {
	mediumPatterns := []string{
		"tags", "labels", "description", "name",
		"backup", "logging", "monitoring",
	}
	for _, pattern := range mediumPatterns {
		if pathContains(path, pattern) {
			return true
		}
	}
	return false
}

func pathContains(path, pattern string) bool {
	return len(path) >= len(pattern) && (path == pattern ||
		path[len(path)-len(pattern):] == pattern ||
		containsSubstring(path, "."+pattern+".") ||
		containsSubstring(path, "."+pattern))
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub) >= 0
}

func searchSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// Attributor resolves drift events to specific identities using cloud audit trails.
type Attributor struct {
	logger *zap.Logger
}

// NewAttributor creates a new drift attributor.
func NewAttributor(logger *zap.Logger) *Attributor {
	return &Attributor{logger: logger}
}

// Attribute enriches a drift event with identity attribution data.
func (a *Attributor) Attribute(_ context.Context, event *types.DriftEvent) error {
	a.logger.Debug("attributing drift event",
		zap.String("resource_id", event.ResourceID),
	)
	// Implementation will:
	// 1. Query CloudTrail/Activity Log/Audit Log for recent changes
	// 2. Match change timestamps to drift detection time
	// 3. Resolve IAM principal to human identity
	// 4. Enrich with Slack/GitHub/Jira identity
	return nil
}

// Fingerprint generates a unique fingerprint for a drift event for deduplication.
func Fingerprint(event *types.DriftEvent) string {
	data := fmt.Sprintf("%s:%s:%s", event.ResourceID, event.Type, event.DetectedAt.Format(time.RFC3339))
	for _, diff := range event.Diff {
		data += fmt.Sprintf(":%s", diff.Path)
	}
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// Scheduler manages periodic and on-demand drift scan scheduling.
type Scheduler struct {
	interval time.Duration
	logger   *zap.Logger
	stop     chan struct{}
}

// NewScheduler creates a new drift scan scheduler.
func NewScheduler(interval time.Duration, logger *zap.Logger) *Scheduler {
	if interval == 0 {
		interval = 5 * time.Minute
	}
	return &Scheduler{
		interval: interval,
		logger:   logger,
		stop:     make(chan struct{}),
	}
}

// Start begins periodic scan execution.
func (s *Scheduler) Start(ctx context.Context, scanFn func(context.Context) error) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := scanFn(ctx); err != nil {
		s.logger.Error("initial drift scan failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stop:
			return nil
		case <-ticker.C:
			if err := scanFn(ctx); err != nil {
				s.logger.Error("scheduled drift scan failed", zap.Error(err))
			}
		}
	}
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() error {
	close(s.stop)
	return nil
}
