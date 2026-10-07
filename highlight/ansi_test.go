package highlight_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	xclerrors "github.com/jumppad-labs/xcl/errors"
	"github.com/jumppad-labs/xcl/highlight"
	"github.com/jumppad-labs/xcl/internal/testutil"
)

// sgrSequence captures the parameter list of an SGR escape sequence
var sgrSequence = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func readSample(t *testing.T) []byte {
	t.Helper()

	input, err := os.ReadFile("testdata/sample.xcl")
	require.NoError(t, err)

	return input
}

func renderSampleWithDefaultTheme(t *testing.T) string {
	t.Helper()

	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	return string(highlight.Text(readSample(t), renderer))
}

func referenceThemeRenderer(t *testing.T) *highlight.ANSIRenderer {
	t.Helper()

	renderer, err := highlight.NewANSIRenderer(highlight.WithThemeFile("testdata/reference-theme.json"))
	require.NoError(t, err)

	return renderer
}

func renderSampleWithReferenceTheme(t *testing.T) string {
	t.Helper()

	return string(highlight.Text(readSample(t), referenceThemeRenderer(t)))
}

func TestANSIRendererDefaultUsesOnlyBasicColours(t *testing.T) {
	output := renderSampleWithDefaultTheme(t)

	allowed := map[string]bool{"0": true, "1": true, "3": true, "4": true, "9": true}
	for code := 30; code <= 37; code++ {
		allowed[strconv.Itoa(code)] = true
	}
	for code := 90; code <= 97; code++ {
		allowed[strconv.Itoa(code)] = true
	}

	colourCodes := 0
	for _, match := range sgrSequence.FindAllStringSubmatch(output, -1) {
		for _, parameter := range strings.Split(match[1], ";") {
			require.NotEqual(t, "38", parameter, "extended colour in %q", match[0])
			require.True(t, allowed[parameter], "unexpected SGR parameter %q in %q", parameter, match[0])

			if parameter != "0" && parameter != "1" && parameter != "3" && parameter != "4" && parameter != "9" {
				colourCodes++
			}
		}
	}

	require.Greater(t, colourCodes, 0)
}

func TestANSIRendererDefaultColoursBlockTypeMagentaBold(t *testing.T) {
	output := renderSampleWithDefaultTheme(t)

	require.Contains(t, output, "\x1b[1;35mresource\x1b[0m")
}

func TestANSIRendererThemeColoursBlockType(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[1;38;2;197;134;192mresource\x1b[0m ")
}

func TestANSIRendererThemeColoursTypeLabel(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;78;201;176m\"network\"\x1b[0m")
}

func TestANSIRendererThemeColoursNameLabel(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[4;38;2;86;156;214m\"onprem\"\x1b[0m")
}

func TestANSIRendererThemeColoursAttribute(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;156;220;254msubnet\x1b[0m")
}

func TestANSIRendererThemeColoursString(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;206;145;120m\"10.6.0.0/16\"\x1b[0m")
}

func TestANSIRendererThemeColoursHeredocBody(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;206;145;120m\"\x1b[0m\n\x1b[38;2;206;145;120m    server = true\x1b[0m\n")
}

func TestANSIRendererThemeColoursNumber(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;181;206;168m2048\x1b[0m")
}

func TestANSIRendererThemeColoursLanguageConstant(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;181;206;168mnull\x1b[0m")
}

func TestANSIRendererThemeColoursEscape(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;181;206;168m\\t\x1b[0m")
}

func TestANSIRendererThemeColoursReferenceRoot(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;79;193;255mdeployment\x1b[0m.")
}

func TestANSIRendererThemeColoursComment(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[3;38;2;106;153;85m# hash comment\x1b[0m\n")
}

func TestANSIRendererThemeAppliesBold(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeBlockType, "resource")

	require.Equal(t, "\x1b[1;38;2;197;134;192mresource\x1b[0m", output)
}

func TestANSIRendererThemeAppliesItalic(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeLineComment, "// note")

	require.Equal(t, "\x1b[3;38;2;106;153;85m// note\x1b[0m", output)
}

func TestANSIRendererThemeAppliesUnderline(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeNameLabel, `"main"`)

	require.Equal(t, "\x1b[4;38;2;86;156;214m\"main\"\x1b[0m", output)
}

func TestANSIRendererThemeAppliesStrikethrough(t *testing.T) {
	theme := `{"tokenColors": [{"scope": "comment", "settings": {"fontStyle": "strikethrough"}}]}`
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader(theme)))
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeLineComment, "// old")

	require.Equal(t, "\x1b[9m// old\x1b[0m", output)
}

func TestANSIRendererThemeAppliesEveryFontStyleInOrder(t *testing.T) {
	theme := `{"tokenColors": [{"scope": "comment", "settings": {"foreground": "#010203", "fontStyle": "strikethrough underline italic bold"}}]}`
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader(theme)))
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeLineComment, "// all")

	require.Equal(t, "\x1b[1;3;4;9;38;2;1;2;3m// all\x1b[0m", output)
}

func TestANSIRendererSpecificRuleOverridesGeneral(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeNameLabel, `"main"`)

	require.Equal(t, "\x1b[4;38;2;86;156;214m\"main\"\x1b[0m", output)
	require.NotContains(t, output, "38;2;78;201;176")
}

