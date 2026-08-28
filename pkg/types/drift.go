package types

import (
	"time"
)

// DriftType represents the type of drift that occurred.
type DriftType string

// Drift types, describing how live infrastructure diverged from state.
const (
	DriftTypeAdded     DriftType = "added"
	DriftTypeModified  DriftType = "modified"
	DriftTypeDeleted   DriftType = "deleted"
	DriftTypeUnmanaged DriftType = "unmanaged"
)

// String implements the fmt.Stringer interface.
func (d DriftType) String() string { return string(d) }

// DriftClassification categorizes the drift intent.
type DriftClassification string

// Drift classifications, describing the likely intent behind a change.
const (
	DriftClassificationIntentional DriftClassification = "intentional"
	DriftClassificationAccidental  DriftClassification = "accidental"
	DriftClassificationMalicious   DriftClassification = "malicious"
	DriftClassificationUnknown     DriftClassification = "unknown"
)

// String implements the fmt.Stringer interface.
func (d DriftClassification) String() string { return string(d) }

// DriftSeverity denotes the criticality of the drift event.
type DriftSeverity string

// Drift severities, ordered from most to least urgent.
const (
	DriftSeverityCritical DriftSeverity = "critical"
	DriftSeverityHigh     DriftSeverity = "high"
	DriftSeverityMedium   DriftSeverity = "medium"
	DriftSeverityLow      DriftSeverity = "low"
	DriftSeverityInfo     DriftSeverity = "info"
)

// String implements the fmt.Stringer interface.
func (d DriftSeverity) String() string { return string(d) }

// Color returns a color code for UI representation of severity.
func (d DriftSeverity) Color() string {
	switch d {
	case DriftSeverityCritical:
		return "red"
	case DriftSeverityHigh:
		return "orange"
	case DriftSeverityMedium:
		return "yellow"
	case DriftSeverityLow:
		return "blue"
	case DriftSeverityInfo:
		return "gray"
	default:
		return "gray"
	}
}

// UserIdentity describes a user actor responsible for an action.
type UserIdentity struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	SlackID        string `json:"slack_id"`
	GitHubUsername string `json:"github_username"`
	Department     string `json:"department"`
}

// DriftAttribution identifies who or what caused the drift.
type DriftAttribution struct {
	Principal       string        `json:"principal"`
	Source          string        `json:"source"` // console, cli, sdk, unknown
	IPAddress       string        `json:"ip_address"`
	EventID         string        `json:"event_id"`
	Timestamp       time.Time     `json:"timestamp"`
	CloudTrailEvent string        `json:"cloudtrail_event"`
	Identity        *UserIdentity `json:"identity"`
}

// PropertyDiff represents a single property change in a resource.
type PropertyDiff struct {
	Path      string `json:"path"`
	Expected  any    `json:"expected"`
	Actual    any    `json:"actual"`
	Sensitive bool   `json:"sensitive"`
}

// DriftEvent details an infrastructure drift occurrence.
type DriftEvent struct {
	ID             string              `json:"id"`
	ResourceID     string              `json:"resource_id"`
	Resource       Resource            `json:"resource"`
	Type           DriftType           `json:"type"`
	Classification DriftClassification `json:"classification"`
	Severity       DriftSeverity       `json:"severity"`
	ExpectedState  map[string]any      `json:"expected_state"`
	ActualState    map[string]any      `json:"actual_state"`
	Diff           []PropertyDiff      `json:"diff"`
	Attribution    *DriftAttribution   `json:"attribution"`
	RiskScore      float64             `json:"risk_score"`
	BlastRadius    int                 `json:"blast_radius"`
	Fingerprint    string              `json:"fingerprint"`
	DetectedAt     time.Time           `json:"detected_at"`
	ResolvedAt     *time.Time          `json:"resolved_at,omitempty"`
	Resolution     string              `json:"resolution"`
}

// DriftSummary aggregates statistics on drift events.
type DriftSummary struct {
	Total        int                   `json:"total"`
	Critical     int                   `json:"critical"`
	High         int                   `json:"high"`
	Medium       int                   `json:"medium"`
	Low          int                   `json:"low"`
	Info         int                   `json:"info"`
	ByProvider   map[CloudProvider]int `json:"by_provider"`
	ByType       map[string]int        `json:"by_type"`
	TopResources []DriftEvent          `json:"top_resources"`
}

// DriftFilter is used to search for specific drift events.
type DriftFilter struct {
	ResourceIDs     []string              `json:"resource_ids,omitempty"`
	Types           []DriftType           `json:"types,omitempty"`
	Classifications []DriftClassification `json:"classifications,omitempty"`
	Severities      []DriftSeverity       `json:"severities,omitempty"`
	DetectedAfter   *time.Time            `json:"detected_after,omitempty"`
	DetectedBefore  *time.Time            `json:"detected_before,omitempty"`
	UnresolvedOnly  bool                  `json:"unresolved_only"`
	Limit           int                   `json:"limit,omitempty"`
	Offset          int                   `json:"offset,omitempty"`
}
