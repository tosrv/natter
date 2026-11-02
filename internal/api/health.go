package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Pinger checks that a dependency (the database) is reachable.
type Pinger interface{ Ping(context.Context) error }

// Health serves GET /healthz: 200 when p.Ping succeeds within 2 s, else 503.
// A nil p is reported as unavailable.
func Health(p Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			if p.Ping(ctx) == nil {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
				return
			}
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
	}
}
