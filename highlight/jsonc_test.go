package highlight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStripJSONCKeepsSlashesInsideStrings(t *testing.T) {
	stripped := stripJSONC([]byte(`{"url": "https://example.com/a//b"}`))

	require.Equal(t, `{"url": "https://example.com/a//b"}`, string(stripped))
}

func TestStripJSONCKeepsEscapedQuoteInsideString(t *testing.T) {
	stripped := stripJSONC([]byte(`{"a": "say \"// hi\""}`))

	require.Equal(t, `{"a": "say \"// hi\""}`, string(stripped))
}

func TestStripJSONCRemovesLineComment(t *testing.T) {
	stripped := stripJSONC([]byte("{\n  // a comment\n  \"a\": 1\n}"))

	require.Equal(t, "{\n  \n  \"a\": 1\n}", string(stripped))
}

func TestStripJSONCReplacesBlockCommentWithSpace(t *testing.T) {
	stripped := stripJSONC([]byte(`[1,/* two */2]`))

	require.Equal(t, `[1, 2]`, string(stripped))
}

func TestStripJSONCRemovesTrailingCommaBeforeBracket(t *testing.T) {
	stripped := stripJSONC([]byte("[1, 2,\n ]"))

	require.Equal(t, "[1, 2\n ]", string(stripped))
}

func TestStripJSONCRemovesTrailingCommaBeforeBrace(t *testing.T) {
	stripped := stripJSONC([]byte(`{"a": 1, }`))

	require.Equal(t, `{"a": 1 }`, string(stripped))
}

func TestStripJSONCRemovesTrailingCommaBeforeComment(t *testing.T) {
	stripped := stripJSONC([]byte("[1, // last\n]"))

	require.Equal(t, "[1 \n]", string(stripped))
}

func TestStripJSONCKeepsCommaBeforeBracketInsideString(t *testing.T) {
	stripped := stripJSONC([]byte(`["a,]"]`))

	require.Equal(t, `["a,]"]`, string(stripped))
}
