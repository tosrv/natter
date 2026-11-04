package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/natter/internal/chat"
)

// Guest creates a guest account with a conversation to the developer.
func (a *Auth) Guest(c *gin.Context) {
	// Verify the captcha first so failed attempts never consume a limiter token or slot;
	// an invalid name is rejected before the limiter for the same reason.
	var body struct {
		TurnstileToken string `json:"turnstile_token"`
		DisplayName    string `json:"display_name"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
	_ = c.ShouldBindJSON(&body) // empty/invalid/oversized body => empty fields
	if a.Turnstile.Enabled() {
		if len(body.TurnstileToken) > 2048 {
			WriteError(c, chat.ErrCaptchaFailed)
			return
		}
		if err := a.Turnstile.Verify(c.Request.Context(), body.TurnstileToken, ClientIP(c)); err != nil {
			WriteError(c, err)
			return
		}
	}
	name, err := chat.ValidateDisplayName(body.DisplayName)
	if err != nil {
		WriteError(c, err)
		return
	}
	if !a.guestLimiter.Allow("guest:" + ClientIP(c)) {
		WriteError(c, chat.ErrRateLimited)
		return
	}
	u, conv, err := a.svc.CreateGuest(c.Request.Context(), name)
	if err != nil {
		WriteError(c, err)
		return
	}
	tok, ok := a.startSession(c, u, guestTTL)
	if !ok {
		return
	}
	uname := ""
	if u.Username != nil {
		uname = *u.Username
	}
	resp := gin.H{"id": u.ID, "username": uname, "conversation_id": conv}
	if embedOrigin(c, a.cfg.AllowedOrigins, a.cfg.CookieSecure) {
		resp["token"] = tok
	}
	c.JSON(http.StatusCreated, resp)
}
