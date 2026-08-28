package postgres

import (
	"context"
	"fmt"

	"github.com/ashutosh0x/infra-control/pkg/types"
	"go.uber.org/zap"
)

// PolicyStore provides data access for policies.
type PolicyStore struct {
	db     *DB
	logger *zap.Logger
}

// NewPolicyStore creates a new PolicyStore.
func NewPolicyStore(db *DB, logger *zap.Logger) *PolicyStore {
	return &PolicyStore{
		db:     db,
		logger: logger,
	}
}

func (s *PolicyStore) Create(ctx context.Context, policy *types.Policy) error {
	return fmt.Errorf("not implemented")
}

func (s *PolicyStore) Get(ctx context.Context, id string) (*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PolicyStore) List(ctx context.Context) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PolicyStore) Update(ctx context.Context, policy *types.Policy) error {
	return fmt.Errorf("not implemented")
}

func (s *PolicyStore) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("not implemented")
}

func (s *PolicyStore) GetByType(ctx context.Context, policyType types.PolicyType) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *PolicyStore) GetEnabled(ctx context.Context) ([]*types.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}
