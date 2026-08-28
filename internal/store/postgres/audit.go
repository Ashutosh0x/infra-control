package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// AuditStore provides append-only data access for audit entries.
//
// The audit_entries table is protected by a database trigger that rejects all
// UPDATE and DELETE statements, so this store deliberately exposes no mutation
// methods beyond Append.
type AuditStore struct {
	db     *DB
	logger *zap.Logger
}

// NewAuditStore creates a new AuditStore.
func NewAuditStore(db *DB, logger *zap.Logger) *AuditStore {
	return &AuditStore{db: db, logger: logger}
}

// auditColumns is the canonical projection used by every read path so that
// scanAuditEntry stays in sync with the queries.
const auditColumns = `id, action, actor, resource_id, correlation_id, details, timestamp, hash_chain`

// Append writes a single immutable audit entry.
func (s *AuditStore) Append(ctx context.Context, entry *types.AuditEntry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	// resource_id is a nullable UUID column; an empty string is not a valid UUID.
	var resourceID any
	if entry.Resource.ID != "" {
		resourceID = entry.Resource.ID
	}

	const q = `INSERT INTO audit_entries (` + auditColumns + `) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := s.db.Pool.Exec(ctx, q,
		entry.ID,
		string(entry.Action),
		entry.Actor.Email,
		resourceID,
		entry.CorrelationID,
		details,
		entry.Timestamp,
		entry.Hash,
	); err != nil {
		return fmt.Errorf("append audit entry: %w", err)
	}
	return nil
}

// Get returns a single audit entry by ID.
func (s *AuditStore) Get(ctx context.Context, id string) (*types.AuditEntry, error) {
	const q = `SELECT ` + auditColumns + ` FROM audit_entries WHERE id = $1`
	row := s.db.Pool.QueryRow(ctx, q, id)
	entry, err := scanAuditEntry(row)
	if err != nil {
		return nil, fmt.Errorf("get audit entry %s: %w", id, err)
	}
	return entry, nil
}

// List returns audit entries matching the filter, plus the total match count
// ignoring limit/offset.
func (s *AuditStore) List(ctx context.Context, filter types.AuditFilter) ([]*types.AuditEntry, int, error) {
	where, args := auditFilterClause(filter)

	var total int
	countQ := `SELECT COUNT(*) FROM audit_entries` + where
	if err := s.db.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit entries: %w", err)
	}

	q := `SELECT ` + auditColumns + ` FROM audit_entries` + where + ` ORDER BY timestamp DESC`
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := s.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	var entries []*types.AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

// GetByCorrelation returns every entry sharing a correlation ID, oldest first,
// which reconstructs the full causal chain of one logical operation.
func (s *AuditStore) GetByCorrelation(ctx context.Context, correlationID string) ([]*types.AuditEntry, error) {
	const q = `SELECT ` + auditColumns + ` FROM audit_entries WHERE correlation_id = $1 ORDER BY timestamp ASC`
	rows, err := s.db.Pool.Query(ctx, q, correlationID)
	if err != nil {
		return nil, fmt.Errorf("get audit entries by correlation: %w", err)
	}
	defer rows.Close()

	var entries []*types.AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Export serialises filtered audit entries for offline retention.
func (s *AuditStore) Export(ctx context.Context, filter types.AuditFilter, format string) ([]byte, error) {
	entries, _, err := s.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "json", "":
		return json.MarshalIndent(entries, "", "  ")
	case "jsonl", "ndjson":
		var buf strings.Builder
		for _, e := range entries {
			line, err := json.Marshal(e)
			if err != nil {
				return nil, fmt.Errorf("marshal audit entry %s: %w", e.ID, err)
			}
			buf.Write(line)
			buf.WriteByte('\n')
		}
		return []byte(buf.String()), nil
	default:
		return nil, fmt.Errorf("unsupported audit export format %q (want json or jsonl)", format)
	}
}

// auditFilterClause builds a parameterised WHERE clause from the filter.
func auditFilterClause(f types.AuditFilter) (string, []any) {
	var conds []string
	var args []any

	if len(f.Actions) > 0 {
		actions := make([]string, len(f.Actions))
		for i, a := range f.Actions {
			actions[i] = string(a)
		}
		args = append(args, actions)
		conds = append(conds, fmt.Sprintf("action = ANY($%d)", len(args)))
	}
	if f.ActorEmail != "" {
		args = append(args, f.ActorEmail)
		conds = append(conds, fmt.Sprintf("actor = $%d", len(args)))
	}
	if f.ResourceID != "" {
		args = append(args, f.ResourceID)
		conds = append(conds, fmt.Sprintf("resource_id = $%d", len(args)))
	}
	if f.StartTime != nil {
		args = append(args, *f.StartTime)
		conds = append(conds, fmt.Sprintf("timestamp >= $%d", len(args)))
	}
	if f.EndTime != nil {
		args = append(args, *f.EndTime)
		conds = append(conds, fmt.Sprintf("timestamp <= $%d", len(args)))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// scanAuditEntry maps one row of auditColumns onto an AuditEntry.
func scanAuditEntry(row rowScanner) (*types.AuditEntry, error) {
	var (
		entry      types.AuditEntry
		action     string
		actor      string
		resourceID *string
		details    []byte
	)
	if err := row.Scan(
		&entry.ID,
		&action,
		&actor,
		&resourceID,
		&entry.CorrelationID,
		&details,
		&entry.Timestamp,
		&entry.Hash,
	); err != nil {
		return nil, err
	}

	entry.Action = types.AuditAction(action)
	entry.Actor = types.UserIdentity{Email: actor}
	if resourceID != nil {
		entry.Resource.ID = *resourceID
	}
	if len(details) > 0 {
		if err := json.Unmarshal(details, &entry.Details); err != nil {
			return nil, fmt.Errorf("unmarshal audit details: %w", err)
		}
	}
	return &entry, nil
}
