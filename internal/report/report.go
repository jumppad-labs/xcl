// Package report lays a configuration problem out as a report for a person to
// read, and finds the problems inside an error.
//
// It is the one place that decides what a report says: its severity and
// summary, where the problem is, the numbered lines of source around it with
// the fault marked, and its detail. It knows nothing of colour or of labelled
// text, so both the errors package, which writes the plain report for an
// error's detailed formatting, and the root xcl package, which labels the
// report for highlighting, can use it without importing each other.
package report

import (
	"fmt"

	"github.com/apparentlymart/go-textseg/v15/textseg"

	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// Part is what a segment of a report is
type Part int

const (
	// Severity is "Error" or "Warning"
	Severity Part = iota

	// Summary is the problem's summary, after its severity
	Summary

	// Location says where the problem is: the file, the line, and the block
	// and field it is in
	Location

	// Gutter is the line number written before a line of source
	Gutter

	// Source is one whole line of configuration, without its line ending
	Source

	// Detail is the problem's detail, unwrapped
	Detail

	// Plain is the text between the other parts: line breaks, blank lines
	// and fixed wording
	Plain
)

// Segment is one piece of a report, in order. Joining every segment's Text
// gives the plain report.
type Segment struct {
	// Part is what the segment is
	Part Part

	// Text is the segment's text
	Text string

	// Warning is true when the problem is a warning rather than an error. It
	// is set on the Severity segment and on every Source segment.
	Warning bool

	// Fault is the byte range within Text that the problem covers, for a
	// Source segment whose HasFault is true
	Fault [2]int

	// HasFault reports whether Fault is set
	HasFault bool
}

// Problem is what a report is laid out from: the fields of a diagnostic,
// without the diagnostic itself, so this package never imports the errors
// package.
type Problem struct {
	// Warning is true for a warning, false for an error
	Warning bool

	// Summary and Detail say what is wrong
	Summary string
	Detail  string

	// Entity and Field name the block and the field the problem is in, and
	// are empty when it is in none
	Entity string
	Field  string

	// Subject is the fault's range, or nil when the problem is in no place
	Subject *hcl.Range

	// File, Start and Lines are the lines of source around the fault, with
	// Start the number of the first, when HasSource is true
	File      string
	Start     int
	Lines     []string
	HasSource bool
}

// Layout lays one problem out as a report, its detail unwrapped, ending with
// the blank line that separates it from the next:
//
//	Error: Unsupported argument
//
//	  on main.xcl line 2, in agent.claude:
//	     2 |   modls = ["opus"]
//
//	An argument named "modls" is not expected here.
func Layout(p Problem) []Segment {
	var segments []Segment
	add := func(segment Segment) {
		if segment.Text != "" {
			segments = append(segments, segment)
		}
	}

	label := "Error"
	if p.Warning {
		label = "Warning"
	}

	add(Segment{Part: Severity, Text: label, Warning: p.Warning})
	add(Segment{Part: Summary, Text: ": " + p.Summary})
	add(Segment{Part: Plain, Text: "\n"})

	if p.Subject != nil {
		add(Segment{Part: Plain, Text: "\n"})
		add(Segment{Part: Location, Text: location(p) + ":"})
		add(Segment{Part: Plain, Text: "\n"})

		if !p.HasSource {
			add(Segment{Part: Plain, Text: "  (source code not available)\n"})
		} else {
			for i, line := range p.Lines {
				number := p.Start + i

				add(Segment{Part: Gutter, Text: fmt.Sprintf("  %4d | ", number)})

				source := Segment{Part: Source, Text: line, Warning: p.Warning}
				if start, end, ok := fault(line, number, *p.Subject); ok {
					source.Fault = [2]int{start, end}
					source.HasFault = true
				}
				add(source)

				add(Segment{Part: Plain, Text: "\n"})
			}
		}
	}

	if p.Detail != "" {
		add(Segment{Part: Plain, Text: "\n"})
		add(Segment{Part: Detail, Text: p.Detail})
		add(Segment{Part: Plain, Text: "\n"})
	}

	add(Segment{Part: Plain, Text: "\n"})

	return segments
}

// location says where a problem is: the file and line, and the block and
// field it is in when it is in one
func location(p Problem) string {
	where := fmt.Sprintf("  on %s line %d", p.Subject.Filename, p.Subject.Start.Line)

	if p.Entity != "" {
		where += ", in " + p.Entity
	}

	if p.Field != "" {
		where += ", field " + p.Field
	}

	return where
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
