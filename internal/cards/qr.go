package cards

import (
	"fmt"
	"image"

	"github.com/fogleman/gg"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	slideW, slideH = 1920, 1080
	qrSide         = 432
	qrMark         = 52
)

// RenderQRSlide draws the projectable 1920x1080 slide: title, QR of shortURL
// (error correction High, accent mark over the centre) and the URL below.
func RenderQRSlide(title, shortURL string) ([]byte, error) {
	fs, err := loadFonts()
	if err != nil {
		return nil, err
	}
	q, err := qrcode.New(shortURL, qrcode.Highest)
	if err != nil {
		return nil, fmt.Errorf("qr: %w", err)
	}
	var qimg image.Image = q.Image(qrSide)

	dc := gg.NewContext(slideW, slideH)
	setHex(dc, paper)
	dc.Clear()

	title = keepGlyphs(fs.display, title)
	if title == "" {
		title = "Launch Crew"
	}
	const maxW = slideW - 2*96
	var tf = must(face(fs.display, 96))
	for px := 96.0; px >= 48; px -= 4 {
		tf = must(face(fs.display, px))
		if textWidth(tf, title) <= maxW {
			break
		}
	}
	title = ellipsize(tf, title, maxW)
	dc.SetFontFace(tf)
	setHex(dc, ink)
	dc.DrawStringAnchored(title, slideW/2, 190, 0.5, 0)

	qx, qy := (slideW-qrSide)/2, (slideH-qrSide)/2
	dc.DrawImage(qimg, qx, qy)
	setHex(dc, accent)
	dc.DrawRectangle(float64(slideW-qrMark)/2, float64(slideH-qrMark)/2, qrMark, qrMark)
	dc.Fill()

	mf := must(face(fs.mono, 40))
	dc.SetFontFace(mf)
	setHex(dc, ink)
	dc.DrawStringAnchored(shortURL, slideW/2, float64(qy+qrSide+90), 0.5, 0)

	setHex(dc, accent)
	dc.DrawRectangle(96, slideH-96-24, 24, 24)
	dc.Fill()
	setHex(dc, ink)
	dc.SetFontFace(must(face(fs.display, 32)))
	dc.DrawString("Launch Crew", 96+24+12, slideH-96)
	return encode(dc.Image())
}
