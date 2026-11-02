package cards

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fogleman/gg"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
)

const (
	cardW, cardH = 1200, 630
	margin       = 72
	avatarSize   = 96
	maxAvatar    = 5 << 20
	maxAvatarDim = 4096
)

// HTTPClient fetches avatars. Tests replace it.
var HTTPClient = &http.Client{Timeout: 3 * time.Second}

// ContributorCard is the card for one contributor's merged PR.
type ContributorCard struct {
	Name, AvatarURL, Project, Version, PRTitle string
}

// ReleaseCard is the card for a release.
type ReleaseCard struct {
	Project, Version, Headline string
}

func hexColor(h string) color.RGBA {
	r, g, b := parseHex(h)
	return color.RGBA{uint8(r*255 + 0.5), uint8(g*255 + 0.5), uint8(b*255 + 0.5), 255}
}

func setHex(dc *gg.Context, h string) { dc.SetColor(hexColor(h)) }

// Render draws a 1200x630 PNG for a ContributorCard or ReleaseCard.
func Render(c any, p Preset) ([]byte, error) {
	fs, err := loadFonts()
	if err != nil {
		return nil, err
	}
	var title, fallback, version string
	var sub []string
	var avatarURL string
	switch v := c.(type) {
	case ContributorCard:
		title, fallback, version, avatarURL = v.Name, "Contributor", v.Version, v.AvatarURL
		sub = []string{strings.TrimSpace("shipped in " + v.Project + " " + v.Version), v.PRTitle}
	case ReleaseCard:
		title, fallback, version = v.Project, "Release", v.Version
		sub = []string{v.Headline}
	default:
		return nil, fmt.Errorf("cards: unsupported card type %T", c)
	}

	dc := gg.NewContext(cardW, cardH)
	setHex(dc, p.Field)
	dc.Clear()

	drawAvatar(dc, avatarURL, p)

	title = titleText(fs.display, title, fallback)
	lines, size := fitTitle(fs.display, title, cardW-2*margin)
	tf, err := face(fs.display, size)
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(tf)
	setHex(dc, p.Type)
	lh := size * 1.1
	y := 200.0 + size
	for _, l := range lines {
		dc.DrawString(l, margin, y)
		y += lh
	}
	y += 8

	bf, err := face(fs.body, 32)
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(bf)
	for _, s := range sub {
		s = keepGlyphs(fs.body, s)
		if s == "" {
			continue
		}
		dc.DrawString(ellipsize(bf, s, cardW-2*margin), margin, y)
		y += 42
	}

	setHex(dc, p.Mark)
	dc.DrawRectangle(margin, cardH-margin-24, 24, 24)
	dc.Fill()

	if version = keepGlyphs(fs.mono, version); version != "" {
		mf, err := face(fs.mono, 28)
		if err != nil {
			return nil, err
		}
		dc.SetFontFace(mf)
		setHex(dc, p.Type)
		dc.DrawStringAnchored(version, cardW-margin, cardH-margin, 1, 0)
	}
	return encode(dc.Image())
}

// titleText strips unrenderable runes, using fallback when nothing is left.
func titleText(f *sfnt.Font, s, fallback string) string {
	if s = keepGlyphs(f, s); s != "" {
		return s
	}
	return fallback
}

func encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawAvatar(dc *gg.Context, url string, p Preset) {
	cx, cy, r := margin+avatarSize/2.0, margin+avatarSize/2.0, avatarSize/2.0
	img, err := fetchAvatar(url)
	if err != nil {
		t := hexColor(p.Type)
		dc.SetColor(color.NRGBA{t.R, t.G, t.B, 64})
		dc.DrawCircle(cx, cy, r)
		dc.Fill()
		return
	}
	// Cover-crop to a square, scale to 96, clip to a circle.
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	sq := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, 0, 0)
	sq.Max = image.Pt(sq.Min.X+side, sq.Min.Y+side)
	dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, sq, xdraw.Over, nil)
	dc.DrawCircle(cx, cy, r)
	dc.Clip()
	dc.DrawImage(dst, margin, margin)
	dc.ResetClip()
}

func fetchAvatar(url string) (image.Image, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, errors.New("no avatar url")
	}
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("avatar status " + strconv.Itoa(resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAvatar))
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width > maxAvatarDim || cfg.Height > maxAvatarDim {
		return nil, errors.New("avatar too large")
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// fitTitle picks the title size and lines: 72px on one line, else 64px on one
// line, else 64px wrapped to at most three lines with an ellipsis.
func fitTitle(f *sfnt.Font, s string, maxW float64) ([]string, float64) {
	big, err1 := face(f, 72)
	small, err2 := face(f, 64)
	if err1 != nil || err2 != nil {
		return []string{s}, 72
	}
	if textWidth(big, s) <= maxW {
		return []string{s}, 72
	}
	if textWidth(small, s) <= maxW {
		return []string{s}, 64
	}
	return wrap(small, s, maxW, 3), 64
}

// wrap breaks s into at most n lines, splitting over-long words by rune and
// ellipsizing the last line when text remains.
func wrap(f font.Face, s string, maxW float64, n int) []string {
	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, w := range strings.Fields(s) {
		for textWidth(f, w) > maxW { // word wider than a line: split by rune
			flush()
			rs := []rune(w)
			i := len(rs)
			for i > 1 && textWidth(f, string(rs[:i])) > maxW {
				i--
			}
			lines = append(lines, string(rs[:i]))
			w = string(rs[i:])
		}
		if cur == "" {
			cur = w
		} else if textWidth(f, cur+" "+w) <= maxW {
			cur += " " + w
		} else {
			flush()
			cur = w
		}
	}
	flush()
	if len(lines) > n {
		last := strings.Join(lines[n-1:], " ")
		lines = append(lines[:n-1], ellipsize(f, last, maxW))
	}
	return lines
}

func ellipsize(f font.Face, s string, maxW float64) string {
	if textWidth(f, s) <= maxW {
		return s
	}
	rs := []rune(s)
	for len(rs) > 0 {
		rs = rs[:len(rs)-1]
		t := strings.TrimRight(string(rs), " ") + "…"
		if textWidth(f, t) <= maxW {
			return t
		}
	}
	return ""
}

func textWidth(f font.Face, s string) float64 {
	return float64(font.MeasureString(f, s)) / 64
}
