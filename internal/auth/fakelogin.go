package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/pmess/internal/chat"
)

// FakeLoginEnabled reports whether the E2E-only login route should be
// registered (E2E_FAKE_LOGIN=1). It must never be enabled in production.
func (a *Auth) FakeLoginEnabled() bool { return a.cfg.E2EFakeLogin }

// FakeLogin (POST /api/test/login {sub,email,username?}) logs in as an
// arbitrary Google identity without OAuth. Only for end-to-end tests.
func (a *Auth) FakeLogin(c *gin.Context) {
	var in struct {
		Sub         string `json:"sub"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || in.Sub == "" || in.Email == "" {
		WriteError(c, &chat.Error{Code: "bad_request", Status: http.StatusBadRequest})
		return
	}
	ctx := c.Request.Context()
	u, err := a.svc.LoginGoogle(ctx, in.Sub, in.Email, in.DisplayName)
	if err != nil {
		WriteError(c, err)
		return
	}
	if in.Username != "" && u.Username == nil {
		if u, err = a.svc.SetUsername(ctx, u, in.Username); err != nil {
			WriteError(c, err)
			return
		}
	}
	if a.startSession(c, u, googleTTL) {
		c.JSON(http.StatusOK, gin.H{"id": u.ID, "username": u.Username})
	}
}
