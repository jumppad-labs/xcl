package coreservice

import (
	"testing"

	"github.com/stretchr/testify/require"

	corepb "github.com/jumppad-labs/xcl/coreservice/proto"
)

// orangePropertiesTheme is a VS Code colour theme that colours properties
// orange
const orangePropertiesTheme = `{
  "name": "Orange properties",
  "tokenColors": [
    {"scope": "variable.other.property", "settings": {"foreground": "#ff8800"}}
  ]
}`

// convertTheme is ConvertTheme on handle 0, requiring it to succeed
func (c *testClient) convertTheme(theme string) map[int32]string {
	c.t.Helper()

	resp := &corepb.ConvertThemeResponse{}
	c.ok(0, "ConvertTheme", &corepb.ConvertThemeRequest{VscodeThemeJson: []byte(theme)}, resp)

	return resp.GetStyles()
}

func TestConvertThemeColoursAttributeOrange(t *testing.T) {
	c := newTestClient(t)

	styles := c.convertTheme(orangePropertiesTheme)

	require.Contains(t, styles, int32(corepb.TokenKind_TOKEN_KIND_ATTRIBUTE))
	require.Contains(t, styles[int32(corepb.TokenKind_TOKEN_KIND_ATTRIBUTE)], "#ff8800")
}

func TestConvertThemeReturnsOnlyStyledKinds(t *testing.T) {
	c := newTestClient(t)

	styles := c.convertTheme(orangePropertiesTheme)

	require.NotContains(t, styles, int32(corepb.TokenKind_TOKEN_KIND_COMMENT))
}

func TestConvertThemeAcceptsComments(t *testing.T) {
	c := newTestClient(t)

	styles := c.convertTheme(`{
  // properties are orange
  "tokenColors": [
    {"scope": "variable.other.property", "settings": {"foreground": "#ff8800"}},
  ],
}`)

	require.Contains(t, styles[int32(corepb.TokenKind_TOKEN_KIND_ATTRIBUTE)], "#ff8800")
}

func TestConvertThemeInvalidColourIsInvalidTheme(t *testing.T) {
	c := newTestClient(t)

	out := c.fail(0, "ConvertTheme", &corepb.ConvertThemeRequest{VscodeThemeJson: []byte(`{
  "tokenColors": [
    {"scope": "variable.other.property", "settings": {"foreground": "#zz0000"}}
  ]
}`)})

	require.Equal(t, CodeInvalidTheme, out.GetCode())
	require.Equal(t, "theme", out.GetDetails()["source"])
	require.Contains(t, out.GetDetails()["reason"], "#zz0000")
}

func TestConvertThemeNotJSONIsInvalidTheme(t *testing.T) {
	c := newTestClient(t)

	out := c.fail(0, "ConvertTheme", &corepb.ConvertThemeRequest{VscodeThemeJson: []byte("not a theme")})

	require.Equal(t, CodeInvalidTheme, out.GetCode())
	require.Equal(t, "theme", out.GetDetails()["source"])
	require.NotEmpty(t, out.GetDetails()["reason"])
}

func TestConvertThemeEmptyIsInvalidArgument(t *testing.T) {
	c := newTestClient(t)

	out := c.fail(0, "ConvertTheme", &corepb.ConvertThemeRequest{})

	require.Equal(t, CodeInvalidArgument, out.GetCode())
	require.Equal(t, "vscode_theme_json", out.GetDetails()["argument"])
}

func TestConvertThemeWithConfigHandleIsMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	c := newTestClient(t)
	handle := c.createConfig(appRequest(t))

	body, status := c.call(handle, "ConvertTheme", &corepb.ConvertThemeRequest{VscodeThemeJson: []byte(orangePropertiesTheme)})

	require.Equal(t, StatusHandleMismatch, status)
	require.Nil(t, body)
}
