package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func clearSetup(t *testing.T) (*store.Store, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 2)
	conv, err := s.GetOrCreateConversation(ctx, u[0], u[1])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, _, err := s.SendMessage(ctx, conv, u[1], uuid.New(), "old"); err != nil {
			t.Fatal(err)
		}
	}
	return s, u[0], u[1], conv
}

func hasConv(t *testing.T, s *store.Store, user, conv uuid.UUID) bool {
	t.Helper()
	cs, err := s.ListConversations(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if c.ID == conv {
			return true
		}
	}
	return false
}

func TestClearConversation(t *testing.T) {
	s, a, b, conv := clearSetup(t)
	ctx := context.Background()
	if err := s.ClearConversation(ctx, conv, a); err != nil {
		t.Fatal(err)
	}
	if hasConv(t, s, a, conv) || !hasConv(t, s, b, conv) {
		t.Fatal("list visibility wrong")
	}
	for _, args := range [][2]int64{{0, 0}, {0, 1 << 62}} {
		ms, err := s.ListMessages(ctx, conv, a, args[0], args[1], 50)
		if err != nil || len(ms) != 0 {
			t.Fatalf("A msgs=%d err=%v", len(ms), err)
		}
	}
	if ms, err := s.ListMessages(ctx, conv, a, 1, 0, 50); err != nil || len(ms) != 0 {
		t.Fatalf("A after_seq msgs=%d err=%v", len(ms), err)
	}
	if ms, err := s.ListMessages(ctx, conv, b, 0, 0, 50); err != nil || len(ms) != 3 {
		t.Fatalf("B msgs=%d err=%v", len(ms), err)
	}
}

func TestClearThenSendShowsNew(t *testing.T) {
	s, a, b, conv := clearSetup(t)
	ctx := context.Background()
	if err := s.ClearConversation(ctx, conv, a); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, conv, b, uuid.New(), "new"); err != nil {
		t.Fatal(err)
	}
	cs, err := s.ListConversations(ctx, a)
	if err != nil || len(cs) != 1 || cs[0].ID != conv {
		t.Fatalf("convs=%+v err=%v", cs, err)
	}
	if cs[0].Unread != 1 {
		t.Fatalf("unread=%d want 1", cs[0].Unread)
	}
	ms, err := s.ListMessages(ctx, conv, a, 0, 0, 50)
	if err != nil || len(ms) != 1 || ms[0].Body != "new" {
		t.Fatalf("msgs=%+v err=%v", ms, err)
	}
}

func TestClearTwice(t *testing.T) {
	s, a, _, conv := clearSetup(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := s.ClearConversation(ctx, conv, a); err != nil {
			t.Fatalf("clear %d: %v", i, err)
		}
	}
}

func TestClearNonMember(t *testing.T) {
	s, _, _, conv := clearSetup(t)
	out := mkUsersN(t, s)
	if err := s.ClearConversation(context.Background(), conv, out); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
	if err := s.ClearConversation(context.Background(), uuid.New(), out); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func mkUsersN(t *testing.T, s *store.Store) uuid.UUID {
	t.Helper()
	u, _, err := s.UpsertGoogleUser(context.Background(), "outsider", "outsider@x.io", "", false)
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestClearEmptyConversation(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUsers(t, s, 2)
	conv, err := s.GetOrCreateConversation(ctx, u[0], u[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ClearConversation(ctx, conv, u[0]); err != nil {
		t.Fatal(err)
	}
	if hasConv(t, s, u[0], conv) || !hasConv(t, s, u[1], conv) {
		t.Fatal("empty conv visibility wrong after clear")
	}
	if _, _, err := s.SendMessage(ctx, conv, u[1], uuid.New(), "back"); err != nil {
		t.Fatal(err)
	}
	if !hasConv(t, s, u[0], conv) {
		t.Fatal("new message did not restore conversation")
	}
}
