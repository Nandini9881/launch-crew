package cards

import (
	"bytes"
	"context"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	"github.com/shubham-tomar/launchcrew/internal/testfakes/r2fake"
)

var update = flag.Bool("update", false, "rewrite golden PNGs")

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

func TestEveryPresetMeetsSevenToOne(t *testing.T) {
	for _, p := range Presets {
		if got := Contrast(p.Type, p.Field); got < 7.0 {
			t.Errorf("%s: contrast %.2f < 7", p.Name, got)
		}
	}
	if got := Contrast("000000", "#FFFFFF"); math.Abs(got-21) > 0.01 {
		t.Errorf("black on white = %.2f, want 21", got)
	}
}

func hue(hex string) float64 {
	r, g, b := parseHex(hex)
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	d := mx - mn
	if d == 0 {
		return 0
	}
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h
}

func TestNoPresetIsPurple(t *testing.T) {
	for _, p := range Presets {
		if h := hue(p.Field); h >= 255 && h <= 300 {
			t.Errorf("%s hue %.0f is purple", p.Name, h)
		}
	}
}

func TestPresetForIsStable(t *testing.T) {
	// Pinned: changing the hash input or modulus would re-colour every launch.
	pinned := map[string]string{
		"6f1c1f0e-3a8e-4f8a-9d57-0a1b2c3d4e5f": "vermilion",
		"00000000-0000-4000-8000-000000000001": "teal",
		"c0ffee00-1234-4abc-8def-0123456789ab": "orange",
	}
	for id, want := range pinned {
		if got := PresetFor(uuid.MustParse(id)).Name; got != want {
			t.Errorf("PresetFor(%s) = %s, want %s", id, got, want)
		}
	}
	seen := map[string]bool{}
	for i := range 400 {
		seen[PresetFor(uuid.NewSHA1(uuid.NameSpaceOID, []byte{byte(i), byte(i >> 8)})).Name] = true
	}
	if len(seen) != len(Presets) {
		t.Errorf("400 ids hit %d presets, want %d", len(seen), len(Presets))
	}
}

func TestUploadTrimsPublicBaseSlash(t *testing.T) {
	srv := r2fake.New("b")
	defer srv.Close()
	u := NewUploader(srv.URL, "b", "ak", "sk", "https://cdn.example/")
	url, sha, err := u.Put(context.Background(), []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://cdn.example/cards/" + sha + ".png"; url != want {
		t.Errorf("url = %s, want %s", url, want)
	}
	u.PublicBase = "https://cdn.example///"
	if url, _, _ = u.Put(context.Background(), []byte("x")); strings.Contains(strings.TrimPrefix(url, "https://"), "//") {
		t.Errorf("double slash in %s", url)
	}
}

func TestAvatarOverSizeRefused(t *testing.T) {
	big := image.NewGray(image.Rect(0, 0, 4097, 2))
	var buf bytes.Buffer
	_ = png.Encode(&buf, big)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(buf.Bytes()) }))
	defer srv.Close()
	old := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = old }()
	if _, err := fetchAvatar(srv.URL); err == nil {
		t.Error("want error for 4097px-wide avatar")
	}
}

var goldenPreset = Presets[0]

func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wantB, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	g, w := decodePNG(t, got), decodePNG(t, wantB)
	if g.Bounds() != w.Bounds() {
		t.Fatalf("bounds %v != golden %v", g.Bounds(), w.Bounds())
	}
	diff, total := 0, g.Bounds().Dx()*g.Bounds().Dy()
	for y := g.Bounds().Min.Y; y < g.Bounds().Max.Y; y++ {
		for x := g.Bounds().Min.X; x < g.Bounds().Max.X; x++ {
			if color.NRGBAModel.Convert(g.At(x, y)) != color.NRGBAModel.Convert(w.At(x, y)) {
				diff++
			}
		}
	}
	if float64(diff)/float64(total) > 0.01 {
		t.Errorf("%s: %d of %d pixels differ (>1%%)", name, diff, total)
	}
}

func TestRenderGoldenSize(t *testing.T) {
	cases := map[string]any{
		"contributor.png": ContributorCard{Name: "Ada Lovelace", Project: "analytical-engine", Version: "v2.4.0", PRTitle: "Add Bernoulli number routine"},
		"release.png":     ReleaseCard{Project: "analytical-engine", Version: "v2.4.0", Headline: "Faster loops, fewer punch cards"},
	}
	for name, c := range cases {
		b, err := Render(c, goldenPreset)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bd := decodePNG(t, b).Bounds(); bd.Dx() != 1200 || bd.Dy() != 630 {
			t.Errorf("%s size = %v", name, bd)
		}
		compareGolden(t, name, b)
	}
	if _, err := Render(42, goldenPreset); err == nil {
		t.Error("want error for unsupported card type")
	}
}

