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
	sub, email, name, err := a.fetchIdentity(c)
	if err != nil {
		_ = c.Error(err)
		c.Redirect(http.StatusFound, "/?error=oauth_failed")
		return
	}
	u, err := a.svc.LoginGoogle(ctx, sub, email, name)
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

// fetchIdentity exchanges the code and returns the verified Google identity.
// Any provider-side failure (error param, missing code, failed exchange,
// bad userinfo, unverified email) is returned as an error.
func (a *Auth) fetchIdentity(c *gin.Context) (string, string, string, error) {
	if e := c.Query("error"); e != "" {
		return "", "", "", fmt.Errorf("oauth provider error: %s", e)
	}
	code := c.Query("code")
	if code == "" {
		return "", "", "", errors.New("oauth: missing code")
	}
	ctx := c.Request.Context()
	tok, err := a.OAuth.Exchange(ctx, code)
	if err != nil {
		return "", "", "", fmt.Errorf("oauth exchange: %w", err)
	}
	resp, err := a.OAuth.Client(ctx, tok).Get(a.UserInfoURL)
	if err != nil {
		return "", "", "", fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("userinfo: status %d", resp.StatusCode)
	}
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", "", fmt.Errorf("userinfo decode: %w", err)
	}
	if info.Sub == "" || info.Email == "" || !info.EmailVerified {
		return "", "", "", errors.New("userinfo: missing sub/email or email not verified")
	}
	return info.Sub, info.Email, info.Name, nil
}
