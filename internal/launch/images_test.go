package launch_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/google/uuid"

	"github.com/shubham-tomar/launchcrew/internal/db/sqlc"
	"github.com/shubham-tomar/launchcrew/internal/launch"
)

// pngHeader is the start of a PNG claiming w×h: enough for DecodeConfig,
// without the memory of a real image that size.
func pngHeader(w, h int) []byte {
	var b bytes.Buffer
	b.Write([]byte("\x89PNG\r\n\x1a\n"))
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, 6 // 8-bit RGBA
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], 13)
	b.Write(length[:])
	chunk := append([]byte("IHDR"), ihdr...)
	b.Write(chunk)
	var crc [4]byte
	binary.BigEndian.PutUint32(crc[:], crc32.ChecksumIEEE(chunk))
	b.Write(crc[:])
	return b.Bytes()
}

func encodePNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// redSquarePNG is a solid w×h opaque image whose bytes differ per call site
// through the colour, so two calls can be made to produce distinct files.
func solidPNG(t testing.TB, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return encodePNG(t, img)
}

func TestShrinkToJPEGBounds(t *testing.T) {
	// 5000×3000 with a transparent corner and an opaque red block in the middle.
	img := image.NewNRGBA(image.Rect(0, 0, 5000, 3000))
	for y := 1000; y < 2000; y++ {
		for x := 2000; x < 3000; x++ {
			img.Set(x, y, color.NRGBA{220, 20, 20, 255})
		}
	}
	out, err := launch.ShrinkToJPEG(encodePNG(t, img))
	if err != nil {
		t.Fatalf("shrink: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" {
		t.Fatalf("output format %q err %v", format, err)
	}
	if cfg.Width != 4096 || cfg.Height != 2458 {
		t.Errorf("size = %dx%d, want 4096x2458", cfg.Width, cfg.Height)
	}
	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := got.At(0, 0).RGBA()
	if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Errorf("transparent corner = %d,%d,%d; want white", r>>8, g>>8, b>>8)
	}
	r, g, b, _ = got.At(2048, 1229).RGBA()
	if r>>8 < 180 || g>>8 > 80 || b>>8 > 80 {
		t.Errorf("centre = %d,%d,%d; want red", r>>8, g>>8, b>>8)
	}
}

func TestShrinkKeepsSmallImages(t *testing.T) {
	out, err := launch.ShrinkToJPEG(solidPNG(t, 300, 200, color.RGBA{0, 0, 200, 255}))
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 300 || cfg.Height != 200 {
		t.Errorf("size = %dx%d, want 300x200 (no upscaling)", cfg.Width, cfg.Height)
	}
}

func TestShrinkAcceptsGIFAndJPEG(t *testing.T) {
	src := image.NewPaletted(image.Rect(0, 0, 10, 10), color.Palette{color.Black, color.White})
	var g bytes.Buffer
	if err := gif.Encode(&g, src, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.ShrinkToJPEG(g.Bytes()); err != nil {
		t.Errorf("gif: %v", err)
	}
	var j bytes.Buffer
	if err := jpeg.Encode(&j, src, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.ShrinkToJPEG(j.Bytes()); err != nil {
		t.Errorf("jpeg: %v", err)
	}
}

func TestShrinkRefusesHugePixels(t *testing.T) {
	_, err := launch.ShrinkToJPEG(pngHeader(8000, 6000))
	if !errors.Is(err, launch.ErrImageNotImage) {
		t.Fatalf("err = %v, want ErrImageNotImage", err)
	}
	if !bytes.Contains([]byte(err.Error()), []byte("too large")) {
		t.Errorf("err = %q, want it to say too large", err)
	}
}

func TestShrinkRefusesUndecodable(t *testing.T) {
	_, err := launch.ShrinkToJPEG([]byte("RIFF....WEBPVP8 this is a HEIC renamed .jpg really"))
	if !errors.Is(err, launch.ErrImageNotImage) {
		t.Fatalf("err = %v, want ErrImageNotImage", err)
	}
}

func (e *env) images(t *testing.T) []sqlc.LaunchAsset {
	t.Helper()
	rows, err := e.q.ListLaunchImages(e.ctx, e.launch.ID)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestUploadRejectsUndecodable(t *testing.T) {
	e := setup(t)
	_, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, []byte("not an image at all"))
	if !errors.Is(err, launch.ErrImageNotImage) {
		t.Fatalf("err = %v, want ErrImageNotImage", err)
	}
	if n := len(e.images(t)); n != 0 {
		t.Errorf("%d image rows after a refused upload", n)
	}
	if e.r2.Len() != 0 {
		t.Errorf("%d objects uploaded for a refused image", e.r2.Len())
	}
}

func TestAddImageStoresJPEGInOrder(t *testing.T) {
	e := setup(t)
	a, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 20, 10, color.RGBA{255, 0, 0, 255}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 20, 10, color.RGBA{0, 255, 0, 255}))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID || a.URL == b.URL {
		t.Errorf("two different images share id or url: %+v %+v", a, b)
	}
	rows := e.images(t)
	if len(rows) != 2 || rows[0].ID != a.ID || rows[1].ID != b.ID {
		t.Fatalf("rows = %+v, want a then b", rows)
	}
	if rows[0].Position.Int16 != 1 || rows[1].Position.Int16 != 2 {
		t.Errorf("positions = %d,%d, want 1,2", rows[0].Position.Int16, rows[1].Position.Int16)
	}
	key := "uploads/" + rows[0].Sha + ".jpg"
	o, ok := e.r2.Object(key)
	if !ok {
		t.Fatalf("no object at %s", key)
	}
	if o.ContentType != "image/jpeg" {
		t.Errorf("content type %q", o.ContentType)
	}
	if a.URL != "https://cdn.example/"+key {
		t.Errorf("url = %s", a.URL)
	}
}

func TestSameBytesTwice(t *testing.T) {
	e := setup(t)
	data := solidPNG(t, 8, 8, color.RGBA{9, 9, 9, 255})
	a, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, data)
	if err != nil {
		t.Fatalf("second identical upload: %v", err)
	}
	if a.ID == b.ID {
		t.Fatal("identical bytes collapsed into one row")
	}
	if a.URL != b.URL {
		t.Errorf("identical bytes got different keys: %s vs %s", a.URL, b.URL)
	}
	if err := launch.RemoveImage(e.ctx, e.deps, e.launch.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	rows := e.images(t)
	if len(rows) != 1 || rows[0].ID != b.ID {
		t.Errorf("after removing a, rows = %+v", rows)
	}
}

func TestAddImageCapsAtTen(t *testing.T) {
	e := setup(t)
	for i := 0; i < launch.MaxImages; i++ {
		if _, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{uint8(i), 0, 0, 255})); err != nil {
			t.Fatalf("image %d: %v", i+1, err)
		}
	}
	_, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{99, 0, 0, 255}))
	if !errors.Is(err, launch.ErrTooManyImages) {
		t.Fatalf("11th image: err = %v, want ErrTooManyImages", err)
	}
	if n := len(e.images(t)); n != launch.MaxImages {
		t.Errorf("%d rows, want %d", n, launch.MaxImages)
	}
}

