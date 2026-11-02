package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/chat"
)

var errInvalidState = &chat.Error{Code: "invalid_state", Status: http.StatusBadRequest}

// GoogleLogin sets a random state cookie and redirects to Google.
func (a *Auth) GoogleLogin(c *gin.Context) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		WriteError(c, err)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)
	a.setCookie(c, StateCookie, state, int(stateTTL.Seconds()))
	c.Redirect(http.StatusFound, a.OAuth.AuthCodeURL(state))
}

// GoogleCallback verifies state, exchanges the code, and starts a session.
func (a *Auth) GoogleCallback(c *gin.Context) {
	want, err := c.Cookie(StateCookie)
	got := c.Query("state")
	if err != nil || want == "" || subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		WriteError(c, errInvalidState)
		return
	}
	a.setCookie(c, StateCookie, "", -1)
	ctx := c.Request.Context()
	tok, err := a.OAuth.Exchange(ctx, c.Query("code"))
	if err != nil {
		WriteError(c, fmt.Errorf("oauth exchange: %w", err))
		return
	}
	resp, err := a.OAuth.Client(ctx, tok).Get(a.UserInfoURL)
	if err != nil {
		WriteError(c, fmt.Errorf("userinfo: %w", err))
		return
	}
	defer resp.Body.Close()
	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if resp.StatusCode != http.StatusOK {
		WriteError(c, fmt.Errorf("userinfo: status %d", resp.StatusCode))
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Sub == "" {
		WriteError(c, fmt.Errorf("userinfo decode: %v", err))
		return
	}
	u, err := a.svc.LoginGoogle(ctx, info.Sub, info.Email)
	if errors.Is(err, chat.ErrUserLimitReached) {
		c.Redirect(http.StatusFound, "/?error=user_limit_reached")
		return
	}
	if err != nil {
		WriteError(c, err)
		return
	}
	if a.startSession(c, u, googleTTL) {
		c.Redirect(http.StatusFound, "/")
	}
}
