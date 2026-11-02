package chat_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/store/storetest"
)

func TestValidateDisplayName(t *testing.T) {
	ok := map[string]string{
		"Rahmat Tomy":           "Rahmat Tomy",
		"José":                  "José",
		"Users":                 "Users",
		"  a   b ":              "a b",
		"O'Neil-Jr_2.":          "O'Neil-Jr_2.",
		strings.Repeat("é", 40): strings.Repeat("é", 40),
	}
	for in, want := range ok {
		got, err := chat.ValidateDisplayName(in)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"😀", "a★b", "User", "user", "USER", "<img>", "   ", "", strings.Repeat("a", 41)} {
		_, err := chat.ValidateDisplayName(in)
		wantCode(t, err, "display_name_invalid")
	}
}

func TestLoginGoogleNameFallback(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	for i, c := range []struct{ name, want string }{
		{"Budi 🚀", "User"},
		{strings.Repeat("a", 60), "User"},
		{"Budi Santoso", "Budi Santoso"},
	} {
		sub := string(rune('a' + i))
		u, err := svc.LoginGoogle(ctx, sub, sub+"@x.com", c.name)
		if err != nil || u.DisplayName != c.want {
			t.Errorf("%q: got %q, %v", c.name, u.DisplayName, err)
		}
	}
}

func TestSetDisplayName(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	googleUser(t, svc, "d", devEmail, "dev")
	g, _, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetDisplayName(ctx, g, "Hacker")
	wantCode(t, err, "forbidden")
	u := googleUser(t, svc, "o", "o@example.com", "other")
	_, err = svc.SetDisplayName(ctx, u, "<b>")
	wantCode(t, err, "display_name_invalid")
	nu, err := svc.SetDisplayName(ctx, u, "  New   Name ")
	if err != nil || nu.DisplayName != "New Name" {
		t.Fatalf("%q %v", nu.DisplayName, err)
	}
}

func TestDeleteAccountDeveloperForbidden(t *testing.T) {
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	dev := googleUser(t, svc, "d", devEmail, "dev")
	_, err := svc.DeleteAccount(context.Background(), dev)
	wantCode(t, err, "forbidden")
}

func TestDeleteAccountOK(t *testing.T) {
	ctx := context.Background()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	googleUser(t, svc, "d", devEmail, "dev")
	a := googleUser(t, svc, "a", "a@example.com", "alice")
	b := googleUser(t, svc, "b", "b@example.com", "bob")
	conv, err := svc.OpenConversation(ctx, a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = uuid.Nil
	before := usedSlots(t, st)
	del, err := svc.DeleteAccount(ctx, a)
	if err != nil || len(del) != 1 || del[0].PeerID != b.ID || del[0].ConversationID != conv {
		t.Fatalf("%+v %v", del, err)
	}
	if usedSlots(t, st) != before-1 {
		t.Fatal("slot not released")
	}
}
