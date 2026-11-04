package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
	"github.com/tosrv/pmess/internal/store/storetest"
)

const devEmail = "dev@example.com"

func init() { gin.SetMode(gin.TestMode) }

type env struct {
	st  *store.Store
	svc *chat.Service
	a   *auth.Auth
	r   *gin.Engine
}

func setup(t *testing.T) *env { return setupOrigins(t, nil) }

func setupOrigins(t *testing.T, origins []string) *env {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1",
		GoogleClientID: "cid", GoogleClientSecret: "sec", OAuthRedirectURL: "http://app/api/auth/google/callback",
		AllowedOrigins: origins}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	r := gin.New()
	r.GET("/api/auth/google/login", a.GoogleLogin)
	r.GET("/api/auth/google/callback", a.GoogleCallback)
	r.POST("/api/auth/guest", a.Guest)
	r.POST("/api/auth/logout", a.Middleware(), a.Logout)
	r.GET("/api/whoami", a.Middleware(), func(c *gin.Context) {
		c.String(200, auth.CurrentUser(c).ID.String())
	})
	return &env{st, svc, a, r}
}

func do(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	if req.RemoteAddr == "" || req.RemoteAddr == "192.0.2.1:1234" {
		req.RemoteAddr = "10.0.0.1:5555"
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func devUser(t *testing.T, e *env) store.User {
	u, err := e.svc.LoginGoogle(context.Background(), "devsub", devEmail, "")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestMiddlewareCookieAndBearer(t *testing.T) {
	e := setup(t)
	u := devUser(t, e)
	tok, err := e.st.CreateSession(context.Background(), u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/whoami", nil)
	req.AddCookie(&http.Cookie{Name: "pm_session", Value: tok})
	if w := do(e.r, req); w.Code != 200 || w.Body.String() != u.ID.String() {
		t.Fatalf("cookie: %d %s", w.Code, w.Body)
	}
	req = httptest.NewRequest("GET", "/api/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	if w := do(e.r, req); w.Code != 200 || w.Body.String() != u.ID.String() {
		t.Fatalf("bearer: %d %s", w.Code, w.Body)
	}
}

func TestMiddlewareEmptyBearerFallsBackToCookie(t *testing.T) {
	e := setup(t)
	u := devUser(t, e)
	tok, err := e.st.CreateSession(context.Background(), u.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Bearer ", "Bearer    "} {
		req := httptest.NewRequest("GET", "/api/whoami", nil)
		req.Header.Set("Authorization", h)
		req.AddCookie(&http.Cookie{Name: "pm_session", Value: tok})
		if w := do(e.r, req); w.Code != 200 || w.Body.String() != u.ID.String() {
			t.Fatalf("%q: %d %s", h, w.Code, w.Body)
		}
	}
}

func wantErr(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body)
	}
	var b struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || b.Error.Code != code || b.Error.Message == "" {
		t.Fatalf("body %s", w.Body)
	}
}

func TestMiddlewareRejectsMissing(t *testing.T) {
	e := setup(t)
	wantErr(t, do(e.r, httptest.NewRequest("GET", "/api/whoami", nil)), 401, "unauthorized")
	req := httptest.NewRequest("GET", "/api/whoami", nil)
	req.Header.Set("Authorization", "Bearer bogus")
	wantErr(t, do(e.r, req), 401, "unauthorized")
}

func TestCallbackBadState(t *testing.T) {
	e := setup(t)
	req := httptest.NewRequest("GET", "/api/auth/google/callback?state=a&code=x", nil)
	req.AddCookie(&http.Cookie{Name: "pm_oauth_state", Value: "b"})
	wantErr(t, do(e.r, req), 400, "invalid_state")
	req = httptest.NewRequest("GET", "/api/auth/google/callback?state=a&code=x", nil)
	wantErr(t, do(e.r, req), 400, "invalid_state")
}

func TestCallbackCreatesSession(t *testing.T) {
	e := setup(t)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("code") != "thecode" {
				w.WriteHeader(400)
				return
			}
			w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600}`))
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer at" {
				w.WriteHeader(401)
				return
			}
			w.Write([]byte(`{"sub":"g123","email":"Dev@example.com","email_verified":true}`))
		}
	}))
	defer fake.Close()
	e.a.OAuth.Endpoint.TokenURL = fake.URL + "/token"
	e.a.UserInfoURL = fake.URL + "/userinfo"

	// login sets state cookie and redirects
	w := do(e.r, httptest.NewRequest("GET", "/api/auth/google/login", nil))
	if w.Code != http.StatusFound {
		t.Fatalf("login %d", w.Code)
	}
	var state *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_oauth_state" {
			state = c
		}
	}
	if state == nil || state.MaxAge != 600 || !state.HttpOnly {
		t.Fatalf("state cookie %+v", state)
	}
	loc, _ := url.Parse(w.Header().Get("Location"))
	if loc.Query().Get("state") != state.Value {
		t.Fatal("state mismatch in redirect")
	}

	req := httptest.NewRequest("GET", "/api/auth/google/callback?code=thecode&state="+state.Value, nil)
	req.AddCookie(state)
	w = do(e.r, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Fatalf("callback %d %s %s", w.Code, w.Header().Get("Location"), w.Body)
	}
	var sess *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_session" {
			sess = c
		}
	}
	if sess == nil || !sess.HttpOnly || sess.SameSite != http.SameSiteLaxMode || sess.MaxAge != 30*24*3600 {
		t.Fatalf("session cookie %+v", sess)
	}
	u, err := e.st.UserBySession(context.Background(), sess.Value)
	if err != nil || !u.IsDeveloper || *u.GoogleSub != "g123" {
		t.Fatalf("user %+v %v", u, err)
	}
}

func guest(e *env, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/auth/guest", nil)
	req.RemoteAddr = ip + ":1000"
	return do(e.r, req)
}

func TestGuestRateLimit(t *testing.T) {
	e := setup(t)
	devUser(t, e)
	for i := 0; i < 5; i++ {
		if w := guest(e, "10.1.1.1"); w.Code != 201 {
			t.Fatalf("guest %d: %d %s", i, w.Code, w.Body)
		}
	}
	wantErr(t, guest(e, "10.1.1.1"), 429, "rate_limited")
	if w := guest(e, "10.1.1.2"); w.Code != 201 {
		t.Fatalf("other ip %d", w.Code)
	}
}

func TestGuestLogoutDeletesAccount(t *testing.T) {
	e := setup(t)
	devUser(t, e)
	w := guest(e, "10.2.2.2")
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var b struct {
		ID             uuid.UUID `json:"id"`
		Username       string    `json:"username"`
		ConversationID uuid.UUID `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || b.ConversationID == uuid.Nil || !strings.HasPrefix(b.Username, "guest-") {
		t.Fatalf("body %s", w.Body)
	}
	var sess *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_session" {
			sess = c
		}
	}
	if sess == nil || sess.MaxAge != 24*3600 {
		t.Fatalf("cookie %+v", sess)
	}
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(sess)
	w = do(e.r, req)
	if w.Code != 204 {
		t.Fatalf("logout %d %s", w.Code, w.Body)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookie not cleared")
	}
	if _, err := e.st.GetUser(context.Background(), b.ID); err == nil {
		t.Fatal("guest still exists")
	}
	if _, err := e.st.UserBySession(context.Background(), sess.Value); err == nil {
		t.Fatal("session still valid")
	}
}

