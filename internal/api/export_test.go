package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// EndGuestAndClear exposes the guest-end helper to external tests.
func EndGuestAndClear(d Deps, c *gin.Context, id uuid.UUID) {
	h := &handlers{chat: d.Chat, limiter: d.SearchLimiter, auth: d.Auth, notifier: d.Notifier}
	h.endGuestAndClear(c, id)
}
