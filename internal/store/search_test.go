package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func mkUser(t *testing.T, s *store.Store, sub string) store.User {
	t.Helper()
	u, _, err := s.UpsertGoogleUser(context.Background(), sub, sub+"@x.com", "", false)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestUsernameChangeLimit(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	u := mkUser(t, s, "s1")
	for _, name := range []string{"first", "second", "third", "fourth"} {
		got, err := s.SetUsername(ctx, u.ID, name)
		if err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
		if got.Username == nil || *got.Username != name {
			t.Fatalf("username = %v", got.Username)
		}
	}
	got, err := s.SetUsername(ctx, u.ID, "fourth")
	if err != nil || got.UsernameChanges != 3 {
		t.Fatalf("same name should be no-op: %v %d", err, got.UsernameChanges)
	}
	if _, err := s.SetUsername(ctx, u.ID, "fifth"); !errors.Is(err, store.ErrUsernameChangeLimit) {
		t.Fatalf("err = %v, want ErrUsernameChangeLimit", err)
	}
	if _, err := s.SetUsername(ctx, uuid.New(), "nobody"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUsernameTaken(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	a, b := mkUser(t, s, "a"), mkUser(t, s, "b")
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, id := range []uuid.UUID{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = s.SetUsername(ctx, id, "rahmat")
		}()
	}
	close(start)
	wg.Wait()
	ok, taken := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			ok++
		case errors.Is(e, store.ErrUsernameTaken):
			taken++
		default:
			t.Fatalf("unexpected: %v", e)
		}
	}
	if ok != 1 || taken != 1 {
		t.Fatalf("ok=%d taken=%d", ok, taken)
	}
	// case-insensitive
	c := mkUser(t, s, "c")
	if _, err := s.SetUsername(ctx, c.ID, "RAHMAT"); !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchPrefix(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	names := map[string]string{"u1": "rahmat", "u2": "rani_99", "u3": "bara", "u4": "raxmat", "me": "ra_me"}
	ids := map[string]uuid.UUID{}
	for sub, name := range names {
		u := mkUser(t, s, sub)
		if _, err := s.SetUsername(ctx, u.ID, name); err != nil {
			t.Fatal(err)
		}
		ids[sub] = u.ID
	}
	dev := mkUser(t, s, "dev")
	if _, _, err := s.CreateGuest(ctx, dev.ID, "Guest Test"); err != nil {
		t.Fatal(err)
	}
	names2 := func(r []store.UserBrief) []string {
		var o []string
		for _, b := range r {
			o = append(o, b.Username)
		}
		return o
	}
	got, err := s.SearchUsers(ctx, "ra", ids["me"], 10)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(names2(got)) != "[rahmat rani_99 raxmat]" {
		t.Fatalf("got %v", names2(got))
	}
	got, err = s.SearchUsers(ctx, "ra_", ids["u1"], 10)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(names2(got)) != "[ra_me]" {
		t.Fatalf("underscore literal: got %v", names2(got))
	}
	if got, err = s.SearchUsers(ctx, "gu", ids["u1"], 10); err != nil || len(got) != 0 {
		t.Fatalf("guest leaked: %v %v", got, err)
	}
	got, err = s.SearchUsers(ctx, "ra", ids["me"], 2)
	if err != nil || len(got) != 2 {
		t.Fatalf("limit: %v %v", got, err)
	}
}
