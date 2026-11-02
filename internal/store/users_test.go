package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
)

func setUsed(t *testing.T, s *store.Store, n int) {
	t.Helper()
	if _, err := s.Pool().Exec(context.Background(), `UPDATE user_slots SET used=$1`, n); err != nil {
		t.Fatal(err)
	}
}

func usedSlots(t *testing.T, s *store.Store) int {
	t.Helper()
	var n int
	if err := s.Pool().QueryRow(context.Background(), `SELECT used FROM user_slots`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSlotRace(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	setUsed(t, s, 299)
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, sub := range []string{"sub-a", "sub-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, errs[i] = s.UpsertGoogleUser(ctx, sub, sub+"@x.com", "", false)
		}()
	}
	close(start)
	wg.Wait()
	ok, full := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, store.ErrSlotsFull):
			full++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 || full != 1 {
		t.Fatalf("ok=%d full=%d, want 1/1", ok, full)
	}
	if n := usedSlots(t, s); n != 300 {
		t.Fatalf("used=%d, want 300", n)
	}
}

func TestExistingUserLoginWhenFull(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u1, created, err := s.UpsertGoogleUser(ctx, "sub-1", "a@x.com", "", false)
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	setUsed(t, s, 300)
	u2, created, err := s.UpsertGoogleUser(ctx, "sub-1", "b@x.com", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if created || u2.ID != u1.ID {
		t.Fatalf("created=%v id match=%v", created, u2.ID == u1.ID)
	}
	if u2.Email == nil || *u2.Email != "b@x.com" || !u2.IsDeveloper {
		t.Fatalf("not updated: %+v", u2)
	}
	if n := usedSlots(t, s); n != 300 {
		t.Fatalf("used=%d, want 300", n)
	}
}

func TestDeleteGuestFreesSlot(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.com", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.GetDeveloper(ctx); err != nil || got.ID != dev.ID {
		t.Fatalf("GetDeveloper: %v %v", got.ID, err)
	}
	g, convID, err := s.CreateGuest(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.Kind != "guest" || g.Username == nil || len(*g.Username) != len("guest-")+4 {
		t.Fatalf("bad guest: %+v", g)
	}
	if n := usedSlots(t, s); n != 2 { // developer + guest
		t.Fatalf("used=%d, want 2", n)
	}
	var members int
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM conversation_members WHERE conversation_id=$1`, convID).Scan(&members)
	if members != 2 {
		t.Fatalf("members=%d", members)
	}
	if _, err := s.Pool().Exec(ctx,
		`INSERT INTO messages (conversation_id, seq, sender_id, client_msg_id, body) VALUES ($1,1,$2,$3,'hi')`,
		convID, g.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}

	// Deleting a Google user is a no-op.
	if err := s.DeleteGuest(ctx, dev.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetUser(ctx, dev.ID); err != nil {
		t.Fatalf("google user removed: %v", err)
	}
	if n := usedSlots(t, s); n != 2 {
		t.Fatalf("used=%d after google delete, want 2", n)
	}

	if err := s.DeleteGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if n := usedSlots(t, s); n != 1 {
		t.Fatalf("used=%d, want 1", n)
	}
	var convs, msgs int
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM conversations WHERE id=$1`, convID).Scan(&convs)
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM messages WHERE conversation_id=$1`, convID).Scan(&msgs)
	if convs != 0 || msgs != 0 {
		t.Fatalf("convs=%d msgs=%d", convs, msgs)
	}
	if _, err := s.GetUser(ctx, g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestGetDeveloperNotFound(t *testing.T) {
	s := storetest.New(t)
	if _, err := s.GetDeveloper(context.Background()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}
