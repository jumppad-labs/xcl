package xcl

import (
	"io"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/report"
)

// defaultPrintWidth is the column a report's detail is wrapped at unless
// WithWidth says otherwise
const defaultPrintWidth = 80

// printOptions holds what the PrintOption values configure
type printOptions struct {
	formatter highlight.Formatter
	theme     highlight.Theme
	width     int
	encode    []EncodeOption
}

// newPrintOptions returns the defaults, plain text with the default theme and
// detail wrapped at 80 columns, with options applied
func newPrintOptions(options []PrintOption) *printOptions {
	p := &printOptions{
		formatter: highlight.Plain,
		theme:     highlight.DefaultTheme,
		width:     defaultPrintWidth,
	}

	for _, option := range options {
		if option != nil {
			option.applyPrint(p)
		}
	}

	return p
}

// PrintOption configures PrintError, PrintEntity and PrintDiff. Construct one
// with WithFormatter, WithTheme or WithWidth. Every EncodeOption is a
// PrintOption too, so PrintEntity takes the choices about which values to
// write in the same list:
//
//	xcl.PrintEntity(os.Stdout, db, xcl.IncludeComputed(), xcl.WithFormatter(highlight.For(os.Stdout)))
type PrintOption interface {
	applyPrint(*printOptions)
}

// printOptionFunc lets a function act as a PrintOption
type printOptionFunc func(*printOptions)

func (f printOptionFunc) applyPrint(p *printOptions) {
	f(p)
}

// applyPrint makes an EncodeOption a PrintOption, used by PrintEntity to
// encode the entity and ignored by PrintError and PrintDiff
func (o EncodeOption) applyPrint(p *printOptions) {
	p.encode = append(p.encode, o)
}

// WithFormatter prints in the format f writes, such as highlight.Terminal for
// a terminal, highlight.HTML for a web page, a format chosen from the
// destination with highlight.For, or one of your own. The default is
// highlight.Plain: xcl never decides on its own to colour output.
func WithFormatter(f highlight.Formatter) PrintOption {
	return printOptionFunc(func(p *printOptions) {
		if f != nil {
			p.formatter = f
		}
	})
}

// WithTheme styles what is printed with t, such as
// highlight.DefaultTheme.With(...) or a theme loaded with
// highlight.LoadVSCodeTheme. The default is highlight.DefaultTheme. Formats
// that do not style text, such as highlight.Plain and highlight.HTML, ignore
// it.
func WithTheme(t highlight.Theme) PrintOption {
	return printOptionFunc(func(p *printOptions) {
		p.theme = t
	})
}

// WithWidth wraps the detail of each problem PrintError prints at columns
// columns. The default is 80; 0, or a negative width, never wraps. Source
// lines are never wrapped.
func WithWidth(columns int) PrintOption {
	return printOptionFunc(func(p *printOptions) {
		p.width = max(columns, 0)
	})
}

// PrintError prints err to w for a person to read, in one call.
//
// Every configuration problem inside err is printed as a report, however the
// application wrapped it: its severity and summary, where it is — the file,
// the line, and the block and field it is in — the numbered lines of source
// around it with the fault marked, and its detail, wrapped at 80 columns.
// Any other error beside them is printed as its message on a line of its own,
// and an error holding no configuration problem is printed as its message and
// a line break, as go/scanner.PrintError prints one. A nil err prints
// nothing.
//
//	if err := cfg.Apply("./config"); err != nil {
//		xcl.PrintError(os.Stderr, err, xcl.WithFormatter(highlight.For(os.Stderr)))
//		os.Exit(1)
//	}
//
// The output is plain text unless WithFormatter chooses another format. HTML
// output is already escaped, so it goes into a page as template.HTML:
//
//	var report strings.Builder
//	xcl.PrintError(&report, err, xcl.WithFormatter(highlight.HTML))
//	page.Execute(w, map[string]any{"Report": template.HTML(report.String())})
//
// It returns the first error writing to w.
func PrintError(w io.Writer, err error, options ...PrintOption) error {
	if err == nil {
		return nil
	}

	p := newPrintOptions(options)

	tokens := ErrorTokens(err)
	for i := range tokens {
		if tokens[i].Kind == highlight.Detail {
			tokens[i].Text = report.Wrap(tokens[i].Text, p.width)
		}
	}

	return p.formatter.Format(w, p.theme, tokens)
}

// PrintEntity prints entity to w as configuration text, in one call: the text
// EncodeEntity returns, in the format and theme the options choose. The
// options take EncodeOptions too, such as IncludeComputed, to choose which
// values are written:
//
//	xcl.PrintEntity(os.Stdout, db, xcl.IncludeComputed(), xcl.WithFormatter(highlight.Terminal))
//
// It fails with ErrNotEncodable, writing nothing, when EncodeEntity would,
// and otherwise returns the first error writing to w.
func PrintEntity(w io.Writer, entity any, options ...PrintOption) error {
	p := newPrintOptions(options)

	tokens, err := EntityTokens(entity, p.encode...)
	if err != nil {
		return err
	}

	return p.formatter.Format(w, p.theme, tokens)
}

// PrintDiff prints the change summary d to w, in one call: the git-style
// summary diff.(*Diff).Tokens lays out, in the format and theme the options
// choose. A nil d prints as a summary with nothing to change. It returns the
// first error writing to w.
func PrintDiff(w io.Writer, d *diff.Diff, options ...PrintOption) error {
	p := newPrintOptions(options)

	return p.formatter.Format(w, p.theme, d.Tokens())
}
