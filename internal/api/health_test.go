package api_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tosrv/natter/internal/api"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/config"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store/storetest"
)

type pingFn func(context.Context) error

func (f pingFn) Ping(ctx context.Context) error { return f(ctx) }

func opsRouter(t *testing.T, p interface{ Ping(context.Context) error }, l *slog.Logger) http.Handler {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	return api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil),
		Pinger: p, Logger: l, StaticDir: t.TempDir()})
}

func get(h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestHealthOK(t *testing.T) {
	h := opsRouter(t, pingFn(func(context.Context) error { return nil }), nil)
	w := get(h, "/healthz", nil)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestHealthDown(t *testing.T) {
	h := opsRouter(t, pingFn(func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }), nil)
	start := time.Now()
	w := get(h, "/healthz", nil)
	if d := time.Since(start); d > 2500*time.Millisecond {
		t.Fatalf("took %v", d)
	}
	if w.Code != 503 || strings.TrimSpace(w.Body.String()) != `{"status":"unavailable"}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestHealthNilPinger(t *testing.T) {
	w := get(opsRouter(t, nil, nil), "/healthz", nil)
	if w.Code != 503 {
		t.Fatalf("%d", w.Code)
	}
}

func TestHealthNoAuthNoRateLimit(t *testing.T) {
	h := opsRouter(t, pingFn(func(context.Context) error { return nil }), nil)
	for i := 0; i < 101; i++ {
		if w := get(h, "/healthz", map[string]string{"Origin": "http://evil.example"}); w.Code != 200 {
			t.Fatalf("req %d: %d", i, w.Code)
		}
	}
}

func TestHealthzLoggedAtDebug(t *testing.T) {
	var b bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&b, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h := opsRouter(t, pingFn(func(context.Context) error { return nil }), l)
	get(h, "/healthz", nil)
	if b.Len() != 0 {
		t.Fatalf("healthz logged at info: %s", b.String())
	}
}

func TestHealthzLoggedWhenDebug(t *testing.T) {
	var b bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&b, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := opsRouter(t, pingFn(func(context.Context) error { return nil }), l)
	get(h, "/healthz", nil)
	if !strings.Contains(b.String(), `"path":"/healthz"`) {
		t.Fatalf("not logged: %s", b.String())
	}
}