func TestClientIPIgnoresSpoofedHeader(t *testing.T) {
	e := setup(t)
	r := gin.New()
	r.GET("/ip", func(c *gin.Context) { c.String(200, auth.ClientIP(c)) })
	_ = e
	req := httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = "203.0.113.9:4444"
	req.Header.Set("X-Real-IP", "1.2.3.4")
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	if w := do(r, req); w.Body.String() != "203.0.113.9" {
		t.Fatalf("spoofed: %s", w.Body)
	}
	req = httptest.NewRequest("GET", "/ip", nil)
	req.RemoteAddr = "127.0.0.1:4444"
	req.Header.Set("X-Real-IP", "1.2.3.4")
	if w := do(r, req); w.Body.String() != "1.2.3.4" {
		t.Fatalf("proxy: %s", w.Body)
	}
}

func fakeGoogle(t *testing.T, e *env, tokenStatus, infoStatus int, info string) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			w.WriteHeader(tokenStatus)
			w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600}`))
		case "/userinfo":
			w.WriteHeader(infoStatus)
			w.Write([]byte(info))
		}
	}))
	t.Cleanup(fake.Close)
	e.a.OAuth.Endpoint.TokenURL = fake.URL + "/token"
	e.a.UserInfoURL = fake.URL + "/userinfo"
}

func callback(e *env, query string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/auth/google/callback?state=s"+query, nil)
	req.AddCookie(&http.Cookie{Name: "pm_oauth_state", Value: "s"})
	return do(e.r, req)
}

func wantOAuthFailed(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/?error=oauth_failed" {
		t.Fatalf("got %d %q %s", w.Code, w.Header().Get("Location"), w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_session" {
			t.Fatal("session cookie set")
		}
	}
}

func TestCallbackRejectsUnverifiedEmail(t *testing.T) {
	for _, info := range []string{
		`{"sub":"g1","email":"dev@example.com","email_verified":false}`,
		`{"sub":"g1","email":"dev@example.com"}`,
	} {
		e := setup(t)
		fakeGoogle(t, e, 200, 200, info)
		wantOAuthFailed(t, callback(e, "&code=c"))
		if _, err := e.st.GetDeveloper(context.Background()); err == nil {
			t.Fatal("developer created from unverified email")
		}
	}
}

func TestCallbackProviderErrorRedirects(t *testing.T) {
	ok := `{"sub":"g1","email":"a@b.c","email_verified":true}`
	cases := []struct {
		name     string
		tok, inf int
		query    string
	}{
		{"missing code", 200, 200, ""},
		{"access denied", 200, 200, "&error=access_denied"},
		{"exchange fails", 400, 200, "&code=c"},
		{"userinfo non-200", 200, 500, "&code=c"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			fakeGoogle(t, e, tc.tok, tc.inf, ok)
			wantOAuthFailed(t, callback(e, tc.query))
		})
	}
}

func TestCallbackStoresGoogleName(t *testing.T) {
	e := setup(t)
	fakeGoogle(t, e, 200, 200, `{"sub":"g9","email":"b@x.com","email_verified":true,"name":"Budi Santoso"}`)
	w := callback(e, "&code=c")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/" {
		t.Fatalf("callback %d %s", w.Code, w.Header().Get("Location"))
	}
	var n string
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT display_name FROM users WHERE google_sub='g9'`).Scan(&n); err != nil || n != "Budi Santoso" {
		t.Fatalf("display_name %q %v", n, err)
	}
}

