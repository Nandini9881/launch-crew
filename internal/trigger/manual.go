package trigger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxRoles     = 4
	maxRoleRunes = 24
	heroProbe    = 64 << 10
)

var (
	// ErrEmptyTitle rejects a manual launch without a title.
	ErrEmptyTitle = errors.New("trigger: title is required")
	// ErrHeroNotImage rejects a hero URL that does not answer with an image.
	ErrHeroNotImage = errors.New("trigger: hero URL is not an image")
	// ErrBadRoles rejects more than four roles or a blank or overlong name.
	ErrBadRoles = errors.New("trigger: up to 4 roles, names 1 to 24 characters")
)

// Manual builds a LaunchSpec from the manual form. HTTP checks the hero URL;
// nil uses a client with a 5 second timeout. The hero is only checked here,
// never downloaded.
type Manual struct {
	HTTP *http.Client
}

// Build validates the form and returns the spec.
func (m Manual) Build(ctx context.Context, in Input) (LaunchSpec, error) {
	f := in.Manual
	if f == nil {
		return LaunchSpec{}, errors.New("trigger: manual input missing")
	}
	title := strings.TrimSpace(f.Title)
	if title == "" {
		return LaunchSpec{}, ErrEmptyTitle
	}
	roles, err := normaliseRoles(f.Roles)
	if err != nil {
		return LaunchSpec{}, err
	}
	hero := strings.TrimSpace(f.HeroURL)
	if hero != "" {
		if err := m.checkHero(ctx, hero); err != nil {
			return LaunchSpec{}, err
		}
	}
	return LaunchSpec{
		Title:   title,
		Summary: strings.TrimSpace(f.Summary),
		Source:  "manual",
		HeroURL: hero,
		Roles:   roles,
	}, nil
}

func normaliseRoles(in []Role) ([]Role, error) {
	if len(in) == 0 {
		return append([]Role(nil), DefaultRoles...), nil
	}
	if len(in) > maxRoles {
		return nil, ErrBadRoles
	}
	out := make([]Role, len(in))
	for i, r := range in {
		name := strings.TrimSpace(r.Name)
		if name == "" || utf8.RuneCountInString(name) > maxRoleRunes {
			return nil, ErrBadRoles
		}
		out[i] = Role{Name: name, Hint: strings.TrimSpace(r.Hint)}
	}
	return out, nil
}

// checkHero asks for the URL's content type with HEAD, falling back to a GET
// of the first 64 KiB for servers that refuse HEAD.
func (m Manual) checkHero(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%w: not an http(s) URL", ErrHeroNotImage)
	}
	hc := m.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	if isImage(ctx, hc, http.MethodHead, raw) {
		return nil
	}
	if isImage(ctx, hc, http.MethodGet, raw) {
		return nil
	}
	return ErrHeroNotImage
}

func isImage(ctx context.Context, hc *http.Client, method, raw string) bool {
	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		return false
	}
	if method == http.MethodGet {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", heroProbe-1))
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, heroProbe))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return false
	}
	mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return err == nil && strings.HasPrefix(mt, "image/")
}
