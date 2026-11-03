package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ConfigHandler serves public client configuration (GET /api/config).
func ConfigHandler(turnstileSiteKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"turnstile_site_key": turnstileSiteKey})
	}
}
