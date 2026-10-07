package diff

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jumppad-labs/xcl/highlight"
)

const (
	// unknownPlaceholder is written in place of a value only known once an
	// apply has run
	unknownPlaceholder = "(known after apply)"

	// sensitivePlaceholder is written in place of a sensitive value the
	// result does not carry
	sensitivePlaceholder = "(sensitive value)"

	// changeIndent comes before the marker of every change line
	changeIndent = "      "

	// closingIndent comes before the brace that ends a resource block
	closingIndent = "    "
)

// markKind says which kind of change a marker stands for, and so which diff
// colour it is given when the output is highlighted
type markKind int

const (
	markInserted markKind = iota
	markDeleted
	markChanged
)

// renderOptions are the settings of Render, built from RenderOption values
type renderOptions struct {
	renderer highlight.Renderer
}

// RenderOption changes how Render writes a diff. Construct one with
// Highlight.
type RenderOption func(*renderOptions)

// Highlight colours the rendered diff through renderer, the same
// highlight.Renderer the encoder's Highlight option takes. Markers and paths
// are labelled highlight.ScopeInserted, highlight.ScopeDeleted or
// highlight.ScopeChanged, by the kind of change; a replaced resource uses the
// changed scope. Comment lines are labelled as comments, and block headers
// and values are labelled as configuration text, exactly as highlight.Text
// labels them. Placeholders, the = and -> signs and the summary line reach
// the renderer with the empty scope, so a renderer sees every byte.
//
// With highlight.NewANSIRenderer, a theme's markup.inserted, markup.deleted
// and markup.changed colours apply, the colours VS Code themes give diffs; a
// theme without them leaves markers and paths in the default colour. The text
// is unchanged apart from what the renderer adds, so removing its codes gives
// back the plain rendering. Sensitive values are hidden exactly as in plain
// output. xcl never checks whether output is a terminal, so whether to ask
// for colour is the caller's decision. A nil renderer means no colour.
func Highlight(renderer highlight.Renderer) RenderOption {
	return func(o *renderOptions) {
		o.renderer = renderer
	}
}

// Render writes d as text in the style of a git diff, ready to print or log.
//
// Each resource in d.Resources is written in the result's order as a comment
// line saying what an apply would do with it, a block header marked with the
// action, one line per change with the = signs aligned within the block, and
// a closing brace:
//
//	# resource.container.api will be updated
//	~ resource "container" "api" {
//	    ~ ports[0].host = 8080 -> 9090
//	    + ports[2]      = { host = 443, local = 8443 }
//	  }
//
// The header is marked + for a resource that would be created, - for one
// that would be deleted, ~ for one that would be updated and -/+ for one that
// would be replaced. A change line is marked + for an added value, - for a
// removed one and ~ for a changed one, written as before -> after. A value
// only known once an apply has run is written as (known after apply). A
// resource with no changes is written as a header with an empty body, and an
// update with no changes says the resource changed outside xcl.
//
// A changed sensitive value is written as (sensitive value). Its values are
// written only when the result carries them, which it does only when the
// diff ran with RevealSensitive, so Render never shows a value the result
// does not hold.
//
// The text ends with one summary line giving the number of resources an
// apply would create, update, replace and delete and the number it would
// leave unchanged; when nothing would change it is the only line. A nil d
// renders as a diff with nothing to change. Without Highlight the text is
// plain, with no escape codes.
func Render(d *Diff, options ...RenderOption) []byte {
	resolved := renderOptions{}
	for _, option := range options {
		if option != nil {
			option(&resolved)
		}
	}

	if d == nil {
		d = &Diff{}
	}

	r := &renderer{style: newStyler(resolved.renderer)}

	for _, resource := range d.Resources {
		r.resource(resource)
	}

	r.summary(d)

	return []byte(r.out.String())
}

// renderer builds the text of a diff, sending every piece through style
type renderer struct {
	out   strings.Builder
	style styler
}

// write appends pieces to the output
func (r *renderer) write(pieces ...string) {
	for _, piece := range pieces {
		r.out.WriteString(piece)
	}
}

// resource writes one resource's block followed by a blank line
func (r *renderer) resource(resource Resource) {
	r.write("  ", r.style.comment("# "+resource.Address+" "+actionPhrase(resource)), "\n")

	marker, kind := headerMarker(resource.Action)
	indent := strings.Repeat(" ", 3-len(marker))
	header := blockHeader(resource.Address)

	if len(resource.Changes) == 0 {
		r.write(indent, r.style.marker(kind, marker), r.style.plain(" "), r.style.hcl(header+" {}"), "\n\n")
		return
	}

	r.write(indent, r.style.marker(kind, marker), r.style.plain(" "), r.style.hcl(header+" {"), "\n")

	width := 0
	paths := make([]string, len(resource.Changes))
	for i, change := range resource.Changes {
		paths[i] = change.Path.String()
		width = max(width, utf8.RuneCountInString(paths[i]))
	}

	for i, change := range resource.Changes {
		r.change(resource.Action, change, paths[i], width)
	}

	r.write(closingIndent, r.style.plain("}"), "\n\n")
}

