// Package types defines the shared domain types used across infra-control.
package types

import (
	"time"
)

// AuditAction defines the operation performed.
type AuditAction string

// Audit actions, one per kind of operation the system records.
const (
	AuditActionCreate  AuditAction = "create"
	AuditActionRead    AuditAction = "read"
	AuditActionUpdate  AuditAction = "update"
	AuditActionDelete  AuditAction = "delete"
	AuditActionExecute AuditAction = "execute"
	AuditActionApprove AuditAction = "approve"
	AuditActionDeny    AuditAction = "deny"
	AuditActionLogin   AuditAction = "login"
	AuditActionLogout  AuditAction = "logout"
)

// String implements fmt.Stringer
func (a AuditAction) String() string { return string(a) }

// AuditResourceInfo encapsulates basic resource identification.
type AuditResourceInfo struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AuditEntry is an immutable log of an action taken in the system.
type AuditEntry struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Action        AuditAction       `json:"action"`
	Actor         UserIdentity      `json:"actor"`
	Resource      AuditResourceInfo `json:"resource"`
	Details       map[string]any    `json:"details"`
	CorrelationID string            `json:"correlation_id"`
	IPAddress     string            `json:"ip_address"`
	UserAgent     string            `json:"user_agent"`
	Result        string            `json:"result"` // e.g., "success", "failure"
	PreviousState any               `json:"previous_state,omitempty"`
	NewState      any               `json:"new_state,omitempty"`
	Hash          string            `json:"hash"`
	PreviousHash  string            `json:"previous_hash"`
}

// AuditFilter criteria for searching audit logs.
type AuditFilter struct {
	Actions    []AuditAction `json:"actions,omitempty"`
	ActorEmail string        `json:"actor_email,omitempty"`
	ResourceID string        `json:"resource_id,omitempty"`
	StartTime  *time.Time    `json:"start_time,omitempty"`
	EndTime    *time.Time    `json:"end_time,omitempty"`
	Result     string        `json:"result,omitempty"`
	Limit      int           `json:"limit,omitempty"`
	Offset     int           `json:"offset,omitempty"`
}