func TestLongAndCJKNamesWrap(t *testing.T) {
	fs, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("abcdefghi ", 6)[:60]
	for _, s := range []string{long, strings.Repeat("W", 60), strings.Repeat("wide words here ", 12)} {
		lines, size := fitTitle(fs.display, s, 1056)
		if len(lines) == 0 || len(lines) > 3 {
			t.Errorf("%q: %d lines", s[:10], len(lines))
		}
		if size != 64 && size != 72 {
			t.Errorf("size %v", size)
		}
	}
	if lines, size := fitTitle(fs.display, "Ada", 1056); len(lines) != 1 || size != 72 {
		t.Errorf("short title = %v @%v, want 1 line at 72", lines, size)
	}
	if got := keepGlyphs(fs.display, "山田太郎"); got != "" {
		t.Errorf("CJK kept: %q", got)
	}
	if got := titleText(fs.display, "山田太郎", "Contributor"); got != "Contributor" {
		t.Errorf("CJK-only title = %q, want Contributor", got)
	}
	if got := titleText(fs.display, "Ada 山田", "Contributor"); got != "Ada" {
		t.Errorf("mixed title = %q, want Ada", got)
	}
	if got := keepGlyphs(fs.display, "Sam 🚀 Lee"); got != "Sam Lee" {
		t.Errorf("emoji kept: %q", got)
	}
	for _, name := range []string{long, "山田太郎", ""} {
		b, err := Render(ContributorCard{Name: name, Project: "p", Version: "v1", PRTitle: "t"}, goldenPreset)
		if err != nil {
			t.Fatalf("render %q: %v", name, err)
		}
		decodePNG(t, b)
	}
}

func solid(c color.Color) []byte {
	m := image.NewRGBA(image.Rect(0, 0, 200, 120))
	draw.Draw(m, m.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var buf bytes.Buffer
	_ = png.Encode(&buf, m)
	return buf.Bytes()
}

func TestMissingAvatarUsesPlaceholder(t *testing.T) {
	red := color.RGBA{255, 0, 0, 255}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok.png" {
			w.Write(solid(red))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	old := HTTPClient
	HTTPClient = srv.Client()
	defer func() { HTTPClient = old }()

	centre := func(url string) color.Color {
		b, err := Render(ContributorCard{Name: "Ada", AvatarURL: url, Project: "p", Version: "v1"}, goldenPreset)
		if err != nil {
			t.Fatal(err)
		}
		return decodePNG(t, b).At(120, 120)
	}
	field := hexColor(goldenPreset.Field)
	typ := hexColor(goldenPreset.Type)
	want := color.RGBA{ // type at 25% over field
		uint8(float64(field.R)*0.75 + float64(typ.R)*0.25 + 0.5),
		uint8(float64(field.G)*0.75 + float64(typ.G)*0.25 + 0.5),
		uint8(float64(field.B)*0.75 + float64(typ.B)*0.25 + 0.5), 255}
	near := func(a, b color.Color) bool {
		ar, ag, ab, _ := a.RGBA()
		br, bg, bb, _ := b.RGBA()
		d := func(x, y uint32) bool { return math.Abs(float64(x)-float64(y)) < 3*257 }
		return d(ar, br) && d(ag, bg) && d(ab, bb)
	}
	for _, u := range []string{srv.URL + "/missing.png", "", "http://127.0.0.1:1/x.png"} {
		if got := centre(u); !near(got, want) {
			t.Errorf("%q: centre %v, want placeholder %v", u, got, want)
		}
	}
	if got := centre(srv.URL + "/ok.png"); !near(got, red) {
		t.Errorf("fetched avatar centre %v, want red", got)
	}
}

func TestQRSlideDecodes(t *testing.T) {
	const url = "https://lc.example/l/aB3xYz9"
	b, err := RenderQRSlide("Shubham shipped v2.4.0", url)
	if err != nil {
		t.Fatal(err)
	}
	img := decodePNG(t, b)
	if bd := img.Bounds(); bd.Dx() != 1920 || bd.Dy() != 1080 {
		t.Fatalf("size %v", bd)
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		t.Fatal(err)
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]any{gozxing.DecodeHintType_TRY_HARDER: true})
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.GetText() != url {
		t.Errorf("decoded %q, want %q", res.GetText(), url)
	}
}

func TestUploadIsFetchable(t *testing.T) {
	srv := r2fake.New("lc-cards")
	defer srv.Close()
	u := NewUploader(srv.URL, "lc-cards", "ak", "sk", srv.URL+"/lc-cards")
	b, err := Render(ReleaseCard{Project: "p", Version: "v1", Headline: "h"}, goldenPreset)
	if err != nil {
		t.Fatal(err)
	}
	url, sha, err := u.Put(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if want := srv.URL + "/lc-cards/cards/" + sha + ".png"; url != want {
		t.Errorf("url = %s, want %s", url, want)
	}
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "image/png" {
		t.Errorf("GET = %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", cc)
	}
	obj, ok := srv.Object("cards/" + sha + ".png")
	if !ok || !bytes.Equal(obj.Body, b) {
		t.Error("stored body differs")
	}
}
