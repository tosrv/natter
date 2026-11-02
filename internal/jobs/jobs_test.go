package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/jobs"
	"github.com/tosrv/pmess/internal/store/storetest"
)

type fakeHub struct {
	mu  sync.Mutex
	ids []uuid.UUID
}

func (f *fakeHub) Disconnect(id uuid.UUID) { f.mu.Lock(); f.ids = append(f.ids, id); f.mu.Unlock() }

func (f *fakeHub) got() []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]uuid.UUID(nil), f.ids...)
}

func TestNextDailyRun(t *testing.T) {
	loc := time.UTC
	got := jobs.NextDailyRun(time.Date(2026, 11, 2, 2, 59, 0, 0, loc), 3)
	if want := time.Date(2026, 11, 2, 3, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	got = jobs.NextDailyRun(time.Date(2026, 11, 2, 3, 0, 0, 0, loc), 3)
	if want := time.Date(2026, 11, 3, 3, 0, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCleanupGuests(t *testing.T) {
	ctx := context.Background()
	s := storetest.New(t)
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.io", "", true)
	if err != nil {
		t.Fatal(err)
	}
	g, conv, err := s.CreateGuest(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	hub := &fakeHub{}
	n, err := jobs.CleanupGuests(ctx, s, hub, time.Now)
	if err != nil || n != 0 {
		t.Fatalf("early: n=%d err=%v", n, err)
	}
	n, err = jobs.CleanupGuests(ctx, s, hub, func() time.Time { return time.Now().Add(31 * time.Minute) })
	if ids := hub.got(); err != nil || n != 1 || len(ids) != 1 || ids[0] != g.ID {
		t.Fatalf("n=%d err=%v ids=%v", n, err, ids)
	}
	var used int
	if err := s.Pool().QueryRow(ctx, `SELECT used FROM user_slots WHERE id=1`).Scan(&used); err != nil {
		t.Fatal(err)
	}
	if used != 1 {
		t.Fatalf("slots used=%d want 1", used)
	}
}

func TestRetention(t *testing.T) {
	ctx := context.Background()
	s := storetest.New(t)
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.io", "", true)
	if err != nil {
		t.Fatal(err)
	}
	a, _, err := s.UpsertGoogleUser(ctx, "a", "a@x.io", "", false)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.GetOrCreateConversation(ctx, dev.ID, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, c, a.ID, uuid.New(), "yo"); err != nil {
		t.Fatal(err)
	}
	if d, err := jobs.RunRetention(ctx, s, time.Now); err != nil || d != 0 {
		t.Fatalf("0d: d=%d err=%v", d, err)
	}
	if d, err := jobs.RunRetention(ctx, s, func() time.Time { return time.Now().AddDate(0, 0, 20) }); err != nil || d != 0 {
		t.Fatalf("20d: d=%d err=%v", d, err)
	}
	d, err := jobs.RunRetention(ctx, s, func() time.Time { return time.Now().AddDate(0, 0, 32) })
	if err != nil || d != 1 {
		t.Fatalf("32d: d=%d err=%v", d, err)
	}
}
