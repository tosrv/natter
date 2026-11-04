package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func guestSetup(t *testing.T, s *store.Store) (store.User, store.User, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	dev, _, err := s.UpsertGoogleUser(ctx, "dev", "dev@x.com", "", true)
	if err != nil {
		t.Fatal(err)
	}
	g, conv, err := s.CreateGuest(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	return dev, g, conv
}

func countRows(t *testing.T, s *store.Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := s.Pool().QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEndGuestWithoutMessagesDeletes(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, g, conv := guestSetup(t, s)
	before := usedSlots(t, s)
	out, ended, err := s.EndGuest(ctx, g.ID)
	if err != nil || out != store.GuestDeleted {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if len(ended) != 1 || ended[0].ConversationID != conv || ended[0].PeerID != dev.ID {
		t.Fatalf("ended=%+v", ended)
	}
	if _, err := s.GetUser(ctx, g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("user still exists: %v", err)
	}
	if n := countRows(t, s, `SELECT count(*) FROM conversations WHERE id=$1`, conv); n != 0 {
		t.Fatalf("conversation remains")
	}
	if n := usedSlots(t, s); n != before-1 {
		t.Fatalf("used=%d want %d", n, before-1)
	}
}

func TestEndGuestWithMessagesDeactivates(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, g, conv := guestSetup(t, s)
	tok, err := s.CreateSession(ctx, g.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	before := usedSlots(t, s)
	out, ended, err := s.EndGuest(ctx, g.ID)
	if err != nil || out != store.GuestDeactivated {
		t.Fatalf("out=%v err=%v", out, err)
	}
	if len(ended) != 1 || ended[0].ConversationID != conv || ended[0].PeerID != dev.ID {
		t.Fatalf("ended=%+v", ended)
	}
	u, err := s.GetUser(ctx, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.DeactivatedAt == nil || u.Username != nil || u.DisplayName != g.DisplayName {
		t.Fatalf("bad user: %+v", u)
	}
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("session still valid: %v", err)
	}
	msgs, err := s.ListMessages(ctx, conv, 0, 0, 50)
	if err != nil || len(msgs) != 2 {
		t.Fatalf("msgs=%d err=%v", len(msgs), err)
	}
	if n := usedSlots(t, s); n != before-1 {
		t.Fatalf("used=%d want %d", n, before-1)
	}
}

func TestUserBySessionRejectsDeactivated(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	_, g, _ := guestSetup(t, s)
	tok, _ := s.CreateSession(ctx, g.ID, time.Hour)
	if _, err := s.Pool().Exec(ctx, `UPDATE users SET deactivated_at=now() WHERE id=$1`, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UserBySession(ctx, tok); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestEndGuestTwice(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, g, conv := guestSetup(t, s)
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	before := usedSlots(t, s)
	if _, _, err := s.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second: %v", err)
	}
	// Non-guest and unknown ids.
	if _, _, err := s.EndGuest(ctx, dev.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("google: %v", err)
	}
	if _, _, err := s.EndGuest(ctx, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if n := usedSlots(t, s); n != before-1 {
		t.Fatalf("used=%d want %d", n, before-1)
	}
	// A guest without messages: second call also ErrNotFound.
	_, g2, _ := guestSetupExtra(t, s, dev.ID)
	if _, _, err := s.EndGuest(ctx, g2.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g2.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second (deleted): %v", err)
	}
}

func guestSetupExtra(t *testing.T, s *store.Store, devID uuid.UUID) (store.User, store.User, uuid.UUID) {
	t.Helper()
	g, conv, err := s.CreateGuest(context.Background(), devID)
	if err != nil {
		t.Fatal(err)
	}
	return store.User{ID: devID}, g, conv
}

func TestEndGuestConcurrentWithSend(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, _, _ := guestSetup(t, s)
	for i := 0; i < 20; i++ {
		_, g, conv := guestSetupExtra(t, s, dev.ID)
		start := make(chan struct{})
		var (
			wg      sync.WaitGroup
			out     store.EndGuestOutcome
			endErr  error
			sendErr error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			out, _, endErr = s.EndGuest(ctx, g.ID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_, _, sendErr = s.SendMessage(ctx, conv, g.ID, uuid.New(), "race")
		}()
		close(start)
		wg.Wait()
		if endErr != nil {
			t.Fatalf("round %d: end: %v", i, endErr)
		}
		msgs := countRows(t, s, `SELECT count(*) FROM messages WHERE sender_id=$1`, g.ID)
		if out == store.GuestDeleted && sendErr == nil {
			t.Fatalf("round %d: GuestDeleted but SendMessage succeeded (message lost)", i)
		}
		if out == store.GuestDeleted && msgs != 0 {
			t.Fatalf("round %d: GuestDeleted with %d messages", i, msgs)
		}
		if out == store.GuestDeactivated && msgs != 1 {
			t.Fatalf("round %d: deactivated with %d messages", i, msgs)
		}
	}
}

func TestExpiredGuestsSkipsDeactivated(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, g, conv := guestSetup(t, s)
	_, g2, _ := guestSetupExtra(t, s, dev.ID)
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	ids, err := s.ExpiredGuests(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != g2.ID {
		t.Fatalf("ids=%v want [%v]", ids, g2.ID)
	}
}

func TestPurgeDeactivatedGuests(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, gGone, convGone := guestSetup(t, s)
	_, gKeep, convKeep := guestSetupExtra(t, s, dev.ID)
	_, gActive, _ := guestSetupExtra(t, s, dev.ID)
	for _, p := range []struct{ g, c uuid.UUID }{{gGone.ID, convGone}, {gKeep.ID, convKeep}} {
		if _, _, err := s.SendMessage(ctx, p.c, p.g, uuid.New(), "hi"); err != nil {
			t.Fatal(err)
		}
		if out, _, err := s.EndGuest(ctx, p.g); err != nil || out != store.GuestDeactivated {
			t.Fatalf("%v %v", out, err)
		}
	}
	if _, err := s.Pool().Exec(ctx, `DELETE FROM conversations WHERE id=$1`, convGone); err != nil {
		t.Fatal(err)
	}
	before := usedSlots(t, s)
	n, err := s.PurgeDeactivatedGuests(ctx)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if _, err := s.GetUser(ctx, gGone.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("gone guest remains: %v", err)
	}
	for _, id := range []uuid.UUID{gKeep.ID, gActive.ID, dev.ID} {
		if _, err := s.GetUser(ctx, id); err != nil {
			t.Fatalf("%v removed: %v", id, err)
		}
	}
	if n := usedSlots(t, s); n != before {
		t.Fatalf("used=%d want %d", n, before)
	}
}

func TestSummaryPeerEnded(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	dev, g, conv := guestSetup(t, s)
	_, _, conv2 := guestSetupExtra(t, s, dev.ID)
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListConversations(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, c := range list {
		got[c.ID] = c.PeerEnded
	}
	if !got[conv] || got[conv2] {
		t.Fatalf("peer_ended=%v", got)
	}
}

func TestSearchExcludesDeactivated(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	me, _, _ := s.UpsertGoogleUser(ctx, "me", "me@x.com", "", false)
	u, _, _ := s.UpsertGoogleUser(ctx, "other", "o@x.com", "", false)
	if _, err := s.SetUsername(ctx, u.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool().Exec(ctx, `UPDATE users SET deactivated_at=now() WHERE id=$1`, u.ID); err != nil {
		t.Fatal(err)
	}
	res, err := s.SearchUsers(ctx, "ali", me.ID, 10)
	if err != nil || len(res) != 0 {
		t.Fatalf("res=%v err=%v", res, err)
	}
}

func TestSendAfterGuestDeactivatedRejected(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	_, g, conv := guestSetup(t, s)
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "first"); err != nil {
		t.Fatal(err)
	}
	tx, err := s.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT 1 FROM conversations WHERE id=$1 FOR UPDATE`, conv); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "late")
		done <- err
	}()
	// Wait until SendMessage is blocked on the conversation lock.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if countRows(t, s, `SELECT count(*) FROM pg_stat_activity WHERE wait_event_type='Lock'`) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("SendMessage never blocked")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET deactivated_at=now(), username=NULL WHERE id=$1`, g.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("send after deactivation: %v", err)
	}
	if n := countRows(t, s, `SELECT count(*) FROM messages WHERE sender_id=$1`, g.ID); n != 1 {
		t.Fatalf("messages=%d want 1", n)
	}
	if n := countRows(t, s, `SELECT last_seq FROM conversations WHERE id=$1`, conv); n != 1 {
		t.Fatalf("last_seq=%d want 1", n)
	}
}

func TestDeleteAccountRejectsGuest(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	_, g, conv := guestSetup(t, s)
	before := usedSlots(t, s)
	if _, err := s.DeleteAccount(ctx, g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("guest: %v", err)
	}
	if _, _, err := s.SendMessage(ctx, conv, g.ID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteAccount(ctx, g.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deactivated: %v", err)
	}
	// A deactivated non-guest row is also refused.
	u, _, _ := s.UpsertGoogleUser(ctx, "x", "x@x.com", "", false)
	s.Pool().Exec(ctx, `UPDATE users SET deactivated_at=now() WHERE id=$1`, u.ID)
	if _, err := s.DeleteAccount(ctx, u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deactivated google: %v", err)
	}
	if n := usedSlots(t, s); n != before { // before-1 (EndGuest) +1 (google)
		t.Fatalf("used=%d want %d", n, before)
	}
}
