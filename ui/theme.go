package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Syntax colour constants — registered as custom theme colour names so that
// widget.RichText segments can reference them by name.
const (
	ColorSyntaxKey     fyne.ThemeColorName = "syntax-key"
	ColorSyntaxString  fyne.ThemeColorName = "syntax-string"
	ColorSyntaxNumber  fyne.ThemeColorName = "syntax-number"
	ColorSyntaxBool    fyne.ThemeColorName = "syntax-bool"
	ColorSyntaxComment fyne.ThemeColorName = "syntax-comment"
	ColorSyntaxPunct   fyne.ThemeColorName = "syntax-punct"
)

// kvTheme wraps the built-in dark theme and adds syntax-highlighting colours.
type kvTheme struct{}

func (kvTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case ColorSyntaxKey:
		return color.RGBA{R: 86, G: 182, B: 194, A: 255} // teal
	case ColorSyntaxString:
		return color.RGBA{R: 152, G: 195, B: 121, A: 255} // green
	case ColorSyntaxNumber:
		return color.RGBA{R: 209, G: 154, B: 102, A: 255} // orange
	case ColorSyntaxBool:
		return color.RGBA{R: 198, G: 120, B: 221, A: 255} // purple
	case ColorSyntaxComment:
		return color.RGBA{R: 92, G: 99, B: 112, A: 255} // grey
	case ColorSyntaxPunct:
		return color.RGBA{R: 171, G: 178, B: 191, A: 255} // light grey

	// ── Editor cursor / focus ring ──────────────────────────────────────────
	// Fyne v2 uses ColorNameFocus for the Entry text cursor and the focused
	// input border.  #61AFEF is One Dark Pro's signature bright blue — high
	// contrast on any dark background and instantly recognisable as a cursor.
	case theme.ColorNameFocus:
		return color.RGBA{R: 97, G: 175, B: 239, A: 255} // #61AFEF
	}
	return theme.DarkTheme().Color(name, theme.VariantDark)
}

func (kvTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

func (kvTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

func (kvTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInputBorder {
		// Default is 2 — bump to 3 for a noticeably bolder cursor line.
		return 3
	}
	return theme.DarkTheme().Size(name)
}