// change writes one change line, its path padded so the = sits one space
// after the longest path in the block
func (r *renderer) change(action Action, change Change, path string, width int) {
	padding := strings.Repeat(" ", width-utf8.RuneCountInString(path)+1)

	write := func(kind markKind, value ...string) {
		r.write(changeIndent, r.style.marker(kind, markerFor(kind)), r.style.plain(" "))
		r.write(r.style.path(kind, path), r.style.plain(padding+"= "))
		r.write(value...)
		r.write("\n")
	}

	switch {
	case change.Sensitive && change.Before == nil && change.After == nil:
		kind := markChanged
		if action == ActionCreate {
			kind = markInserted
		}
		write(kind, r.style.plain(sensitivePlaceholder))

	case change.Unknown && change.Before != nil:
		write(markChanged, r.style.hcl(formatValue(change.Before)), r.style.plain(" -> "+unknownPlaceholder))

	case change.Unknown:
		write(markInserted, r.style.plain(unknownPlaceholder))

	case change.Before == nil:
		write(markInserted, r.style.hcl(formatValue(change.After)))

	case change.After == nil:
		write(markDeleted, r.style.hcl(formatValue(change.Before)))

	default:
		write(markChanged, r.style.hcl(formatValue(change.Before)), r.style.plain(" -> "), r.style.hcl(formatValue(change.After)))
	}
}

// summary writes the closing summary line
func (r *renderer) summary(d *Diff) {
	unchanged := strconv.Itoa(d.Summary.Unchanged)

	if d.Changed() == 0 {
		r.write(r.style.plain("Diff: no changes, "+unchanged+" unchanged."), "\n")
		return
	}

	line := "Diff: " +
		strconv.Itoa(d.Summary.Create) + " to create, " +
		strconv.Itoa(d.Summary.Update) + " to update, " +
		strconv.Itoa(d.Summary.Replace) + " to replace, " +
		strconv.Itoa(d.Summary.Delete) + " to delete, " +
		unchanged + " unchanged."

	r.write(r.style.plain(line), "\n")
}

// actionPhrase returns what the comment line says an apply would do with
// resource
func actionPhrase(resource Resource) string {
	switch resource.Action {
	case ActionCreate:
		return "will be created"
	case ActionUpdate:
		if len(resource.Changes) == 0 {
			return "changed outside xcl and will be updated"
		}
		return "will be updated"
	case ActionReplace:
		return "will be replaced, its last apply failed"
	case ActionDelete:
		return "will be deleted"
	}

	return "will be changed"
}

// headerMarker returns the marker of a block header for action and the kind
// of change it stands for
func headerMarker(action Action) (string, markKind) {
	switch action {
	case ActionCreate:
		return "+", markInserted
	case ActionDelete:
		return "-", markDeleted
	case ActionReplace:
		return "-/+", markChanged
	}

	return "~", markChanged
}

// markerFor returns the marker of a change line of kind
func markerFor(kind markKind) string {
	switch kind {
	case markInserted:
		return "+"
	case markDeleted:
		return "-"
	}

	return "~"
}

// styler styles each piece of rendered text. Every piece of output except
// indentation and line breaks passes through it, so plain and styled output
// differ only by what the styler adds. Without a renderer it returns every
// piece unchanged.
type styler struct {
	renderer highlight.Renderer
}

// newStyler returns the styler for a render, colouring through renderer when
// it is not nil
func newStyler(renderer highlight.Renderer) styler {
	return styler{renderer: renderer}
}

// marker styles an action or change marker of kind
func (s styler) marker(kind markKind, text string) string {
	return s.render(scopeFor(kind), text)
}

// path styles the path of a change line of kind
func (s styler) path(kind markKind, text string) string {
	return s.render(scopeFor(kind), text)
}

// comment styles a comment line
func (s styler) comment(text string) string {
	return s.render(highlight.ScopeHashComment, text)
}

// hcl styles configuration text: a block header or a value
func (s styler) hcl(text string) string {
	if s.renderer == nil {
		return text
	}

	return string(highlight.Text([]byte(text), s.renderer))
}

// plain styles text that has no label of its own
func (s styler) plain(text string) string {
	return s.render("", text)
}

// render passes text to the renderer with scope
func (s styler) render(scope, text string) string {
	if s.renderer == nil {
		return text
	}

	return s.renderer.Render(scope, text)
}

// scopeFor returns the highlight scope of a change of kind
func scopeFor(kind markKind) string {
	switch kind {
	case markInserted:
		return highlight.ScopeInserted
	case markDeleted:
		return highlight.ScopeDeleted
	}

	return highlight.ScopeChanged
}
