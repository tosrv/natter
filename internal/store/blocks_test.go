package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func named(t *testing.T, s *store.Store, sub string) store.User {
	t.Helper()
	u := mkUser(t, s, sub)
	u, err := s.SetUsername(context.Background(), u.ID, sub)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestBlockIdempotentAndUnblock(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b := named(t, s, "alice"), named(t, s, "bob")
	for i := 0; i < 2; i++ {
		if err := s.Block(ctx, a.ID, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	l, err := s.ListBlocks(ctx, a.ID)
	if err != nil || len(l) != 1 || l[0].ID != b.ID || l[0].Username != "bob" || l[0].BlockedAt.IsZero() {
		t.Fatalf("list=%+v err=%v", l, err)
	}
	for _, p := range [][2]uuid.UUID{{a.ID, b.ID}, {b.ID, a.ID}} {
		if ok, err := s.BlockRelated(ctx, p[0], p[1]); err != nil || !ok {
			t.Fatalf("related=%v err=%v", ok, err)
		}
	}
	if err := s.Block(ctx, a.ID, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing target: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Unblock(ctx, a.ID, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := s.BlockRelated(ctx, a.ID, b.ID); ok {
		t.Fatal("still related")
	}
	if l, _ := s.ListBlocks(ctx, a.ID); len(l) != 0 {
		t.Fatalf("list=%+v", l)
	}
}

func TestBlockSelfRejectedByCheck(t *testing.T) {
	s := storetest.New(t)
	a := named(t, s, "alice")
	if err := s.Block(context.Background(), a.ID, a.ID); err == nil {
		t.Fatal("self block accepted")
	}
}

func TestBlocksCascadeOnDelete(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b := named(t, s, "alice"), named(t, s, "bob")
	if err := s.Block(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteAccount(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if l, err := s.ListBlocks(ctx, a.ID); err != nil || len(l) != 0 {
		t.Fatalf("list=%+v err=%v", l, err)
	}
}

func TestSearchExcludesBlockRelated(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b := named(t, s, "alice"), named(t, s, "albert")
	if err := s.Block(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]uuid.UUID{{a.ID, b.ID}, {b.ID, a.ID}} {
		res, err := s.SearchUsers(ctx, "al", p[0], 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res {
			if r.ID == p[1] {
				t.Fatalf("search by %v returned blocked %v", p[0], p[1])
			}
		}
	}
	if err := s.Unblock(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if res, _ := s.SearchUsers(ctx, "al", a.ID, 10); len(res) != 1 {
		t.Fatalf("after unblock: %+v", res)
	}
}

func TestSummaryBlockedByMe(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b := named(t, s, "alice"), named(t, s, "bob")
	if _, err := s.GetOrCreateConversation(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Block(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	ca, _ := s.ListConversations(ctx, a.ID)
	cb, _ := s.ListConversations(ctx, b.ID)
	if len(ca) != 1 || !ca[0].BlockedByMe || len(cb) != 1 || cb[0].BlockedByMe {
		t.Fatalf("a=%+v b=%+v", ca, cb)
	}
}
