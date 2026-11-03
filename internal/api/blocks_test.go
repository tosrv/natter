package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBlocksAPI(t *testing.T) {
	e := setup(t, "")
	_, atok := e.user(t, "a", "a@x.com", "alice")
	b, _ := e.user(t, "b", "b@x.com", "bobby")
	body := `{"user_id":"` + b.ID.String() + `"}`
	for i := 0; i < 2; i++ {
		if w := e.do("POST", "/api/blocks", atok, body); w.Code != 204 {
			t.Fatalf("post %d: %d %s", i, w.Code, w.Body)
		}
	}
	w := e.do("GET", "/api/blocks", atok, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"display_name"`) || !strings.Contains(w.Body.String(), b.ID.String()) {
		t.Fatalf("get: %d %s", w.Code, w.Body)
	}
	if w := e.do("DELETE", "/api/blocks/"+b.ID.String(), atok, ""); w.Code != 204 {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}
	if w := e.do("GET", "/api/blocks", atok, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty: %d %s", w.Code, w.Body)
	}
	if w := e.do("DELETE", "/api/blocks/zzz", atok, ""); w.Code != 404 {
		t.Fatalf("bad uuid delete: %d", w.Code)
	}
	if w := e.do("POST", "/api/blocks", atok, `nope`); w.Code != 400 {
		t.Fatalf("bad json: %d", w.Code)
	}
	if w := e.do("POST", "/api/blocks", atok, `{"user_id":"zzz"}`); w.Code != 400 {
		t.Fatalf("bad id: %d", w.Code)
	}
	a, _ := e.user(t, "a3", "a3@x.com", "alice3")
	if w := e.do("POST", "/api/blocks", atok, `{"user_id":"`+a.ID.String()+`"}`); w.Code != 204 {
		t.Fatalf("other: %d", w.Code)
	}
	self := e.me(t, atok)
	if w := e.do("POST", "/api/blocks", atok, `{"user_id":"`+self+`"}`); w.Code != 400 || errCode(t, w) != "bad_request" {
		t.Fatalf("self: %d %s", w.Code, w.Body)
	}
	e.user(t, "dev", devEmail, "devuser")
	gu, _, err := e.svc.CreateGuest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	gtok, err := e.st.CreateSession(context.Background(), gu.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if w := e.do("POST", "/api/blocks", gtok, body); w.Code != 403 {
		t.Fatalf("guest: %d %s", w.Code, w.Body)
	}
}

func (e *env) me(t *testing.T, tok string) string {
	t.Helper()
	var m struct{ ID string }
	json.Unmarshal(e.do("GET", "/api/me", tok, "").Body.Bytes(), &m)
	return m.ID
}

func TestConversationsHidesBlockedMe(t *testing.T) {
	e := setup(t, "")
	a, atok := e.user(t, "a", "a@x.com", "alice")
	b, btok := e.user(t, "b", "b@x.com", "bobby")
	if _, err := e.svc.OpenConversation(context.Background(), a, b.ID); err != nil {
		t.Fatal(err)
	}
	if w := e.do("POST", "/api/blocks", atok, `{"user_id":"`+b.ID.String()+`"}`); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := e.do("GET", "/api/conversations", atok, ""); !strings.Contains(w.Body.String(), `"blocked_by_me":true`) {
		t.Fatalf("A: %s", w.Body)
	}
	w := e.do("GET", "/api/conversations", btok, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), `"blocked_by_me":true`) {
		t.Fatalf("B: %d %s", w.Code, w.Body)
	}
}
