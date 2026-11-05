package api_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	idxBody     = "<html>spa-index</html>"
	privacyBody = "<html>privacy-page</html>"
	termsBody   = "<html>terms-page</html>"
	appBody     = "<html>app-shell</html>"
)

func seoEnv(t *testing.T) *env { return seoEnvWith(t, true) }

func seoEnvWith(t *testing.T, withApp bool) *env {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":         idxBody,
		"privacy/index.html": privacyBody,
		"terms/index.html":   termsBody,
		"robots.txt":         "User-agent: *",
		"sitemap.xml":        "<urlset/>",
		"llms.txt":           "# natter",
	}
	if withApp {
		files["app.html"] = appBody
	}
	for p, s := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := writeFile(full, s); err != nil {
			t.Fatal(err)
		}
	}
	return setup(t, dir)
}

func TestSEOStatusAndRobotsTag(t *testing.T) {
	e := seoEnv(t)
	cases := []struct {
		path   string
		status int
		body   string
		robots string
	}{
		{"/", 200, idxBody, ""},
		{"/privacy", 200, privacyBody, ""},
		{"/terms", 200, termsBody, ""},
		{"/chat", 200, appBody, "noindex, nofollow"},
		{"/settings", 200, appBody, "noindex, nofollow"},
		{"/onboarding", 200, appBody, "noindex, nofollow"},
		{"/nope", 404, appBody, "noindex"},
		{"/a/b", 404, appBody, "noindex"},
		{"/app.html", 200, appBody, "noindex, nofollow"},
	}
	for _, c := range cases {
		w := e.do("GET", c.path, "", "")
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.body) {
			t.Errorf("%s: %d %q", c.path, w.Code, w.Body)
		}
		if got := w.Header().Get("X-Robots-Tag"); got != c.robots {
			t.Errorf("%s: X-Robots-Tag %q, want %q", c.path, got, c.robots)
		}
	}
}

func TestSEOStaticContentTypes(t *testing.T) {
	e := seoEnv(t)
	for path, want := range map[string][]string{
		"/robots.txt":  {"text/plain"},
		"/sitemap.xml": {"application/xml", "text/xml"},
		"/llms.txt":    {"text/plain"},
	} {
		w := e.do("GET", path, "", "")
		ct := w.Header().Get("Content-Type")
		ok := false
		for _, p := range want {
			ok = ok || strings.HasPrefix(ct, p)
		}
		if w.Code != 200 || !ok {
			t.Errorf("%s: %d %q", path, w.Code, ct)
		}
	}
}

func TestSEOAPIRobotsAndAuth(t *testing.T) {
	e := seoEnv(t)
	w := e.do("GET", "/api/me", "", "")
	if w.Code != 401 || w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	w = e.do("GET", "/api/zzz", "", "")
	if w.Code != 404 || w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	id := "00000000-0000-0000-0000-000000000001"
	for _, c := range [][2]string{
		{"POST", "/api/auth/logout"}, {"GET", "/api/me"}, {"PUT", "/api/me/username"},
		{"PUT", "/api/me/display-name"}, {"DELETE", "/api/me"}, {"GET", "/api/users/search"},
		{"POST", "/api/conversations"}, {"GET", "/api/conversations"}, {"POST", "/api/blocks"},
		{"DELETE", "/api/blocks/" + id}, {"GET", "/api/blocks"},
		{"DELETE", "/api/conversations/" + id}, {"GET", "/api/conversations/" + id + "/messages"},
	} {
		w := e.do(c[0], c[1], "", "")
		if w.Code != 401 {
			t.Errorf("%s %s: %d", c[0], c[1], w.Code)
		}
		if w.Header().Get("X-Robots-Tag") != "noindex, nofollow" {
			t.Errorf("%s %s: no robots tag", c[0], c[1])
		}
	}
}

func TestSEOPublicRouteVariants(t *testing.T) {
	e := seoEnv(t)
	if w := e.do("GET", "/privacy/", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), privacyBody) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w := e.do("GET", "/?ref=x", "", ""); w.Code != 200 || !strings.Contains(w.Body.String(), idxBody) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestSEOHeadRequests(t *testing.T) {
	e := seoEnv(t)
	for _, p := range []string{"/", "/chat", "/nope"} {
		g := e.do("GET", p, "", "")
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, httptest.NewRequest(http.MethodHead, p, nil))
		if w.Code != g.Code || w.Header().Get("X-Robots-Tag") != g.Header().Get("X-Robots-Tag") {
			t.Errorf("%s: HEAD %d/%q vs GET %d/%q", p, w.Code, w.Header().Get("X-Robots-Tag"), g.Code, g.Header().Get("X-Robots-Tag"))
		}
	}
}

func TestSEONoTraversal(t *testing.T) {
	e := seoEnv(t)
	for _, p := range []string{"/../go.mod", "/..%2f..%2fgo.mod", "/privacy/../../go.mod"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.Path = p
		w := httptest.NewRecorder()
		e.r.ServeHTTP(w, req)
		if strings.Contains(w.Body.String(), "module ") {
			t.Errorf("%s leaked: %s", p, w.Body)
		}
	}
}

func TestSEOAppShellFallsBackToIndex(t *testing.T) {
	e := seoEnvWith(t, false)
	for path, status := range map[string]int{"/chat": 200, "/nope": 404} {
		w := e.do("GET", path, "", "")
		if w.Code != status || !strings.Contains(w.Body.String(), idxBody) {
			t.Errorf("%s: %d %q", path, w.Code, w.Body)
		}
	}
	if w := e.do("GET", "/", "", ""); w.Code != 200 || w.Header().Get("X-Robots-Tag") != "" {
		t.Errorf("/: %d robots %q", w.Code, w.Header().Get("X-Robots-Tag"))
	}
}
