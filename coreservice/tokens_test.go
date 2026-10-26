package coreservice

import (
	"bytes"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
	"github.com/jumppad-labs/xcl/highlight"
)

// tokenKindValues is the generated descriptor's list of TokenKind values
func tokenKindValues(t *testing.T) protoreflect.EnumValueDescriptors {
	t.Helper()

	enum := corepb.File_xcl_core_v1_core_proto.Enums().ByName("TokenKind")
	require.NotNil(t, enum, "the protocol defines no TokenKind enum")

	return enum.Values()
}

// upperSnake writes a Go kind name such as BlockType as BLOCK_TYPE
func upperSnake(name string) string {
	var out strings.Builder
	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			out.WriteByte('_')
		}

		out.WriteRune(unicode.ToUpper(r))
	}

	return out.String()
}

func TestTokenKindsMatchGoKinds(t *testing.T) {
	values := tokenKindValues(t)

	for _, kind := range highlight.Kinds() {
		value := values.ByNumber(protoreflect.EnumNumber(kind))
		require.NotNil(t, value, "Go kind %s (%d) has no TokenKind", kind, int(kind))
		require.Equal(t, "TOKEN_KIND_"+upperSnake(kind.String()), string(value.Name()))
	}
}

func TestEveryTokenKindIsAGoKind(t *testing.T) {
	values := tokenKindValues(t)

	goKinds := map[highlight.Kind]bool{}
	for _, kind := range highlight.Kinds() {
		goKinds[kind] = true
	}

	for i := 0; i < values.Len(); i++ {
		value := values.Get(i)
		require.True(t, goKinds[highlight.Kind(value.Number())], "TokenKind %s (%d) is not a Go kind", value.Name(), value.Number())
	}
}

func TestTokenKindCountMatchesGoKinds(t *testing.T) {
	require.Equal(t, len(highlight.Kinds()), tokenKindValues(t).Len())
}

func TestTokenKindCategoryMatchesGoParent(t *testing.T) {
	for _, kind := range highlight.Kinds() {
		wire := int32(corepb.TokenKind(kind))
		require.Equal(t, int32(kind.Parent()), wire/100*100, "category of %s", kind)
	}
}

func TestTokensToProtoKeepsKindAndText(t *testing.T) {
	tokens := []highlight.Token{
		{Kind: highlight.Attribute, Text: "image"},
		{Kind: highlight.Text, Text: " = "},
		{Kind: highlight.String, Text: `"nginx"`},
	}

	out := tokensToProto(tokens)

	require.Len(t, out, 3)
	require.Equal(t, corepb.TokenKind_TOKEN_KIND_ATTRIBUTE, out[0].GetKind())
	require.Equal(t, "image", out[0].GetText())
	require.Equal(t, corepb.TokenKind_TOKEN_KIND_TEXT, out[1].GetKind())
	require.Equal(t, " = ", out[1].GetText())
	require.Equal(t, corepb.TokenKind_TOKEN_KIND_STRING, out[2].GetKind())
	require.Equal(t, `"nginx"`, out[2].GetText())
}

func TestTokensToProtoKeepsAnUnknownKindNumber(t *testing.T) {
	out := tokensToProto([]highlight.Token{{Kind: highlight.Kind(1199), Text: "# x"}})

	require.Len(t, out, 1)
	require.Equal(t, int32(1199), int32(out[0].GetKind()))
}

func TestTokensToProtoEmptyIsNil(t *testing.T) {
	require.Nil(t, tokensToProto(nil))
	require.Nil(t, tokensToProto([]highlight.Token{}))
}

func TestUnknownTokenKindSurvivesTheWire(t *testing.T) {
	body, err := proto.Marshal(&corepb.Token{Kind: corepb.TokenKind(1199), Text: "# x"})
	require.NoError(t, err)

	received := &corepb.Token{}
	require.NoError(t, proto.Unmarshal(body, received))

	require.Equal(t, int32(1199), int32(received.GetKind()))
	require.Equal(t, "# x", received.GetText())
}

func TestUnknownTokenKindFallsIntoItsCategory(t *testing.T) {
	received := corepb.TokenKind(1199)

	require.Equal(t, int32(corepb.TokenKind_TOKEN_KIND_COMMENT), int32(received)/100*100)
	require.Equal(t, highlight.Comment, highlight.Kind(received).Parent())
}

// Go is the reference other languages are held to: a kind it does not know
// prints in its category's style
func TestUnknownTokenKindPrintsInItsCategoryStyle(t *testing.T) {
	var unknown, comment bytes.Buffer

	err := highlight.Terminal.Format(&unknown, highlight.DefaultTheme, []highlight.Token{{Kind: highlight.Kind(1199), Text: "# x"}})
	require.NoError(t, err)

	err = highlight.Terminal.Format(&comment, highlight.DefaultTheme, []highlight.Token{{Kind: highlight.Comment, Text: "# x"}})
	require.NoError(t, err)

	require.Contains(t, comment.String(), "\x1b[", "the default theme styles comments")
	require.Equal(t, comment.String(), unknown.String())
}
