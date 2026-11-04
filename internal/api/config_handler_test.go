package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tosrv/natter/internal/api"
	"github.com/tosrv/natter/internal/auth"
	"github.com/tosrv/natter/internal/chat"
	"github.com/tosrv/natter/internal/config"
	"github.com/tosrv/natter/internal/ratelimit"
	"github.com/tosrv/natter/internal/store/storetest"
)

func TestConfigEndpoint(t *testing.T) {
	st := storetest.New(t)
	svc := chat.New(st, devEmail)
	cfg := config.Config{DeveloperEmail: devEmail, TrustedProxy: "127.0.0.1", TurnstileSiteKey: "site-abc", TurnstileSecretKey: "sec"}
	a := auth.New(st, svc, cfg, ratelimit.New(5, time.Hour, nil))
	r := api.NewRouter(api.Deps{Auth: a, Chat: svc, SearchLimiter: ratelimit.New(30, time.Minute, nil), TurnstileSiteKey: cfg.TurnstileSiteKey})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if w.Code != 200 || w.Body.String() != `{"turnstile_site_key":"site-abc"}` {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
