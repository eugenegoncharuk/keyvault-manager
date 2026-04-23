// Package ui — syntax.go
// JSON and YAML tokenisers that produce widget.RichTextSegment slices
// suitable for displaying syntax-highlighted secret values in a RichText widget.
package ui

import (
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// tok associates a piece of text with a theme colour name.
type tok struct {
	color fyne.ThemeColorName
	text  string
}

// DetectLanguage guesses whether content is JSON or YAML.
func DetectLanguage(content string) string {
	t := strings.TrimSpace(content)
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		return "json"
	}
	return "yaml"
}

// HighlightContent converts raw text to syntax-coloured RichText segments.
func HighlightContent(content string) []widget.RichTextSegment {
	if content == "" {
		return nil
	}
	lang := DetectLanguage(content)
	var tokens []tok
	if lang == "json" {
		tokens = tokenizeJSON(content)
	} else {
		tokens = tokenizeYAML(content)
	}

	segs := make([]widget.RichTextSegment, 0, len(tokens))
	for _, t := range tokens {
		c := t.color
		if c == "" {
			c = theme.ColorNameForeground
		}
		segs = append(segs, &widget.TextSegment{
			Style: widget.RichTextStyle{
				ColorName: c,
				TextStyle: fyne.TextStyle{Monospace: true},
				Inline:    true,
			},
			Text: t.text,
		})
	}
	return segs
}

// ── JSON tokeniser ─────────────────────────────────────────────────────────

func tokenizeJSON(input string) []tok {
	var tokens []tok
	runes := []rune(input)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Whitespace
		if unicode.IsSpace(ch) {
			j := i + 1
			for j < len(runes) && unicode.IsSpace(runes[j]) {
				j++
			}
			tokens = append(tokens, tok{"", string(runes[i:j])})
			i = j
			continue
		}

		// String literal
		if ch == '"' {
			j := i + 1
			for j < len(runes) {
				if runes[j] == '\\' {
					j += 2
					continue
				}
				if runes[j] == '"' {
					j++
					break
				}
				j++
			}
			if j > len(runes) {
				j = len(runes)
			}
			text := string(runes[i:j])
			// A key is a string immediately followed by ':' (whitespace allowed).
			k := j
			for k < len(runes) && unicode.IsSpace(runes[k]) {
				k++
			}
			if k < len(runes) && runes[k] == ':' {
				tokens = append(tokens, tok{ColorSyntaxKey, text})
			} else {
				tokens = append(tokens, tok{ColorSyntaxString, text})
			}
			i = j
			continue
		}

		// Number
		if unicode.IsDigit(ch) || (ch == '-' && i+1 < len(runes) && unicode.IsDigit(runes[i+1])) {
			j := i + 1
			for j < len(runes) {
				r := runes[j]
				if unicode.IsDigit(r) || r == '.' || r == 'e' || r == 'E' || r == '+' || r == '-' {
					j++
					continue
				}
				break
			}
			tokens = append(tokens, tok{ColorSyntaxNumber, string(runes[i:j])})
			i = j
			continue
		}

		// Keywords: true / false / null
		rem := string(runes[i:])
		if kw := matchJSONKeyword(rem); kw != "" {
			tokens = append(tokens, tok{ColorSyntaxBool, kw})
			i += len([]rune(kw))
			continue
		}

		// Structural punctuation
		if ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',' {
			tokens = append(tokens, tok{ColorSyntaxPunct, string(ch)})
			i++
			continue
		}

		tokens = append(tokens, tok{"", string(ch)})
		i++
	}
	return tokens
}

func matchJSONKeyword(s string) string {
	for _, kw := range []string{"true", "false", "null"} {
		if strings.HasPrefix(s, kw) {
			rest := s[len(kw):]
			if rest == "" {
				return kw
			}
			r := []rune(rest)[0]
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return kw
			}
		}
	}
	return ""
}

// ── YAML tokeniser ─────────────────────────────────────────────────────────

func tokenizeYAML(input string) []tok {
	var tokens []tok
	lines := strings.Split(input, "\n")

	for idx, line := range lines {
		if idx > 0 {
			tokens = append(tokens, tok{"", "\n"})
		}

		if strings.TrimSpace(line) == "" {
			tokens = append(tokens, tok{"", line})
			continue
		}

		trimmed := strings.TrimSpace(line)

		// Full-line comment
		if strings.HasPrefix(trimmed, "#") {
			tokens = append(tokens, tok{ColorSyntaxComment, line})
			continue
		}

		// Leading indentation
		indent := countLeadingSpaces(line)
		if indent > 0 {
			tokens = append(tokens, tok{"", line[:indent]})
		}
		rest := line[indent:]

		// YAML document markers
		if rest == "---" || rest == "..." {
			tokens = append(tokens, tok{ColorSyntaxPunct, rest})
			continue
		}

		// List item prefix "- "
		if strings.HasPrefix(rest, "- ") {
			tokens = append(tokens, tok{ColorSyntaxPunct, "- "})
			rest = rest[2:]
		} else if rest == "-" {
			tokens = append(tokens, tok{ColorSyntaxPunct, "-"})
			continue
		}

		// key: value
		if colonIdx := strings.Index(rest, ": "); colonIdx > 0 {
			key := rest[:colonIdx]
			value := rest[colonIdx+2:]
			tokens = append(tokens, tok{ColorSyntaxKey, key})
			tokens = append(tokens, tok{ColorSyntaxPunct, ": "})
			// Check for inline comment
			if ci := findInlineComment(value); ci >= 0 {
				tokens = append(tokens, yamlValueTok(value[:ci]))
				tokens = append(tokens, tok{ColorSyntaxComment, value[ci:]})
			} else {
				tokens = append(tokens, yamlValueTok(value))
			}
			continue
		}

		// Bare key: (mapping start)
		if strings.HasSuffix(rest, ":") && !strings.Contains(rest[:len(rest)-1], ":") {
			tokens = append(tokens, tok{ColorSyntaxKey, rest[:len(rest)-1]})
			tokens = append(tokens, tok{ColorSyntaxPunct, ":"})
			continue
		}

		// Plain scalar or multi-line block
		tokens = append(tokens, yamlValueTok(rest))
	}
	return tokens
}

func countLeadingSpaces(s string) int {
	for i, ch := range s {
		if ch != ' ' && ch != '\t' {
			return i
		}
	}
	return len(s)
}

func findInlineComment(s string) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '#' && !inSingle && !inDouble && i > 0 && s[i-1] == ' ':
			return i
		}
	}
	return -1
}

func yamlValueTok(value string) tok {
	v := strings.TrimSpace(value)
	switch strings.ToLower(v) {
	case "true", "false", "null", "~", "yes", "no", "on", "off":
		return tok{ColorSyntaxBool, value}
	}
	if isYAMLNumber(v) {
		return tok{ColorSyntaxNumber, value}
	}
	if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') ||
		(v[0] == '\'' && v[len(v)-1] == '\'')) {
		return tok{ColorSyntaxString, value}
	}
	// Block scalars (| or >) are treated as plain text
	return tok{"", value}
}

func isYAMLNumber(s string) bool {
	if s == "" {
		return false
	}
	i := 0
	if s[i] == '-' || s[i] == '+' {
		i++
	}
	if i >= len(s) {
		return false
	}
	hasDot, hasDigit := false, false
	for ; i < len(s); i++ {
		ch := s[i]
		if ch >= '0' && ch <= '9' {
			hasDigit = true
		} else if ch == '.' && !hasDot {
			hasDot = true
		} else {
			return false
		}
	}
	return hasDigit
}
