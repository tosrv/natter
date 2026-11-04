package chat_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func msgCount(t *testing.T, st *store.Store, conv uuid.UUID) int {
	t.Helper()
	var n int
	if err := st.Pool().QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE conversation_id=$1`, conv).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func pair(t *testing.T) (*store.Store, *chat.Service, store.User, store.User, uuid.UUID) {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	a := googleUser(t, svc, "a", "a@example.com", "alice")
	b := googleUser(t, svc, "b", "b@example.com", "bob")
	conv, err := svc.OpenConversation(context.Background(), a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	return st, svc, a, b, conv
}

func TestSendBlockedBothWays(t *testing.T) {
	ctx := context.Background()
	st, svc, a, b, conv := pair(t)
	if err := svc.Block(ctx, a, b.ID); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := svc.Send(ctx, a, conv, uuid.New(), "hi")
	wantCode(t, err, "not_found")
	_, _, _, err = svc.Send(ctx, b, conv, uuid.New(), "hi")
	wantCode(t, err, "not_found")
	if n := msgCount(t, st, conv); n != 0 {
		t.Fatalf("messages=%d", n)
	}
}

func TestOpenConversationBlocked(t *testing.T) {
	ctx := context.Background()
	_, svc, a, b, _ := pair(t)
	if err := svc.Block(ctx, b, a.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.OpenConversation(ctx, a, b.ID)
	wantCode(t, err, "not_found")
	_, err = svc.OpenConversation(ctx, b, a.ID)
	wantCode(t, err, "not_found")
}

func TestUnblockRestoresSend(t *testing.T) {
	ctx := context.Background()
	_, svc, a, b, conv := pair(t)
	if err := svc.Block(ctx, a, b.ID); err != nil {
		t.Fatal(err)
	}
	if bl, err := svc.Blocks(ctx, a); err != nil || len(bl) != 1 {
		t.Fatalf("blocks=%v err=%v", bl, err)
	}
	for i := 0; i < 2; i++ {
		if err := svc.Unblock(ctx, a, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, _, err := svc.Send(ctx, b, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
}

func TestBlockSelfBadRequest(t *testing.T) {
	ctx := context.Background()
	_, svc, a, _, _ := pair(t)
	wantCode(t, svc.Block(ctx, a, a.ID), "bad_request")
	wantCode(t, svc.Block(ctx, a, uuid.New()), "not_found")
}

func TestGuestCannotBlock(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	g, _, err := svc.CreateGuest(ctx, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, svc.Block(ctx, g, dev.ID), "forbidden")
}

func TestDeveloperBlocksGuest(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	g, conv, err := svc.CreateGuest(ctx, "Guest Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Block(ctx, dev, g.ID); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = svc.Send(ctx, g, conv, uuid.New(), "hi")
	wantCode(t, err, "not_found")
	if _, ok, err := svc.PeerOf(ctx, g, conv); err != nil || ok {
		t.Fatalf("peerOf ok=%v err=%v", ok, err)
	}
}

func TestPeerOfAndMarkReadSuppressed(t *testing.T) {
	ctx := context.Background()
	st, svc, a, b, conv := pair(t)
	if _, _, _, err := svc.Send(ctx, a, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	if p, ok, err := svc.PeerOf(ctx, b, conv); err != nil || !ok || p != a.ID {
		t.Fatalf("before: p=%v ok=%v err=%v", p, ok, err)
	}
	if err := svc.Block(ctx, a, b.ID); err != nil {
		t.Fatal(err)
	}
	for _, u := range []store.User{a, b} {
		if _, ok, err := svc.PeerOf(ctx, u, conv); err != nil || ok {
			t.Fatalf("peerOf ok=%v err=%v", ok, err)
		}
	}
	seq, _, ok, err := svc.MarkRead(ctx, b, conv, 1)
	if err != nil || ok || seq != 1 {
		t.Fatalf("markRead seq=%d ok=%v err=%v", seq, ok, err)
	}
	var lr int64
	if err := st.Pool().QueryRow(ctx, `SELECT last_read_seq FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`, conv, b.ID).Scan(&lr); err != nil || lr != 1 {
		t.Fatalf("last_read_seq=%d err=%v", lr, err)
	}
}