func TestANSIRendererGeneralRuleColoursScopeWithoutSpecificRule(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeTypeLabel, `"network"`)

	require.Equal(t, "\x1b[38;2;78;201;176m\"network\"\x1b[0m", output)
}

func TestANSIRendererLeavesUnmatchedTokenUncoloured(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeOperator, "=")

	require.Equal(t, "=", output)
}

func TestANSIRendererLeavesUnmatchedTokensInTextUncoloured(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Contains(t, output, "\x1b[38;2;156;220;254msubnet\x1b[0m = \x1b[38;2;206;145;120m\"10.6.0.0/16\"\x1b[0m")
}

func TestANSIRendererDefaultLeavesUnmatchedTokenUncoloured(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeOperator, "=")

	require.Equal(t, "=", output)
}

func TestANSIRendererThemeColoursInserted(t *testing.T) {
	theme := `{"tokenColors": [{"scope": "markup.inserted", "settings": {"foreground": "#00ff00"}}]}`
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader(theme)))
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeInserted, "+ added")

	require.Equal(t, "\x1b[38;2;0;255;0m+ added\x1b[0m", output)
}

func TestANSIRendererThemeWithoutMarkupRuleLeavesDeletedUncoloured(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeDeleted, "- removed")

	require.Equal(t, "- removed", output)
}

func TestANSIRendererDefaultColoursInsertedGreen(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer()
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeInserted, "+ added")

	require.Equal(t, "\x1b[32m+ added\x1b[0m", output)
}

func TestANSIRendererLeavesUnlabelledTextUncoloured(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render("", " {\n")

	require.Equal(t, " {\n", output)
}

func TestANSIRendererStylesMultiLineTokenPerLine(t *testing.T) {
	renderer := referenceThemeRenderer(t)

	output := renderer.Render(highlight.ScopeBlockComment, "/* first\n\n   last */")

	require.Equal(t,
		"\x1b[3;38;2;106;153;85m/* first\x1b[0m\n\n\x1b[3;38;2;106;153;85m   last */\x1b[0m",
		output)
}

func TestANSIRendererDefaultOutputStripsToInput(t *testing.T) {
	output := renderSampleWithDefaultTheme(t)

	require.Equal(t, string(readSample(t)), testutil.StripANSI(output))
}

func TestANSIRendererThemeOutputStripsToInput(t *testing.T) {
	output := renderSampleWithReferenceTheme(t)

	require.Equal(t, string(readSample(t)), testutil.StripANSI(output))
}

func TestNewANSIRendererFailsForUnreadableThemeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	renderer, err := highlight.NewANSIRenderer(highlight.WithThemeFile(path))

	require.Nil(t, renderer)
	require.ErrorIs(t, err, xclerrors.ErrInvalidTheme)
	require.ErrorIs(t, err, fs.ErrNotExist)

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, path, detail.Source)
	require.Equal(t, "it cannot be read", detail.Reason)
}

func TestNewANSIRendererFailsForInvalidTheme(t *testing.T) {
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader("{")))

	require.Nil(t, renderer)
	require.ErrorIs(t, err, xclerrors.ErrInvalidTheme)

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, "theme", detail.Source)
}

func TestNewANSIRendererFailsForInvalidThemeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	theme := `{"tokenColors": [{"scope": "comment", "settings": {"foreground": "#zz0000"}}]}`
	require.NoError(t, os.WriteFile(path, []byte(theme), 0o600))

	renderer, err := highlight.NewANSIRenderer(highlight.WithThemeFile(path))

	require.Nil(t, renderer)
	require.ErrorIs(t, err, xclerrors.ErrInvalidTheme)

	var detail *xclerrors.InvalidThemeError
	require.True(t, errors.As(err, &detail))
	require.Equal(t, path, detail.Source)
	require.Equal(t, `tokenColors[0].settings.foreground "#zz0000" is not a colour`, detail.Reason)
}

func TestNewANSIRendererReadsThemeFromReader(t *testing.T) {
	theme := `{"tokenColors": [{"scope": "string", "settings": {"foreground": "#102030"}}]}`
	renderer, err := highlight.NewANSIRenderer(highlight.WithTheme(strings.NewReader(theme)))
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeString, `"x"`)

	require.Equal(t, "\x1b[38;2;16;32;48m\"x\"\x1b[0m", output)
}

func TestNewANSIRendererReadsThemeFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	theme := `{"tokenColors": [{"scope": "string", "settings": {"foreground": "#102030"}}]}`
	require.NoError(t, os.WriteFile(path, []byte(theme), 0o600))

	renderer, err := highlight.NewANSIRenderer(highlight.WithThemeFile(path))
	require.NoError(t, err)

	output := renderer.Render(highlight.ScopeString, `"x"`)

	require.Equal(t, "\x1b[38;2;16;32;48m\"x\"\x1b[0m", output)
}

func TestANSIRendererIsSafeForConcurrentUse(t *testing.T) {
	renderer := referenceThemeRenderer(t)
	input := readSample(t)
	want := string(highlight.Text(input, renderer))

	outputs := make([]string, 8)
	var group sync.WaitGroup
	for i := range outputs {
		group.Add(1)
		go func() {
			defer group.Done()
			outputs[i] = string(highlight.Text(input, renderer))
		}()
	}
	group.Wait()

	for _, output := range outputs {
		require.Equal(t, want, output)
	}
}
