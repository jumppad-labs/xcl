package xcl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// printBadFixture is a configuration with one problem: a block of a type a
// variable does not take, opened on line 7, with characters HTML escapes on
// line 6
const printBadFixture = "testdata/print/bad/main.xcl"

// printTwoProblemsFixture is a configuration with two problems, each a block
// of a type a variable does not take, in two variables: the first on line 3,
// the second on line 10
const printTwoProblemsFixture = "testdata/print/two/main.xcl"

// printLongDetail is a detail longer than 80 columns, made of words of
// ordinary length so it can be wrapped at 40 columns
const printLongDetail = "The value given for the port is not a number, so the container cannot be started until the port is written as a whole number between 1 and 65535."

// applyPrintFixture applies the configuration in file with a fresh Config
// and returns the error it fails with
func applyPrintFixture(t *testing.T, file string) error {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	path, err := filepath.Abs(file)
	require.NoError(t, err)

	c, err := NewConfig(WithStatePath(t.TempDir()))
	require.NoError(t, err)

	err = c.Apply(path)
	require.Error(t, err)

	return err
}

// applyPrintFixtureConfigError applies the configuration in file and returns
// the configuration error it fails with
func applyPrintFixtureConfigError(t *testing.T, file string) *xclerrors.ConfigError {
	t.Helper()

	err := applyPrintFixture(t, file)

	var configError *xclerrors.ConfigError
	require.True(t, errors.As(err, &configError), "applying %s must fail with a *ConfigError, got %T", file, err)

	return configError
}

// printedError returns err printed with options
func printedError(t *testing.T, err error, options ...PrintOption) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, PrintError(&out, err, options...))

	return out.String()
}

// longDetailDiagnostic is a problem with no location whose detail is longer
// than 80 columns
func longDetailDiagnostic() *xclerrors.Diagnostic {
	return xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid port",
		Detail:   printLongDetail,
	}, nil)
}

// requireNoLineLongerThan requires every line of text be at most columns long
func requireNoLineLongerThan(t *testing.T, text string, columns int) {
	t.Helper()

	for _, line := range strings.Split(text, "\n") {
		require.LessOrEqual(t, len(line), columns, "line %q is longer than %d columns", line, columns)
	}
}

// joinedTokenText returns the text of tokens, in order
func joinedTokenText(tokens []highlight.Token) string {
	var text strings.Builder
	for _, token := range tokens {
		text.WriteString(token.Text)
	}

	return text.String()
}

// sgrCode matches one SGR escape sequence
var sgrCode = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// requireNoLineBreakInsideColour requires every colour opened in text be
// reset before the next line break, so no line break is written inside an
// SGR span
func requireNoLineBreakInsideColour(t *testing.T, text string) {
	t.Helper()

	coloured := false
	position := 0
	for _, match := range sgrCode.FindAllStringIndex(text, -1) {
		require.False(t, coloured && strings.Contains(text[position:match[0]], "\n"), "a line break is written inside a colour, before byte %d", match[0])

		coloured = text[match[0]:match[1]] != "\x1b[0m"
		position = match[1]
	}

	require.False(t, coloured, "the output ends inside a colour")
}

// recordingFormatter is a format of a person's own: it wraps each token's
// text in [ and ], and records each call it is given
type recordingFormatter struct {
	calls  int
	tokens []highlight.Token
}

func (f *recordingFormatter) Format(w io.Writer, _ highlight.Theme, tokens []highlight.Token) error {
	f.calls++
	f.tokens = append(f.tokens, tokens...)

	for _, token := range tokens {
		if _, err := io.WriteString(w, "["+token.Text+"]"); err != nil {
			return err
		}
	}

	return nil
}

// logErrorWithZap logs err with zap.Error to a JSON encoder and returns the
// entry it wrote
func logErrorWithZap(t *testing.T, err error) map[string]any {
	t.Helper()

	var out bytes.Buffer
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&out),
		zap.DebugLevel,
	))

	logger.Error("applying configuration", zap.Error(err))
	require.NoError(t, logger.Sync())

	entry := map[string]any{}
	require.NoError(t, json.Unmarshal(out.Bytes(), &entry))

	return entry
}

func TestPrintErrorReportsAnUnsupportedBlockWithItsSourceInOneCall(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	var out bytes.Buffer
	require.NoError(t, PrintError(&out, err))
	report := out.String()

	lines := strings.Split(report, "\n")
	require.Equal(t, "Error: Unsupported block type", lines[0])

	location := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "  on ") {
			location = line
		}
	}
	require.True(t, strings.HasPrefix(location, "  on "), "the report has no location line")
	require.Contains(t, location, "main.xcl")
	require.Contains(t, location, " line 7, in ")
	require.Contains(t, location, ", in variable.name, field timeout[0]:")

	require.Contains(t, report, "     5 |   // the default holds characters a web page escapes\n")
	require.Contains(t, report, "     6 |   default = \"a<b & \\\"c\\\"\"\n")
	require.Contains(t, report, "     7 |   timeout \"http\" {\n")
	require.Contains(t, report, "     8 |     seconds = 30\n")
	require.Contains(t, report, "     9 |   }\n")
	require.NotContains(t, report, "     4 |")
	require.NotContains(t, report, "    10 |")

	nonEmpty := []string{}
	for _, line := range lines {
		if line != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	require.Equal(t, `Blocks of type "timeout" are not expected here.`, nonEmpty[len(nonEmpty)-1])
}

