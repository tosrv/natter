package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func TestSessionRoundTrip(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u, _, err := s.UpsertGoogleUser(ctx, "sub-s", "s@example.com", "", false)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.CreateSession(ctx, u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.UserBySession(ctx, tok)
	if err != nil || got.ID != u.ID {
		t.Fatalf("got %v, %v", got.ID, err)
	}
	var n int
	rows, err := s.Pool().Query(ctx, `SELECT token_hash::text FROM sessions`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			t.Fatal(err)
		}
		n++
		if strings.Contains(h, tok) {
			t.Fatal("raw token stored")
		}
	}
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}
	if err := s.DeleteSession(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSessionExpired(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u, _, _ := s.UpsertGoogleUser(ctx, "sub-e", "e@example.com", "", false)
	tok, err := s.CreateSession(ctx, u.ID, -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSessionGoneAfterGuestDeleted(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, _, _ := s.UpsertGoogleUser(ctx, "sub-d", "d@example.com", "", true)
	g, _, err := s.CreateGuest(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.CreateSession(ctx, g.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
