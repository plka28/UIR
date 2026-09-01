package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a PostgreSQL connection pool, verifies connectivity, and
// returns a repository that owns the pool.
func Open(ctx context.Context, config Config) (*Repository, error) {
	connectionString, err := config.connectionString()
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: invalid configuration: %w", err)
	}

	poolConfig, err := pgxpool.ParseConfig(connectionString)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: parse pool configuration: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL: create pool: %w", err)
	}

	repository, err := NewRepository(pool)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("open PostgreSQL: %w", err)
	}
	if err := repository.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repository, nil
}
