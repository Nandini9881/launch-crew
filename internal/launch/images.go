package launch

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // upload formats
	"image/jpeg"
	_ "image/png"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
)

// ImageStore keeps the captain's uploads; cards.Uploader is one.
type ImageStore interface {
	PutImage(ctx context.Context, data []byte, contentType string) (url, sha string, err error)
}

const (
	// MaxImages is how many images one launch can carry.
	MaxImages = 10
	// MaxImageBytes is the largest upload accepted; Buffer's own cap is the
	// same, and it makes smaller copies for networks with lower limits.
	MaxImageBytes = 10 << 20
	// BlueskyMaxBytes is Bluesky's per-image limit, which Buffer cannot
	// resize down to; a copy under it is made for images over it.
	BlueskyMaxBytes = 1_000_000
	// blueskyCopyMaxPixels bounds what the copy decodes into memory on the
	// 512 MB server: 24 MP as RGBA is about 100 MB. Bigger originals get no
	// copy and are left out of Bluesky posts.
	blueskyCopyMaxPixels = 24_000_000
	blueskyCopyEdge      = 2000
)

var (
	// ErrImageNotImage means the upload is not a JPEG, PNG, GIF or WebP.
	ErrImageNotImage = errors.New("launch: upload is not a usable image")
	// ErrTooManyImages means the launch already has MaxImages.
	ErrTooManyImages = errors.New("launch: too many images")
	// ErrImagesLocked means the launch is no longer a draft.
	ErrImagesLocked = errors.New("launch: images are locked once the launch is out")
	// ErrTooBig means the upload is over MaxImageBytes.
	ErrTooBig = errors.New("launch: image over 10 MB")
)

// LaunchImage is one captain upload as the API shows it.
type LaunchImage struct {
	ID  uuid.UUID `json:"id"`
	URL string    `json:"url"`
}

func imageOf(a sqlc.LaunchAsset) LaunchImage { return LaunchImage{ID: a.ID, URL: a.R2Url} }

var imageTypes = map[string]string{"jpeg": "image/jpeg", "png": "image/png", "gif": "image/gif", "webp": "image/webp"}

// sniffImage reads only the file's header and says which image type it is.
// The bytes themselves are what gets stored and served, so anything that is
// not one of the four formats is refused.
func sniffImage(data []byte) (contentType string, err error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return "", fmt.Errorf("%w: unreadable", ErrImageNotImage)
	}
	ct, ok := imageTypes[format]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrImageNotImage, format)
	}
	return ct, nil
}

// blueskyCopy re-encodes an image as a JPEG under BlueskyMaxBytes: scaled so
// its long edge is at most blueskyCopyEdge, turned the way its Exif
// orientation says, then smaller and coarser until it fits.
func blueskyCopy(ctx context.Context, data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable", ErrImageNotImage)
	}
	if cfg.Width*cfg.Height > blueskyCopyMaxPixels {
		return nil, fmt.Errorf("launch: %dx%d is too large to copy for Bluesky", cfg.Width, cfg.Height)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: unreadable", ErrImageNotImage)
	}
	o := orientation(data)
	for edge, quality := blueskyCopyEdge, 85; ; edge, quality = edge*3/4, max(quality-10, 50) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out, err := encodeJPEG(src, o, edge, quality)
		if err != nil {
			return nil, err
		}
		if len(out) <= BlueskyMaxBytes {
			return out, nil
		}
		if edge < 200 {
			return nil, errors.New("launch: could not fit the image under Bluesky's limit")
		}
	}
}

// encodeJPEG draws src onto white at most edge px on its long side (never
// upscaled), applies the Exif orientation o, and encodes at quality.
func encodeJPEG(src image.Image, o, edge, quality int) ([]byte, error) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if long := max(w, h); long > edge {
		scale := float64(edge) / float64(long)
		w = max(1, int(float64(w)*scale+0.5))
		h = max(1, int(float64(h)*scale+0.5))
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if w == src.Bounds().Dx() && h == src.Bounds().Dy() {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
	} else {
		// ApproxBiLinear works per destination pixel with no scratch buffer;
		// the kernel scalers allocate width×height×32 bytes, too much here.
		xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, orient(dst, o), &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// orientation reads the Exif Orientation tag (1 to 8) of a JPEG; 1 when
// there is none or the file is not a JPEG. Only the APP1 segment is walked.
func orientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan, end of image
			return 1
		}
		n := int(binary.BigEndian.Uint16(data[i+2:]))
		if n < 2 || i+2+n > len(data) {
			return 1
		}
		if marker == 0xE1 && n >= 16 && bytes.Equal(data[i+4:i+10], []byte("Exif\x00\x00")) {
			return tiffOrientation(data[i+10 : i+2+n])
		}
		i += 2 + n
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	if bo.Uint16(t[2:]) != 42 {
		return 1
	}
	ifd := int(bo.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[ifd:]))
	for k := 0; k < count; k++ {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 {
			if v := int(bo.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient returns img turned so it displays upright for Exif orientation o.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return img
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	// Orientations 5 to 8 swap the axes.
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < oh; y++ {
		for x := 0; x < ow; x++ {
			var sx, sy int
			switch o {
			case 2: // mirrored
				sx, sy = w-1-x, y
			case 3: // upside down
				sx, sy = w-1-x, h-1-y
			case 4: // flipped
				sx, sy = x, h-1-y
			case 5: // transposed
				sx, sy = y, x
			case 6: // rotated 90° clockwise to display
				sx, sy = y, h-1-x
			case 7: // transverse
				sx, sy = w-1-y, h-1-x
			case 8: // rotated 90° anticlockwise to display
				sx, sy = w-1-y, x
			}
			copy(out.Pix[out.PixOffset(x, y):out.PixOffset(x, y)+4], img.Pix[img.PixOffset(sx, sy):img.PixOffset(sx, sy)+4])
		}
	}
	return out
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

// AddImage checks an upload is an image, stores it as it is and appends it to
// the launch's images. An original over Bluesky's limit also gets a copy
// under it; when that copy cannot be made the image is left out of Bluesky
// posts. The stored objects are content-addressed and expire in R2 after
// 60 days, so one left behind by a failed insert costs nothing.
func AddImage(ctx context.Context, d Deps, launchID uuid.UUID, data []byte) (LaunchImage, error) {
	if len(data) > MaxImageBytes {
		return LaunchImage{}, fmt.Errorf("%w: over %d bytes", ErrTooBig, MaxImageBytes)
	}
	// Cheap checks first; both are re-checked under the row lock below.
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
	contentType, err := sniffImage(data)
	if err != nil {
		return LaunchImage{}, err
	}
	if d.Images == nil {
		return LaunchImage{}, errors.New("launch: no image store")
	}
	url, sha, err := d.Images.PutImage(ctx, data, contentType)
	if err != nil {
		return LaunchImage{}, fmt.Errorf("launch: upload image: %w", err)
	}
	small := ""
	if len(data) > BlueskyMaxBytes {
		if c, err := blueskyCopy(ctx, data); err != nil {
			log.Printf("launch %s: no Bluesky copy: %v", launchID, err)
		} else if small, _, err = d.Images.PutImage(ctx, c, "image/jpeg"); err != nil {
			return LaunchImage{}, fmt.Errorf("launch: upload Bluesky copy: %w", err)
		}
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
	a, err := q.InsertLaunchImage(ctx, sqlc.InsertLaunchImageParams{
		LaunchID: launchID, R2Url: url, Sha: sha, Bytes: int64(len(data)), SmallUrl: small,
	})
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
