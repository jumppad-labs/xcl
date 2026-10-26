package highlight_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/diff"
	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	hcl "github.com/jumppad-labs/xcl/internal/xcl"
)

// update rewrites the reference outputs from the code instead of checking
// them. A change to them is a change to a public contract, reviewed like any
// other.
var update = flag.Bool("update", false, "rewrite the reference outputs in testdata/reference")

// referenceDirectory holds the reference outputs every language's printing is
// checked against
const referenceDirectory = "testdata/reference"

// referenceToken is a token as the reference inputs write it
type referenceToken struct {
	Kind int    `json:"kind"`
	Text string `json:"text"`
}

// referenceSource is the configuration the reference report is about
const referenceSource = `variable "name" {
  default = "a<b"

  timeout "http" {
    connect = 30
  }

  retries = 2
}
`

// referenceReportTokens builds the reference report: an unsupported block
// with its entity, field and a detail longer than 80 characters, and a
// warning with no source
func referenceReportTokens() []highlight.Token {
	sources := map[string][]byte{"main.xcl": []byte(referenceSource)}

	unsupported := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Unsupported block type",
		Detail:   `Blocks of type "timeout" are not expected here. A variable takes only default, description, type and sensitive, so move the timeout to the resource that uses it.`,
		Subject: &hcl.Range{
			Filename: "main.xcl",
			Start:    hcl.Pos{Line: 4, Column: 3, Byte: 39},
			End:      hcl.Pos{Line: 4, Column: 10, Byte: 46},
		},
	}, sources)
	unsupported.Entity = "variable.name"
	unsupported.Field = "timeout[0]"

	deprecated := xclerrors.NewDiagnostic(&hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  "Deprecated attribute",
		Detail:   "retries is deprecated, use a retry block.",
	}, sources)

	return xcl.ErrorTokens(xclerrors.Diagnostics{unsupported, deprecated})
}

// referenceEntityTokens labels the reference resource
func referenceEntityTokens(t *testing.T) []highlight.Token {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(referenceDirectory, "inputs", "entity.xcl"))
	require.NoError(t, err)

	return highlight.Tokenize(src)
}

// referenceDiffTokens lays out the reference change summary, one resource of
// each action
func referenceDiffTokens() []highlight.Token {
	d := &diff.Diff{
		Summary: diff.Summary{Create: 1, Update: 2, Replace: 1, Delete: 1, Unchanged: 4},
		Resources: []diff.Resource{
			{
				Address: "resource.container.api",
				Action:  diff.ActionUpdate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("ports").Index(0).Attribute("host"), Before: float64(8080), After: float64(9090)},
					{Path: diff.Path{}.Attribute("ports").Index(2), After: map[string]any{"host": float64(443), "local": float64(8443)}},
					{Path: diff.Path{}.Attribute("env").Key("DB_PASSWORD"), Sensitive: true},
				},
			},
			{Address: "resource.container.cache", Action: diff.ActionUpdate},
			{
				Address: "resource.container.web",
				Action:  diff.ActionCreate,
				Changes: []diff.Change{
					{Path: diff.Path{}.Attribute("image"), After: "nginx"},
					{Path: diff.Path{}.Attribute("db_host"), Unknown: true},
				},
			},
			{Address: "resource.network.app", Action: diff.ActionReplace, Reason: diff.ReplaceFailed},
			{Address: "resource.postgres.old", Action: diff.ActionDelete},
		},
	}

	return d.Tokens()
}

// encodeReferenceTokens writes tokens as a reference input
func encodeReferenceTokens(t *testing.T, tokens []highlight.Token) []byte {
	t.Helper()

	written := make([]referenceToken, len(tokens))
	for i, token := range tokens {
		written[i] = referenceToken{Kind: int(token.Kind), Text: token.Text}
	}

	data, err := json.MarshalIndent(written, "", "  ")
	require.NoError(t, err)

	return append(data, '\n')
}

// readReferenceTokens reads the stored reference input called name
func readReferenceTokens(t *testing.T, name string) []highlight.Token {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(referenceDirectory, name+".tokens.json"))
	require.NoError(t, err)

	var read []referenceToken
	require.NoError(t, json.Unmarshal(data, &read))

	tokens := make([]highlight.Token, len(read))
	for i, token := range read {
		tokens[i] = highlight.Token{Kind: highlight.Kind(token.Kind), Text: token.Text}
	}

	return tokens
}

