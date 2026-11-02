package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
)

func TestExpiredGuests(t *testing.T) {
	ctx := context.Background()
	s := storetest.New(t)
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.io", true)
	if err != nil {
		t.Fatal(err)
	}
	mk := func() (store.User, uuid.UUID) {
		g, conv, err := s.CreateGuest(ctx, dev.ID)
		if err != nil {
			t.Fatal(err)
		}
		return g, conv
	}
	age := func(g store.User, conv uuid.UUID, d time.Duration, msg bool) {
		q := `UPDATE users SET created_at = now() - $1::interval WHERE id=$2`
		if _, err := s.Pool().Exec(ctx, q, d.String(), g.ID); err != nil {
			t.Fatal(err)
		}
		if msg {
			if _, err := s.Pool().Exec(ctx, `UPDATE conversations SET last_activity_at = now() - $1::interval WHERE id=$2`, d.String(), conv); err != nil {
				t.Fatal(err)
			}
		}
	}
	old, oc := mk()
	age(old, oc, 31*time.Minute, false)
	active, ac := mk()
	age(active, ac, 40*time.Minute, false)
	if _, err := s.Pool().Exec(ctx, `UPDATE conversations SET last_activity_at = now() - interval '10 minutes' WHERE id=$1`, ac); err != nil {
		t.Fatal(err)
	}
	// developer replied recently: guest clock reset
	replied, rc := mk()
	age(replied, rc, 45*time.Minute, false)
	if _, err := s.Pool().Exec(ctx, `UPDATE conversations SET last_activity_at = now() WHERE id=$1`, rc); err != nil {
		t.Fatal(err)
	}
	fresh, _ := mk()

	got, err := s.ExpiredGuests(ctx, time.Now().Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != old.ID {
		t.Fatalf("expired = %v, want only %v (active %v replied %v fresh %v)", got, old.ID, active.ID, replied.ID, fresh.ID)
	}
}

func TestRetention(t *testing.T) {
	ctx := context.Background()
	s := storetest.New(t)
	ids := mkUsers(t, s, 4)
	c1, _ := s.GetOrCreateConversation(ctx, ids[0], ids[1])
	c2, _ := s.GetOrCreateConversation(ctx, ids[2], ids[3])
	for _, c := range []struct {
		conv   uuid.UUID
		sender uuid.UUID
	}{{c1, ids[0]}, {c2, ids[2]}} {
		if _, _, err := s.SendMessage(ctx, c.conv, c.sender, uuid.New(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	set := func(c uuid.UUID, days int) {
		if _, err := s.Pool().Exec(ctx, `UPDATE conversations SET last_activity_at = now() - make_interval(days => $1) WHERE id=$2`, days, c); err != nil {
			t.Fatal(err)
		}
	}
	set(c1, 32)
	set(c2, 20)
	n, err := s.DeleteInactiveConversations(ctx, time.Now().AddDate(0, -1, 0))
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	var convs, msgs int
	s.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM conversations), (SELECT count(*) FROM messages)`).Scan(&convs, &msgs)
	if convs != 1 || msgs != 1 {
		t.Fatalf("convs=%d msgs=%d, want 1/1", convs, msgs)
	}
}
