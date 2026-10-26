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
// kind it is labelled with
type markKind int

const (
	markInserted markKind = iota
	markDeleted
	markChanged
)

// Tokens lays d out as text in the style of a git diff, as labelled tokens
// for a highlight.Formatter. To print a change summary, as plain text ready
// to print or log, in terminal colour, as HTML or in any other format, use
// xcl.PrintDiff; Tokens is for extenders formatting it themselves.
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
// would be replaced. The comment line above a replacement says why:
//
//	# resource.container.web will be replaced because resource.network.app is replaced
//	-/+ resource "container" "web" {
//
// or "its last apply failed", or "it cannot be updated in place" when its
// provider decided it alone. A change line is marked + for an added value, - for a
// removed one and ~ for a changed one, written as before -> after. A value
// only known once an apply has run is written as (known after apply). A
// resource with no changes is written as a header with an empty body. An
// update with no changes names the updated or replaced dependencies behind
// it, such as
//
//	# resource.container.web will be updated because resource.network.app is replaced
//
// and, when there are none, says the resource changed outside xcl.
//
// A changed sensitive value is written as (sensitive value). Its values are
// written only when the result carries them, which it does only when the
// diff ran with RevealSensitive, so the tokens never hold a value the result
// does not.
//
// The text ends with one summary line giving the number of resources an
// apply would create, update, replace and delete and the number it would
// leave unchanged; when nothing would change it is the only line. A nil d is
// laid out as a diff with nothing to change.
//
// Markers and paths are labelled highlight.Inserted, highlight.Deleted or
// highlight.Changed, by the kind of change; a replaced resource is labelled
// changed. Comment lines are labelled highlight.HashComment, and block
// headers and values as configuration, exactly as highlight.Tokenize labels
// them. Indentation, line breaks, placeholders, the = and -> signs and the
// summary line are highlight.Text, so the tokens hold every byte of the text.
func (d *Diff) Tokens() []highlight.Token {
	if d == nil {
		d = &Diff{}
	}

	l := &layout{}

	for _, resource := range d.Resources {
		l.resource(resource)
	}

	l.summary(d)

	return l.tokens
}

// layout builds the tokens of a diff in order
type layout struct {
	tokens []highlight.Token
}

// add appends text of kind, joining it to the previous token when both are of
// the same kind, as highlight.Tokenize does
func (l *layout) add(kind highlight.Kind, text string) {
	if text == "" {
		return
	}

	if n := len(l.tokens); n > 0 && l.tokens[n-1].Kind == kind {
		l.tokens[n-1].Text += text
		return
	}

	l.tokens = append(l.tokens, highlight.Token{Kind: kind, Text: text})
}

// plain appends text that has no label of its own
func (l *layout) plain(text string) {
	l.add(highlight.Text, text)
}

// marker appends an action or change marker, or a path, of kind
func (l *layout) marker(kind markKind, text string) {
	l.add(kindFor(kind), text)
}

// hcl appends configuration text, a block header or a value, labelled as
// configuration
func (l *layout) hcl(text string) {
	for _, token := range highlight.Tokenize([]byte(text)) {
		l.add(token.Kind, token.Text)
	}
}

// resource lays out one resource's block followed by a blank line
func (l *layout) resource(resource Resource) {
	l.plain("  ")
	l.add(highlight.HashComment, "# "+resource.Address+" "+actionPhrase(resource))
	l.plain("\n")

	marker, kind := headerMarker(resource.Action)
	header := blockHeader(resource.Address)

	l.plain(strings.Repeat(" ", 3-len(marker)))
	l.marker(kind, marker)
	l.plain(" ")

	if len(resource.Changes) == 0 {
		l.hcl(header + " {}")
		l.plain("\n\n")
		return
	}

	l.hcl(header + " {")
	l.plain("\n")

	width := 0
	paths := make([]string, len(resource.Changes))
	for i, change := range resource.Changes {
		paths[i] = change.Path.String()
		width = max(width, utf8.RuneCountInString(paths[i]))
	}

	for i, change := range resource.Changes {
		l.change(resource.Action, change, paths[i], width)
	}

	l.plain(closingIndent + "}\n\n")
}

