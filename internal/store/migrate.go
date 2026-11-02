package store

import (
	"context"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/tosrv/pmess/migrations"
	"strings"
)

// Migrate applies all pending up-migrations.
func (s *Store) Migrate(ctx context.Context) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}
	url := strings.Replace(s.url, "postgres://", "pgx5://", 1)
	url = strings.Replace(url, "postgresql://", "pgx5://", 1)
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}
