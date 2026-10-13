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

// text joins the text of pieces, in order
func text(pieces []highlight.Piece) string {
	joined := ""
	for _, p := range pieces {
		joined += p.Text
	}

	return joined
}

func TestPiecesJoinToThePlainDiff(t *testing.T) {
	d := updated()

	require.Equal(t, string(diff.Render(d)), text(d.Pieces()))
}

func TestPiecesOfANilDiffSayNothingChanges(t *testing.T) {
	var d *diff.Diff

	require.Equal(t, "Diff: no changes, 0 unchanged.\n", text(d.Pieces()))
}

func TestPiecesLabelTheCommentLine(t *testing.T) {
	pieces := updated().Pieces()

	require.Contains(t, pieces, highlight.Piece{
		Scope: highlight.ScopeHashComment,
		Text:  "# resource.container.web will be updated",
	})
}

func TestPiecesLabelAChangeMarkerAndPath(t *testing.T) {
	pieces := updated().Pieces()

	require.Contains(t, pieces, highlight.Piece{Scope: highlight.ScopeChanged, Text: "image"})
}

func TestPiecesLabelValuesAsConfiguration(t *testing.T) {
	pieces := updated().Pieces()

	require.Contains(t, pieces, highlight.Piece{Scope: highlight.ScopeString, Text: `"caddy"`})
}

func TestPiecesLabelTheBlockHeaderAsConfiguration(t *testing.T) {
	pieces := updated().Pieces()

	require.Contains(t, pieces, highlight.Piece{Scope: highlight.ScopeBlockType, Text: "resource"})
}

func TestPiecesLeaveIndentationAndSignsUnlabelled(t *testing.T) {
	pieces := updated().Pieces()

	signs := 0
	for _, p := range pieces {
		if strings.Contains(p.Text, "= ") || strings.Contains(p.Text, " -> ") {
			require.Empty(t, p.Scope, "piece %q", p.Text)
			signs++
		}
	}
	require.Equal(t, 2, signs, "the = and the -> of the change line")
	require.Equal(t, highlight.Piece{Scope: "", Text: "  "}, pieces[0])
}

func TestPiecesHoldTheSensitivePlaceholderUnlabelled(t *testing.T) {
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

	require.Contains(t, text(d.Pieces()), "(sensitive value)")
}

func TestPiecesNeverShareAScopeWithTheirNeighbour(t *testing.T) {
	pieces := updated().Pieces()

	for i := 1; i < len(pieces); i++ {
		require.NotEqual(t, pieces[i-1].Scope, pieces[i].Scope, "pieces %d and %d", i-1, i)
	}
}
