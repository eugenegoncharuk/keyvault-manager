package ui

import (
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// macEntry is a multiline Entry that adds macOS-style keyboard navigation:
//
//	Cmd  + Left  → beginning of the current line
//	Cmd  + Right → end of the current line
//	Opt  + Left  → one "token" (word / punctuation run) to the left
//	Opt  + Right → one "token" to the right
//
// All other shortcuts and keys are delegated to widget.Entry unchanged.
type macEntry struct {
	widget.Entry
}

// newMacEntry creates a fully initialised macEntry.
func newMacEntry() *macEntry {
	e := &macEntry{}
	e.ExtendBaseWidget(e)
	e.MultiLine = true
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.Wrapping = fyne.TextWrapBreak
	return e
}

// TypedShortcut intercepts Cmd/Opt + Arrow shortcuts before the default handler.
func (e *macEntry) TypedShortcut(s fyne.Shortcut) {
	cs, ok := s.(*desktop.CustomShortcut)
	if !ok {
		e.Entry.TypedShortcut(s)
		return
	}

	switch {
	case cs.KeyName == fyne.KeyLeft && cs.Modifier == fyne.KeyModifierSuper:
		e.moveToLineStart()
	case cs.KeyName == fyne.KeyRight && cs.Modifier == fyne.KeyModifierSuper:
		e.moveToLineEnd()
	case cs.KeyName == fyne.KeyLeft && cs.Modifier == fyne.KeyModifierAlt:
		e.moveWordLeft()
	case cs.KeyName == fyne.KeyRight && cs.Modifier == fyne.KeyModifierAlt:
		e.moveWordRight()
	default:
		e.Entry.TypedShortcut(s)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

// runeLines splits the entry text into slices of runes, one per line.
func (e *macEntry) runeLines() [][]rune {
	raw := strings.Split(e.Entry.Text, "\n")
	out := make([][]rune, len(raw))
	for i, s := range raw {
		out[i] = []rune(s)
	}
	return out
}

func (e *macEntry) setCursor(row, col int) {
	e.Entry.CursorRow = row
	e.Entry.CursorColumn = col
	e.Refresh()
}

// ── Navigation ─────────────────────────────────────────────────────────────

func (e *macEntry) moveToLineStart() {
	e.setCursor(e.Entry.CursorRow, 0)
}

func (e *macEntry) moveToLineEnd() {
	ls := e.runeLines()
	row := e.Entry.CursorRow
	col := 0
	if row >= 0 && row < len(ls) {
		col = len(ls[row])
	}
	e.setCursor(row, col)
}

// moveWordLeft moves the cursor one "token" to the left.
//
// A token is either a run of word-characters (letters, digits, _) or a run of
// non-word characters (punctuation, whitespace treated as its own run).
// This mirrors Option+Left on macOS.
func (e *macEntry) moveWordLeft() {
	ls := e.runeLines()
	row, col := e.Entry.CursorRow, e.Entry.CursorColumn

	if col == 0 {
		// Jump to end of the previous line.
		if row > 0 {
			row--
			col = len(ls[row])
		}
		e.setCursor(row, col)
		return
	}

	line := ls[row]
	col-- // step back at least one position

	if col < len(line) && !isWordRune(line[col]) {
		// We stepped onto a non-word rune; skip the whole non-word run.
		for col > 0 && !isWordRune(line[col-1]) {
			col--
		}
	} else {
		// We stepped onto a word rune; skip the whole word run.
		for col > 0 && isWordRune(line[col-1]) {
			col--
		}
	}

	e.setCursor(row, col)
}

// moveWordRight moves the cursor one "token" to the right.
//
// Mirrors Option+Right on macOS.
func (e *macEntry) moveWordRight() {
	ls := e.runeLines()
	row, col := e.Entry.CursorRow, e.Entry.CursorColumn

	if row >= len(ls) {
		return
	}
	line := ls[row]

	if col >= len(line) {
		// Jump to the start of the next line.
		if row < len(ls)-1 {
			row++
			col = 0
		}
		e.setCursor(row, col)
		return
	}

	if isWordRune(line[col]) {
		// On a word rune: skip forward through the whole word.
		for col < len(line) && isWordRune(line[col]) {
			col++
		}
	} else {
		// On a non-word rune: skip forward through the non-word run.
		for col < len(line) && !isWordRune(line[col]) {
			col++
		}
	}

	e.setCursor(row, col)
}

// isWordRune returns true for letters, digits, and underscores.
func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
