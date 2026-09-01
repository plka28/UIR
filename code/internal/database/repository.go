package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const batchSize = 1000

var (
	// ErrNotFound is returned when a requested database entity does not exist.
	ErrNotFound = errors.New("database entity not found")
	// ErrInvalidRoute identifies a route whose endpoint types or values cannot
	// be represented by the database schema.
	ErrInvalidRoute = errors.New("invalid route cost")
	// ErrInvalidAssignment identifies an unsupported assignment destination.
	ErrInvalidAssignment = errors.New("invalid source assignment")
)

// Repository provides schema-qualified access to the waste-routing tables.
// It owns the supplied pool and closes it when Close is called.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository wraps an existing pgx pool.
func NewRepository(pool *pgxpool.Pool) (*Repository, error) {
	if pool == nil {
		return nil, fmt.Errorf("create repository: PostgreSQL pool is nil")
	}
	return &Repository{pool: pool}, nil
}

// Ping verifies that the repository can reach PostgreSQL.
func (repository *Repository) Ping(ctx context.Context) error {
	if err := repository.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return nil
}

// Close releases all connections owned by the repository.
func (repository *Repository) Close() {
	if repository == nil || repository.pool == nil {
		return
	}
	repository.pool.Close()
}

type batchStatement struct {
	query string
	args  []any
}

type batchStatementProvider func(index int) (batchStatement, error)

func (repository *Repository) executeBatchTransaction(
	ctx context.Context,
	operation string,
	itemCount int,
	statementAt batchStatementProvider,
) error {
	if itemCount == 0 {
		return nil
	}

	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("%s: begin transaction: %w", operation, err)
	}

	rollback := func() {
		rollbackContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = transaction.Rollback(rollbackContext)
	}

	for offset := 0; offset < itemCount; offset += batchSize {
		end := min(offset+batchSize, itemCount)
		batch := &pgx.Batch{}
		for index := offset; index < end; index++ {
			statement, err := statementAt(index)
			if err != nil {
				rollback()
				return fmt.Errorf("%s: item %d: %w", operation, index, err)
			}
			batch.Queue(statement.query, statement.args...)
		}

		results := transaction.SendBatch(ctx, batch)
		for index := offset; index < end; index++ {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				rollback()
				return fmt.Errorf("%s: item %d: %w", operation, index, err)
			}
		}
		if err := results.Close(); err != nil {
			rollback()
			return fmt.Errorf("%s: close batch: %w", operation, err)
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		rollback()
		return fmt.Errorf("%s: commit transaction: %w", operation, err)
	}
	return nil
}

func mapRowError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func requireRowsAffected(operation string, affected int64) error {
	if affected == 0 {
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	}
	return nil
}
