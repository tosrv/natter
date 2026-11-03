package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/chat"
)

// Guest creates a guest account with a conversation to the developer.
func (a *Auth) Guest(c *gin.Context) {
	// Verify the captcha first so failed attempts never consume a limiter token or slot.
	var body struct {
		TurnstileToken string `json:"turnstile_token"`
	}
	if a.Turnstile.Enabled() {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
		_ = c.ShouldBindJSON(&body) // empty/invalid/oversized body => empty token => captcha_failed
		if len(body.TurnstileToken) > 2048 {
			WriteError(c, chat.ErrCaptchaFailed)
			return
		}
		if err := a.Turnstile.Verify(c.Request.Context(), body.TurnstileToken, ClientIP(c)); err != nil {
			WriteError(c, err)
			return
		}
	}
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
