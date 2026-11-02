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
	"github.com/tosrv/pmess/internal/api"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
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
