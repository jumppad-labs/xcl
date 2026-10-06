package prettylog

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// blockHeader matches the line opening a block: its type, any labels, and the
// brace, i.e. `deployment "api" {` or `resource "network" "main" {`
var blockHeader = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_-]*)((?:\s+"[^"]*")*)(\s*\{\s*)$`)

// blockLabel matches one label of a block header
var blockLabel = regexp.MustCompile(`(\s+)("[^"]*")`)

// attributeLine matches a line setting an attribute, its name and the rest of
// the line holding the value
var attributeLine = regexp.MustCompile(`^(\s*)([A-Za-z_][A-Za-z0-9_-]*)(\s*=\s*)(.*)$`)

// valueToken matches one piece of a value, in order of precedence: a comment,
// a string, a number, a constant, and an identifier starting a reference
var valueToken = regexp.MustCompile(`#.*$|//.*$|"(?:[^"\\]|\\.)*"|-?\b[0-9]+(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?\b|\b(?:true|false|null)\b|\b[A-Za-z_][A-Za-z0-9_-]*\b(?:\.[A-Za-z_][A-Za-z0-9_-]*)*`)

// highlighter colours configuration text the way the xcl-vscode grammar
// does, so the configuration in the log reads like it does in the editor. It
// works line by line on the text xcl's encoder writes, which is always
// formatted: one block header, attribute or value per line.
type highlighter struct {
	blockType lipgloss.Style
	typeLabel lipgloss.Style
	nameLabel lipgloss.Style
	attribute lipgloss.Style
	str       lipgloss.Style
	constant  lipgloss.Style
	reference lipgloss.Style
	comment   lipgloss.Style
}

// newHighlighter returns a highlighter whose styles render through r, which
// decides whether the writer it was made for shows colour at all
func newHighlighter(r *lipgloss.Renderer) highlighter {
	return highlighter{
		blockType: r.NewStyle().Foreground(lipgloss.Color("5")).Bold(true),
		typeLabel: r.NewStyle().Foreground(lipgloss.Color("6")),
		nameLabel: r.NewStyle().Foreground(lipgloss.Color("2")),
		attribute: r.NewStyle().Foreground(lipgloss.Color("4")),
		str:       r.NewStyle().Foreground(lipgloss.Color("3")),
		constant:  r.NewStyle().Foreground(lipgloss.Color("13")),
		reference: r.NewStyle().Foreground(lipgloss.Color("6")),
		comment:   r.NewStyle().Foreground(lipgloss.Color("8")).Italic(true),
	}
}

// highlight returns text with each line coloured. Only colour is added, the
// text itself is unchanged.
func (h highlighter) highlight(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = h.line(line)
	}

	return strings.Join(lines, "\n")
}

// line colours one line: a block header, an attribute, or a value continuing
// one, such as an element of a list written across several lines
func (h highlighter) line(line string) string {
	if parts := blockHeader.FindStringSubmatch(line); parts != nil {
		return parts[1] + h.blockType.Render(parts[2]) + h.labels(parts[3]) + parts[4]
	}

	if parts := attributeLine.FindStringSubmatch(line); parts != nil {
		return parts[1] + h.attribute.Render(parts[2]) + parts[3] + h.value(parts[4])
	}

	return h.value(line)
}

// labels colours the labels of a block header. With two labels the first is
// the block's type and the second its name, a single label is its name.
func (h highlighter) labels(labels string) string {
	found := blockLabel.FindAllStringSubmatch(labels, -1)

	out := ""
	for i, label := range found {
		style := h.nameLabel
		if len(found) == 2 && i == 0 {
			style = h.typeLabel
		}

		out += label[1] + style.Render(label[2])
	}

	return out
}

// value colours the tokens of a value, leaving punctuation such as brackets,
// braces and commas as they are
func (h highlighter) value(value string) string {
	return valueToken.ReplaceAllStringFunc(value, func(token string) string {
		switch {
		case strings.HasPrefix(token, "#"), strings.HasPrefix(token, "//"):
			return h.comment.Render(token)
		case strings.HasPrefix(token, `"`):
			return h.str.Render(token)
		case token == "true", token == "false", token == "null", isNumber(token):
			return h.constant.Render(token)
		case strings.Contains(token, "."):
			root, rest, _ := strings.Cut(token, ".")
			return h.reference.Render(root) + "." + rest
		}

		return token
	})
}

// isNumber reports whether token is a number, its first character being a
// digit or a minus sign
func isNumber(token string) bool {
	return token != "" && (token[0] == '-' || (token[0] >= '0' && token[0] <= '9'))
}
