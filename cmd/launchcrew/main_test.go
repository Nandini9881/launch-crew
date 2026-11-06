package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDirectDatabaseURL(t *testing.T) {
	for url, ok := range map[string]bool{
		"postgres://u:p@ep-cool-123-pooler.us-east-2.aws.neon.tech/db?sslmode=require": false,
		"postgres://u:p@ep-cool-123.us-east-2.aws.neon.tech/db?sslmode=require":        true,
		"postgres://postgres:postgres@localhost:5432/launchcrew?sslmode=disable":       true,
	} {
		err := checkDirectURL(url)
		if (err == nil) != ok {
			t.Errorf("%s: %v", url, err)
		}
		if err != nil && !strings.Contains(err.Error(), "advisory lock") {
			t.Errorf("error does not say why: %v", err)
		}
	}
}

func TestServerTimeouts(t *testing.T) {
	s := newServer("8080", nil)
	if s.ReadHeaderTimeout != 10*time.Second || s.ReadTimeout != 30*time.Second ||
		s.WriteTimeout != 60*time.Second || s.IdleTimeout != 120*time.Second {
		t.Fatalf("timeouts: %+v", s)
	}
}

func TestAwaitWorkerDrainIsBounded(t *testing.T) {
	done := make(chan struct{})
	close(done)
	if !awaitDrain(done, time.Second) {
		t.Fatal("a finished worker should report drained")
	}
	stuck := make(chan struct{})
	start := time.Now()
	if awaitDrain(stuck, 50*time.Millisecond) {
		t.Fatal("a stuck worker should report not drained")
	}
	if d := time.Since(start); d < 50*time.Millisecond || d > time.Second {
		t.Fatalf("waited %v, want about 50ms", d)
	}
}

// Fly sends SIGTERM and kills the machine kill_timeout later; that must
// cover the HTTP shutdown plus the worker drain, or in-flight jobs are
// cut mid-write.
func TestFlyKillTimeoutCoversShutdown(t *testing.T) {
	b, err := os.ReadFile("../../fly.toml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^kill_timeout\s*=\s*(\d+)`).FindSubmatch(b)
	if m == nil {
		t.Fatal("fly.toml has no kill_timeout; Fly's default of 5s is shorter than the shutdown")
	}
	kill, _ := strconv.Atoi(string(m[1]))
	if need := int((shutdownTimeout + drainTimeout) / time.Second); kill < need {
		t.Fatalf("kill_timeout = %ds, want at least %ds", kill, need)
	}
}
