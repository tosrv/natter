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
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":8080" || c.TrustedProxy != "127.0.0.1" || !c.CookieSecure {
		t.Fatalf("bad defaults: %+v", c)
	}
	if len(c.AllowedOrigins) != 2 || c.AllowedOrigins[1] != "http://b.com" {
		t.Fatalf("bad origins: %v", c.AllowedOrigins)
	}
}
