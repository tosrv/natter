package config_test

import (
	"testing"

	"github.com/tosrv/natter/internal/config"
)

func TestLoadRequiresDeveloperEmail(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEVELOPER_EMAIL", "")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error without DEVELOPER_EMAIL")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DEVELOPER_EMAIL", "a@b.c")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error without DATABASE_URL")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEVELOPER_EMAIL", "a@b.c")
	t.Setenv("ALLOWED_ORIGINS", "http://a.com, http://b.com")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("TRUSTED_PROXY", "")
	t.Setenv("STATIC_DIR", "")
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8080" || c.TrustedProxy != "127.0.0.1" || !c.CookieSecure || c.StaticDir != "web/dist" {
		t.Fatalf("bad defaults: %+v", c)
	}
	if len(c.AllowedOrigins) != 2 || c.AllowedOrigins[1] != "http://b.com" {
		t.Fatalf("bad origins: %v", c.AllowedOrigins)
	}
}

func TestLoadRefusesFakeLoginWithSecureCookie(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("DEVELOPER_EMAIL", "a@b.c")
	t.Setenv("E2E_FAKE_LOGIN", "1")
	t.Setenv("COOKIE_SECURE", "true")
	if _, err := config.Load(); err == nil {
		t.Fatal("expected error with E2E_FAKE_LOGIN=1 and COOKIE_SECURE=true")
	}
	t.Setenv("COOKIE_SECURE", "false")
	c, err := config.Load()
	if err != nil || !c.E2EFakeLogin {
		t.Fatalf("fake login with insecure cookie should load: %v", err)
	}
}

func TestGuestLimitPerHour(t *testing.T) {
	if got := (config.Config{}).GuestLimitPerHour(); got != 5 {
		t.Fatalf("default = %d, want 5", got)
	}
	if got := (config.Config{E2EFakeLogin: true}).GuestLimitPerHour(); got != 1000 {
		t.Fatalf("e2e = %d, want 1000", got)
	}
}

func TestLoadTurnstilePair(t *testing.T) {
	cases := []struct {
		site, secret string
		ok           bool
	}{{"s", "", false}, {"", "k", false}, {"s", "k", true}, {"", "", true}}
	for _, tc := range cases {
		t.Setenv("DATABASE_URL", "postgres://x")
		t.Setenv("DEVELOPER_EMAIL", "a@b.c")
		t.Setenv("TURNSTILE_SITE_KEY", tc.site)
		t.Setenv("TURNSTILE_SECRET_KEY", tc.secret)
		c, err := config.Load()
		if (err == nil) != tc.ok {
			t.Fatalf("site=%q secret=%q: err=%v", tc.site, tc.secret, err)
		}
		if tc.ok && (c.TurnstileSiteKey != tc.site || c.TurnstileSecretKey != tc.secret) {
			t.Fatalf("keys not loaded: %+v", c)
		}
	}
}
