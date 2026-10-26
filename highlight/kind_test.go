package highlight_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

func TestKindParentOfAKindIsItsCategory(t *testing.T) {
	require.Equal(t, highlight.Comment, highlight.LineComment.Parent())
}

func TestKindParentOfAKindInAnotherCategoryIsThatCategory(t *testing.T) {
	require.Equal(t, highlight.String, highlight.HeredocMarker.Parent())
}

func TestKindParentOfACategoryIsItself(t *testing.T) {
	require.Equal(t, highlight.Comment, highlight.Comment.Parent())
}

func TestKindParentOfTextIsText(t *testing.T) {
	require.Equal(t, highlight.Text, highlight.Text.Parent())
}

func TestKindParentOfAnUnknownKindIsItsBlockOfAHundred(t *testing.T) {
	require.Equal(t, highlight.Comment, highlight.Kind(1105).Parent())
}

func TestKindStringIsTheKindName(t *testing.T) {
	require.Equal(t, "BlockType", highlight.BlockType.String())
}

func TestKindStringOfACategoryIsItsName(t *testing.T) {
	require.Equal(t, "Comment", highlight.Comment.String())
}

func TestKindStringOfAnUnknownKindIsItsNumber(t *testing.T) {
	require.Equal(t, "Kind(1105)", highlight.Kind(1105).String())
}

func TestKindClassOfBlockType(t *testing.T) {
	require.Equal(t, "xcl-block-type", highlight.BlockType.Class())
}

func TestKindClassOfStringEscape(t *testing.T) {
	require.Equal(t, "xcl-string-escape", highlight.StringEscape.Class())
}

func TestKindClassOfHeredocMarker(t *testing.T) {
	require.Equal(t, "xcl-heredoc-marker", highlight.HeredocMarker.Class())
}

func TestKindClassOfLineComment(t *testing.T) {
	require.Equal(t, "xcl-line-comment", highlight.LineComment.Class())
}

func TestKindClassOfACategory(t *testing.T) {
	require.Equal(t, "xcl-attribute", highlight.Attribute.Class())
}

func TestKindClassOfText(t *testing.T) {
	require.Equal(t, "xcl-text", highlight.Text.Class())
}

func TestKindClassOfAnUnknownKindIsItsParentClass(t *testing.T) {
	require.Equal(t, "xcl-comment", highlight.Kind(1105).Class())
}

func TestKindClassOfAnUnknownKindWithAnUnknownParentIsEmpty(t *testing.T) {
	require.Equal(t, "", highlight.Kind(9901).Class())
}

func TestKindScopeOfAttribute(t *testing.T) {
	require.Equal(t, "variable.other.property.xcl", highlight.Attribute.Scope())
}

func TestKindScopeOfLineComment(t *testing.T) {
	require.Equal(t, "comment.line.double-slash.xcl", highlight.LineComment.Scope())
}

func TestKindScopeOfTextIsEmpty(t *testing.T) {
	require.Equal(t, "", highlight.Text.Scope())
}

func TestKindScopeOfAnUnknownKindIsItsParentScope(t *testing.T) {
	require.Equal(t, "comment.xcl", highlight.Kind(1105).Scope())
}

func TestKindScopeOfAnUnknownKindWithAnUnknownParentIsEmpty(t *testing.T) {
	require.Equal(t, "", highlight.Kind(9901).Scope())
}

func TestKindsStartsWithText(t *testing.T) {
	kinds := highlight.Kinds()

	require.Equal(t, highlight.Text, kinds[0])
}

func TestKindsAreInNumericOrder(t *testing.T) {
	kinds := highlight.Kinds()

	for i := 1; i < len(kinds); i++ {
		require.Less(t, kinds[i-1], kinds[i], "kinds %d and %d are out of order", i-1, i)
	}
}

func TestKindsPutsEachCategoryBeforeItsKinds(t *testing.T) {
	kinds := highlight.Kinds()

	require.Equal(t, []highlight.Kind{
		highlight.Text,
		highlight.Block, highlight.BlockType, highlight.NestedBlock,
		highlight.Label, highlight.TypeLabel, highlight.NameLabel,
		highlight.Attribute,
		highlight.String, highlight.StringEscape, highlight.Heredoc, highlight.HeredocMarker, highlight.HeredocOperator,
		highlight.Number,
		highlight.Constant,
		highlight.Function, highlight.BuiltinFunction,
		highlight.Reference, highlight.Member,
		highlight.Operator,
		highlight.Punctuation, highlight.InterpolationBegin, highlight.InterpolationEnd, highlight.TemplateDirective, highlight.Accessor,
		highlight.Comment, highlight.LineComment, highlight.HashComment, highlight.BlockComment,
		highlight.Diff, highlight.Inserted, highlight.Deleted, highlight.Changed,
		highlight.Problem, highlight.Error, highlight.Warning,
		highlight.Report, highlight.Summary, highlight.Location, highlight.Gutter, highlight.Detail,
	}, kinds)
}

func TestKindsReturnsACopy(t *testing.T) {
	first := highlight.Kinds()
	first[0] = highlight.Comment

	second := highlight.Kinds()

	require.Equal(t, highlight.Text, second[0])
}

func TestKindNumbersAreFixed(t *testing.T) {
	require.Equal(t, highlight.Kind(0), highlight.Text)
	require.Equal(t, highlight.Kind(101), highlight.BlockType)
	require.Equal(t, highlight.Kind(300), highlight.Attribute)
	require.Equal(t, highlight.Kind(1101), highlight.LineComment)
	require.Equal(t, highlight.Kind(1404), highlight.Detail)
}
