package xcl

import (
	"fmt"

	"github.com/apparentlymart/go-textseg/v15/textseg"
	"github.com/mitchellh/go-wordwrap"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// DiagnosticPieces lays diagnostics out as a report for a person to read, as
// labelled pieces for a renderer: highlight.Text for plain text,
// highlight.ANSI for a terminal, or any other highlight.Renderer.
//
// Each diagnostic is written as its severity and summary, where it is — the
// file, the line, and the block and field it is in — the lines of source
// around it, and its detail:
//
//	Error: Unsupported argument
//
//	  on main.xcl line 2, in agent.claude:
//	     2 |   modls = ["opus"]
//
//	An argument named "modls" is not expected here.
//
// The severity is labelled highlight.ScopeError or highlight.ScopeWarning,
// and so is the source at fault; the rest of the source is labelled as
// configuration, as highlight.Pieces labels it. The source is the lines the
// diagnostic took when it was made, so laying it out never reads a file.
//
// The detail is wrapped at width columns; a width of 0 leaves it unwrapped.
// Source lines are never wrapped.
func DiagnosticPieces(diagnostics xclerrors.Diagnostics, width int) []highlight.Piece {
	r := &report{width: width}

	for _, diagnostic := range diagnostics {
		r.diagnostic(diagnostic)
	}

	return r.pieces
}

// report builds the pieces of a diagnostics report in order
type report struct {
	width  int
	pieces []highlight.Piece
}

// add appends text with scope, joining it to the previous piece when both
// have the same scope
func (r *report) add(scope, text string) {
	if text == "" {
		return
	}

	if n := len(r.pieces); n > 0 && r.pieces[n-1].Scope == scope {
		r.pieces[n-1].Text += text
		return
	}

	r.pieces = append(r.pieces, highlight.Piece{Scope: scope, Text: text})
}

// diagnostic lays out one diagnostic, followed by a blank line
func (r *report) diagnostic(d *xclerrors.Diagnostic) {
	scope, label := highlight.ScopeError, "Error"
	if d.IsWarning() {
		scope, label = highlight.ScopeWarning, "Warning"
	}

	r.add(scope, label)
	r.add("", ": "+d.Summary+"\n")

	if d.Subject != nil {
		r.add("", "\n"+location(d)+":\n")

		if d.Source == nil {
			r.add("", "  (source code not available)\n")
		} else {
			r.source(d.Source, *d.Subject, scope)
		}
	}

	if d.Detail != "" {
		detail := d.Detail
		if r.width > 0 {
			detail = wordwrap.WrapString(detail, uint(r.width))
		}

		r.add("", "\n"+detail+"\n")
	}

	r.add("", "\n")
}

// location says where a diagnostic is: the file and line, and the block and
// field it is in when it is in one
func location(d *xclerrors.Diagnostic) string {
	where := fmt.Sprintf("  on %s line %d", d.Subject.Filename, d.Subject.Start.Line)

	if d.Entity != "" {
		where += ", in " + d.Entity
	}

	if d.Field != "" {
		where += ", field " + d.Field
	}

	return where
}

// source lays out the lines of source, numbered, each labelled as
// configuration with the part subject covers labelled scope
func (r *report) source(src *xclerrors.SourceLines, subject hcl.Range, scope string) {
	for i, line := range src.Lines {
		number := src.Start + i

		r.add("", fmt.Sprintf("  %4d | ", number))

		pieces := highlight.Pieces([]byte(line))
		if start, end, ok := fault(line, number, subject); ok {
			pieces = overlay(pieces, start, end, scope)
		}

		for _, p := range pieces {
			r.add(p.Scope, p.Text)
		}

		r.add("", "\n")
	}
}

// fault returns the byte range of line, line number number, that subject
// covers, and false when it covers none of it. A subject that covers nothing,
// such as a missing value, is shown as covering the character it starts at.
func fault(line string, number int, subject hcl.Range) (int, int, bool) {
	if number < subject.Start.Line || number > subject.End.Line {
		return 0, 0, false
	}

	startColumn := 1
	if number == subject.Start.Line {
		startColumn = subject.Start.Column
	}

	endColumn := -1
	if number == subject.End.Line {
		endColumn = subject.End.Column
	}

	if subject.Start.Line == subject.End.Line && subject.Start.Column == subject.End.Column {
		endColumn = startColumn + 1
	}

	start := byteAt(line, startColumn)
	end := len(line)
	if endColumn >= 0 {
		end = byteAt(line, endColumn)
	}

	if end <= start {
		return 0, 0, false
	}

	return start, end, true
}

// byteAt returns the byte offset in line of column, counting columns from 1
// in grapheme clusters as HCL counts them, or the length of line for a column
// past its end
func byteAt(line string, column int) int {
	offset := 0
	for current := 1; current < column && offset < len(line); current++ {
		advance, _, _ := textseg.ScanGraphemeClusters([]byte(line[offset:]), true)
		if advance == 0 {
			break
		}

		offset += advance
	}

	return min(offset, len(line))
}

// overlay labels the bytes from start to end of the text pieces hold with
// scope, splitting pieces where the range starts and ends
func overlay(pieces []highlight.Piece, start, end int, scope string) []highlight.Piece {
	result := make([]highlight.Piece, 0, len(pieces)+2)
	position := 0

	for _, p := range pieces {
		pieceStart, pieceEnd := position, position+len(p.Text)
		position = pieceEnd

		cutStart := max(start, pieceStart)
		cutEnd := min(end, pieceEnd)
		if cutStart >= cutEnd {
			result = append(result, p)
			continue
		}

		if before := p.Text[:cutStart-pieceStart]; before != "" {
			result = append(result, highlight.Piece{Scope: p.Scope, Text: before})
		}

		result = append(result, highlight.Piece{Scope: scope, Text: p.Text[cutStart-pieceStart : cutEnd-pieceStart]})

		if after := p.Text[cutEnd-pieceStart:]; after != "" {
			result = append(result, highlight.Piece{Scope: p.Scope, Text: after})
		}
	}

	return result
}
