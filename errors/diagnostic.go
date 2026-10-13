package errors

import (
	"fmt"
	"strings"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// Diagnostic is one problem found in configuration: what is wrong, where, in
// which block and which of its fields, and the source around it.
//
// It carries everything the HCL diagnostic it was made from does — severity,
// summary, detail, and the exact range of the fault and of its context — and
// adds what a reader of configuration needs as well: the block it is about,
// the field, where that block starts, and the lines of source around the
// fault, taken when the diagnostic was made so that showing them never reads
// a file again.
//
// Error is one plain line. To show a diagnostic to a person, with its source,
// render it: see xcl's diagnostics rendering.
type Diagnostic struct {
	hcl.Diagnostic

	// Entity is the id of the block the problem is in, such as
	// resource.container.web, or empty when it is in no block, such as a
	// syntax error between blocks
	Entity string

	// Field is the path of the attribute or nested block at fault within the
	// entity, such as workspace[0].directory, or empty when the problem is
	// with the block as a whole
	Field string

	// EntityPos is the range of the entity's header, where the block starts.
	// Subject is the fault itself, which can be anywhere inside it.
	EntityPos *hcl.Range

	// Source is the configuration around the fault, or nil when the source
	// was not to hand
	Source *SourceLines
}

// SourceLines are whole lines of configuration, as they were read.
type SourceLines struct {
	// File is the file they were read from
	File string

	// Start is the line number of the first line, counting from 1
	Start int

	// Lines are the lines, without their line endings. A range is placed in
	// them by its lines and columns.
	Lines []string
}

// NewDiagnostic makes a Diagnostic from an HCL diagnostic, taking the lines of
// source its subject and context cover from sources, by file name. Source is
// left nil when the diagnostic has no subject or its file is not in sources.
//
// The expression and evaluation context are left out: they hold the values
// an expression was evaluated with, which may be sensitive, and a diagnostic
// is printed, logged and sent over the wire.
func NewDiagnostic(d *hcl.Diagnostic, sources map[string][]byte) *Diagnostic {
	diagnostic := &Diagnostic{Diagnostic: *d}
	diagnostic.Expression = nil
	diagnostic.EvalContext = nil

	if d.Subject == nil {
		return diagnostic
	}

	covered := *d.Subject
	if d.Context != nil {
		covered = hcl.RangeOver(covered, *d.Context)
	}

	if src, ok := sources[covered.Filename]; ok {
		diagnostic.Source = linesOf(src, covered)
	}

	return diagnostic
}

// linesOf returns the whole lines of src that r covers, or nil when r is not
// within src
func linesOf(src []byte, r hcl.Range) *SourceLines {
	first, last := r.Start.Line, max(r.End.Line, r.Start.Line)
	if first < 1 {
		return nil
	}

	// a range ending at the start of a later line ends with the line break
	// before it, and covers nothing of that line
	if r.End.Line > r.Start.Line && r.End.Column <= 1 {
		last = r.End.Line - 1
	}

	lines := strings.Split(string(src), "\n")
	if first > len(lines) {
		return nil
	}
	last = min(last, len(lines))

	covered := make([]string, 0, last-first+1)
	for _, line := range lines[first-1 : last] {
		covered = append(covered, strings.TrimSuffix(line, "\r"))
	}

	return &SourceLines{File: r.Filename, Start: first, Lines: covered}
}

// IsError reports whether the problem stops the configuration being used
func (d *Diagnostic) IsError() bool {
	return d.Severity == hcl.DiagError
}

// IsWarning reports whether the problem is worth knowing about but stops
// nothing
func (d *Diagnostic) IsWarning() bool {
	return d.Severity == hcl.DiagWarning
}

// Error returns the diagnostic as one plain line: where, what, and in which
// block and field, such as
//
//	main.xcl:2,3-8: Unsupported argument; An argument named "modls" is not expected here. (agent.claude)
//
// It never holds source, colour or line breaks.
func (d *Diagnostic) Error() string {
	var line strings.Builder

	if d.Subject != nil {
		line.WriteString(d.Subject.String())
		line.WriteString(": ")
	}

	line.WriteString(oneLine(d.Summary))

	if d.Detail != "" {
		line.WriteString("; ")
		line.WriteString(oneLine(d.Detail))
	}

	switch {
	case d.Entity != "" && d.Field != "":
		fmt.Fprintf(&line, " (%s, %s)", d.Entity, d.Field)
	case d.Entity != "":
		fmt.Fprintf(&line, " (%s)", d.Entity)
	}

	return line.String()
}

// oneLine joins the lines of text with spaces
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// Diagnostics are the problems found in configuration, in the order they were
// found.
type Diagnostics []*Diagnostic

// HasErrors reports whether any of the diagnostics is an error
func (d Diagnostics) HasErrors() bool {
	for _, diagnostic := range d {
		if diagnostic.IsError() {
			return true
		}
	}

	return false
}

// Errs returns the diagnostics that are errors
func (d Diagnostics) Errs() Diagnostics {
	return d.filter((*Diagnostic).IsError)
}

// Warnings returns the diagnostics that are warnings
func (d Diagnostics) Warnings() Diagnostics {
	return d.filter((*Diagnostic).IsWarning)
}

func (d Diagnostics) filter(keep func(*Diagnostic) bool) Diagnostics {
	kept := Diagnostics{}
	for _, diagnostic := range d {
		if keep(diagnostic) {
			kept = append(kept, diagnostic)
		}
	}

	return kept
}

// Error returns each diagnostic's line, one per line
func (d Diagnostics) Error() string {
	lines := make([]string, len(d))
	for i, diagnostic := range d {
		lines[i] = diagnostic.Error()
	}

	return strings.Join(lines, "\n")
}

// Unwrap returns the diagnostics as errors, so errors.As finds each one
func (d Diagnostics) Unwrap() []error {
	errs := make([]error, len(d))
	for i, diagnostic := range d {
		errs[i] = diagnostic
	}

	return errs
}

// Diagnostics returns the diagnostics among the errors collected, in order.
// Errors that are not diagnostics are left out.
func (p *ConfigError) Diagnostics() Diagnostics {
	found := Diagnostics{}
	for _, err := range p.Errors {
		if diagnostic, ok := err.(*Diagnostic); ok {
			found = append(found, diagnostic)
		}
	}

	return found
}
