package launch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shubham-tomar/launchcrew/internal/cards"
	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/trigger"
)

// The QR centre mark settings, as stored in launches.qr_logo.
const (
	qrPlane  = cards.LogoPlane
	qrNone   = cards.LogoNone
	qrCustom = cards.LogoCustom

	// kindQRLogo is the launch asset kind of a custom centre image.
	kindQRLogo = "qrlogo"
)

var (
	// ErrLogoRefused means the logo URL points somewhere the server must not
	// fetch from, redirects too often, or is too large.
	ErrLogoRefused = errors.New("launch: logo URL refused")
	// ErrLogoNotImage means the logo URL does not answer with an image.
	ErrLogoNotImage = errors.New("launch: logo URL is not an image")
)

// validateLogo checks a captain's QR centre choice. An empty mode means "not
// given". Custom needs an https image URL (http only when allowHTTP, for tests).
func validateLogo(mode, raw string, allowHTTP bool) error {
	switch mode {
	case "", qrPlane, qrNone:
		return nil
	case qrCustom:
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
			return invalid("use an https:// image URL for the QR logo")
		}
		return nil
	}
	return invalid("the QR logo is plane, none or custom")
}

// fetchLogo downloads the captain's logo through the same address guard as
// the hero image and returns it re-encoded as PNG. Its errors say "logo", so
// the captain knows which URL was refused.
func fetchLogo(ctx context.Context, raw string, allowLoopback bool) ([]byte, error) {
	b, err := fetchHero(ctx, strings.TrimSpace(raw), allowLoopback)
	switch {
	case errors.Is(err, ErrHeroRefused):
		return nil, fmt.Errorf("%w: %v", ErrLogoRefused, err)
	case errors.Is(err, trigger.ErrHeroNotImage):
		return nil, fmt.Errorf("%w: %v", ErrLogoNotImage, err)
	}
	return b, err
}

// preparedLogo is a logo choice with its image already fetched, ready to be
// written inside a transaction.
type preparedLogo struct {
	mode string
	png  []byte
}

// prepareLogo validates mode and url and, for custom, fetches the image. An
// empty mode returns nil: nothing to write.
func prepareLogo(ctx context.Context, d Deps, mode, raw string) (*preparedLogo, error) {
	if err := validateLogo(mode, raw, d.AllowLoopback); err != nil {
		return nil, err
	}
	if mode == "" {
		return nil, nil
	}
	p := &preparedLogo{mode: mode}
	if mode == qrCustom {
		b, err := fetchLogo(ctx, raw, d.AllowLoopback)
		if err != nil {
			return nil, err
		}
		p.png = b
	}
	return p, nil
}

// writeLogo stores the choice on the launch, uploading the image first for custom.
func writeLogo(ctx context.Context, d Deps, tx pgx.Tx, q *sqlc.Queries, launchID uuid.UUID, p *preparedLogo) error {
	if p == nil {
		return nil
	}
	if p.mode == qrCustom {
		if _, err := putAsset(ctx, d, q, launchID, kindQRLogo, p.png); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE launches SET qr_logo = $2 WHERE id = $1`, launchID, p.mode); err != nil {
		return fmt.Errorf("launch: set qr logo: %w", err)
	}
	return nil
}

// qrLogoOf reads the launch's QR centre setting: the mode, and for custom the
// URL of the stored image. Anything unreadable is the plane.
func qrLogoOf(ctx context.Context, d Deps, launchID uuid.UUID) (mode, logoURL string) {
	if err := d.Pool.QueryRow(ctx, `SELECT qr_logo FROM launches WHERE id = $1`, launchID).Scan(&mode); err != nil {
		return qrPlane, ""
	}
	if mode == qrCustom {
		if a, err := d.Q.GetLatestLaunchAssetByKind(ctx, sqlc.GetLatestLaunchAssetByKindParams{LaunchID: launchID, Kind: kindQRLogo}); err == nil {
			return qrCustom, a.R2Url
		}
		return qrPlane, ""
	}
	if mode != qrNone {
		mode = qrPlane
	}
	return mode, ""
}