type guestNotifier struct {
	calls   int
	outcome store.EndGuestOutcome
	user    uuid.UUID
}

func (g *guestNotifier) NotifyGuestEnded(o store.EndGuestOutcome, _ []store.EndedConversation, u uuid.UUID) {
	g.calls++
	g.outcome, g.user = o, u
}

func sessionCookie(t *testing.T, w *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == "pm_session" {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

// talkingGuest creates a guest via HTTP who sends one message to the developer.
func talkingGuest(t *testing.T, e *env) (store.User, *http.Cookie) {
	t.Helper()
	ctx := context.Background()
	w := guest(e, "10.3.3.3")
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var b struct {
		ID             uuid.UUID `json:"id"`
		ConversationID uuid.UUID `json:"conversation_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	g, err := e.st.GetUser(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.svc.Send(ctx, g, b.ConversationID, uuid.New(), "hi"); err != nil {
		t.Fatal(err)
	}
	return g, sessionCookie(t, w)
}

func TestGuestLogoutDeactivates(t *testing.T) {
	e := setup(t)
	n := &guestNotifier{}
	e.a.Notifier = n
	ctx := context.Background()
	dev, err := e.svc.SetUsername(ctx, devUser(t, e), "devuser")
	if err != nil {
		t.Fatal(err)
	}
	g, sess := talkingGuest(t, e)
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(sess)
	if w := do(e.r, req); w.Code != 204 {
		t.Fatalf("logout %d %s", w.Code, w.Body)
	}
	req = httptest.NewRequest("GET", "/api/whoami", nil)
	req.AddCookie(sess)
	wantErr(t, do(e.r, req), 401, "unauthorized")
	if n.calls != 1 || n.outcome != store.GuestDeactivated || n.user != g.ID {
		t.Fatalf("notifier %+v", n)
	}
	cs, err := e.svc.Conversations(ctx, dev)
	if err != nil || len(cs) != 1 || !cs[0].PeerEnded {
		t.Fatalf("cs=%+v err=%v", cs, err)
	}
}

func TestEndedGuestCookieRejected(t *testing.T) {
	e := setup(t)
	devUser(t, e)
	ctx := context.Background()
	g, sess := talkingGuest(t, e)
	if _, _, err := e.svc.EndGuest(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/api/whoami", nil)
	req.AddCookie(sess)
	wantErr(t, do(e.r, req), 401, "unauthorized")
	req = httptest.NewRequest("POST", "/api/auth/guest", nil)
	req.RemoteAddr = "10.4.4.4:1000"
	req.AddCookie(sess)
	w := do(e.r, req)
	if w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var b struct {
		ID uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil || b.ID == g.ID || b.ID == uuid.Nil {
		t.Fatalf("new guest %s (old %s)", b.ID, g.ID)
	}
	if old, err := e.st.GetUser(ctx, g.ID); err != nil || old.DeactivatedAt == nil {
		t.Fatalf("old guest %+v %v", old, err)
	}
}

func guestWithOrigin(e *env, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/auth/guest", nil)
	req.RemoteAddr = "10.2.2.2:1000"
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return do(e.r, req)
}

func guestBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	if w.Code != 201 {
		t.Fatalf("status %d %s", w.Code, w.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestGuestTokenAllowedOrigin(t *testing.T) {
	e := setupOrigins(t, []string{"https://port.example"})
	devUser(t, e)
	w := guestWithOrigin(e, "https://port.example")
	body := guestBody(t, w)
	tok, _ := body["token"].(string)
	if tok == "" {
		t.Fatalf("no token: %v", body)
	}
	u, err := e.st.UserBySession(context.Background(), tok)
	if err != nil || u.ID.String() != body["id"] {
		t.Fatalf("token user %+v %v, want %v", u, err, body["id"])
	}
	if sessionCookie(t, w).Value != tok {
		t.Fatal("cookie not set to same session")
	}
}

func TestGuestTokenSameOrigin(t *testing.T) {
	e := setupOrigins(t, []string{"https://port.example", "http://example.com"})
	devUser(t, e)
	if _, ok := guestBody(t, guestWithOrigin(e, "http://example.com"))["token"]; ok {
		t.Fatal("token returned for same origin")
	}
}

func TestGuestTokenForeignOrigin(t *testing.T) {
	e := setupOrigins(t, []string{"https://port.example"})
	devUser(t, e)
	if _, ok := guestBody(t, guestWithOrigin(e, "https://evil.example"))["token"]; ok {
		t.Fatal("token returned for foreign origin")
	}
}

func TestGuestTokenNoOrigin(t *testing.T) {
	e := setupOrigins(t, []string{"https://port.example"})
	devUser(t, e)
	if _, ok := guestBody(t, guestWithOrigin(e, ""))["token"]; ok {
		t.Fatal("token returned without origin")
	}
}