func TestImagesLockedAfterLaunch(t *testing.T) {
	e := setup(t)
	a, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{1, 2, 3, 255}))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.q.UpdateLaunchStatus(e.ctx, sqlc.UpdateLaunchStatusParams{ID: e.launch.ID, Status: "launched"}); err != nil {
		t.Fatal(err)
	}
	if _, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{4, 5, 6, 255})); !errors.Is(err, launch.ErrImagesLocked) {
		t.Errorf("add: err = %v, want ErrImagesLocked", err)
	}
	if err := launch.RemoveImage(e.ctx, e.deps, e.launch.ID, a.ID); !errors.Is(err, launch.ErrImagesLocked) {
		t.Errorf("remove: err = %v, want ErrImagesLocked", err)
	}
	if err := launch.ReorderImages(e.ctx, e.deps, e.launch.ID, []uuid.UUID{a.ID}); !errors.Is(err, launch.ErrImagesLocked) {
		t.Errorf("reorder: err = %v, want ErrImagesLocked", err)
	}
	if n := len(e.images(t)); n != 1 {
		t.Errorf("%d rows after locked edits, want 1", n)
	}
}

func TestRemoveUnknownImage(t *testing.T) {
	e := setup(t)
	if err := launch.RemoveImage(e.ctx, e.deps, e.launch.ID, uuid.New()); !errors.Is(err, launch.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestReorderNeedsExactSet(t *testing.T) {
	e := setup(t)
	a, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{10, 0, 0, 255}))
	if err != nil {
		t.Fatal(err)
	}
	b, err := launch.AddImage(e.ctx, e.deps, e.launch.ID, solidPNG(t, 4, 4, color.RGBA{20, 0, 0, 255}))
	if err != nil {
		t.Fatal(err)
	}
	for name, ids := range map[string][]uuid.UUID{
		"missing one": {a.ID},
		"foreign id":  {a.ID, uuid.New()},
		"duplicate":   {a.ID, a.ID},
		"extra":       {a.ID, b.ID, uuid.New()},
	} {
		if err := launch.ReorderImages(e.ctx, e.deps, e.launch.ID, ids); !errors.Is(err, launch.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if err := launch.ReorderImages(e.ctx, e.deps, e.launch.ID, []uuid.UUID{b.ID, a.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	rows := e.images(t)
	if len(rows) != 2 || rows[0].ID != b.ID || rows[1].ID != a.ID {
		t.Errorf("order after reorder = %v, want b then a", []uuid.UUID{rows[0].ID, rows[1].ID})
	}
}
