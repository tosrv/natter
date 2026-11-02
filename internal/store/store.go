// Package store is the PostgreSQL persistence layer.
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrSlotsFull           = errors.New("user slots full")
	ErrUsernameTaken       = errors.New("username taken")
	ErrUsernameChangeLimit = errors.New("username change limit reached")
	ErrDuplicate           = errors.New("duplicate")
)

type Store struct {
	pool *pgxpool.Pool
	url  string
}

func New(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, url: url}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Pool exposes the underlying pool for queries.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// WithTx runs fn in a transaction, committing on nil and rolling back on error.
func (s *Store) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
