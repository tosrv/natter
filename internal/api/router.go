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
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/ratelimit"
	"github.com/tosrv/pmess/internal/store"
)

// Notifier is told about conversations removed by an account deletion.
type Notifier interface {
	NotifyConversationDeleted(peers []store.DeletedConversation, deletedUser uuid.UUID)
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
	authed.GET("/conversations/:id/messages", h.messages)
	if d.WS != nil {
		authed.GET("/ws", d.WS)
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") || d.StaticDir == "" ||
			(c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			WriteError(c, chat.ErrNotFound)
			return
		}
		f := filepath.Join(d.StaticDir, filepath.FromSlash(path.Clean("/"+p)))
		if st, err := os.Stat(f); err != nil || st.IsDir() {
			f = filepath.Join(d.StaticDir, "index.html")
		}
		c.File(f)
	})
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
