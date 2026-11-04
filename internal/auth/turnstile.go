package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tosrv/natter/internal/chat"
)

const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile verifies Cloudflare Turnstile tokens. A nil *Turnstile or an empty
// secret disables verification.
type Turnstile struct {
	secret string
	// VerifyURL is the siteverify endpoint; overridable for tests.
	VerifyURL string
	Client    *http.Client
}

func NewTurnstile(secret string) *Turnstile {
	return &Turnstile{secret: secret, VerifyURL: turnstileVerifyURL, Client: &http.Client{Timeout: 5 * time.Second}}
}

// Enabled reports whether tokens are verified.
func (t *Turnstile) Enabled() bool { return t != nil && t.secret != "" }

// Verify returns nil when disabled or the token is valid, captcha_failed for
// an empty/rejected token, and captcha_unavailable when siteverify cannot be
// reached or answers non-200.
func (t *Turnstile) Verify(ctx context.Context, token, remoteIP string) error {
	if !t.Enabled() {
		return nil
	}
	if strings.TrimSpace(token) == "" {
		return chat.ErrCaptchaFailed
	}
	form := url.Values{"secret": {t.secret}, "response": {token}, "remoteip": {remoteIP}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.VerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return chat.ErrCaptchaUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.Client.Do(req)
	if err != nil {
		return chat.ErrCaptchaUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return chat.ErrCaptchaUnavailable
	}
	var out struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return chat.ErrCaptchaUnavailable
	}
	if !out.Success {
		return chat.ErrCaptchaFailed
	}
	return nil
}
