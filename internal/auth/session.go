// Package auth implements sessions, Google OAuth login and guest accounts.
package auth

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/config"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	SessionCookie = "pm_session"
	StateCookie   = "pm_oauth_state"
	googleTTL     = 30 * 24 * time.Hour
	guestTTL      = 24 * time.Hour
	stateTTL      = 10 * time.Minute
	userKey       = "user"
	tokenKey      = "session_token"
)

// Auth holds the dependencies of the auth handlers.
type Auth struct {
	st           *store.Store
	svc          *chat.Service
	cfg          config.Config
	guestLimiter *ratelimit.Limiter
	// OAuth is the Google OAuth2 config; Endpoint may be overridden in tests.
	OAuth *oauth2.Config
	// UserInfoURL is the OpenID Connect userinfo endpoint.
	UserInfoURL string
}

var trustedProxy atomic.Value // string

func init() { trustedProxy.Store("127.0.0.1") }

func New(st *store.Store, svc *chat.Service, cfg config.Config, guestLimiter *ratelimit.Limiter) *Auth {
	if cfg.TrustedProxy != "" {
		trustedProxy.Store(cfg.TrustedProxy)
	}
	return &Auth{
		st: st, svc: svc, cfg: cfg, guestLimiter: guestLimiter,
		OAuth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.OAuthRedirectURL,
			Scopes:       []string{"openid", "email"},
			Endpoint:     google.Endpoint,
		},
		UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
	}
}

var messages = map[string]string{
	"unauthorized":          "authentication required",
	"rate_limited":          "too many requests",
	"user_limit_reached":    "user limit reached",
	"developer_unavailable": "developer is not available",
	"invalid_state":         "invalid oauth state",
	"internal":              "internal server error",
}

// WriteError writes err as {"error":{"code","message"}} and aborts. Non
// chat.Error values become 500 "internal".
func WriteError(c *gin.Context, err error) {
	var ce *chat.Error
	if !errors.As(err, &ce) {
		_ = c.Error(err)
		ce = &chat.Error{Code: "internal", Status: http.StatusInternalServerError}
	}
	msg := messages[ce.Code]
	if msg == "" {
		msg = strings.ReplaceAll(ce.Code, "_", " ")
	}
	c.AbortWithStatusJSON(ce.Status, gin.H{"error": gin.H{"code": ce.Code, "message": msg}})
}

func token(c *gin.Context) string {
	if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if t := strings.TrimSpace(h[len("Bearer "):]); t != "" {
			return t
		}
	}
	if v, err := c.Cookie(SessionCookie); err == nil {
		return v
	}
	return ""
}

// Middleware authenticates via the pm_session cookie or Bearer token.
func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tok := token(c)
		if tok == "" {
			WriteError(c, chat.ErrUnauthorized)
			return
		}
		u, err := a.st.UserBySession(c.Request.Context(), tok)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				WriteError(c, chat.ErrUnauthorized)
			} else {
				WriteError(c, err)
			}
			return
		}
		c.Set(userKey, u)
		c.Set(tokenKey, tok)
		c.Next()
	}
}

// CurrentUser returns the user set by Middleware.
func CurrentUser(c *gin.Context) store.User {
	u, _ := c.MustGet(userKey).(store.User)
	return u
}

func (a *Auth) setCookie(c *gin.Context, name, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *Auth) startSession(c *gin.Context, u store.User, ttl time.Duration) bool {
	tok, err := a.st.CreateSession(c.Request.Context(), u.ID, ttl)
	if err != nil {
		WriteError(c, err)
		return false
	}
	a.setCookie(c, SessionCookie, tok, int(ttl/time.Second))
	return true
}

// ClearSession expires the session cookie.
func (a *Auth) ClearSession(c *gin.Context) { a.setCookie(c, SessionCookie, "", -1) }

// Logout deletes the session; a guest account is deleted entirely.
func (a *Auth) Logout(c *gin.Context) {
	ctx := c.Request.Context()
	if err := a.st.DeleteSession(ctx, c.GetString(tokenKey)); err != nil {
		WriteError(c, err)
		return
	}
	if u := CurrentUser(c); u.Kind == "guest" {
		if err := a.st.DeleteGuest(ctx, u.ID); err != nil {
			WriteError(c, err)
			return
		}
	}
	a.ClearSession(c)
	c.Status(http.StatusNoContent)
}

// ClientIP trusts X-Real-IP only when the peer is the trusted proxy.
// The trusted proxy address is process-global: it is set by New from
// cfg.TrustedProxy (default 127.0.0.1), so one Auth per process is assumed.
var untrustedXRIOnce sync.Once

func ClientIP(c *gin.Context) string {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	if host == trustedProxy.Load().(string) {
		if x := strings.TrimSpace(c.GetHeader("X-Real-IP")); x != "" {
			return x
		}
	} else if c.GetHeader("X-Real-IP") != "" {
		untrustedXRIOnce.Do(func() {
			slog.Warn("X-Real-IP header ignored: peer is not TRUSTED_PROXY (check TRUSTED_PROXY setting)",
				"peer", host, "trusted_proxy", trustedProxy.Load().(string))
		})
	}
	return host
}
