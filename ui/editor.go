package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Note: macOS-specific keyboard navigation (Cmd+Arrow, Opt+Arrow) is
// implemented in macentry.go via the macEntry custom widget used below.

// SyntaxEditor is a dual-mode code-editing component.
//
//   - Edit mode  — plain multiline Entry (keyboard-editable, monospace font).
//   - Preview mode — syntax-highlighted RichText (read-only, colour-coded).
//
// Toggle between modes with ToggleButton().
// The underlying text is always kept in sync via the Entry widget.
type SyntaxEditor struct {
	entry      *macEntry
	richText   *widget.RichText
	toggleBtn  *widget.Button
	entryScroll *container.Scroll
	richScroll  *container.Scroll
	wrapper    *fyne.Container
	editMode   bool

	// OnChanged is called whenever the text in the entry changes.
	OnChanged func(string)
}

// NewSyntaxEditor creates a new, blank SyntaxEditor starting in Edit mode.
func NewSyntaxEditor() *SyntaxEditor {
	e := &SyntaxEditor{editMode: true}

	e.entry = newMacEntry() // includes TextStyle{Monospace} + WrapOff
	e.entry.OnChanged = func(text string) {
		if e.OnChanged != nil {
			e.OnChanged(text)
		}
	}

	e.richText = widget.NewRichText()
	e.richText.Wrapping = fyne.TextWrapBreak

	e.entryScroll = container.NewScroll(e.entry)
	e.richScroll = container.NewScroll(e.richText)

	e.toggleBtn = widget.NewButton("Preview ✨", e.toggle)

	// Start in edit mode — only the entry scroll is in the wrapper.
	e.wrapper = container.NewStack(e.entryScroll)

	return e
}

func (e *SyntaxEditor) toggle() {
	if e.editMode {
		// Render highlights from current entry text.
		segs := HighlightContent(e.entry.Text)
		e.richText.Segments = segs
		e.richText.Refresh()
		// Swap entry scroll for rich text scroll.
		e.wrapper.Objects = []fyne.CanvasObject{e.richScroll}
		e.wrapper.Refresh()
		e.toggleBtn.SetText("Edit ✏️")
		e.editMode = false
	} else {
		// Switch back to edit mode.
		e.wrapper.Objects = []fyne.CanvasObject{e.entryScroll}
		e.wrapper.Refresh()
		e.toggleBtn.SetText("Preview ✨")
		e.editMode = true
	}
}

// ToggleButton returns the Edit/Preview toggle button for inclusion in a toolbar.
func (e *SyntaxEditor) ToggleButton() fyne.CanvasObject {
	return e.toggleBtn
}

// Body returns the scrollable editor body that should fill available space.
func (e *SyntaxEditor) Body() fyne.CanvasObject {
	return e.wrapper
}

// SetText replaces the editor content.
// Safe to call from any goroutine (delegates to widget.Entry.SetText which is
// goroutine-safe in Fyne v2).
func (e *SyntaxEditor) SetText(text string) {
	e.entry.SetText(text)
	// If already in preview mode, refresh the highlights too.
	if !e.editMode {
		segs := HighlightContent(text)
		e.richText.Segments = segs
		e.richText.Refresh()
	}
}

// GetText returns the current editor text (always from the Entry widget).
func (e *SyntaxEditor) GetText() string {
	return e.entry.Text
}

// Clear empties the editor and returns to Edit mode.
func (e *SyntaxEditor) Clear() {
	if !e.editMode {
		// Force back to edit mode so the entry is visible.
		e.wrapper.Objects = []fyne.CanvasObject{e.entryScroll}
		e.wrapper.Refresh()
		e.toggleBtn.SetText("Preview ✨")
		e.editMode = true
	}
	e.entry.SetText("")
	e.richText.Segments = nil
	e.richText.Refresh()
}
