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
	return c, nil
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
