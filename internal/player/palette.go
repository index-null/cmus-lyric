package player

import (
	"hash/fnv"
	"math"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/lucasb-eyer/go-colorful"
)

type palette struct {
	Primary   colorful.Color
	Secondary colorful.Color
	Accent    colorful.Color
	Border    colorful.Color
	Title     colorful.Color
	TitleBg   colorful.Color
	Artist    colorful.Color
	Trans     colorful.Color
	Upcoming  colorful.Color
	Passed    colorful.Color
	Dim       colorful.Color
}

const minLuminance = 0.18

func ensureLuminance(c colorful.Color, minL float64) colorful.Color {
	h, s, l := c.Hsl()
	if l < minL {
		l = minL
	}
	return colorful.Hsl(h, s, l)
}

func ensureContrast(fg colorful.Color, minRatio float64) colorful.Color {
	bg := colorful.Color{R: 0, G: 0, B: 0}
	for range 20 {
		if contrastRatio(fg, bg) >= minRatio {
			return fg
		}
		h, s, l := fg.Hsl()
		l = math.Min(l+0.05, 0.95)
		fg = colorful.Hsl(h, s, l)
	}
	return fg
}

func contrastRatio(fg, bg colorful.Color) float64 {
	l1 := relativeLuminance(fg)
	l2 := relativeLuminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func relativeLuminance(c colorful.Color) float64 {
	r := linearize(c.R)
	g := linearize(c.G)
	b := linearize(c.B)
	return 0.2126*r + 0.7152*g + 0.0722*b
}

func linearize(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func generatePalette(seed string) palette {
	h := fnv.New64a()
	h.Write([]byte(seed))
	hash := h.Sum64()

	hue := float64(hash%360) + float64(hash%100)/100.0
	hue = math.Mod(hue, 360)

	hue2 := math.Mod(hue+30, 360)
	hue3 := math.Mod(hue+180, 360)

	primary := ensureContrast(ensureLuminance(colorful.Hsl(hue, 0.75, 0.60), minLuminance), 4.5)
	secondary := ensureContrast(ensureLuminance(colorful.Hsl(hue2, 0.60, 0.55), minLuminance), 4.5)
	accent := ensureContrast(ensureLuminance(colorful.Hsl(hue3, 0.70, 0.60), minLuminance), 4.5)

	titleBg := ensureLuminance(colorful.Hsl(hue, 0.65, 0.35), 0.10)
	titleFg := ensureContrast(colorful.Hsl(hue, 0.10, 0.95), 7.0)

	artist := ensureContrast(ensureLuminance(colorful.Hsl(hue2, 0.50, 0.65), minLuminance), 4.5)
	trans := ensureContrast(ensureLuminance(colorful.Hsl(hue2, 0.45, 0.55), minLuminance), 3.0)
	upcoming := ensureContrast(colorful.Hsl(hue, 0.10, 0.70), 4.5)
	passed := colorful.Hsl(hue, 0.10, 0.40)
	dim := colorful.Hsl(hue, 0.05, 0.20)

	border := ensureLuminance(colorful.Hsl(hue, 0.50, 0.40), 0.10)

	return palette{
		Primary:   primary,
		Secondary: secondary,
		Accent:    accent,
		Border:    border,
		Title:     titleFg,
		TitleBg:   titleBg,
		Artist:    artist,
		Trans:     trans,
		Upcoming:  upcoming,
		Passed:    passed,
		Dim:       dim,
	}
}

func (p palette) progressOpts() []progress.Option {
	return []progress.Option{
		progress.WithGradient(p.Primary.Hex(), p.Secondary.Hex()),
		progress.WithoutPercentage(),
	}
}
