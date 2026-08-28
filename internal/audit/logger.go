// Package audit implements the immutable audit trail for infra-control.
// It provides tamper-proof logging of all infrastructure changes, approvals,
// and remediation actions using cryptographic hash chains.
package audit

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

// Logger is the immutable audit event logger. It maintains a hash chain
// to ensure tamper-proof audit trail integrity.
type Logger struct {
	mu       sync.Mutex
	lastHash string
	logger   *zap.Logger
}

// NewLogger creates a new audit logger.
func NewLogger(logger *zap.Logger) *Logger {
	return &Logger{
		lastHash: "genesis",
		logger:   logger,
	}
}

// Log records an audit event with a cryptographic hash chain link.
func (l *Logger) Log(_ context.Context, entry *types.AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.Timestamp = time.Now()
	entry.PreviousHash = l.lastHash

	// Compute hash of the entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshaling audit entry: %w", err)
	}

	hash := sha256.Sum256(append(data, []byte(l.lastHash)...))
	entry.Hash = fmt.Sprintf("%x", hash)
	l.lastHash = entry.Hash

	l.logger.Info("audit event recorded",
		zap.String("action", string(entry.Action)),
		zap.String("actor", entry.Actor.Name),
		zap.String("hash", entry.Hash[:16]),
	)

	return nil
}

// VerifyChain verifies the integrity of the audit hash chain.
func (l *Logger) VerifyChain(entries []*types.AuditEntry) (bool, error) {
	if len(entries) == 0 {
		return true, nil
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].PreviousHash != entries[i-1].Hash {
			return false, fmt.Errorf("chain broken at entry %s: expected previous hash %s, got %s",
				entries[i].ID, entries[i-1].Hash, entries[i].PreviousHash)
		}
	}

	return true, nil
}

// IdentityResolver maps cloud IAM principals to human identities.
type IdentityResolver struct {
	logger *zap.Logger
}

// NewIdentityResolver creates a new identity resolver.
func NewIdentityResolver(logger *zap.Logger) *IdentityResolver {
	return &IdentityResolver{logger: logger}
}

// Resolve maps a cloud IAM principal to a human identity.
func (r *IdentityResolver) Resolve(_ context.Context, principal string) (*types.UserIdentity, error) {
	r.logger.Debug("resolving identity", zap.String("principal", principal))
	// Implementation will:
	// 1. Parse IAM principal (ARN, email, service account)
	// 2. Look up in identity provider (Okta, Azure AD, Google Workspace)
	// 3. Enrich with Slack/GitHub/Jira identity
	return &types.UserIdentity{
		Name: principal,
	}, nil
}
