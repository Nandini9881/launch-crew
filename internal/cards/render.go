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
	pad          = 64
	stubW        = 288
	perfW        = 48
	mainW        = cardW - stubW - perfW // 864
	textW        = mainW - 2*pad         // 736
	avatarSize   = 72
	maxAvatar    = 5 << 20
	maxAvatarDim = 4096
	notchR       = 24
)

// HTTPClient fetches avatars. Tests replace it.
var HTTPClient = &http.Client{Timeout: 3 * time.Second}

// ContributorCard is the card for one contributor's merged PR. Handle, Role,
// PRNumber, PRCount, CrewCount, Date and Hero are optional. PRCount is how many
// PRs the person has in the release; PRTitle is the first of them.
type ContributorCard struct {
	Name, AvatarURL, Project, Version, PRTitle string
	Handle, Role                               string
	PRNumber, PRCount, CrewCount               int
	Date                                       time.Time
	Hero                                       image.Image
}

// ReleaseCard is the card for a release. CrewCount, Date and Hero are optional.
type ReleaseCard struct {
	Project, Version, Headline string
	CrewCount                  int
	Date                       time.Time
	Hero                       image.Image
}

func hexColor(h string) color.RGBA {
	r, g, b := parseHex(h)
	return color.RGBA{uint8(r*255 + 0.5), uint8(g*255 + 0.5), uint8(b*255 + 0.5), 255}
}

func setHex(dc *gg.Context, h string) { dc.SetColor(hexColor(h)) }

// pass is what both card kinds reduce to before drawing.
type pass struct {
	eyebrow, title, meta, detail, repo string
	// more is "and N more", kept on its own line under a clipped detail.
	more                           string
	stubLabel, stubValue, stubDate string
	avatarURL                      string
	hero                           image.Image
}

// Render draws a 1200x630 PNG boarding pass for a ContributorCard or ReleaseCard.
func Render(c any, p Preset) ([]byte, error) {
	fs, err := loadFonts()
	if err != nil {
		return nil, err
	}
	var d pass
	switch v := c.(type) {
	case ContributorCard:
		d.title = titleText(fs.display, v.Name, "Contributor")
		d.avatarURL, d.hero = v.AvatarURL, v.Hero
		ref := prRef(v.PRNumber)
		if v.PRCount > 1 {
			ref = strconv.Itoa(v.PRCount) + " PRs"
		}
		d.eyebrow = joinDot(v.Version, ref)
		if d.eyebrow == "" {
			d.eyebrow = v.Project
		}
		role := v.Role
		if strings.TrimSpace(role) == "" {
			role = "Contributor"
		}
		handle := strings.TrimPrefix(strings.TrimSpace(v.Handle), "@")
		if handle != "" {
			handle = "@" + handle
		}
		d.meta = joinDot(handle, role)
		d.detail = v.PRTitle
		if v.PRCount > 1 && strings.TrimSpace(v.PRTitle) != "" {
			d.more = "and " + strconv.Itoa(v.PRCount-1) + " more"
		}
		d.repo = v.Project
		d.stubLabel, d.stubValue, d.stubDate = stubText(v.CrewCount, v.Version, v.Date)
	case ReleaseCard:
		d.title = titleText(fs.display, v.Project, "Release")
		d.hero = v.Hero
		d.eyebrow = joinDot("Release", v.Version)
		d.detail = v.Headline
		d.stubLabel, d.stubValue, d.stubDate = stubText(v.CrewCount, v.Version, v.Date)
	default:
		return nil, fmt.Errorf("cards: unsupported card type %T", c)
	}

	dc := gg.NewContext(cardW, cardH)
	setHex(dc, p.Field)
	dc.Clear()
	if d.hero != nil {
		drawHero(dc, d.hero, p)
	}

	// Eyebrow: a small dot in the mark colour, then the uppercase line.
	ef, err := face(fs.label, 27)
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(ef)
	setHex(dc, p.Mark)
	dc.DrawCircle(pad+8, pad+14, 8)
	dc.Fill()
	setHex(dc, p.eyebrow())
	eyebrow := ellipsize(ef, strings.ToUpper(keepGlyphs(fs.label, d.eyebrow)), textW-28)
	dc.DrawString(eyebrow, pad+28, pad+24)

	// Name: one line at 80px, else wrapped at 64px to two lines.
	lines, size := fitTitle(fs.display, d.title, textW)
	tf, err := face(fs.display, size)
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(tf)
	setHex(dc, p.Type)
	lh := size * 1.06
	y := 150.0 + size*0.8
	for _, l := range lines {
		dc.DrawString(l, pad, y)
		y += lh
	}
	y = y - lh + 34

	// What they shipped: up to two lines of body text.
	bf, err := face(fs.body, 32)
	if err != nil {
		return nil, err
	}
	dc.SetFontFace(bf)
	setHex(dc, blend(p.Type, p.Field, 0.9))
	if detail := keepGlyphs(fs.body, d.detail); detail != "" {
		lines := wrap(bf, detail, textW, 2)
		if d.more != "" { // first title on one line, then "and N more"
			lines = append(wrap(bf, detail, textW, 1), d.more)
		}
		for _, l := range lines {
			dc.DrawString(l, pad, y+26)
			y += 44
		}
	}

	// Divider and the bottom row: avatar, handle and role, repo.
	const rowY = 482
	dc.SetLineWidth(3)
	setHex(dc, blend(p.Type, p.Field, 0.18))
	dc.DrawLine(pad, rowY-22, mainW-pad, rowY-22)
	dc.Stroke()
	drawAvatar(dc, d.avatarURL, p, pad+avatarSize/2.0, rowY+avatarSize/2.0-8, avatarSize/2.0)
	tx := float64(pad + avatarSize + 24)
	avail := float64(mainW-pad) - tx
	mf, err := face(fs.body, 30)
	if err != nil {
		return nil, err
	}
	rf, err := face(fs.body, 26)
	if err != nil {
		return nil, err
	}
	meta, repo := keepGlyphs(fs.body, d.meta), keepGlyphs(fs.body, d.repo)
	switch {
	case meta != "" && repo != "":
		dc.SetFontFace(mf)
		setHex(dc, p.muted())
		dc.DrawString(ellipsize(mf, meta, avail), tx, rowY+30-8)
		dc.SetFontFace(rf)
		setHex(dc, blend(p.Type, p.Field, 0.5))
		dc.DrawString(ellipsize(rf, repo, avail), tx, rowY+66-8)
	case meta != "" || repo != "":
		t, f := meta, mf
		if t == "" {
			t, f = repo, rf
		}
		dc.SetFontFace(f)
		setHex(dc, p.muted())
		dc.DrawString(ellipsize(f, t, avail), tx, rowY+avatarSize/2.0+10-8)
	}

	// Perforation: dashed line with a notch cut at the top and bottom edge.
	px := float64(mainW) + perfW/2.0
	setHex(dc, blend(p.Type, p.Field, 0.22))
	dc.SetLineWidth(5)
	dc.SetDash(15, 11)
	dc.DrawLine(px, notchR+14, px, cardH-notchR-14)
	dc.Stroke()
	dc.SetDash()
	setHex(dc, ground)
	dc.DrawCircle(px, 0, notchR)
	dc.Fill()
	dc.DrawCircle(px, cardH, notchR)
	dc.Fill()

	if err := drawStub(dc, fs, p, d); err != nil {
		return nil, err
	}
	return encode(dc.Image())
}