// requireReference checks got against the reference file, or rewrites the
// file with -update
func requireReference(t *testing.T, file string, got []byte) {
	t.Helper()

	path := filepath.Join(referenceDirectory, file)

	if *update {
		require.NoError(t, os.WriteFile(path, got, 0o644))
		return
	}

	want, err := os.ReadFile(path)
	require.NoError(t, err, "the reference output %s is missing; write it with -update", path)

	require.Equal(t, string(want), string(got))
}

// requireReferenceTokens checks the freshly built tokens against the stored
// input called name, or rewrites it with -update
func requireReferenceTokens(t *testing.T, name string, tokens []highlight.Token) {
	t.Helper()

	requireReference(t, name+".tokens.json", encodeReferenceTokens(t, tokens))
}

// requireReferenceFormat formats the stored input called name with
// formatter and the default theme, and checks it against its reference
// output file
func requireReferenceFormat(t *testing.T, name string, formatter highlight.Formatter, extension string) {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, formatter.Format(&out, highlight.DefaultTheme, readReferenceTokens(t, name)))

	requireReference(t, name+extension, out.Bytes())
}

func TestReferenceReportTokens(t *testing.T) {
	requireReferenceTokens(t, "report", referenceReportTokens())
}

func TestReferenceEntityTokens(t *testing.T) {
	requireReferenceTokens(t, "entity", referenceEntityTokens(t))
}

func TestReferenceDiffTokens(t *testing.T) {
	requireReferenceTokens(t, "diff", referenceDiffTokens())
}

func TestReferenceReportPlain(t *testing.T) {
	requireReferenceFormat(t, "report", highlight.Plain, ".plain.txt")
}

func TestReferenceReportTerminal(t *testing.T) {
	requireReferenceFormat(t, "report", highlight.Terminal, ".terminal.txt")
}

func TestReferenceReportHTML(t *testing.T) {
	requireReferenceFormat(t, "report", highlight.HTML, ".html")
}

func TestReferenceReportHTMLInline(t *testing.T) {
	requireReferenceFormat(t, "report", highlight.HTMLInline, ".inline.html")
}

func TestReferenceReportSVG(t *testing.T) {
	requireReferenceFormat(t, "report", highlight.SVG, ".svg")
}

func TestReferenceEntityPlain(t *testing.T) {
	requireReferenceFormat(t, "entity", highlight.Plain, ".plain.txt")
}

func TestReferenceEntityTerminal(t *testing.T) {
	requireReferenceFormat(t, "entity", highlight.Terminal, ".terminal.txt")
}

func TestReferenceEntityHTML(t *testing.T) {
	requireReferenceFormat(t, "entity", highlight.HTML, ".html")
}

func TestReferenceEntityHTMLInline(t *testing.T) {
	requireReferenceFormat(t, "entity", highlight.HTMLInline, ".inline.html")
}

func TestReferenceEntitySVG(t *testing.T) {
	requireReferenceFormat(t, "entity", highlight.SVG, ".svg")
}

func TestReferenceDiffPlain(t *testing.T) {
	requireReferenceFormat(t, "diff", highlight.Plain, ".plain.txt")
}

func TestReferenceDiffTerminal(t *testing.T) {
	requireReferenceFormat(t, "diff", highlight.Terminal, ".terminal.txt")
}

func TestReferenceDiffHTML(t *testing.T) {
	requireReferenceFormat(t, "diff", highlight.HTML, ".html")
}

func TestReferenceDiffHTMLInline(t *testing.T) {
	requireReferenceFormat(t, "diff", highlight.HTMLInline, ".inline.html")
}

func TestReferenceDiffSVG(t *testing.T) {
	requireReferenceFormat(t, "diff", highlight.SVG, ".svg")
}

func TestReferenceDefaultStylesheet(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, highlight.WriteCSS(&out, highlight.DefaultTheme))

	requireReference(t, "default.css", out.Bytes())
}

func TestReferenceClassNames(t *testing.T) {
	var out bytes.Buffer
	out.WriteString("xcl\n")
	for _, kind := range highlight.Kinds() {
		if kind == highlight.Text {
			continue
		}

		out.WriteString(kind.Class() + "\n")
	}

	requireReference(t, "classes.txt", out.Bytes())
}
