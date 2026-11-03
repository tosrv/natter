package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store/storetest"
)

// setupTS builds an env with Turnstile enabled against a fake siteverify
// server driven by h.
func setupTS(t *testing.T, h http.HandlerFunc) *env {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1", TurnstileSecretKey: "sekrit", TurnstileSiteKey: "site"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	a.Turnstile.VerifyURL = srv.URL
	r := gin.New()
	r.POST("/api/auth/guest", a.Guest)
	return &env{st, svc, a, r}
}

func guestTok(e *env, ip, tok string) *httptest.ResponseRecorder {
	body := `{"turnstile_token":"` + tok + `"}`
	req := httptest.NewRequest("POST", "/api/auth/guest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1000"
	return do(e.r, req)
}

func usedSlots(t *testing.T, e *env) int {
	t.Helper()
	var n int
	if err := e.st.Pool().QueryRow(context.Background(), `SELECT used FROM user_slots WHERE id=1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func okSiteverify(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"success":true}`)) }

func TestGuestTurnstileOK(t *testing.T) {
	var got struct{ secret, response, remoteip, method string }
	e := setupTS(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got.method, got.secret, got.response, got.remoteip = r.Method, r.PostForm.Get("secret"), r.PostForm.Get("response"), r.PostForm.Get("remoteip")
		okSiteverify(w, r)
	})
	devUser(t, e)
	if w := guestTok(e, "10.3.3.3", "tok123"); w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if got.method != "POST" || got.secret != "sekrit" || got.response != "tok123" || got.remoteip != "10.3.3.3" {
		t.Fatalf("siteverify got %+v", got)
	}
}

func TestGuestTurnstileFailed(t *testing.T) {
	e := setupTS(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("response") == "good" {
			okSiteverify(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	})
	devUser(t, e)
	before := usedSlots(t, e)
	for i := 0; i < 6; i++ {
		wantErr(t, guestTok(e, "10.4.4.4", "bad"), 400, "captcha_failed")
	}
	if usedSlots(t, e) != before {
		t.Fatal("slot changed after failed captcha")
	}
	if w := guestTok(e, "10.4.4.4", "good"); w.Code != 201 {
		t.Fatalf("failed captchas consumed rate limit: %d %s", w.Code, w.Body)
	}
}

func TestGuestTurnstileEmptyToken(t *testing.T) {
	var calls atomic.Int32
	e := setupTS(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); okSiteverify(w, r) })
	devUser(t, e)
	wantErr(t, guestTok(e, "10.4.4.5", ""), 400, "captcha_failed")
	wantErr(t, guest(e, "10.4.4.5"), 400, "captcha_failed") // no body at all
	if calls.Load() != 0 {
		t.Fatal("siteverify should not be called for empty token")
	}
}

func TestGuestTurnstileUnavailable(t *testing.T) {
	e := setupTS(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
	devUser(t, e)
	before := usedSlots(t, e)
	wantErr(t, guestTok(e, "10.5.5.5", "x"), 503, "captcha_unavailable")
	if usedSlots(t, e) != before {
		t.Fatal("slot changed")
	}

	e = setupTS(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
		okSiteverify(w, r)
	})
	devUser(t, e)
	e.a.Turnstile.Client.Timeout = 100 * time.Millisecond
	wantErr(t, guestTok(e, "10.5.5.6", "x"), 503, "captcha_unavailable")

	// network error
	e.a.Turnstile.VerifyURL = "http://127.0.0.1:1"
	wantErr(t, guestTok(e, "10.5.5.7", "x"), 503, "captcha_unavailable")
}

func TestGuestTurnstileDisabled(t *testing.T) {
	e := setup(t)
	devUser(t, e)
	if w := guest(e, "10.6.6.6"); w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestGuestValidTokenSlotFull(t *testing.T) {
	e := setupTS(t, okSiteverify)
	devUser(t, e)
	if _, err := e.st.Pool().Exec(context.Background(), `UPDATE user_slots SET used=300`); err != nil {
		t.Fatal(err)
	}
	wantErr(t, guestTok(e, "10.7.7.7", "good"), 403, "user_limit_reached")
}