func TestPrintErrorOfAFaultOnTheFirstLineShowsTheTwoLinesAfterIt(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported argument",
		Detail:   `An argument named "a" is not expected here.`,
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
			End:      hcl.Pos{Line: 1, Column: 2, Byte: 1},
		},
	}, map[string][]byte{"main.xcl": []byte("a = 1\nb = 2\nc = 3\n")})

	expected := "Error: Unsupported argument\n" +
		"\n" +
		"  on main.xcl line 1:\n" +
		"     1 | a = 1\n" +
		"     2 | b = 2\n" +
		"     3 | c = 3\n" +
		"\n" +
		"An argument named \"a\" is not expected here.\n" +
		"\n"
	require.Equal(t, expected, printedError(t, d, WithWidth(0)))
}

func TestPrintErrorOfAFaultOnTheLastLineShowsTheTwoLinesBeforeIt(t *testing.T) {
	d := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported argument",
		Detail:   `An argument named "e" is not expected here.`,
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 5, Column: 1, Byte: 24},
			End:      hcl.Pos{Line: 5, Column: 2, Byte: 25},
		},
	}, map[string][]byte{"main.xcl": []byte("a = 1\nb = 2\nc = 3\nd = 4\ne = 5\n")})

	expected := "Error: Unsupported argument\n" +
		"\n" +
		"  on main.xcl line 5:\n" +
		"     3 | c = 3\n" +
		"     4 | d = 4\n" +
		"     5 | e = 5\n" +
		"\n" +
		"An argument named \"e\" is not expected here.\n" +
		"\n"
	require.Equal(t, expected, printedError(t, d, WithWidth(0)))
}

func TestPrintErrorOfTwoProblemsWrappedByTheApplicationPrintsBothReportsInOrder(t *testing.T) {
	err := fmt.Errorf("loading: %w", applyPrintFixture(t, printTwoProblemsFixture))

	report := printedError(t, err)

	require.Equal(t, 2, strings.Count(report, "Error: Unsupported block type\n"))

	first := strings.Index(report, ", in variable.first, field timeout[0]:\n")
	second := strings.Index(report, ", in variable.second, field retry[0]:\n")
	require.NotEqual(t, -1, first, "the first problem is not reported")
	require.NotEqual(t, -1, second, "the second problem is not reported")
	require.Less(t, first, second)

	require.Contains(t, report, "     3 |   timeout \"http\" {\n")
	require.Contains(t, report, "    10 |   retry \"http\" {\n")
	require.Contains(t, report, "\nBlocks of type \"timeout\" are not expected here.\n")
	require.Contains(t, report, "\nBlocks of type \"retry\" are not expected here.\n")
}

func TestPrintErrorOfAnErrorWithoutAConfigurationProblemPrintsItsMessageAndALineBreak(t *testing.T) {
	require.Equal(t, "boom\n", printedError(t, errors.New("boom")))
}

func TestPrintErrorOfNilPrintsNothing(t *testing.T) {
	var out bytes.Buffer

	err := PrintError(&out, nil)

	require.NoError(t, err)
	require.Empty(t, out.String())
}

func TestConfigErrorFormattedWithPlusVEqualsThePlainPrintedReport(t *testing.T) {
	configError := applyPrintFixtureConfigError(t, printBadFixture)

	require.Equal(t, printedError(t, configError), fmt.Sprintf("%+v", configError))
}

func TestConfigErrorOfOneProblemFormattedWithVIsOneLine(t *testing.T) {
	configError := applyPrintFixtureConfigError(t, printBadFixture)

	text := fmt.Sprintf("%v", configError)

	require.NotEmpty(t, text)
	require.NotContains(t, text, "\n")
}

func TestPrintErrorWrapsTheDetailAt80ColumnsByDefault(t *testing.T) {
	report := printedError(t, longDetailDiagnostic())

	require.Greater(t, len(printLongDetail), 80)
	require.NotContains(t, report, printLongDetail)
	requireNoLineLongerThan(t, report, 80)
}

func TestPrintErrorWrapsTheDetailAtTheChosenWidth(t *testing.T) {
	report := printedError(t, longDetailDiagnostic(), WithWidth(40))

	require.NotContains(t, report, printLongDetail)
	requireNoLineLongerThan(t, report, 40)
}

func TestPrintErrorWithoutAWidthLeavesTheDetailOnOneLine(t *testing.T) {
	report := printedError(t, longDetailDiagnostic(), WithWidth(0))

	require.Contains(t, report, "\n"+printLongDetail+"\n")
}

