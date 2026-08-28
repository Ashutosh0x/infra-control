package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DB wraps a PostgreSQL connection pool.
type DB struct {
	Pool   *pgxpool.Pool
	logger *zap.Logger
}

// NewDB creates a new database connection pool.
func NewDB(ctx context.Context, dsn string, logger *zap.Logger) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{
		Pool:   pool,
		logger: logger,
	}, nil
}

// Close closes the database connection pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// Health checks database connectivity.
func (db *DB) Health(ctx context.Context) error {
	if err := db.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	return nil
}

// Migrate runs database migrations.
func (db *DB) Migrate(ctx context.Context, migrationsDir string) error {
	db.logger.Info("running migrations", zap.String("dir", migrationsDir))

	// A real implementation would use a robust migration library like golang-migrate
	// For simplicity, this loops over .up.sql files in lexical order.
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
			if err != nil {
				return fmt.Errorf("failed to read migration %s: %w", f.Name(), err)
			}
			_, err = db.Pool.Exec(ctx, string(content))
			if err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", f.Name(), err)
			}
			db.logger.Info("applied migration", zap.String("file", f.Name()))
		}
	}

	return nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows, letting a single scan
// helper serve QueryRow and Query call sites.
type rowScanner interface {
	Scan(dest ...any) error
}
