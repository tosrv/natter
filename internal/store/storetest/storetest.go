// Package storetest provides a Postgres-backed store for tests.
package storetest

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tosrv/pmess/internal/store"
)

var (
	once    sync.Once
	baseURL string
	initErr error
	counter atomic.Int64
)

func start() {
	ctx := context.Background()
	c, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		initErr = err
		return
	}
	baseURL, initErr = c.ConnectionString(ctx, "sslmode=disable")
	// Container is reaped by testcontainers' Ryuk when the test binary exits.
}

// New returns a Store on a fresh database with migrations applied.
func New(t *testing.T) *store.Store {
	t.Helper()
	once.Do(start)
	if initErr != nil {
		t.Fatalf("start postgres container: %v", initErr)
	}
	ctx := context.Background()
	name := fmt.Sprintf("t_%d_%d", time.Now().UnixNano(), counter.Add(1))
	admin, err := pgx.Connect(ctx, baseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	s, err := store.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}
