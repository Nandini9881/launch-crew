package launch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // hero formats
	_ "image/jpeg"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	_ "golang.org/x/image/webp"

	"github.com/shubham-tomar/launchcrew/internal/trigger"
)

// ErrHeroRefused means the hero URL points somewhere the server must not
// fetch from, redirects too often, or is too large.
var ErrHeroRefused = errors.New("launch: hero URL refused")

const (
	heroMaxBytes     = 5 << 20
	heroMaxRedirects = 3
	heroMaxPixels    = 40_000_000
	heroTimeout      = 10 * time.Second
)

// Ranges no server-side fetch may reach, beyond what netip classifies.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64 can reach any IPv4
	netip.MustParsePrefix("2002::/16"),    // 6to4, likewise
}

// Blocked reports whether a fetch must not connect to addr: loopback,
// unspecified, link-local (cloud metadata), private (RFC 1918, ULA),
// multicast and the extra ranges above. Unparseable addresses are blocked.
// allowLoopback admits loopback only; tests use it to reach httptest.
func Blocked(addr string, allowLoopback bool) bool {
	ip, err := netip.ParseAddr(strings.Trim(addr, "[]"))
	if err != nil {
		return true
	}
	ip = ip.Unmap().WithZone("")
	if ip.IsLoopback() {
		return !allowLoopback
	}
	if ip.IsUnspecified() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// checkHost resolves host and refuses it if any address is blocked.
func checkHost(ctx context.Context, host string, allowLoopback bool) error {
	addrs, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: %s does not resolve", ErrHeroRefused, host)
	}
	for _, a := range addrs {
		if Blocked(a, allowLoopback) {
			return fmt.Errorf("%w: %s is not a public address", ErrHeroRefused, host)
		}
	}
	return nil
}

// heroClient checks every connection it dials, so a redirect or a DNS answer
// that changes between the check and the dial cannot reach a blocked range.
func heroClient(allowLoopback bool) *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil || Blocked(host, allowLoopback) {
				return fmt.Errorf("%w: %s is not a public address", ErrHeroRefused, address)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: heroTimeout,
		Transport: &http.Transport{
			Proxy:                  nil, // a proxy would dial on our behalf, past the check
			DialContext:            dialer.DialContext,
			TLSHandshakeTimeout:    5 * time.Second,
			ResponseHeaderTimeout:  5 * time.Second,
			MaxResponseHeaderBytes: 64 << 10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > heroMaxRedirects {
				return fmt.Errorf("%w: more than %d redirects", ErrHeroRefused, heroMaxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect to %s", ErrHeroRefused, req.URL.Scheme)
			}
			return checkHost(req.Context(), req.URL.Hostname(), allowLoopback)
		},
	}
}

// fetchHero downloads a captain's hero image through heroClient and returns
// it re-encoded as PNG, so what is stored is always a plain image.
func fetchHero(ctx context.Context, raw string, allowLoopback bool) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, fmt.Errorf("%w: not an http(s) URL", trigger.ErrHeroNotImage)
	}
	if err := checkHost(ctx, u.Hostname(), allowLoopback); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", trigger.ErrHeroNotImage, err)
	}
	resp, err := heroClient(allowLoopback).Do(req)
	if err != nil {
		if errors.Is(err, ErrHeroRefused) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: could not be fetched", trigger.ErrHeroNotImage)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: answered %d", trigger.ErrHeroNotImage, resp.StatusCode)
	}
	if mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err != nil || !strings.HasPrefix(mt, "image/") {
		return nil, trigger.ErrHeroNotImage
	}
	if resp.ContentLength > heroMaxBytes {
		return nil, fmt.Errorf("%w: over 5 MB", ErrHeroRefused)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, heroMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: could not be read", trigger.ErrHeroNotImage)
	}
	if len(body) > heroMaxBytes {
		return nil, fmt.Errorf("%w: over 5 MB", ErrHeroRefused)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable image", trigger.ErrHeroNotImage)
	}
	if cfg.Width*cfg.Height > heroMaxPixels {
		return nil, fmt.Errorf("%w: image too large", ErrHeroRefused)
	}
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable image", trigger.ErrHeroNotImage)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
