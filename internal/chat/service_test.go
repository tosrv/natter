package chat_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
)

const devEmail = "dev@example.com"

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ce *chat.Error
	if !errors.As(err, &ce) || ce.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func googleUser(t *testing.T, svc *chat.Service, sub, email, username string) store.User {
	t.Helper()
	ctx := context.Background()
	u, err := svc.LoginGoogle(ctx, sub, email)
	if err != nil {
		t.Fatal(err)
	}
	if username != "" {
		if u, err = svc.SetUsername(ctx, u, username); err != nil {
			t.Fatal(err)
		}
	}
	return u
}

func usedSlots(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if err := st.Pool().QueryRow(context.Background(), `SELECT used FROM user_slots WHERE id=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGuestBeforeDeveloper(t *testing.T) {
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	_, _, err := svc.CreateGuest(context.Background())
	wantCode(t, err, "developer_unavailable")
	if n := usedSlots(t, st); n != 0 {
		t.Fatalf("used=%d", n)
	}
}

func TestGuestOnlyDeveloper(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", "DEV@example.com", "dev")
	if !dev.IsDeveloper {
		t.Fatal("expected developer")
	}
	other := googleUser(t, svc, "o", "o@example.com", "other")
	g, conv, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.OpenConversation(ctx, g, other.ID)
	wantCode(t, err, "forbidden")
	_, err = svc.Search(ctx, g, "ot")
	wantCode(t, err, "forbidden")
	_, peer, dup, err := svc.Send(ctx, g, conv, uuid.New(), "hi")
	if err != nil || dup || peer != dev.ID {
		t.Fatalf("send: peer=%v dup=%v err=%v", peer, dup, err)
	}
}

func TestAuthzNotMember(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	a := googleUser(t, svc, "a", "a@x.com", "alice")
	b := googleUser(t, svc, "b", "b@x.com", "bob")
	c := googleUser(t, svc, "c", "c@x.com", "carol")
	conv, err := svc.OpenConversation(ctx, a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Messages(ctx, c, conv, 0, 0, 50)
	wantCode(t, err, "not_found")
	_, _, _, err = svc.Send(ctx, c, conv, uuid.New(), "x")
	wantCode(t, err, "not_found")
	_, _, err = svc.MarkRead(ctx, c, conv, 1)
	wantCode(t, err, "not_found")
	_, err = svc.PeerOf(ctx, c, conv)
	wantCode(t, err, "not_found")
	_, err = svc.OpenConversation(ctx, a, a.ID)
	wantCode(t, err, "not_found")
	nou := googleUser(t, svc, "n", "n@x.com", "")
	_, err = svc.OpenConversation(ctx, a, nou.ID)
	wantCode(t, err, "not_found")
}

func TestBodyValidation(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	a := googleUser(t, svc, "a", "a@x.com", "alice")
	b := googleUser(t, svc, "b", "b@x.com", "bob")
	conv, err := svc.OpenConversation(ctx, a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Send(ctx, a, conv, uuid.New(), strings.Repeat("😀", 2000)); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = svc.Send(ctx, a, conv, uuid.New(), strings.Repeat("😀", 2001))
	wantCode(t, err, "message_too_long")
	_, _, _, err = svc.Send(ctx, a, conv, uuid.New(), "  \n ")
	wantCode(t, err, "message_empty")
	xss := "<script>alert(1)</script>"
	m, _, _, err := svc.Send(ctx, a, conv, uuid.New(), xss)
	if err != nil || m.Body != xss {
		t.Fatalf("body=%q err=%v", m.Body, err)
	}
	msgs, err := svc.Messages(ctx, b, conv, 0, 0, 50)
	if err != nil || len(msgs) != 2 || msgs[1].Body != xss {
		t.Fatalf("msgs=%v err=%v", msgs, err)
	}
}

func TestSendToDeletedGuest(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	g, conv, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = svc.Send(ctx, dev, conv, uuid.New(), "hello?")
	wantCode(t, err, "not_found")
}

func TestUsernameRequired(t *testing.T) {
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	u := googleUser(t, svc, "a", "a@x.com", "")
	_, err := svc.Conversations(context.Background(), u)
	wantCode(t, err, "username_required")
}

func TestSetUsernameRules(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	a := googleUser(t, svc, "a", "a@x.com", "")
	b := googleUser(t, svc, "b", "b@x.com", "")
	_, err := svc.SetUsername(ctx, a, "ab")
	wantCode(t, err, "username_invalid")
	_, err = svc.SetUsername(ctx, a, "bad-name")
	wantCode(t, err, "username_invalid")
	a, err = svc.SetUsername(ctx, a, "  Alice_1 ")
	if err != nil || *a.Username != "alice_1" {
		t.Fatalf("got %v %v", a.Username, err)
	}
	_, err = svc.SetUsername(ctx, b, "ALICE_1")
	wantCode(t, err, "username_taken")
	_, err = svc.Search(ctx, b, "a")
	wantCode(t, err, "username_required")
	b, err = svc.SetUsername(ctx, b, "bob")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Search(ctx, b, "a")
	wantCode(t, err, "query_too_short")
	res, err := svc.Search(ctx, b, "Al")
	if err != nil || len(res) != 1 {
		t.Fatalf("res=%v err=%v", res, err)
	}
}
