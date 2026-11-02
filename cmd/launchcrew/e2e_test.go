package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/httpserver"
)

// Without LAUNCHCREW_E2E=1 no fakes start and nothing is mounted at /e2e.
func TestE2EOffByDefault(t *testing.T) {
	t.Setenv("LAUNCHCREW_E2E", "")
	_, m, _ := loadConfig()
	if m != nil {
		t.Fatal("E2E mode started without LAUNCHCREW_E2E=1")
	}
	if m.bufferOptions() != nil || m.githubOptions() != nil || m.claudeOptions() != nil {
		t.Fatal("options added outside E2E")
	}
	var d httpserver.Deps
	m.wire(&d, nil)
	if d.E2E != nil {
		t.Fatal("E2E handler set")
	}
	rr := httptest.NewRecorder()
	httpserver.New(d).ServeHTTP(rr, httptest.NewRequest("POST", "/e2e/reset", nil))
	if rr.Code != 404 {
		t.Fatalf("POST /e2e/reset: %d", rr.Code)
	}
}

// E2E mode never starts on Fly, even if the flag leaks into its env.
func TestE2ERefusedOnFly(t *testing.T) {
	t.Setenv("LAUNCHCREW_E2E", "1")
	t.Setenv("FLY_APP_NAME", "launchcrew")
	t.Setenv("PORT", "0")
	_, m, err := loadConfig()
	if m != nil || err == nil || !strings.Contains(err.Error(), "FLY_APP_NAME") {
		t.Fatalf("mode %v, err %v", m, err)
	}
}
