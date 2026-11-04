package auth

import (
	"slices"

	"github.com/gin-gonic/gin"
)

// embedOrigin reports whether the request comes from an allowed cross-site
// origin (e.g. the chat widget embedded on another site). Such clients cannot
// rely on cookies, so they receive the session token in the response body.
func embedOrigin(c *gin.Context, allowed []string, cookieSecure bool) bool {
	origin := c.GetHeader("Origin")
	if origin == "" || !slices.Contains(allowed, origin) {
		return false
	}
	scheme := "http"
	if cookieSecure || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return origin != scheme+"://"+c.Request.Host
}
