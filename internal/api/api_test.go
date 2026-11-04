package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/api"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/config"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store"
	"github.com/tosrv/natter/internal/store/storetest"
)

func init() { gin.SetMode(gin.TestMode) }

const devEmail = "dev@example.com"

type env struct {
	st  *store.Store
	svc *chat.Service
	r   *gin.Engine
}

func setup(t *testing.T, static string) *env {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	r := api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil),
		StaticDir: static, AllowedOrigins: []string{"http://ok.example"}})
	return &env{st, svc, r}
}

func (e *env) user(t *testing.T, sub, email, username string) (store.User, string) {
	t.Helper()
	ctx := context.Background()
	u, err := e.svc.LoginGoogle(ctx, sub, email, "")
	if err != nil {
		t.Fatal(err)
	}
	if username != "" {
		if u, err = e.svc.SetUsername(ctx, u, username); err != nil {
			t.Fatal(err)
		}
	}
	tok, err := e.st.CreateSession(ctx, u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return u, tok
}

func (e *env) do(method, path, tok, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:5555"
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func errCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var b struct {
		Error struct{ Code, Message string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return b.Error.Code
}

func TestAPIAuthzReturns404(t *testing.T) {
	e := setup(t, "")
	a, _ := e.user(t, "a", "a@x.com", "alice")
	b, _ := e.user(t, "b", "b@x.com", "bobby")
	_, ctok := e.user(t, "c", "c@x.com", "carol")
	w := e.do("POST", "/api/conversations", "", `{"peer_id":"`+b.ID.String()+`"}`)
	if w.Code != 401 {
		t.Fatalf("unauth: %d", w.Code)
	}
	_, atok := e.user(t, "a2", "a2@x.com", "alice2")
	_ = a
	w = e.do("POST", "/api/conversations", atok, `{"peer_id":"`+b.ID.String()+`"}`)
	if w.Code != 200 {
		t.Fatalf("open: %d %s", w.Code, w.Body)
	}
	var cr struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &cr)
	if w = e.do("GET", "/api/conversations/"+cr.ID+"/messages", atok, ""); w.Code != 200 {
		t.Fatalf("member: %d", w.Code)
	}
	if w = e.do("GET", "/api/conversations/"+cr.ID+"/messages", ctok, ""); w.Code != 404 || errCode(t, w) != "not_found" {
		t.Fatalf("outsider: %d %s", w.Code, w.Body)
	}
	if w = e.do("GET", "/api/conversations/"+uuid.NewString()+"/messages", ctok, ""); w.Code != 404 {
		t.Fatalf("unknown: %d", w.Code)
	}
	if w = e.do("GET", "/api/conversations/not-a-uuid/messages", ctok, ""); w.Code != 404 {
		t.Fatalf("bad uuid: %d", w.Code)
	}
	if w = e.do("POST", "/api/conversations", atok, `{"peer_id":"zzz"}`); w.Code != 400 || errCode(t, w) != "bad_request" {
		t.Fatalf("bad body: %d %s", w.Code, w.Body)
	}
}

func TestSearchHidesEmail(t *testing.T) {
	e := setup(t, "")
	_, tok := e.user(t, "a", "a@x.com", "alice")
	e.user(t, "b", "b@x.com", "alfred")
	w := e.do("GET", "/api/users/search?q=al", tok, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "@") || !strings.Contains(w.Body.String(), `"username":"alfred"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("GET", "/api/users/search?q=a", tok, ""); w.Code != 400 || errCode(t, w) != "query_too_short" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestSearchRateLimit(t *testing.T) {
	e := setup(t, "")
	_, tok := e.user(t, "a", "a@x.com", "alice")
	for i := 0; i < 30; i++ {
		if w := e.do("GET", "/api/users/search?q=al", tok, ""); w.Code != 200 {
			t.Fatalf("req %d: %d", i, w.Code)
		}
	}
	w := e.do("GET", "/api/users/search?q=al", tok, "")
	if w.Code != 429 || errCode(t, w) != "rate_limited" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestMessagesPagination(t *testing.T) {
	e := setup(t, "")
	a, atok := e.user(t, "a", "a@x.com", "alice")
	b, _ := e.user(t, "b", "b@x.com", "bobby")
	ctx := context.Background()
	cid, err := e.svc.OpenConversation(ctx, a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 150; i++ {
		if _, _, _, err := e.svc.Send(ctx, a, cid, uuid.New(), "m"); err != nil {
			t.Fatal(err)
		}
	}
	get := func(q string) []struct{ Seq int64 } {
		w := e.do("GET", "/api/conversations/"+cid.String()+"/messages"+q, atok, "")
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", q, w.Code, w.Body)
		}
		var r struct{ Messages []struct{ Seq int64 } }
		json.Unmarshal(w.Body.Bytes(), &r)
		return r.Messages
	}
	if m := get("?limit=500"); len(m) != 100 || m[99].Seq != 150 {
		t.Fatalf("limit: %d", len(m))
	}
	if m := get("?after_seq=140"); len(m) != 10 || m[0].Seq != 141 {
		t.Fatalf("after: %d", len(m))
	}
	if m := get("?before_seq=11&limit=5"); len(m) != 5 || m[4].Seq != 10 {
		t.Fatalf("before: %+v", m)
	}
	if w := e.do("GET", "/api/conversations/"+cid.String()+"/messages?after_seq=x", atok, ""); w.Code != 400 {
		t.Fatalf("bad param: %d", w.Code)
	}
}

func TestErrorFormat(t *testing.T) {
	e := setup(t, "")
	w := e.do("GET", "/api/nope", "", "")
	if w.Code != 404 || errCode(t, w) != "not_found" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = e.do("GET", "/api/me", "", "")
	if w.Code != 401 || errCode(t, w) != "unauthorized" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	g := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(g)
	c.Request = httptest.NewRequest("GET", "/", nil)
	api.WriteError(c, context.DeadlineExceeded)
	if g.Code != 500 || strings.Contains(g.Body.String(), "deadline") || !strings.Contains(g.Body.String(), `"internal"`) {
		t.Fatalf("%d %s", g.Code, g.Body)
	}
}

func TestMeAndUsername(t *testing.T) {
	e := setup(t, "")
	_, tok := e.user(t, "a", "a@x.com", "alice")
	w := e.do("GET", "/api/me", tok, "")
	var m map[string]any
	json.Unmarshal(w.Body.Bytes(), &m)
	if w.Code != 200 || m["username"] != "alice" || m["username_changes_left"] != float64(3) || m["email"] != "a@x.com" || m["kind"] != "google" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = e.do("PUT", "/api/me/username", tok, `{"username":"alice_2"}`)
	json.Unmarshal(w.Body.Bytes(), &m)
	if w.Code != 200 || m["username_changes_left"] != float64(2) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("PUT", "/api/me/username", tok, `{"username":"A!"}`); w.Code != 400 || errCode(t, w) != "username_invalid" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("PUT", "/api/me/username", tok, `nope`); w.Code != 400 || errCode(t, w) != "bad_request" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	_, ntok := e.user(t, "n", "n@x.com", "")
	if w = e.do("GET", "/api/conversations", ntok, ""); w.Code != 403 || errCode(t, w) != "username_required" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("GET", "/api/conversations", tok, ""); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestCORSAndStatic(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(dir+"/index.html", "<html>spa</html>"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir+"/app.js", "js"); err != nil {
		t.Fatal(err)
	}
	e := setup(t, dir)
	req := httptest.NewRequest("GET", "/api/nope", nil)
	req.Header.Set("Origin", "http://ok.example")
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "http://ok.example" || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("%v", w.Header())
	}
	req.Header.Set("Origin", "http://evil.example")
	w = httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("evil origin allowed")
	}
	if w = e.do("GET", "/app.js", "", ""); w.Code != 200 || w.Body.String() != "js" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("GET", "/chat/123", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), "spa") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = e.do("GET", "/api/zzz", "", ""); w.Code != 404 || errCode(t, w) != "not_found" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

var _ = http.StatusOK

type fakeNotifier struct {
	peers      []store.DeletedConversation
	user       uuid.UUID
	calls      int
	guestCalls int
	outcome    store.EndGuestOutcome
	ended      []store.EndedConversation
}

func (f *fakeNotifier) NotifyGuestEnded(o store.EndGuestOutcome, c []store.EndedConversation, u uuid.UUID) {
	f.outcome, f.ended, f.user = o, c, u
	f.guestCalls++
}

func (f *fakeNotifier) NotifyConversationDeleted(p []store.DeletedConversation, u uuid.UUID) {
	f.peers, f.user = p, u
	f.calls++
}

func setupN(t *testing.T) (*env, *fakeNotifier) {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	n := &fakeNotifier{}
	r := api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil), Notifier: n})
	return &env{st, svc, r}, n
}

func usedSlots(t *testing.T, e *env) int {
	var n int
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT used FROM user_slots WHERE id=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSetDisplayNameAPI(t *testing.T) {
	e := setup(t, "")
	_, tok := e.user(t, "a", "a@x.com", "alice")
	w := e.do("PUT", "/api/me/display-name", tok, `{"display_name":"  Alice   W "}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"display_name":"Alice W"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	w = e.do("PUT", "/api/me/display-name", tok, `{"display_name":"😀"}`)
	if w.Code != 400 || errCode(t, w) != "display_name_invalid" {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestDeleteMeAPI(t *testing.T) {
	e, n := setupN(t)
	a, atok := e.user(t, "a", "a@x.com", "alice")
	b, _ := e.user(t, "b", "b@x.com", "bob")
	cid, err := e.svc.OpenConversation(context.Background(), a, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := usedSlots(t, e)
	w := e.do("DELETE", "/api/me", atok, "")
	if w.Code != 204 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookie not cleared")
	}
	if w := e.do("GET", "/api/me", atok, ""); w.Code != 401 {
		t.Fatalf("after delete %d", w.Code)
	}
	if n.calls != 1 || n.user != a.ID || len(n.peers) != 1 || n.peers[0].ConversationID != cid || n.peers[0].PeerID != b.ID {
		t.Fatalf("notifier %+v", n)
	}
	if usedSlots(t, e) != before-1 {
		t.Fatal("slot not released")
	}
}

func TestDeleteMeTwice(t *testing.T) {
	e, n := setupN(t)
	_, tok := e.user(t, "a", "a@x.com", "alice")
	before := usedSlots(t, e)
	if w := e.do("DELETE", "/api/me", tok, ""); w.Code != 204 {
		t.Fatalf("%d", w.Code)
	}
	if w := e.do("DELETE", "/api/me", tok, ""); w.Code != 401 {
		t.Fatalf("second %d", w.Code)
	}
	if usedSlots(t, e) != before-1 || n.calls != 1 {
		t.Fatalf("used %d calls %d", usedSlots(t, e), n.calls)
	}
}

func TestDeleteMeDeveloper(t *testing.T) {
	e, n := setupN(t)
	_, tok := e.user(t, "d", devEmail, "devuser")
	w := e.do("DELETE", "/api/me", tok, "")
	if w.Code != 403 || errCode(t, w) != "forbidden" || n.calls != 0 {
		t.Fatalf("%d %s calls %d", w.Code, w.Body, n.calls)
	}
}

func TestSearchIncludesDisplayNameNoEmail(t *testing.T) {
	e := setup(t, "")
	_, tok := e.user(t, "a", "a@x.com", "alice")
	e.user(t, "b", "bob@secret.com", "bobby")
	w := e.do("GET", "/api/users/search?q=bobby", tok, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"display_name":"User"`) || strings.Contains(w.Body.String(), "secret.com") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestDeleteMeGuestDeactivates(t *testing.T) {
	e, n := setupN(t)
	ctx := context.Background()
	dev, _ := e.user(t, "d", devEmail, "devuser")
	g, conv, err := e.svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.svc.Send(ctx, g, conv, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	tok, err := e.st.CreateSession(ctx, g.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	before := usedSlots(t, e)
	w := e.do("DELETE", "/api/me", tok, "")
	if w.Code != 204 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookie not cleared")
	}
	if n.guestCalls != 1 || n.calls != 0 || n.outcome != store.GuestDeactivated || n.user != g.ID ||
		len(n.ended) != 1 || n.ended[0].ConversationID != conv || n.ended[0].PeerID != dev.ID {
		t.Fatalf("notifier %+v", n)
	}
	if usedSlots(t, e) != before-1 {
		t.Fatal("slot not released")
	}
	if w := e.do("GET", "/api/me", tok, ""); w.Code != 401 {
		t.Fatalf("after end %d", w.Code)
	}
}

func TestEndGuestAndClearAlreadyEnded(t *testing.T) {
	e, n := setupN(t)
	e.user(t, "d", devEmail, "devuser")
	st, svc := e.st, e.svc
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	d := api.Deps{Auth: a, Chat: svc, Notifier: n}
	ctx := context.Background()
	g, _, err := svc.CreateGuest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/me", nil)
	api.EndGuestAndClear(d, c, g.ID)
	c.Writer.WriteHeaderNow()
	if w.Code != 204 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	cleared := false
	for _, ck := range w.Result().Cookies() {
		if ck.Name == auth.SessionCookie && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookie not cleared")
	}
	if n.guestCalls != 0 || n.calls != 0 {
		t.Fatalf("notifier called %+v", n)
	}
}
