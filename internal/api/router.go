package api

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store"
)

// Notifier is told about conversations removed by a Google account deletion
// and about guests ended via DELETE /api/me.
type Notifier interface {
	NotifyConversationDeleted(peers []store.DeletedConversation, deletedUser uuid.UUID)
	// NotifyConversationCleared tells userID's own connection that convID was cleared.
	NotifyConversationCleared(userID, convID uuid.UUID)
	chat.GuestNotifier
}

type Deps struct {
	Notifier       Notifier // optional
	Auth           *auth.Auth
	Chat           *chat.Service
	SearchLimiter  *ratelimit.Limiter
	WS             gin.HandlerFunc
	StaticDir      string
	AllowedOrigins []string
	// TurnstileSiteKey is exposed by GET /api/config (empty = captcha disabled).
	TurnstileSiteKey string
	Pinger           Pinger       // optional; nil => /healthz 503
	Logger           *slog.Logger // optional; nil => slog.Default()
	// SecureHeadersHSTS adds Strict-Transport-Security (set from COOKIE_SECURE).
	SecureHeadersHSTS bool
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(SecurityHeaders(d.SecureHeadersHSTS), RequestLog(d.Logger), Recover(d.Logger))
	// Registered before cors and outside /api: no auth, CORS rejection or rate limit.
	r.GET("/healthz", Health(d.Pinger))
	r.Use(cors(d.AllowedOrigins))
	h := &handlers{chat: d.Chat, limiter: d.SearchLimiter, auth: d.Auth, notifier: d.Notifier}

	api := r.Group("/api")
	api.Use(noindex)
	api.GET("/config", ConfigHandler(d.TurnstileSiteKey))
	api.GET("/auth/google/login", d.Auth.GoogleLogin)
	api.GET("/auth/google/callback", d.Auth.GoogleCallback)
	api.POST("/auth/guest", d.Auth.Guest)
	if d.Auth.FakeLoginEnabled() {
		api.POST("/test/login", d.Auth.FakeLogin)
	}

	authed := api.Group("", d.Auth.Middleware())
	authed.POST("/auth/logout", d.Auth.Logout)
	authed.GET("/me", h.me)
	authed.PUT("/me/username", h.setUsername)
	authed.PUT("/me/display-name", h.setDisplayName)
	authed.DELETE("/me", h.deleteMe)
	authed.GET("/users/search", h.search)
	authed.POST("/conversations", h.openConversation)
	authed.GET("/conversations", h.conversations)
	authed.POST("/blocks", h.block)
	authed.DELETE("/blocks/:user_id", h.unblock)
	authed.GET("/blocks", h.blocks)
	authed.DELETE("/conversations/:id", h.clearConversation)
	authed.GET("/conversations/:id/messages", h.messages)
	if d.WS != nil {
		authed.GET("/ws", d.WS)
	}

	r.NoRoute(func(c *gin.Context) { serveStatic(c, d.StaticDir) })
	return r
}

func cors(allowed []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		o := c.GetHeader("Origin")
		if o != "" && slices.Contains(allowed, o) {
			hd := c.Writer.Header()
			hd.Set("Access-Control-Allow-Origin", o)
			hd.Set("Access-Control-Allow-Credentials", "true")
			hd.Add("Vary", "Origin")
			if c.Request.Method == http.MethodOptions {
				hd.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				hd.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				hd.Set("Access-Control-Max-Age", "600")
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

// privateSPARoutes are SPA screens served with the clean app.html shell (no
// prerendered landing, canonical or JSON-LD; index.html if app.html is missing) but kept out of search indexes.
var privateSPARoutes = []string{"/chat", "/settings", "/onboarding"}

// publicRoutes are indexable pages (static files or the SPA root).
var publicRoutes = []string{"/", "/privacy", "/terms"}

const robotsNoIndexFollow = "noindex, nofollow"

func noindex(c *gin.Context) {
	c.Header("X-Robots-Tag", robotsNoIndexFollow)
	c.Next()
}

// serveStatic serves files from dir: real files and directory indexes with 200,
// private SPA routes as the app shell with noindex, and anything else as a 404
// that still carries the app shell (so the SPA can render its not-found view).
func serveStatic(c *gin.Context, dir string) {
	p := c.Request.URL.Path
	if p == "/api" || strings.HasPrefix(p, "/api/") {
		c.Header("X-Robots-Tag", robotsNoIndexFollow)
		WriteError(c, chat.ErrNotFound)
		return
	}
	if dir == "" || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
		WriteError(c, chat.ErrNotFound)
		return
	}
	clean := path.Clean("/" + p)
	index := filepath.Join(dir, "index.html")
	shell := filepath.Join(dir, "app.html")
	if st, err := os.Stat(shell); err != nil || !st.Mode().IsRegular() {
		shell = index
	}
	if clean == "/widget.js" {
		hd := c.Writer.Header()
		hd.Set("Cross-Origin-Resource-Policy", "cross-origin")
		hd.Set("Cache-Control", "public, max-age=300")
	}
	f := filepath.Join(dir, filepath.FromSlash(clean))
	if st, err := os.Stat(f); err == nil {
		if st.IsDir() {
			f = filepath.Join(f, "index.html")
			st, err = os.Stat(f)
		}
		if err == nil && st.Mode().IsRegular() {
			switch filepath.Ext(f) {
			case ".txt":
				c.Header("Content-Type", "text/plain; charset=utf-8")
			case ".xml":
				c.Header("Content-Type", "application/xml; charset=utf-8")
			}
			if (f == index && !slices.Contains(publicRoutes, clean)) || f == filepath.Join(dir, "app.html") {
				c.Header("X-Robots-Tag", robotsNoIndexFollow)
			}
			c.File(f)
			return
		}
	}
	if slices.Contains(privateSPARoutes, clean) {
		c.Header("X-Robots-Tag", robotsNoIndexFollow)
		c.File(shell)
		return
	}
	body, err := os.ReadFile(shell)
	if err != nil {
		WriteError(c, chat.ErrNotFound)
		return
	}
	c.Header("X-Robots-Tag", "noindex")
	c.Data(http.StatusNotFound, "text/html; charset=utf-8", body)
}
