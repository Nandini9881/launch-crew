package httpserver_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/auth"
	"github.com/shubham-tomar/launchcrew/internal/crypto"
	"github.com/shubham-tomar/launchcrew/internal/httpserver"
	"github.com/shubham-tomar/launchcrew/internal/session"
)

func TestHealthz(t *testing.T) {
	h := httpserver.New(httpserver.Deps{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if rr.Code != 200 || rr.Body.String() != "ok" {
		t.Fatalf("got %d %q", rr.Code, rr.Body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content type %q", ct)
	}
}

func TestAPINotFound(t *testing.T) {
	h := httpserver.New(httpserver.Deps{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/nope", nil))
	if rr.Code != 404 || strings.TrimSpace(rr.Body.String()) != `{"error":"not found"}` {
		t.Fatalf("got %d %q", rr.Code, rr.Body)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content type %q", ct)
	}
}

func TestSPAFallback(t *testing.T) {
	if _, err := os.Stat("dist/index.html"); err != nil {
		t.Skip("dist/index.html missing: run `npm run build` in web/ first")
	}
	h := httpserver.New(httpserver.Deps{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/captain/anything", nil))
	if rr.Code != 200 || !strings.HasPrefix(rr.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("got %d %q", rr.Code, rr.Header().Get("Content-Type"))
	}
}

func TestE2ERoutesAbsentByDefault(t *testing.T) {
	h := httpserver.New(httpserver.Deps{})
	for _, m := range []string{"GET", "POST"} {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(m, "/e2e/reset", nil))
		if rr.Code != 404 {
			t.Fatalf("%s /e2e/reset: %d", m, rr.Code)
		}
	}
}

func TestHTMLRefusesFraming(t *testing.T) {
	if _, err := os.Stat("dist/index.html"); err != nil {
		t.Skip("dist/index.html missing: run `npm run build` in web/ first")
	}
	h := httpserver.New(httpserver.Deps{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/l/abc/show", nil))
	if rr.Header().Get("X-Frame-Options") != "DENY" || rr.Header().Get("Content-Security-Policy") != "frame-ancestors 'none'" {
		t.Fatalf("headers: %v", rr.Header())
	}
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/l/abc/show.png", nil))
	if rr.Header().Get("X-Frame-Options") != "" {
		t.Fatal("API response got HTML framing headers")
	}
}

// Signing out changes state, so a cross-site <img src="/auth/logout"> must not do it.
func TestLogoutIsPostOnly(t *testing.T) {
	kr, err := crypto.ParseKeyring("k1:" + base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	h := httpserver.New(httpserver.Deps{BufferOAuth: &auth.BufferOAuth{Sessions: &session.Store{Keyring: kr}}})

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/auth/logout", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /auth/logout: %d, want 405", rr.Code)
	}
	if len(rr.Result().Cookies()) != 0 {
		t.Fatal("GET cleared the session cookie")
	}

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("POST", "/auth/logout", nil))
	if rr.Code != http.StatusFound || rr.Header().Get("Location") != "/" {
		t.Fatalf("POST /auth/logout: %d %q", rr.Code, rr.Header().Get("Location"))
	}
	if cs := rr.Result().Cookies(); len(cs) != 1 || cs[0].Name != "lc_s" || cs[0].MaxAge >= 0 {
		t.Fatalf("session cookie not cleared: %v", cs)
	}
}

// Every response, API and HTML alike, carries the two cheap hardening headers;
// the framing headers stay on the SPA index only.
func TestHardeningHeadersOnEveryResponse(t *testing.T) {
	h := httpserver.New(httpserver.Deps{})
	paths := []string{"/healthz", "/api/nope", "/e2e/reset"}
	if _, err := os.Stat("dist/index.html"); err == nil {
		paths = append(paths, "/captain/anything")
	}
	for _, p := range paths {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", p, nil))
		if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q", p, got)
		}
		if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
			t.Errorf("%s: Referrer-Policy = %q", p, got)
		}
		if p == "/healthz" && rr.Header().Get("Content-Security-Policy") != "" {
			t.Errorf("%s: a CSP leaked onto a non-HTML response", p)
		}
	}
}
