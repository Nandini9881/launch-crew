package launch

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/shubham-tomar/launchcrew/internal/buffer"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/trigger"
)

func TestInitialsOf(t *testing.T) {
	for in, want := range map[string]string{
		"Maya Kapoor": "MK", "carol": "C", "dkim": "D", "jo-okafor": "JO", "Hannah Bergström-Lindqvist": "HB",
		"  ": "", "": "", "a b c": "AB", "4chan fan": "4F",
	} {
		if got := initialsOf(in); got != want {
			t.Errorf("initialsOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInitialsOfPeopleIsStableAndCapped(t *testing.T) {
	people := []crewPerson{
		{name: "zed"}, {name: "bob", in: true}, {name: "Alice"}, {name: "amy", in: true},
		{name: "carl"}, {name: "dina"}, {name: "erin", in: true}, {name: "frank"},
	}
	got := initialsOfPeople(slices.Clone(people))
	// People who are in first, by name: amy, bob, erin; then the rest: Alice, carl.
	if !slices.Equal(got, []string{"A", "B", "E", "A", "C"}) {
		t.Fatalf("got %v", got)
	}
	// The same people in another order give the same answer.
	slices.Reverse(people)
	if again := initialsOfPeople(people); !slices.Equal(again, got) {
		t.Fatalf("order changed with input order: %v vs %v", again, got)
	}
	if got := initialsOfPeople(nil); got == nil || len(got) != 0 {
		t.Fatalf("empty crew gives %#v, want an empty non-nil slice (JSON [])", got)
	}
}

func TestCountsAddMatchesDetail(t *testing.T) {
	var c Counts
	for _, st := range []string{trigger.StateSuggested, trigger.StateTicked, "invited", "opened", "connected", "drafting", "queued", "posted", "copied", "opted_out"} {
		c.add(st)
	}
	// Suggested is not counted; queued, posted and copied are in; everything else is only in the total.
	if c.In != 3 || c.Total != 9 {
		t.Fatalf("counts %+v, want In 3 Total 9", c)
	}
}

func TestValidateLogo(t *testing.T) {
	ok := []struct {
		mode, url string
		http      bool
	}{
		{"", "", false}, {"plane", "", false}, {"none", "ignored", false},
		{"custom", "https://cdn.example/logo.png", false}, {"custom", "http://127.0.0.1:1/logo.png", true},
	}
	for _, c := range ok {
		if err := validateLogo(c.mode, c.url, c.http); err != nil {
			t.Errorf("validateLogo(%q, %q, %v) = %v, want nil", c.mode, c.url, c.http, err)
		}
	}
	bad := []struct {
		mode, url string
		http      bool
	}{
		{"custom", "", false}, {"custom", "http://cdn.example/logo.png", false}, {"custom", "ftp://x/y.png", true},
		{"custom", "https://user:pw@cdn.example/logo.png", false}, {"custom", "https:///no-host", false}, {"sparkle", "", false},
	}
	for _, c := range bad {
		if err := validateLogo(c.mode, c.url, c.http); !errors.Is(err, ErrInvalid) {
			t.Errorf("validateLogo(%q, %q, %v) = %v, want ErrInvalid", c.mode, c.url, c.http, err)
		}
	}
}

func TestFetchLogoErrorsNameTheLogo(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{9, 9, 9, 255})
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, img); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes.Bytes())
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html></html>"))
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	if b, err := fetchLogo(ctx, srv.URL+"/ok.png", true); err != nil || len(b) == 0 {
		t.Fatalf("good logo: %d bytes, %v", len(b), err)
	}
	if _, err := fetchLogo(ctx, srv.URL+"/page", true); !errors.Is(err, ErrLogoNotImage) || errors.Is(err, ErrHeroRefused) {
		t.Fatalf("html: %v, want ErrLogoNotImage", err)
	}
	// Without the test allowance a loopback address is refused as a logo, not as a hero.
	if _, err := fetchLogo(ctx, srv.URL+"/ok.png", false); !errors.Is(err, ErrLogoRefused) || errors.Is(err, ErrHeroRefused) {
		t.Fatalf("loopback: %v, want ErrLogoRefused", err)
	}
}

func TestSkippedCaptainPostIsNeverSent(t *testing.T) {
	// A skipped post returns before touching Buffer or the database: zero Deps would panic otherwise.
	cd := sqlc.CaptainDraft{Status: captainSkipped}
	if err := postCaptainDraft(context.Background(), Deps{}, &state{}, cd); err != nil {
		t.Fatalf("skipped post: %v, want nil", err)
	}
}

func TestCaptainNameFallbacks(t *testing.T) {
	if got := captainName(&state{captain: "shubham-tomar"}); got != "shubham-tomar" {
		t.Errorf("login: %q", got)
	}
	if got := captainName(&state{order: []buffer.Channel{{Name: "JobHigh5"}, {Name: "other"}}}); got != "JobHigh5" {
		t.Errorf("first channel: %q", got)
	}
	if got := captainName(&state{}); got != "You" {
		t.Errorf("nothing known: %q", got)
	}
}
