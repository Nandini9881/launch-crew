package config

import "testing"

func TestLoadTrimsBaseURLSlash(t *testing.T) {
	for _, k := range []string{"DATABASE_URL", "KEYRING", "BUFFER_CLIENT_ID", "BUFFER_CLIENT_SECRET",
		"GITHUB_APP_ID", "GITHUB_APP_SLUG", "GITHUB_PRIVATE_KEY", "GITHUB_WEBHOOK_SECRET", "GITHUB_CLIENT_ID",
		"GITHUB_CLIENT_SECRET", "ANTHROPIC_API_KEY", "R2_ENDPOINT", "R2_BUCKET", "R2_ACCESS_KEY", "R2_SECRET_KEY", "R2_PUBLIC_BASE"} {
		t.Setenv(k, "x")
	}
	t.Setenv("BASE_URL", "https://launchcrew.example/")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != "https://launchcrew.example" {
		t.Fatalf("BaseURL = %q", c.BaseURL)
	}
}