func prRef(n int) string {
	if n <= 0 {
		return ""
	}
	return "PR #" + strconv.Itoa(n)
}

func joinDot(parts ...string) string {
	var out []string
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " · ")
}

// stubText picks the stub: crew size when known, else the version.
func stubText(crew int, version string, date time.Time) (label, value, d string) {
	label, value = "Pass", strings.TrimSpace(version)
	if crew > 0 {
		label, value = "Crew", strconv.Itoa(crew)
	}
	if !date.IsZero() {
		d = date.Format("2 Jan")
	}
	return label, value, d
}

func drawStub(dc *gg.Context, fs fontSet, p Preset, d pass) error {
	x := float64(mainW + perfW)
	w := float64(stubW) - 40 - 24
	lf, err := face(fs.label, 24)
	if err != nil {
		return err
	}
	dc.SetFontFace(lf)
	setHex(dc, blend(p.Type, p.Field, 0.55))
	dc.DrawString(strings.ToUpper(keepGlyphs(fs.label, d.stubLabel)), x+4, pad+24)

	if v := keepGlyphs(fs.display, d.stubValue); v != "" {
		var vf font.Face
		for px := 88.0; px >= 36; px -= 4 {
			if vf, err = face(fs.display, px); err != nil {
				return err
			}
			if textWidth(vf, v) <= w {
				break
			}
		}
		dc.SetFontFace(vf)
		setHex(dc, p.Type)
		dc.DrawString(v, x+4, 360)
	}
	if dt := keepGlyphs(fs.body, d.stubDate); dt != "" {
		df, err := face(fs.body, 28)
		if err != nil {
			return err
		}
		dc.SetFontFace(df)
		setHex(dc, blend(p.Type, p.Field, 0.55))
		dc.DrawString(dt, x+4, cardH-pad)
	}
	return nil
}

// drawHero lays a cover-cropped hero image behind the card, then a heavy wash
// of the field colour so every line of text stays readable.
func drawHero(dc *gg.Context, img image.Image, p Preset) {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return
	}
	// Largest centred crop with the card's aspect ratio.
	cw, ch := b.Dx(), b.Dx()*cardH/cardW
	if ch > b.Dy() {
		cw, ch = b.Dy()*cardW/cardH, b.Dy()
	}
	x0, y0 := b.Min.X+(b.Dx()-cw)/2, b.Min.Y+(b.Dy()-ch)/2
	dst := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, image.Rect(x0, y0, x0+cw, y0+ch), xdraw.Over, nil)
	dc.DrawImage(dst, 0, 0)
	f := hexColor(p.Field)
	dc.SetColor(color.NRGBA{f.R, f.G, f.B, 224})
	dc.DrawRectangle(0, 0, cardW, cardH)
	dc.Fill()
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

func drawAvatar(dc *gg.Context, url string, p Preset, cx, cy, r float64) {
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
	dc.DrawImage(dst, int(cx-r), int(cy-r))
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

// fitTitle picks the title size and lines: 80px on one line, else 64px on one
// line, else 64px wrapped to at most two lines with an ellipsis.
func fitTitle(f *sfnt.Font, s string, maxW float64) ([]string, float64) {
	big, err1 := face(f, 80)
	small, err2 := face(f, 64)
	if err1 != nil || err2 != nil {
		return []string{s}, 80
	}
	if textWidth(big, s) <= maxW {
		return []string{s}, 80
	}
	if textWidth(small, s) <= maxW {
		return []string{s}, 64
	}
	return wrap(small, s, maxW, 2), 64
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
