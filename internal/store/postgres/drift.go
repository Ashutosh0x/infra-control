package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// DriftStore provides data access for drift events.
type DriftStore struct {
	db     *DB
	logger *zap.Logger
}

// NewDriftStore creates a new DriftStore.
func NewDriftStore(db *DB, logger *zap.Logger) *DriftStore {
	return &DriftStore{db: db, logger: logger}
}

// Drift lifecycle states as persisted in the drift_events.status column.
const (
	driftStatusOpen     = "open"
	driftStatusResolved = "resolved"
	driftStatusIgnored  = "ignored"
)

const driftColumns = `id, resource_id, severity, status, details, detected_at, resolved_at`

// driftDetails is the JSONB payload carrying the parts of a DriftEvent that
// have no dedicated column. Keeping it in one struct means the write and read
// paths cannot fall out of sync.
type driftDetails struct {
	Type           types.DriftType           `json:"type"`
	Classification types.DriftClassification `json:"classification"`
	ExpectedState  map[string]any            `json:"expected_state,omitempty"`
	ActualState    map[string]any            `json:"actual_state,omitempty"`
	Diff           []types.PropertyDiff      `json:"diff,omitempty"`
	Attribution    *types.DriftAttribution   `json:"attribution,omitempty"`
	RiskScore      float64                   `json:"risk_score"`
	BlastRadius    int                       `json:"blast_radius"`
	Fingerprint    string                    `json:"fingerprint"`
	Resolution     string                    `json:"resolution,omitempty"`
}

// detailsOf projects the JSONB-backed fields out of an event.
func detailsOf(event *types.DriftEvent) driftDetails {
	return driftDetails{
		Type:           event.Type,
		Classification: event.Classification,
		ExpectedState:  event.ExpectedState,
		ActualState:    event.ActualState,
		Diff:           event.Diff,
		Attribution:    event.Attribution,
		RiskScore:      event.RiskScore,
		BlastRadius:    event.BlastRadius,
		Fingerprint:    event.Fingerprint,
		Resolution:     event.Resolution,
	}
}

// statusOf derives the persisted lifecycle state from the event itself, so the
// status column can never disagree with ResolvedAt.
func statusOf(event *types.DriftEvent) string {
	if event.ResolvedAt != nil {
		return driftStatusResolved
	}
	return driftStatusOpen
}

// Create inserts a new drift event.
func (s *DriftStore) Create(ctx context.Context, event *types.DriftEvent) error {
	details, err := json.Marshal(detailsOf(event))
	if err != nil {
		return fmt.Errorf("marshal drift details: %w", err)
	}

	const q = `INSERT INTO drift_events (` + driftColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := s.db.Pool.Exec(ctx, q,
		event.ID,
		event.ResourceID,
		string(event.Severity),
		statusOf(event),
		details,
		event.DetectedAt,
		event.ResolvedAt,
	); err != nil {
		return fmt.Errorf("create drift event: %w", err)
	}
	return nil
}

// Get returns a single drift event by ID.
func (s *DriftStore) Get(ctx context.Context, id string) (*types.DriftEvent, error) {
	const q = `SELECT ` + driftColumns + ` FROM drift_events WHERE id = $1`
	event, err := scanDriftEvent(s.db.Pool.QueryRow(ctx, q, id))
	if err != nil {
		return nil, fmt.Errorf("get drift event %s: %w", id, err)
	}
	return event, nil
}

