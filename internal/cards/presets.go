package cards

import (
	"hash/fnv"
	"math"
	"strings"

	"github.com/google/uuid"
)

// Preset is one card colour set: field fill, type colour and the 24px mark.
type Preset struct {
	Name, Field, Type, Mark string
}

const (
	white  = "#FFFFFF"
	ink    = "#16161A"
	accent = "#E4572E"
	paper  = "#F7F7F5"
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
//	ink        #16161A    #16161A    white  18.04     accent (4.9:1)
//
// The accent mark is used only where it holds 3:1 against the field.
var Presets = [8]Preset{
	{"cobalt", "#1C56AD", white, white},
	{"vermilion", "#EC8669", ink, ink},
	{"teal", "#0A654E", white, white},
	{"ochre", "#D9A520", ink, ink},
	{"magenta", "#9E2C5F", white, white},
	{"forest", "#2B643B", white, white},
	{"orange", "#F38357", ink, ink},
	{"ink", "#16161A", white, accent},
}

// PresetFor picks the launch's preset from its id, stable across runs.
func PresetFor(launchID uuid.UUID) Preset {
	h := fnv.New32a()
	_, _ = h.Write(launchID[:])
	return Presets[h.Sum32()%8]
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
