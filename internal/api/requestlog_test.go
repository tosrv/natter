package api_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tosrv/natter/internal/api"
)

func logOne(t *testing.T, b *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("want 1 line, got %d: %q", len(lines), b.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRequestLogFields(t *testing.T) {
	var b bytes.Buffer
	h := opsRouter(t, nil, slog.New(slog.NewJSONHandler(&b, nil)))
	get(h, "/api/me?token=qtokq", map[string]string{"Authorization": "Bearer secret", "Cookie": "pm_session=xyz"})
	m := logOne(t, &b)
	if m["method"] != "GET" || m["path"] != "/api/me" || m["status"] != float64(401) {
		t.Fatalf("%v", m)
	}
	if _, ok := m["duration_ms"]; !ok {
		t.Fatalf("no duration_ms: %v", m)
	}
	if _, ok := m["request_id"]; !ok {
		t.Fatalf("no request_id: %v", m)
	}
	for _, bad := range []string{"qtokq", "secret", "xyz", "token="} {
		if strings.Contains(b.String(), bad) {
			t.Fatalf("log leaks %q: %s", bad, b.String())
		}
	}
}

func TestRequestIDPropagation(t *testing.T) {
	var b bytes.Buffer
	h := opsRouter(t, nil, slog.New(slog.NewJSONHandler(&b, nil)))
	w := get(h, "/api/me", map[string]string{"X-Request-Id": "r-1"})
	if w.Header().Get("X-Request-Id") != "r-1" || logOne(t, &b)["request_id"] != "r-1" {
		t.Fatalf("%q %s", w.Header().Get("X-Request-Id"), b.String())
	}
	b.Reset()
	w = get(h, "/api/me", nil)
	id := w.Header().Get("X-Request-Id")
	if !regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(id) || logOne(t, &b)["request_id"] != id {
		t.Fatalf("%q %s", id, b.String())
	}
}

func TestPanicLogNoCookie(t *testing.T) {
	var b bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&b, nil))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(api.RequestLog(l), api.Recover(l))
	r.GET("/boom", func(*gin.Context) { panic("kaboom") })
	w := get(r, "/boom?token=qtokq", map[string]string{"Cookie": "pm_session=xyz", "Authorization": "Bearer secret"})
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"code":"internal"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if !strings.Contains(b.String(), `"msg":"panic"`) || !strings.Contains(b.String(), `"level":"ERROR"`) {
		t.Fatalf("no panic log: %s", b.String())
	}
	for _, bad := range []string{"xyz", "secret", "qtokq"} {
		if strings.Contains(b.String(), bad) {
			t.Fatalf("leaks %q: %s", bad, b.String())
		}
	}
}
