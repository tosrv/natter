// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	GoogleClientID     string
	GoogleClientSecret string
	OAuthRedirectURL   string
	DeveloperEmail     string
	AllowedOrigins     []string
	CookieSecure       bool
	ListenAddr         string
	TrustedProxy       string
	StaticDir          string
	LogLevel           string
	// Turnstile keys: both empty disables the captcha; exactly one is an error.
	TurnstileSiteKey   string
	TurnstileSecretKey string
	// E2EFakeLogin enables POST /api/test/login. Never set in production.
	E2EFakeLogin bool
}

// GuestLimitPerHour is the per-IP guest creation limit. It is raised only when
// E2E fake login is on (which Load refuses together with COOKIE_SECURE=true),
// so repeated E2E runs do not hit the production limit.
func (c Config) GuestLimitPerHour() int {
	if c.E2EFakeLogin {
		return 1000
	}
	return 5
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		OAuthRedirectURL:   os.Getenv("OAUTH_REDIRECT_URL"),
		DeveloperEmail:     strings.TrimSpace(os.Getenv("DEVELOPER_EMAIL")),
		CookieSecure:       os.Getenv("COOKIE_SECURE") == "true",
		ListenAddr:         envDefault("LISTEN_ADDR", ":8080"),
		TrustedProxy:       envDefault("TRUSTED_PROXY", "127.0.0.1"),
		StaticDir:          envDefault("STATIC_DIR", "web/dist"),
		LogLevel:           envDefault("LOG_LEVEL", "info"),
		TurnstileSiteKey:   strings.TrimSpace(os.Getenv("TURNSTILE_SITE_KEY")),
		TurnstileSecretKey: strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY")),
		E2EFakeLogin:       os.Getenv("E2E_FAKE_LOGIN") == "1",
	}
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if c.DeveloperEmail == "" {
		return Config{}, errors.New("DEVELOPER_EMAIL is required")
	}
	if (c.TurnstileSiteKey == "") != (c.TurnstileSecretKey == "") {
		return Config{}, errors.New("TURNSTILE_SITE_KEY and TURNSTILE_SECRET_KEY must both be set or both be empty")
	}
	if c.E2EFakeLogin && c.CookieSecure {
		return Config{}, errors.New("refusing to start: E2E_FAKE_LOGIN=1 is not allowed with COOKIE_SECURE=true (production); unset E2E_FAKE_LOGIN")
	}
	return c, nil
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
