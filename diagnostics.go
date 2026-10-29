package xcl

import (
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/report"
)

// ErrorTokens returns the report of every configuration problem in err as
// labelled tokens, for a formatter of your own. PrintError prints the same
// report in one call; use ErrorTokens when formatting it yourself.
//
// Every diagnostic inside err is found, however the application wrapped it,
// through Unwrap() error and Unwrap() []error. Each is written as its
// severity and summary, where it is — the file, the line, and the block and
// field it is in — the numbered lines of source around it, and its detail:
//
//	Error: Unsupported argument
//
//	  on main.xcl line 2, in agent.claude:
//	     1 | agent "claude" {
//	     2 |   modls = ["opus"]
//	     3 | }
//
//	An argument named "modls" is not expected here.
//
// The severity is labelled highlight.Error or highlight.Warning, and so is
// the source at fault, overlaid on the rest of the source, which is labelled
// as configuration, as highlight.Tokenize labels it. The summary, location,
// gutter and detail are labelled highlight.Summary, highlight.Location,
// highlight.Gutter and highlight.Detail. The detail is never wrapped, so the
// tokens are the report as it was laid out; PrintError wraps it when it
// prints. The source is the lines the diagnostic took when it was made, so
// laying a report out never reads a file.
//
// An error beside the diagnostics that is not one is written as its message
// and a line break, as Text. An error holding no diagnostic at all is a single
// Text token of its message and a line break, and nil gives no tokens.
func ErrorTokens(err error) []highlight.Token {
	if err == nil {
		return nil
	}

	if !report.Contains(err, isDiagnostic) {
		return []highlight.Token{{Kind: highlight.Text, Text: err.Error() + "\n"}}
	}

	tokens := &tokenList{}

	report.Walk(err, isDiagnostic, func(leaf error, problem bool) {
		if !problem {
			tokens.add(highlight.Text, leaf.Error()+"\n")
			return
		}

		for _, segment := range report.Layout(problemOf(leaf.(*xclerrors.Diagnostic))) {
			tokens.segment(segment)
		}
	})

	return tokens.tokens
}

// isDiagnostic reports whether err is a diagnostic, which a report is laid
// out for
func isDiagnostic(err error) bool {
	_, ok := err.(*xclerrors.Diagnostic)
	return ok
}

// problemOf takes the fields of a diagnostic that a report is laid out from.
// The errors package has its own copy for an error's %+v formatting, as it
// cannot import this package; a test holds the two to the same output.
func problemOf(d *xclerrors.Diagnostic) report.Problem {
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

// tokenList builds tokens in order
type tokenList struct {
	tokens []highlight.Token
}

// add appends text of kind, joining it to the previous token when both are of
// the same kind
func (l *tokenList) add(kind highlight.Kind, text string) {
	if text == "" {
		return
	}

	if n := len(l.tokens); n > 0 && l.tokens[n-1].Kind == kind {
		l.tokens[n-1].Text += text
		return
	}

	l.tokens = append(l.tokens, highlight.Token{Kind: kind, Text: text})
}

// segment labels one segment of a laid out report: the severity, and the
// fault within the source, as an error or a warning, the rest of the source
// as configuration, the summary, location, gutter and detail with their
// report kinds, and everything else as Text
func (l *tokenList) segment(segment report.Segment) {
	problem := highlight.Error
	if segment.Warning {
		problem = highlight.Warning
	}

	switch segment.Part {
	case report.Severity:
		l.add(problem, segment.Text)
	case report.Summary:
		l.add(highlight.Summary, segment.Text)
	case report.Location:
		l.add(highlight.Location, segment.Text)
	case report.Gutter:
		l.add(highlight.Gutter, segment.Text)
	case report.Source:
		lexed := highlight.Tokenize([]byte(segment.Text))
		if segment.HasFault {
			lexed = overlay(lexed, segment.Fault[0], segment.Fault[1], problem)
		}

		for _, token := range lexed {
			l.add(token.Kind, token.Text)
		}
	case report.Detail:
		l.add(highlight.Detail, segment.Text)
	default:
		l.add(highlight.Text, segment.Text)
	}
}

// overlay labels the bytes from start to end of the text tokens hold with
// kind, splitting tokens where the range starts and ends
func overlay(tokens []highlight.Token, start, end int, kind highlight.Kind) []highlight.Token {
	result := make([]highlight.Token, 0, len(tokens)+2)
	position := 0

	for _, token := range tokens {
		tokenStart, tokenEnd := position, position+len(token.Text)
		position = tokenEnd

		cutStart := max(start, tokenStart)
		cutEnd := min(end, tokenEnd)
		if cutStart >= cutEnd {
			result = append(result, token)
			continue
		}

		if before := token.Text[:cutStart-tokenStart]; before != "" {
			result = append(result, highlight.Token{Kind: token.Kind, Text: before})
		}

		result = append(result, highlight.Token{Kind: kind, Text: token.Text[cutStart-tokenStart : cutEnd-tokenStart]})

		if after := token.Text[cutEnd-tokenStart:]; after != "" {
			result = append(result, highlight.Token{Kind: token.Kind, Text: after})
		}
	}

	return result
}
