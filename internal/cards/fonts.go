package cards

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

//go:embed fonts/*.ttf
var fontFS embed.FS

type fontSet struct {
	// display is Gabarito 700 (names, numbers); label is Hanken Grotesk 600
	// (small uppercase labels); body is Hanken Grotesk 400; mono is for the QR slide URL.
	display, label, body, mono *sfnt.Font
}

var (
	fontsOnce sync.Once
	fontsVal  fontSet
	fontsErr  error
)

func loadFonts() (fontSet, error) {
	fontsOnce.Do(func() {
		load := func(name string) *sfnt.Font {
			if fontsErr != nil {
				return nil
			}
			b, err := fontFS.ReadFile("fonts/" + name)
			if err != nil {
				fontsErr = err
				return nil
			}
			f, err := opentype.Parse(b)
			if err != nil {
				fontsErr = fmt.Errorf("parse %s: %w", name, err)
			}
			return f
		}
		fontsVal = fontSet{
			display: load("Gabarito-700.ttf"),
			label:   load("HankenGrotesk-600.ttf"),
			body:    load("HankenGrotesk-400.ttf"),
			mono:    load("JetBrainsMono-Regular.ttf"),
		}
	})
	return fontsVal, fontsErr
}

func face(f *sfnt.Font, px float64) (font.Face, error) {
	return opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingNone})
}

// keepGlyphs drops runes the font cannot draw (emoji, CJK) and collapses
// whitespace, so no tofu reaches a card.
func keepGlyphs(f *sfnt.Font, s string) string {
	var b sfnt.Buffer
	var out strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if r < ' ' || r == 0x7f {
			continue
		}
		if i, err := f.GlyphIndex(&b, r); err != nil || i == 0 {
			continue
		}
		out.WriteRune(r)
	}
	return strings.Join(strings.Fields(out.String()), " ")
}

// must is for faces built from fonts already parsed at load; opentype only
// fails on invalid options.
func must(f font.Face, err error) font.Face {
	if err != nil {
		panic(err)
	}
	return f
}
