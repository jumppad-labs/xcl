package prettylog

import (
	"bytes"
	"io"
	"regexp"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"
)

// highlightFixture is configuration text as the encoder writes it, holding
// every kind of token the highlighter colours
const highlightFixture = `resource "network" "main" {
  subnet      = "10.0.0.0/16"
  provider_id = "id-main" # set by the provider
}
deployment "api" {
  replicas = 3
  enabled  = true
  service  = deployment.api.meta.id
  ports = [
    8080,
  ]
}`

// ansiEscapes matches the escape sequences a terminal styles text with
var ansiEscapes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// colouredHighlighter returns a highlighter that always writes colour, as it
// would to a terminal
func colouredHighlighter(t *testing.T) highlighter {
	t.Helper()

	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.ANSI)

	return newHighlighter(r)
}

func TestHighlightLeavesTheTextUnchanged(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.NotEqual(t, highlightFixture, highlighted, "nothing was coloured, so the check proves nothing")
	require.Equal(t, highlightFixture, ansiEscapes.ReplaceAllString(highlighted, ""))
}

func TestHighlightColoursTheBlockType(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.blockType.Render("resource"))
	require.Contains(t, highlighted, h.blockType.Render("deployment"))
}

func TestHighlightColoursTheFirstOfTwoLabelsAsTheType(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.typeLabel.Render(`"network"`))
}

func TestHighlightColoursTheSecondOfTwoLabelsAsTheName(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.nameLabel.Render(`"main"`))
}

func TestHighlightColoursASingleLabelAsTheName(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.nameLabel.Render(`"api"`))
}

func TestHighlightColoursAttributeNames(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.attribute.Render("subnet"))
	require.Contains(t, highlighted, h.attribute.Render("ports"))
}

func TestHighlightColoursStrings(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.str.Render(`"10.0.0.0/16"`))
}

func TestHighlightColoursNumbersAndConstants(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.constant.Render("3"))
	require.Contains(t, highlighted, h.constant.Render("true"))
	require.Contains(t, highlighted, h.constant.Render("8080"))
}

func TestHighlightColoursTheRootOfAReference(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.reference.Render("deployment")+".api.meta.id")
}

func TestHighlightColoursComments(t *testing.T) {
	h := colouredHighlighter(t)

	highlighted := h.highlight(highlightFixture)

	require.Contains(t, highlighted, h.comment.Render("# set by the provider"))
}

func TestHighlightWritesNoColourWhenTheWriterIsNotATerminal(t *testing.T) {
	h := newHighlighter(lipgloss.NewRenderer(&bytes.Buffer{}))

	highlighted := h.highlight(highlightFixture)

	require.Equal(t, highlightFixture, highlighted)
}
