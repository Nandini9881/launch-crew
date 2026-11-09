package launch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // upload formats
	"image/jpeg"
	_ "image/png"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// ImageStore keeps the captain's uploads; cards.Uploader is one.
type ImageStore interface {
	PutImage(ctx context.Context, jpg []byte) (url, sha string, err error)
}

const (
	// MaxImages is how many images one launch can carry.
	MaxImages = 10
	// MaxImageBytes is the largest upload accepted, before shrinking.
	MaxImageBytes = 10 << 20
	// maxImageEdge is the stored long edge; larger uploads are scaled down.
	maxImageEdge = 4096
	// imageMaxPixels mirrors the hero limit: anything bigger is refused
	// before it is decoded into memory.
	imageMaxPixels = heroMaxPixels
	jpegQuality    = 85
)

var (
	// ErrImageNotImage means the upload is not a JPEG, PNG, GIF or WebP the
	// server can decode, or is too big to decode.
	ErrImageNotImage = errors.New("launch: upload is not a usable image")
	// ErrTooManyImages means the launch already has MaxImages.
	ErrTooManyImages = errors.New("launch: too many images")
	// ErrImagesLocked means the launch is no longer a draft.
	ErrImagesLocked = errors.New("launch: images are locked once the launch is out")
)

// LaunchImage is one captain upload as the API shows it.
type LaunchImage struct {
	ID  uuid.UUID `json:"id"`
	URL string    `json:"url"`
}

func imageOf(a sqlc.LaunchAsset) LaunchImage { return LaunchImage{ID: a.ID, URL: a.R2Url} }

// shrinkSlot lets one upload decode at a time: a 40-megapixel image is
// 160 MB as RGBA, and the server has 512 MB.
var shrinkSlot = make(chan struct{}, 1)

// shrinkToJPEG decodes data, scales it so its long edge is at most
// maxImageEdge, flattens transparency onto white and re-encodes it as a JPEG.
func shrinkToJPEG(ctx context.Context, data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable", ErrImageNotImage)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > imageMaxPixels {
		return nil, fmt.Errorf("%w: too large (%dx%d)", ErrImageNotImage, cfg.Width, cfg.Height)
	}
	select {
	case shrinkSlot <- struct{}{}:
		defer func() { <-shrinkSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable", ErrImageNotImage)
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if long := max(w, h); long > maxImageEdge {
		scale := float64(maxImageEdge) / float64(long)
		w = max(1, int(float64(w)*scale+0.5))
		h = max(1, int(float64(h)*scale+0.5))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// lockDraft locks the launch row for the transaction and reports whether it
// is still a draft: ErrNotFound when it is missing or deleted, notDraft when
// it has gone out.
func lockDraft(ctx context.Context, tx pgx.Tx, id uuid.UUID, notDraft error) error {
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM launches WHERE id = $1 FOR UPDATE`, id).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	switch status {
	case statusDeleted:
		return ErrNotFound
	case statusDraft:
		return nil
	default:
		return notDraft
	}
}

// AddImage shrinks an upload, stores it and appends it to the launch's images.
func AddImage(ctx context.Context, d Deps, launchID uuid.UUID, data []byte) (LaunchImage, error) {
	if len(data) > MaxImageBytes {
		return LaunchImage{}, fmt.Errorf("%w: over %d bytes", ErrImageNotImage, MaxImageBytes)
	}
	// Cheap checks before the expensive decode: the launch must be a draft
	// with room. Both are re-checked under the row lock below.
	l, err := getLaunch(ctx, d.Q, launchID)
	if err != nil {
		return LaunchImage{}, err
	}
	if l.Status != statusDraft {
		return LaunchImage{}, ErrImagesLocked
	}
	if n, err := d.Q.CountLaunchImages(ctx, launchID); err != nil {
		return LaunchImage{}, err
	} else if n >= MaxImages {
		return LaunchImage{}, ErrTooManyImages
	}
	jpg, err := shrinkToJPEG(ctx, data)
	if err != nil {
		return LaunchImage{}, err
	}
	if d.Images == nil {
		return LaunchImage{}, errors.New("launch: no image store")
	}
	url, sha, err := d.Images.PutImage(ctx, jpg)
	if err != nil {
		return LaunchImage{}, fmt.Errorf("launch: upload image: %w", err)
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return LaunchImage{}, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := lockDraft(ctx, tx, launchID, ErrImagesLocked); err != nil {
		return LaunchImage{}, err
	}
	q := sqlc.New(tx)
	if n, err := q.CountLaunchImages(ctx, launchID); err != nil {
		return LaunchImage{}, err
	} else if n >= MaxImages {
		return LaunchImage{}, ErrTooManyImages
	}
	a, err := q.InsertLaunchImage(ctx, sqlc.InsertLaunchImageParams{LaunchID: launchID, R2Url: url, Sha: sha})
	if err != nil {
		return LaunchImage{}, fmt.Errorf("launch: save image: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return LaunchImage{}, err
	}
	return imageOf(a), nil
}

// RemoveImage drops one image from a draft launch. The R2 object stays: it
// is content-addressed, may be shared, and expires on its own.
func RemoveImage(ctx context.Context, d Deps, launchID, imageID uuid.UUID) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := lockDraft(ctx, tx, launchID, ErrImagesLocked); err != nil {
		return err
	}
	n, err := sqlc.New(tx).DeleteLaunchImage(ctx, sqlc.DeleteLaunchImageParams{ID: imageID, LaunchID: launchID})
	if err != nil {
		return fmt.Errorf("launch: remove image: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// ReorderImages sets the launch's images to the order of ids, which must be
// exactly the launch's current images.
func ReorderImages(ctx context.Context, d Deps, launchID uuid.UUID, ids []uuid.UUID) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := lockDraft(ctx, tx, launchID, ErrImagesLocked); err != nil {
		return err
	}
	q := sqlc.New(tx)
	rows, err := q.ListLaunchImages(ctx, launchID)
	if err != nil {
		return err
	}
	have := make(map[uuid.UUID]bool, len(rows))
	for _, r := range rows {
		have[r.ID] = true
	}
	if len(ids) != len(rows) {
		return invalid("the order must list every image once")
	}
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		if !have[id] || seen[id] {
			return invalid("the order must list every image once")
		}
		seen[id] = true
	}
	for i, id := range ids {
		if err := q.SetLaunchImagePosition(ctx, sqlc.SetLaunchImagePositionParams{
			ID: id, LaunchID: launchID, Position: pgtype.Int2{Int16: int16(i + 1), Valid: true},
		}); err != nil {
			return fmt.Errorf("launch: order images: %w", err)
		}
	}
	return tx.Commit(ctx)
}
