package chat_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
)

// endedGuest creates dev + a guest who sent a message, then ends the guest.
func endedGuest(t *testing.T) (*store.Store, *chat.Service, store.User, store.User, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	g, conv, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Send(ctx, g, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	out, ended, err := svc.EndGuest(ctx, g.ID)
	if err != nil || out != store.GuestDeactivated || len(ended) != 1 || ended[0].PeerID != dev.ID {
		t.Fatalf("out=%v ended=%v err=%v", out, ended, err)
	}
	return st, svc, dev, g, conv
}

func TestSendToEndedGuest(t *testing.T) {
	_, svc, dev, _, conv := endedGuest(t)
	_, _, _, err := svc.Send(context.Background(), dev, conv, uuid.New(), "hello?")
	wantCode(t, err, "peer_ended")
	if chat.ErrPeerEnded.Status != 410 {
		t.Fatal("status")
	}
}

func TestEndedGuestHiddenFromOpen(t *testing.T) {
	_, svc, dev, g, _ := endedGuest(t)
	_, err := svc.OpenConversation(context.Background(), dev, g.ID)
	wantCode(t, err, "not_found")
}

func TestEndedGuestNoDeliver(t *testing.T) {
	ctx := context.Background()
	_, svc, dev, _, conv := endedGuest(t)
	if _, ok, err := svc.PeerOf(ctx, dev, conv); err != nil || ok {
		t.Fatalf("PeerOf deliver=%v err=%v", ok, err)
	}
	if _, _, ok, err := svc.MarkRead(ctx, dev, conv, 1); err != nil || ok {
		t.Fatalf("MarkRead notify=%v err=%v", ok, err)
	}
	if ms, err := svc.Messages(ctx, dev, conv, 0, 0, 50); err != nil || len(ms) != 1 {
		t.Fatalf("messages %v %v", ms, err)
	}
}

func TestEndGuestTwiceNotFound(t *testing.T) {
	_, svc, _, g, _ := endedGuest(t)
	_, _, err := svc.EndGuest(context.Background(), g.ID)
	wantCode(t, err, "not_found")
}

func TestBlockedEndedGuestStillListed(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	g, conv, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Send(ctx, g, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Block(ctx, dev, g.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	cs, err := svc.Conversations(ctx, dev)
	if err != nil || len(cs) != 1 || cs[0].ID != conv || !cs[0].PeerEnded || !cs[0].BlockedByMe {
		t.Fatalf("cs=%+v err=%v", cs, err)
	}
	_, _, _, err = svc.Send(ctx, dev, conv, uuid.New(), "x")
	if err == nil {
		t.Fatal("send to blocked ended guest succeeded")
	}
}

func TestGoogleDeleteAccountStillFull(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	a := googleUser(t, svc, "a", "a@x.com", "alice")
	b := googleUser(t, svc, "b", "b@x.com", "bob")
	conv, err := svc.OpenConversation(ctx, a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Send(ctx, a, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	d, err := svc.DeleteAccount(ctx, a)
	if err != nil || len(d) != 1 || d[0].ConversationID != conv || d[0].PeerID != b.ID {
		t.Fatalf("d=%v err=%v", d, err)
	}
	cs, err := svc.Conversations(ctx, b)
	if err != nil || len(cs) != 0 {
		t.Fatalf("cs=%v err=%v", cs, err)
	}
}

func TestDeleteAccountRejectsGuest(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	googleUser(t, svc, "d", devEmail, "dev")
	g, _, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.DeleteAccount(ctx, g)
	wantCode(t, err, "forbidden")
}
