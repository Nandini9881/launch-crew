package main

import (
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