func TestPrintErrorForATerminalWritesColourCodes(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.Terminal))

	require.True(t, strings.HasPrefix(report, "\x1b[1;31mError\x1b[0m"))
	require.Contains(t, report, "\x1b[1;31mtimeout\x1b[0m")
}

func TestPrintErrorForATerminalResetsColourBeforeEveryLineBreak(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.Terminal))

	requireNoLineBreakInsideColour(t, report)
}

func TestPrintErrorForATerminalStripsToThePlainReport(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	coloured := printedError(t, err, WithFormatter(highlight.Terminal))

	require.Equal(t, printedError(t, err), testutil.StripANSI(coloured))
}

func TestPrintErrorAsHTMLWritesOnePreformattedBlock(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTML))

	require.True(t, strings.HasPrefix(report, `<pre class="xcl">`))
	require.True(t, strings.HasSuffix(report, `</pre>`))
	require.Equal(t, 1, strings.Count(report, "<pre"))
	require.Equal(t, 1, strings.Count(report, "</pre>"))
}

func TestPrintErrorAsHTMLMarksTheFaultAsAnError(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTML))

	require.Contains(t, report, `<span class="xcl-problem xcl-error">timeout</span>`)
}

func TestPrintErrorAsHTMLWritesNoStyles(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTML))

	require.NotContains(t, report, "style=")
}

func TestPrintErrorAsHTMLEscapesTheConfiguration(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTML))

	require.Contains(t, report, "a&lt;b &amp; ")
	require.Contains(t, report, "&quot;")
	require.NotContains(t, report, "a<b")
}

func TestPrintErrorAsInlineHTMLStylesWithAttributes(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTMLInline))

	require.Contains(t, report, "style=")
}

func TestPrintErrorAsInlineHTMLWritesNoClasses(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	report := printedError(t, err, WithFormatter(highlight.HTMLInline))

	require.NotContains(t, report, "class=")
}

func TestPrintErrorCallsAFormatOfYourOwnExactlyOnce(t *testing.T) {
	err := applyPrintFixture(t, printTwoProblemsFixture)
	formatter := &recordingFormatter{}

	printedError(t, err, WithFormatter(formatter))

	require.Equal(t, 1, formatter.calls)
}

func TestPrintErrorGivesAFormatOfYourOwnTheWholeReport(t *testing.T) {
	err := applyPrintFixture(t, printTwoProblemsFixture)
	formatter := &recordingFormatter{}

	printedError(t, err, WithFormatter(formatter))

	require.Equal(t, printedError(t, err), joinedTokenText(formatter.tokens))
}

func TestPrintErrorLetsAFormatOfYourOwnWriteToTheGivenDestination(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	var out bytes.Buffer
	require.NoError(t, PrintError(&out, err, WithFormatter(&recordingFormatter{})))

	require.True(t, strings.HasPrefix(out.String(), "[Error][: Unsupported block type][\n\n][  on "), "got %q", out.String())
}

func TestErrorTokensJoinToThePlainReportWithoutAWidth(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	tokens := ErrorTokens(err)

	require.Equal(t, printedError(t, err, WithWidth(0)), joinedTokenText(tokens))
}

func TestErrorTokensLabelEveryPartOfTheReport(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	kinds := map[highlight.Kind]string{}
	for _, token := range ErrorTokens(err) {
		require.NotEmpty(t, token.Text)
		require.False(t, strings.HasPrefix(token.Kind.String(), "Kind("), "token %q has an unknown kind", token.Text)
		kinds[token.Kind] += token.Text
	}

	require.Equal(t, "Errortimeout", kinds[highlight.Error])
	require.Equal(t, ": Unsupported block type", kinds[highlight.Summary])
	require.Contains(t, kinds[highlight.Location], " line 7, in variable.name, field timeout[0]:")
	require.Equal(t, "     5 |      6 |      7 |      8 |      9 | ", kinds[highlight.Gutter])
	require.Equal(t, `Blocks of type "timeout" are not expected here.`, kinds[highlight.Detail])
	require.Contains(t, kinds[highlight.String], `"a<b & `)
	require.Equal(t, "// the default holds characters a web page escapes", kinds[highlight.LineComment])
}

func TestErrorTokensOfNilAreNil(t *testing.T) {
	require.Nil(t, ErrorTokens(nil))
}

func TestErrorTokensOfAnErrorWithoutAConfigurationProblemAreOneTextToken(t *testing.T) {
	tokens := ErrorTokens(errors.New("boom"))

	require.Equal(t, []highlight.Token{{Kind: highlight.Text, Text: "boom\n"}}, tokens)
}

func TestZapLogsThePlainReportOfAnApplyErrorInErrorVerbose(t *testing.T) {
	err := applyPrintFixture(t, printBadFixture)

	entry := logErrorWithZap(t, err)

	require.Equal(t, printedError(t, err), entry["errorVerbose"])
	require.Contains(t, entry["errorVerbose"], "     7 |   timeout \"http\" {\n")
}
