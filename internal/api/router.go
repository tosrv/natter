package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/auth"
	"github.com/tosrv/pmess/internal/chat"
	"github.com/tosrv/pmess/internal/ratelimit"
)

type Deps struct {
	Auth           *auth.Auth
	Chat           *chat.Service
	SearchLimiter  *ratelimit.Limiter
	WS             gin.HandlerFunc
	StaticDir      string
	AllowedOrigins []string
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), cors(d.AllowedOrigins))
	h := &handlers{chat: d.Chat, limiter: d.SearchLimiter}

	api := r.Group("/api")
	api.GET("/auth/google/login", d.Auth.GoogleLogin)
	api.GET("/auth/google/callback", d.Auth.GoogleCallback)
	api.POST("/auth/guest", d.Auth.Guest)

	authed := api.Group("", d.Auth.Middleware())
	authed.POST("/auth/logout", d.Auth.Logout)
	authed.GET("/me", h.me)
	authed.PUT("/me/username", h.setUsername)
	authed.GET("/users/search", h.search)
	authed.POST("/conversations", h.openConversation)
	authed.GET("/conversations", h.conversations)
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
				hd.Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
				hd.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				hd.Set("Access-Control-Max-Age", "600")
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}
