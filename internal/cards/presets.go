package cards

import (
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
)

// Preset is one card colour set: field fill, type colour and the small mark.
// Eyebrow and Muted are the tinted eyebrow line and the quiet text; when
// empty they are derived from Type over Field.
type Preset struct {
	Name, Field, Type, Mark, Eyebrow, Muted string
}

const (
	white = "#FFFFFF"
	ink   = "#15181F"
	// darkType is the type colour of the light presets, kept at the value they were tuned against.
	darkType = "#16161A"
	accent   = "#4A45D1"
	paper    = "#E8EBF2"
	// ground is the page colour the perforation notches are cut to.
	ground = "#E8EBF2"
	// inkPreset indexes the ink boarding pass in Presets, the default look.
	inkPreset = 7
)

// Presets are the eight card fields, tuned so Type on Field is at least 7:1.
// Hue and saturation are kept from the DESIGN.md hex; only HSL lightness moved.
//
//	name       design     tuned      type   contrast  mark
//	cobalt     #1F5FBF    #1C56AD    white  7.04      white
//	vermilion  #E4572E    #EC8669    ink    7.03      ink
//	teal       #0E8A6B    #0A654E    white  7.04      white
//	ochre      #D9A520    #D9A520    ink    8.04      ink (already passed)
//	magenta    #B3326B    #9E2C5F    white  7.05      white
//	forest     #2E6B3F    #2B643B    white  7.03      white
//	orange     #D9480F    #F38357    ink    7.04      ink
//	ink        #15181F    #15181F    white  17.5      eyebrow #ADA9FF
//
// Index 7 is the ink boarding pass, now the default for every launch. The
// other seven stay valid so cards of earlier launches keep their stored look.
var Presets = [8]Preset{
	{"cobalt", "#1C56AD", white, white, "", ""},
	{"vermilion", "#EC8669", darkType, darkType, "", ""},
	{"teal", "#0A654E", white, white, "", ""},
	{"ochre", "#D9A520", darkType, darkType, "", ""},
	{"magenta", "#9E2C5F", white, white, "", ""},
	{"forest", "#2B643B", white, white, "", ""},
	{"orange", "#F38357", darkType, darkType, "", ""},
	{"ink", "#15181F", white, "#ADA9FF", "#ADA9FF", "#B9C0CE"},
}

// PresetFor returns the preset for a launch: the ink boarding pass for all.
// The id is kept in the signature so a per-launch choice can come back.
func PresetFor(_ uuid.UUID) Preset { return Presets[inkPreset] }

// blend returns fg over bg at opacity a, as a hex colour.
func blend(fg, bg string, a float64) string {
	fr, fgc, fb := parseHex(fg)
	br, bgc, bb := parseHex(bg)
	h := func(f, b float64) int { return int((f*a+b*(1-a))*255 + 0.5) }
	return fmt.Sprintf("#%02X%02X%02X", h(fr, br), h(fgc, bgc), h(fb, bb))
}

func (p Preset) eyebrow() string {
	if p.Eyebrow != "" {
		return p.Eyebrow
	}
	return blend(p.Type, p.Field, 0.9)
}

func (p Preset) muted() string {
	if p.Muted != "" {
		return p.Muted
	}
	return blend(p.Type, p.Field, 0.84)
}

func parseHex(s string) (r, g, b float64) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return 0, 0, 0
	}
	var v [3]float64
	for i := range v {
		var n int
		for _, c := range s[2*i : 2*i+2] {
			n <<= 4
			switch {
			case c >= '0' && c <= '9':
				n |= int(c - '0')
			case c >= 'a' && c <= 'f':
				n |= int(c-'a') + 10
			case c >= 'A' && c <= 'F':
				n |= int(c-'A') + 10
			}
		}
		v[i] = float64(n) / 255
	}
	return v[0], v[1], v[2]
}

func luminance(hex string) float64 {
	r, g, b := parseHex(hex)
	lin := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// Contrast is the WCAG 2.x contrast ratio of two hex colours.
func Contrast(fg, bg string) float64 {
	a, b := luminance(fg), luminance(bg)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}
