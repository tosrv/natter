package config_test

import (
	"testing"

	"github.com/tosrv/pmess/internal/config"
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
