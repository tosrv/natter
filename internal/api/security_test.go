package api_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/tosrv/pmess/internal/api"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store/storetest"
)

func secRouter(t *testing.T, hsts bool) http.Handler {
	t.Helper()
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	dir := t.TempDir()
	if err := writeFile(filepath.Join(dir, "index.html"), "<html></html>"); err != nil {
		t.Fatal(err)
	}
	return api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil),
		Pinger: pingFn(func(context.Context) error { return nil }), StaticDir: dir, SecureHeadersHSTS: hsts})
}

func TestSecurityHeadersEverywhere(t *testing.T) {
	h := secRouter(t, false)
	want := map[string]string{
		"Content-Security-Policy": api.ContentSecurityPolicy,
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
		"X-Frame-Options":         "DENY",
	}
	for _, p := range []string{"/healthz", "/api/me", "/", "/api/nope"} {
		w := get(h, p, nil)
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%s %s = %q, want %q", p, k, got, v)
			}
		}
	}
	if api.ContentSecurityPolicy != "default-src 'self'; script-src 'self' https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'" {
		t.Error("CSP constant differs from spec")
	}
}

func TestHSTSOnlyWhenSecure(t *testing.T) {
	for _, secure := range []bool{false, true} {
		w := get(secRouter(t, secure), "/healthz", nil)
		got := w.Header().Get("Strict-Transport-Security")
		if secure && got != "max-age=31536000" {
			t.Errorf("secure: HSTS = %q", got)
		}
		if !secure && got != "" {
			t.Errorf("insecure: HSTS = %q", got)
		}
	}
}
