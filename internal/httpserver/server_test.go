package httpserver_test

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/httpserver"
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
