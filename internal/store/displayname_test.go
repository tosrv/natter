package store_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func TestMigration0002Constraints(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if _, err := s.Pool().Exec(ctx, `INSERT INTO users(kind) VALUES('guest')`); err == nil {
		t.Fatal("expected NOT NULL violation without display_name")
	}
	if _, err := s.Pool().Exec(ctx, `INSERT INTO users(kind,display_name) VALUES('guest',$1)`, strings.Repeat("x", 41)); err == nil {
		t.Fatal("expected CHECK violation for 41-char display_name")
	}
	if _, err := s.Pool().Exec(ctx, `INSERT INTO users(kind,display_name) VALUES('guest',$1)`, strings.Repeat("x", 40)); err != nil {
		t.Fatalf("40 chars should be ok: %v", err)
	}
	dev := mkUser(t, s, "dev")
	g, _, err := s.CreateGuest(ctx, dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.Username == nil {
		t.Fatal("guest has no username")
	}
	if !regexp.MustCompile(`^guest-[0-9a-f]{4}$`).MatchString(*g.Username) {
		t.Fatalf("username %q", *g.Username)
	}
	want := "Guest " + strings.ToUpper((*g.Username)[len("guest-"):])
	if g.DisplayName != want {
		t.Fatalf("display name %q want %q", g.DisplayName, want)
	}
	if dev.DisplayName != "User" {
		t.Fatalf("empty displayName should store User, got %q", dev.DisplayName)
	}
}

func TestUpsertDisplayNameOnlyOnInsert(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u, _, err := s.UpsertGoogleUser(ctx, "s1", "a@x.com", "Rahmat", false)
	if err != nil || u.DisplayName != "Rahmat" {
		t.Fatalf("%v %q", err, u.DisplayName)
	}
	u2, created, err := s.UpsertGoogleUser(ctx, "s1", "a@x.com", "Other", false)
	if err != nil || created || u2.DisplayName != "Rahmat" {
		t.Fatalf("%v %v %q", err, created, u2.DisplayName)
	}
}

func TestSetDisplayName(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUser(t, s, "s1")
	got, err := s.SetDisplayName(ctx, u.ID, "Rahmat Tomy")
	if err != nil || got.DisplayName != "Rahmat Tomy" {
		t.Fatalf("%v %q", err, got.DisplayName)
	}
	again, err := s.GetUser(ctx, u.ID)
	if err != nil || again.DisplayName != "Rahmat Tomy" {
		t.Fatalf("%v %q", err, again.DisplayName)
	}
	if _, err := s.SetDisplayName(ctx, uuid.New(), "X"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b, c := mkUser(t, s, "a"), mkUser(t, s, "b"), mkUser(t, s, "c")
	ab, err := s.GetOrCreateConversation(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	ac, err := s.GetOrCreateConversation(ctx, a.ID, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, ab, a.ID, uuid.New(), "hi b"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SendMessage(ctx, ac, c.ID, uuid.New(), "hi a"); err != nil {
		t.Fatal(err)
	}
	tok, err := s.CreateSession(ctx, a.ID, 3600e9)
	if err != nil {
		t.Fatal(err)
	}
	before := usedSlots(t, s)

	peers, err := s.DeleteAccount(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 {
		t.Fatalf("peers = %v", peers)
	}
	got := map[uuid.UUID]uuid.UUID{}
	for _, p := range peers {
		got[p.PeerID] = p.ConversationID
	}
	if got[b.ID] != ab || got[c.ID] != ac {
		t.Fatalf("peers = %v", peers)
	}
	var n int
	if err := s.Pool().QueryRow(ctx, `SELECT count(*) FROM conversations`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("conversations %d %v", n, err)
	}
	if err := s.Pool().QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("messages %d %v", n, err)
	}
	if _, err := s.UserBySession(ctx, tok); err == nil {
		t.Fatal("session should be gone")
	}
	if usedSlots(t, s) != before-1 {
		t.Fatalf("used %d want %d", usedSlots(t, s), before-1)
	}
	if _, err := s.DeleteAccount(ctx, a.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if usedSlots(t, s) != before-1 {
		t.Fatal("used decremented twice")
	}
}

func TestSearchAndSummaryIncludeDisplayName(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a := mkUser(t, s, "a")
	b := mkUser(t, s, "b")
	if _, err := s.SetUsername(ctx, b.ID, "bobby"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDisplayName(ctx, b.ID, "Bob Builder"); err != nil {
		t.Fatal(err)
	}
	res, err := s.SearchUsers(ctx, "bob", a.ID, 10)
	if err != nil || len(res) != 1 || res[0].DisplayName != "Bob Builder" {
		t.Fatalf("%v %+v", err, res)
	}
	if _, err := s.GetOrCreateConversation(ctx, a.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListConversations(ctx, a.ID)
	if err != nil || len(list) != 1 || list[0].PeerDisplayName != "Bob Builder" {
		t.Fatalf("%v %+v", err, list)
	}
}
