package types

import (
	"context"
	"time"
)

// EventType specifies the nature of an internal event.
type EventType string

// Event types published on the internal bus.
const (
	EventTypeResourceDiscovered  EventType = "ResourceDiscovered"
	EventTypeResourceUpdated     EventType = "ResourceUpdated"
	EventTypeResourceDeleted     EventType = "ResourceDeleted"
	EventTypeDriftDetected       EventType = "DriftDetected"
	EventTypeDriftResolved       EventType = "DriftResolved"
	EventTypePolicyViolation     EventType = "PolicyViolation"
	EventTypeRemediationProposed EventType = "RemediationProposed"
	EventTypeRemediationApproved EventType = "RemediationApproved"
	EventTypeRemediationApplied  EventType = "RemediationApplied"
	EventTypeRemediationFailed   EventType = "RemediationFailed"
	EventTypeRollbackInitiated   EventType = "RollbackInitiated"
	EventTypeRollbackCompleted   EventType = "RollbackCompleted"
	EventTypeApprovalRequested   EventType = "ApprovalRequested"
	EventTypeApprovalGranted     EventType = "ApprovalGranted"
	EventTypeApprovalDenied      EventType = "ApprovalDenied"
)

// String implements fmt.Stringer
func (e EventType) String() string { return string(e) }

// Event is the generic envelope for system occurrences.
type Event struct {
	ID            string            `json:"id"`
	Type          EventType         `json:"type"`
	Source        string            `json:"source"`
	Subject       string            `json:"subject"`
	Data          any               `json:"data"` // serialized specific event payload
	Metadata      map[string]string `json:"metadata"`
	Timestamp     time.Time         `json:"timestamp"`
	CorrelationID string            `json:"correlation_id"`
}

// EventHandler processes an Event.
type EventHandler func(ctx context.Context, e Event) error

// EventFilter is used to filter events in queries or subscriptions.
type EventFilter struct {
	Types    []EventType `json:"types,omitempty"`
	Sources  []string    `json:"sources,omitempty"`
	Subjects []string    `json:"subjects,omitempty"`
	Since    *time.Time  `json:"since,omitempty"`
}
