package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tosrv/pmess/internal/api"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store/storetest"
)

func fakeLogin(h http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/test/login",
		strings.NewReader(`{"sub":"e2e-1","email":"a@e2e.test","username":"alice_e2e"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestFakeLoginDisabledByDefault(t *testing.T) {
	t.Setenv("E2E_FAKE_LOGIN", "")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEVELOPER_EMAIL", devEmail)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.E2EFakeLogin {
		t.Fatal("E2EFakeLogin enabled without env")
	}
	e := setup(t, "")
	if w := fakeLogin(e.r); w.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestFakeLoginEnabled(t *testing.T) {
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1", E2EFakeLogin: true}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	r := api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil)})
	w := fakeLogin(r)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body)
	}
	var tok string
	for _, c := range w.Result().Cookies() {
		if c.Name == auth.SessionCookie {
			tok = c.Value
		}
	}
	if tok == "" {
		t.Fatal("no session cookie")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != 200 || !strings.Contains(w2.Body.String(), "alice_e2e") {
		t.Fatalf("me: %d %s", w2.Code, w2.Body)
	}
}
