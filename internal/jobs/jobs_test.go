package jobs_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/jobs"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

type fakeHub struct {
	mu       sync.Mutex
	ids      []uuid.UUID
	outcomes map[uuid.UUID]store.EndGuestOutcome
	convs    map[uuid.UUID][]store.EndedConversation
}

func (f *fakeHub) NotifyGuestEnded(o store.EndGuestOutcome, c []store.EndedConversation, id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.outcomes == nil {
		f.outcomes = map[uuid.UUID]store.EndGuestOutcome{}
		f.convs = map[uuid.UUID][]store.EndedConversation{}
	}
	f.ids = append(f.ids, id)
	f.outcomes[id] = o
	f.convs[id] = c
}

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
	g, conv, err := s.CreateGuest(ctx, dev.ID, "Guest Test")
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

func TestCleanupGuestsDeactivates(t *testing.T) {
	ctx := context.Background()
	s := storetest.New(t)
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.io", "", true)
	if err != nil {
		t.Fatal(err)
	}
	talker, conv, err := s.CreateGuest(ctx, dev.ID, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, conv, talker.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	silent, sconv, err := s.CreateGuest(ctx, dev.ID, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	hub := &fakeHub{}
	n, err := jobs.CleanupGuests(ctx, s, hub, func() time.Time { return time.Now().Add(31 * time.Minute) })
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if hub.outcomes[talker.ID] != store.GuestDeactivated || hub.outcomes[silent.ID] != store.GuestDeleted {
		t.Fatalf("outcomes %v", hub.outcomes)
	}
	if c := hub.convs[talker.ID]; len(c) != 1 || c[0].ConversationID != conv || c[0].PeerID != dev.ID {
		t.Fatalf("talker convs %v", c)
	}
	if c := hub.convs[silent.ID]; len(c) != 1 || c[0].ConversationID != sconv {
		t.Fatalf("silent convs %v", c)
	}
	u, err := s.GetUser(ctx, talker.ID)
	if err != nil || u.DeactivatedAt == nil {
		t.Fatalf("talker %+v %v", u, err)
	}
	if _, err := s.GetUser(ctx, silent.ID); err == nil {
		t.Fatal("silent guest still exists")
	}
}