// List returns drift events matching the filter, plus the total match count
// ignoring limit and offset.
func (s *DriftStore) List(ctx context.Context, filter types.DriftFilter) ([]*types.DriftEvent, int, error) {
	where, args := driftFilterClause(filter)

	var total int
	if err := s.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM drift_events`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count drift events: %w", err)
	}

	q := `SELECT ` + driftColumns + ` FROM drift_events` + where + ` ORDER BY detected_at DESC`
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	events, err := s.queryEvents(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

// Update rewrites the mutable fields of an existing drift event.
func (s *DriftStore) Update(ctx context.Context, event *types.DriftEvent) error {
	details, err := json.Marshal(detailsOf(event))
	if err != nil {
		return fmt.Errorf("marshal drift details: %w", err)
	}

	const q = `UPDATE drift_events SET severity = $2, status = $3, details = $4, resolved_at = $5 WHERE id = $1`
	tag, err := s.db.Pool.Exec(ctx, q, event.ID, string(event.Severity), statusOf(event), details, event.ResolvedAt)
	if err != nil {
		return fmt.Errorf("update drift event %s: %w", event.ID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update drift event %s: no such event", event.ID)
	}
	return nil
}

// GetSummary aggregates open drift events by severity in a single round trip.
func (s *DriftStore) GetSummary(ctx context.Context) (*types.DriftSummary, error) {
	const q = `SELECT severity, COUNT(*) FROM drift_events WHERE status = $1 GROUP BY severity`
	rows, err := s.db.Pool.Query(ctx, q, driftStatusOpen)
	if err != nil {
		return nil, fmt.Errorf("summarise drift events: %w", err)
	}
	defer rows.Close()

	summary := &types.DriftSummary{
		ByProvider: map[types.CloudProvider]int{},
		ByType:     map[string]int{},
	}
	for rows.Next() {
		var severity string
		var count int
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, fmt.Errorf("scan drift summary row: %w", err)
		}
		summary.Total += count
		switch types.DriftSeverity(severity) {
		case types.DriftSeverityCritical:
			summary.Critical = count
		case types.DriftSeverityHigh:
			summary.High = count
		case types.DriftSeverityMedium:
			summary.Medium = count
		case types.DriftSeverityLow:
			summary.Low = count
		case types.DriftSeverityInfo:
			summary.Info = count
		}
	}
	return summary, rows.Err()
}

// GetByResourceID returns the drift history for one resource, newest first.
func (s *DriftStore) GetByResourceID(ctx context.Context, resourceID string) ([]*types.DriftEvent, error) {
	const q = `SELECT ` + driftColumns + ` FROM drift_events WHERE resource_id = $1 ORDER BY detected_at DESC`
	return s.queryEvents(ctx, q, resourceID)
}

// Resolve marks a drift event resolved and records the stated reason.
func (s *DriftStore) Resolve(ctx context.Context, id string, resolution string) error {
	return s.setTerminalStatus(ctx, id, driftStatusResolved, resolution)
}

// Ignore suppresses a drift event without claiming it was fixed.
func (s *DriftStore) Ignore(ctx context.Context, id string, reason string) error {
	return s.setTerminalStatus(ctx, id, driftStatusIgnored, reason)
}

// setTerminalStatus closes out an event, merging the reason into the details
// JSONB so the original diff is preserved alongside the resolution.
func (s *DriftStore) setTerminalStatus(ctx context.Context, id, status, reason string) error {
	reasonJSON, err := json.Marshal(map[string]string{"resolution": reason})
	if err != nil {
		return fmt.Errorf("marshal resolution: %w", err)
	}

	const q = `UPDATE drift_events SET status = $2, resolved_at = $3, details = details || $4 WHERE id = $1`
	tag, err := s.db.Pool.Exec(ctx, q, id, status, time.Now().UTC(), reasonJSON)
	if err != nil {
		return fmt.Errorf("set drift event %s to %s: %w", id, status, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set drift event %s to %s: no such event", id, status)
	}
	return nil
}

// GetUnresolved returns every drift event still awaiting a decision.
func (s *DriftStore) GetUnresolved(ctx context.Context) ([]*types.DriftEvent, error) {
	const q = `SELECT ` + driftColumns + ` FROM drift_events WHERE status = $1 ORDER BY detected_at DESC`
	return s.queryEvents(ctx, q, driftStatusOpen)
}

// queryEvents runs a driftColumns projection and scans every row.
func (s *DriftStore) queryEvents(ctx context.Context, q string, args ...any) ([]*types.DriftEvent, error) {
	rows, err := s.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query drift events: %w", err)
	}
	defer rows.Close()

	var events []*types.DriftEvent
	for rows.Next() {
		event, err := scanDriftEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan drift event: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// driftFilterClause builds a parameterised WHERE clause from the filter.
func driftFilterClause(f types.DriftFilter) (string, []any) {
	var conds []string
	var args []any

	if len(f.ResourceIDs) > 0 {
		args = append(args, f.ResourceIDs)
		conds = append(conds, fmt.Sprintf("resource_id = ANY($%d)", len(args)))
	}
	if len(f.Severities) > 0 {
		sevs := make([]string, len(f.Severities))
		for i, s := range f.Severities {
			sevs[i] = string(s)
		}
		args = append(args, sevs)
		conds = append(conds, fmt.Sprintf("severity = ANY($%d)", len(args)))
	}
	if f.DetectedAfter != nil {
		args = append(args, *f.DetectedAfter)
		conds = append(conds, fmt.Sprintf("detected_at >= $%d", len(args)))
	}
	if f.DetectedBefore != nil {
		args = append(args, *f.DetectedBefore)
		conds = append(conds, fmt.Sprintf("detected_at <= $%d", len(args)))
	}
	if f.UnresolvedOnly {
		args = append(args, driftStatusOpen)
		conds = append(conds, fmt.Sprintf("status = $%d", len(args)))
	}

	// Drift type and classification live inside the details JSONB rather than in
	// dedicated columns, so they are filtered with a JSON path predicate.
	if len(f.Types) > 0 {
		vals := make([]string, len(f.Types))
		for i, t := range f.Types {
			vals[i] = string(t)
		}
		args = append(args, vals)
		conds = append(conds, fmt.Sprintf("details->>'type' = ANY($%d)", len(args)))
	}
	if len(f.Classifications) > 0 {
		vals := make([]string, len(f.Classifications))
		for i, c := range f.Classifications {
			vals[i] = string(c)
		}
		args = append(args, vals)
		conds = append(conds, fmt.Sprintf("details->>'classification' = ANY($%d)", len(args)))
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// scanDriftEvent maps one row of driftColumns onto a DriftEvent.
func scanDriftEvent(row rowScanner) (*types.DriftEvent, error) {
	var (
		event    types.DriftEvent
		severity string
		status   string
		details  []byte
	)
	if err := row.Scan(
		&event.ID,
		&event.ResourceID,
		&severity,
		&status,
		&details,
		&event.DetectedAt,
		&event.ResolvedAt,
	); err != nil {
		return nil, err
	}

	event.Severity = types.DriftSeverity(severity)
	if len(details) > 0 {
		var d driftDetails
		if err := json.Unmarshal(details, &d); err != nil {
			return nil, fmt.Errorf("unmarshal drift details: %w", err)
		}
		event.Type = d.Type
		event.Classification = d.Classification
		event.ExpectedState = d.ExpectedState
		event.ActualState = d.ActualState
		event.Diff = d.Diff
		event.Attribution = d.Attribution
		event.RiskScore = d.RiskScore
		event.BlastRadius = d.BlastRadius
		event.Fingerprint = d.Fingerprint
		event.Resolution = d.Resolution
	}
	return &event, nil
}
