package diff_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/diff"
	"github.com/jumppad-labs/xcl/highlight"
)

// updated is a diff with one updated resource whose image changes
func updated() *diff.Diff {
	return &diff.Diff{
		Summary: diff.Summary{Update: 1},
		Resources: []diff.Resource{{
			Address: "resource.container.web",
			Action:  diff.ActionUpdate,
			Changes: []diff.Change{
				{Path: diff.Path{}.Attribute("image"), Before: "nginx", After: "caddy"},
			},
		}},
	}
}

// text joins the text of tokens, in order
func text(tokens []highlight.Token) string {
	joined := ""
	for _, token := range tokens {
		joined += token.Text
	}

	return joined
}

func TestTokensJoinToThePlainDiff(t *testing.T) {
	d := updated()

	expected := "  # resource.container.web will be updated\n" +
		"  ~ resource \"container\" \"web\" {\n" +
		"      ~ image = \"nginx\" -> \"caddy\"\n" +
		"    }\n" +
		"\n" +
		"Diff: 0 to create, 1 to update, 0 to replace, 0 to delete, 0 unchanged.\n"
	require.Equal(t, expected, text(d.Tokens()))
}

func TestTokensOfANilDiffSayNothingChanges(t *testing.T) {
	var d *diff.Diff

	require.Equal(t, "Diff: no changes, 0 unchanged.\n", text(d.Tokens()))
}

func TestTokensLabelTheCommentLine(t *testing.T) {
	tokens := updated().Tokens()

	require.Contains(t, tokens, highlight.Token{
		Kind: highlight.HashComment,
		Text: "# resource.container.web will be updated",
	})
}

func TestTokensLabelAChangeMarkerAndPath(t *testing.T) {
	tokens := updated().Tokens()

	require.Contains(t, tokens, highlight.Token{Kind: highlight.Changed, Text: "image"})
}

func TestTokensLabelValuesAsConfiguration(t *testing.T) {
	tokens := updated().Tokens()

	require.Contains(t, tokens, highlight.Token{Kind: highlight.String, Text: `"caddy"`})
}

func TestTokensLabelTheBlockHeaderAsConfiguration(t *testing.T) {
	tokens := updated().Tokens()

	require.Contains(t, tokens, highlight.Token{Kind: highlight.BlockType, Text: "resource"})
}

func TestTokensLeaveIndentationAndSignsAsText(t *testing.T) {
	tokens := updated().Tokens()

	signs := 0
	for _, token := range tokens {
		if strings.Contains(token.Text, "= ") || strings.Contains(token.Text, " -> ") {
			require.Equal(t, highlight.Text, token.Kind, "token %q", token.Text)
			signs++
		}
	}
	require.Equal(t, 2, signs, "the = and the -> of the change line")
	require.Equal(t, highlight.Token{Kind: highlight.Text, Text: "  "}, tokens[0])
}

func TestTokensHoldTheSensitivePlaceholderAsText(t *testing.T) {
	d := &diff.Diff{
		Summary: diff.Summary{Update: 1},
		Resources: []diff.Resource{{
			Address: "resource.container.web",
			Action:  diff.ActionUpdate,
			Changes: []diff.Change{
				{Path: diff.Path{}.Attribute("password"), Sensitive: true},
			},
		}},
	}

	require.Contains(t, text(d.Tokens()), "(sensitive value)")
}

func TestTokensNeverShareAKindWithTheirNeighbour(t *testing.T) {
	tokens := updated().Tokens()

	for i := 1; i < len(tokens); i++ {
		require.NotEqual(t, tokens[i-1].Kind, tokens[i].Kind, "tokens %d and %d", i-1, i)
	}
}

func TestTokensLabelACreatedHeaderMarkerAsInserted(t *testing.T) {
	d := &diff.Diff{Summary: diff.Summary{Create: 1}, Resources: []diff.Resource{{Address: "resource.container.web", Action: diff.ActionCreate}}}

	require.Contains(t, d.Tokens(), highlight.Token{Kind: highlight.Inserted, Text: "+"})
}

func TestTokensLabelADeletedHeaderMarkerAsDeleted(t *testing.T) {
	d := &diff.Diff{Summary: diff.Summary{Delete: 1}, Resources: []diff.Resource{{Address: "resource.container.web", Action: diff.ActionDelete}}}

	require.Contains(t, d.Tokens(), highlight.Token{Kind: highlight.Deleted, Text: "-"})
}

func TestTokensLabelAReplacedHeaderMarkerAsChanged(t *testing.T) {
	d := &diff.Diff{Summary: diff.Summary{Replace: 1}, Resources: []diff.Resource{{Address: "resource.container.web", Action: diff.ActionReplace}}}

	require.Contains(t, d.Tokens(), highlight.Token{Kind: highlight.Changed, Text: "-/+"})
}

func TestTokensLabelAnAddedValueAsInserted(t *testing.T) {
	d := &diff.Diff{
		Summary: diff.Summary{Update: 1},
		Resources: []diff.Resource{{
			Address: "resource.container.web",
			Action:  diff.ActionUpdate,
			Changes: []diff.Change{{Path: diff.Path{}.Attribute("ports").Index(2), After: float64(443)}},
		}},
	}

	require.Contains(t, d.Tokens(), highlight.Token{Kind: highlight.Inserted, Text: "ports[2]"})
}

func TestTokensLabelARemovedValueAsDeleted(t *testing.T) {
	d := &diff.Diff{
		Summary: diff.Summary{Update: 1},
		Resources: []diff.Resource{{
			Address: "resource.container.web",
			Action:  diff.ActionUpdate,
			Changes: []diff.Change{{Path: diff.Path{}.Attribute("image"), Before: "old"}},
		}},
	}

	require.Contains(t, d.Tokens(), highlight.Token{Kind: highlight.Deleted, Text: "image"})
}
