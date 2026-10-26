package errors

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jumppad-labs/xcl/internal/report"
)

// reportWidth is the column the detail of a report is wrapped at when an
// error is formatted with %+v
const reportWidth = 80

// Format formats the diagnostic for the fmt package. %v and %s give the one
// line Error returns, %q that line quoted, and %+v the plain report a person
// reads: the severity and summary, where the problem is, the numbered lines
// of source around it and its detail, wrapped at 80 columns.
//
// A structured logger that records an error's detailed formatting, such as
// zap's errorVerbose field, carries the report this way. %+v works only on
// the diagnostic itself, not on an error wrapping it: to print the report of
// any error xcl returns, however it was wrapped, use xcl.PrintError.
func (d *Diagnostic) Format(s fmt.State, verb rune) {
	formatError(s, verb, d)
}

// Format formats the configuration error for the fmt package. %v and %s give
// the lines Error returns, one per problem, %q those lines quoted, and %+v the
// plain report of every problem it holds, in order, with each error that is
// not a configuration problem written as its message on a line of its own.
//
// A structured logger that records an error's detailed formatting, such as
// zap's errorVerbose field, carries the report this way. %+v works only on
// the configuration error itself, not on an error wrapping it: to print the
// report of any error xcl returns, however it was wrapped, use
// xcl.PrintError.
func (p *ConfigError) Format(s fmt.State, verb rune) {
	formatError(s, verb, p)
}

// formatError writes err for verb: the plain report for %+v, its message for
// %v and %s, the message quoted for %q, and a bad verb as fmt writes one
// otherwise
func formatError(s fmt.State, verb rune, err error) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, plainReport(err))
			return
		}

		_, _ = io.WriteString(s, err.Error())
	case 's':
		_, _ = io.WriteString(s, err.Error())
	case 'q':
		_, _ = io.WriteString(s, strconv.Quote(err.Error()))
	default:
		_, _ = fmt.Fprintf(s, "%%!%c(%T=%s)", verb, err, err.Error())
	}
}

// plainReport lays out every diagnostic in err as a report, its detail
// wrapped at reportWidth, and writes every other error beneath it as its
// message and a line break. An error holding no diagnostic is its message and
// a line break.
func plainReport(err error) string {
	var text strings.Builder

	report.Walk(err, isDiagnostic, func(leaf error, problem bool) {
		if !problem {
			text.WriteString(leaf.Error())
			text.WriteString("\n")
			return
		}

		for _, segment := range report.Layout(problemOf(leaf.(*Diagnostic))) {
			if segment.Part == report.Detail {
				text.WriteString(report.Wrap(segment.Text, reportWidth))
				continue
			}

			text.WriteString(segment.Text)
		}
	})

	return text.String()
}

// isDiagnostic reports whether err is a diagnostic, which a report is laid
// out for
func isDiagnostic(err error) bool {
	_, ok := err.(*Diagnostic)
	return ok
}

// problemOf takes the fields of a diagnostic that a report is laid out from.
// The root xcl package has its own copy, as it labels the same layout for
// highlighting; a test there holds the two to the same output.
func problemOf(d *Diagnostic) report.Problem {
	problem := report.Problem{
		Warning: d.IsWarning(),
		Summary: d.Summary,
		Detail:  d.Detail,
		Entity:  d.Entity,
		Field:   d.Field,
		Subject: d.Subject,
	}

	if d.Source != nil {
		problem.File = d.Source.File
		problem.Start = d.Source.Start
		problem.Lines = d.Source.Lines
		problem.HasSource = true
	}

	return problem
}
