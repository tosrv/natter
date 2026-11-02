package api

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const requestIDHeader = "X-Request-Id"

// RequestLog logs one line per request: method, path (no query), status,
// duration_ms and request_id. It never logs bodies, query strings, headers
// or cookies. /healthz is logged at debug level, everything else at info.
func RequestLog(l *slog.Logger) gin.HandlerFunc {
	if l == nil {
		l = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		id := c.GetHeader(requestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		c.Writer.Header().Set(requestIDHeader, id)
		c.Next()

		level := slog.LevelInfo
		if c.Request.URL.Path == "/healthz" {
			level = slog.LevelDebug
		}
		l.LogAttrs(c.Request.Context(), level, "request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
			slog.String("request_id", id),
		)
	}
}

// validRequestID accepts short printable-ASCII ids so a client cannot inject
// oversized or odd values into logs and response headers.
func validRequestID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x21 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Recover turns a handler panic into a 500 and logs only err, request_id,
// method and path (gin.Recovery would dump the raw request incl. Cookie).
func Recover(l *slog.Logger) gin.HandlerFunc {
	if l == nil {
		l = slog.Default()
	}
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				if r == http.ErrAbortHandler {
					panic(r)
				}
				l.LogAttrs(c.Request.Context(), slog.LevelError, "panic",
					slog.Any("err", r),
					slog.String("request_id", c.Writer.Header().Get(requestIDHeader)),
					slog.String("method", c.Request.Method),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					gin.H{"error": gin.H{"code": "internal", "message": "internal error"}})
			}
		}()
		c.Next()
	}
}
