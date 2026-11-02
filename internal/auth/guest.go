package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/chat"
)

// Guest creates a guest account with a conversation to the developer.
func (a *Auth) Guest(c *gin.Context) {
	if !a.guestLimiter.Allow("guest:" + ClientIP(c)) {
		WriteError(c, chat.ErrRateLimited)
		return
	}
	u, conv, err := a.svc.CreateGuest(c.Request.Context())
	if err != nil {
		WriteError(c, err)
		return
	}
	if !a.startSession(c, u, guestTTL) {
		return
	}
	name := ""
	if u.Username != nil {
		name = *u.Username
	}
	c.JSON(http.StatusCreated, gin.H{"id": u.ID, "username": name, "conversation_id": conv})
}