// change lays out one change line, its path padded so the = sits one space
// after the longest path in the block
func (l *layout) change(action Action, change Change, path string, width int) {
	line := func(kind markKind, value func()) {
		l.plain(changeIndent)
		l.marker(kind, markerFor(kind))
		l.plain(" ")
		l.marker(kind, path)
		l.plain(strings.Repeat(" ", width-utf8.RuneCountInString(path)+1) + "= ")
		value()
		l.plain("\n")
	}

	switch {
	case change.Sensitive && change.Before == nil && change.After == nil:
		kind := markChanged
		if action == ActionCreate {
			kind = markInserted
		}
		line(kind, func() { l.plain(sensitivePlaceholder) })

	case change.Unknown && change.Before != nil:
		line(markChanged, func() {
			l.hcl(formatValue(change.Before))
			l.plain(" -> " + unknownPlaceholder)
		})

	case change.Unknown:
		line(markInserted, func() { l.plain(unknownPlaceholder) })

	case change.Before == nil:
		line(markInserted, func() { l.hcl(formatValue(change.After)) })

	case change.After == nil:
		line(markDeleted, func() { l.hcl(formatValue(change.Before)) })

	default:
		line(markChanged, func() {
			l.hcl(formatValue(change.Before))
			l.plain(" -> ")
			l.hcl(formatValue(change.After))
		})
	}
}

// summary lays out the closing summary line
func (l *layout) summary(d *Diff) {
	unchanged := strconv.Itoa(d.Summary.Unchanged)

	if d.Changed() == 0 {
		l.plain("Diff: no changes, " + unchanged + " unchanged.\n")
		return
	}

	l.plain("Diff: " +
		strconv.Itoa(d.Summary.Create) + " to create, " +
		strconv.Itoa(d.Summary.Update) + " to update, " +
		strconv.Itoa(d.Summary.Replace) + " to replace, " +
		strconv.Itoa(d.Summary.Delete) + " to delete, " +
		unchanged + " unchanged.\n")
}

// actionPhrase returns what the comment line says an apply would do with
// resource
func actionPhrase(resource Resource) string {
	switch resource.Action {
	case ActionCreate:
		return "will be created"
	case ActionUpdate:
		if len(resource.Changes) == 0 && len(resource.Dependencies) > 0 {
			return "will be updated because " + dependencyReason(resource.Dependencies)
		}
		if len(resource.Changes) == 0 {
			return "changed outside xcl and will be updated"
		}
		return "will be updated"
	case ActionReplace:
		return replacePhrase(resource)
	case ActionDelete:
		return "will be deleted"
	}

	return "will be changed"
}

// dependencyReason returns what happens to the dependencies behind an update,
// such as "resource.network.app is replaced" or
// "resource.network.a, resource.network.b are replaced and resource.volume.c is updated"
func dependencyReason(dependencies []Dependency) string {
	replaced := []string{}
	updated := []string{}
	for _, dependency := range dependencies {
		if dependency.Action == ActionReplace {
			replaced = append(replaced, dependency.Address)
		} else {
			updated = append(updated, dependency.Address)
		}
	}

	parts := []string{}
	if len(replaced) > 0 {
		parts = append(parts, addressesAre(replaced)+" replaced")
	}
	if len(updated) > 0 {
		parts = append(parts, addressesAre(updated)+" updated")
	}

	return strings.Join(parts, " and ")
}

// addressesAre returns the addresses joined by commas followed by "is" or
// "are"
func addressesAre(addresses []string) string {
	verb := "is"
	if len(addresses) > 1 {
		verb = "are"
	}

	return strings.Join(addresses, ", ") + " " + verb
}

// replacePhrase returns what the comment line says about a replacement, by
// its reason
func replacePhrase(resource Resource) string {
	switch resource.Reason {
	case ReplaceFailed:
		return "will be replaced, its last apply failed"
	case ReplaceProvider:
		return "will be replaced, it cannot be updated in place"
	case ReplaceDependency:
		if len(resource.ReplacedDeps) == 0 {
			return "will be replaced because a dependency is replaced"
		}

		verb := "is"
		if len(resource.ReplacedDeps) > 1 {
			verb = "are"
		}

		return "will be replaced because " + strings.Join(resource.ReplacedDeps, ", ") + " " + verb + " replaced"
	}

	return "will be replaced"
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

// kindFor returns the highlight kind of a change of kind
func kindFor(kind markKind) highlight.Kind {
	switch kind {
	case markInserted:
		return highlight.Inserted
	case markDeleted:
		return highlight.Deleted
	}

	return highlight.Changed
}
