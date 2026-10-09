package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// statusLine is a single-line, ellipsis-truncated status label.  Long or
// multi-line messages (e.g. verbose Azure CLI errors) would otherwise force
// the label's MinSize — and therefore the whole window — to grow to the
// full text width.  Errors keep their full text behind a "Details" button.
type statusLine struct {
	label      *widget.Label
	detailsBtn *widget.Button
	full       string
	obj        fyne.CanvasObject
}

func newStatusLine(w fyne.Window) *statusLine {
	s := &statusLine{}
	s.label = widget.NewLabel("")
	s.label.TextStyle = fyne.TextStyle{Italic: true}
	s.label.Truncation = fyne.TextTruncateEllipsis

	s.detailsBtn = widget.NewButton("Details", func() {
		msg := widget.NewLabel(s.full)
		msg.Wrapping = fyne.TextWrapWord
		msg.Selectable = true
		scroll := container.NewVScroll(msg)
		scroll.SetMinSize(fyne.NewSize(600, 300))
		dialog.ShowCustom("Error details", "Close", scroll, w)
	})
	s.detailsBtn.Importance = widget.LowImportance
	s.detailsBtn.Hide()

	s.obj = container.NewBorder(nil, nil, nil, s.detailsBtn, s.label)
	return s
}

// SetText shows an informational message and hides the details button.
func (s *statusLine) SetText(text string) {
	s.full = text
	s.detailsBtn.Hide()
	s.label.SetText(oneLine(text))
}

// SetError shows "❌ <msg>" truncated to one line; the full text is
// available via the details button.
func (s *statusLine) SetError(format string, args ...any) {
	s.full = fmt.Sprintf(format, args...)
	s.label.SetText("❌ " + oneLine(s.full))
	s.detailsBtn.Show()
}

// oneLine collapses all whitespace (including newlines) into single spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
