package highlight_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jumppad-labs/xcl/highlight"
)

// markers is a formatter wrapping every labelled token as [scope]text[/scope],
// the scope being the name the xcl-vscode grammar gives the same text. Text,
// and a kind with no scope, is written unchanged.
var markers = highlight.FormatterFunc(func(w io.Writer, _ highlight.Theme, tokens []highlight.Token) error {
	for _, token := range tokens {
		scope := token.Kind.Scope()

		text := token.Text
		if token.Kind != highlight.Text && scope != "" {
			text = "[" + scope + "]" + token.Text + "[/" + scope + "]"
		}

		if _, err := io.WriteString(w, text); err != nil {
			return err
		}
	}

	return nil
})

// renderText formats xcl text through formatter with the default theme, as
// labelled by Tokenize
func renderText(t *testing.T, formatter highlight.Formatter, text []byte) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, formatter.Format(&out, highlight.DefaultTheme, highlight.Tokenize(text)))

	return out.String()
}

func markedFile(t *testing.T, path string) string {
	t.Helper()

	input, err := os.ReadFile(path)
	require.NoError(t, err)

	return renderText(t, markers, input)
}

func markedSample(t *testing.T) string {
	t.Helper()

	return markedFile(t, "testdata/sample.xcl")
}

func TestTextLabelsBlockTypeAsStorageType(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[storage.type.xcl]resource[/storage.type.xcl] [entity.name.type.xcl]"network"`)
}

func TestTextLabelsKeywordBlockTypeAsStorageType(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[storage.type.xcl]variable[/storage.type.xcl] [entity.name.tag.xcl]"cpu_resources"`)
}

func TestTextLabelsRegisteredBlockTypeAsStorageType(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[storage.type.xcl]deployment[/storage.type.xcl] [entity.name.tag.xcl]"api"`)
}

func TestTextLabelsFirstOfTwoLabelsAsEntityNameType(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[entity.name.type.xcl]"network"[/entity.name.type.xcl] [entity.name.tag.xcl]"onprem"`)
}

func TestTextLabelsSecondOfTwoLabelsAsEntityNameTag(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[entity.name.tag.xcl]"onprem"[/entity.name.tag.xcl] {`)
}

func TestTextLabelsSingleLabelAsEntityNameTag(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[entity.name.tag.xcl]"cpu_resources"[/entity.name.tag.xcl] {`)
}

func TestTextLabelsNestedBlockAsEntityNameFunctionBlock(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "  [entity.name.function.block.xcl]container[/entity.name.function.block.xcl] {\n")
}

func TestTextLabelsAttributeAsVariableOtherProperty(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[variable.other.property.xcl]default[/variable.other.property.xcl] [keyword.operator.xcl]=`)
}

func TestTextLabelsDoubleSlashCommentAsCommentLineDoubleSlash(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "[comment.line.double-slash.xcl]// Comment forms[/comment.line.double-slash.xcl]\n")
}

func TestTextLabelsHashCommentAsCommentLineNumberSign(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "[comment.line.number-sign.xcl]# hash comment[/comment.line.number-sign.xcl]\n")
}

func TestTextLabelsMultiLineBlockCommentAsCommentBlock(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "[comment.block.xcl]/* block\n   comment */[/comment.block.xcl]")
}

func TestTextLabelsInlineBlockCommentAsCommentBlock(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[comment.block.xcl]/* inline */[/comment.block.xcl] [variable.other.property.xcl]enabled`)
}

func TestTextLabelsQuotedStringAsStringQuotedDouble(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[string.quoted.double.xcl]"10.6.0.0/16"[/string.quoted.double.xcl]`)
}

func TestTextLabelsTabEscapeAsConstantCharacterEscape(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[string.quoted.double.xcl]"a[/string.quoted.double.xcl][constant.character.escape.xcl]\t[/constant.character.escape.xcl]`)
}

func TestTextLabelsQuoteEscapeAsConstantCharacterEscape(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[string.quoted.double.xcl]b[/string.quoted.double.xcl][constant.character.escape.xcl]\"[/constant.character.escape.xcl]`)
}

func TestTextLabelsNonASCIICharacterInStringAsStringQuotedDouble(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[constant.character.escape.xcl]\"[/constant.character.escape.xcl][string.quoted.double.xcl]cé"[/string.quoted.double.xcl]`)
}

func TestTextLabelsInterpolationOpenerAsInterpolationBegin(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[string.quoted.double.xcl]"hello [/string.quoted.double.xcl][punctuation.section.interpolation.begin.xcl]${[/punctuation.section.interpolation.begin.xcl]`)
}

func TestTextLabelsInterpolationCloserAsInterpolationEnd(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[punctuation.section.interpolation.end.xcl]}[/punctuation.section.interpolation.end.xcl][string.quoted.double.xcl] world"[/string.quoted.double.xcl]`)
}

func TestTextLabelsTemplateMarkerOpenerAsInterpolationBegin(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[punctuation.section.interpolation.begin.xcl]#{{[/punctuation.section.interpolation.begin.xcl]`)
}

func TestTextLabelsTemplateMarkerCloserAsInterpolationEnd(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[punctuation.section.interpolation.end.xcl]}}[/punctuation.section.interpolation.end.xcl]`)
}

func TestTextLabelsIndentedHeredocOperatorAsKeywordOperatorHeredoc(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.heredoc.xcl]<<-[/keyword.operator.heredoc.xcl][keyword.control.heredoc.xcl]EOF[/keyword.control.heredoc.xcl]`)
}

func TestTextLabelsHeredocOperatorAsKeywordOperatorHeredoc(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.heredoc.xcl]<<[/keyword.operator.heredoc.xcl][keyword.control.heredoc.xcl]EOT[/keyword.control.heredoc.xcl]`)
}

func TestTextLabelsIndentedHeredocCloserAsKeywordControlHeredoc(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "[/string.unquoted.heredoc.xcl]  [keyword.control.heredoc.xcl]EOF[/keyword.control.heredoc.xcl]\n")
}

func TestTextLabelsHeredocCloserAsKeywordControlHeredoc(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, " text\n[/string.unquoted.heredoc.xcl][keyword.control.heredoc.xcl]EOT[/keyword.control.heredoc.xcl]\n")
}

func TestTextLabelsHeredocBodyAsStringUnquotedHeredoc(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, "[string.unquoted.heredoc.xcl]\"\n    server = true\n[/string.unquoted.heredoc.xcl]")
}

func TestTextLabelsTemplateMarkerInsideAsMetaInterpolationTemplate(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `#{{[/punctuation.section.interpolation.begin.xcl][meta.interpolation.template.xcl] [/meta.interpolation.template.xcl][punctuation.accessor.xcl].`)
	require.Contains(t, output, `data_dir[/variable.other.member.xcl][meta.interpolation.template.xcl] [/meta.interpolation.template.xcl][punctuation.section.interpolation.end.xcl]}}`)
}

func TestTextLabelsTemplateMarkerPathAsAccessorsAndMembers(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]Vars[/variable.other.member.xcl][punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]data_dir[/variable.other.member.xcl]`)
}

func TestTextLabelsIntegerAsConstantNumeric(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[constant.numeric.xcl]2048[/constant.numeric.xcl]`)
}

func TestTextLabelsDecimalAsConstantNumeric(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[constant.numeric.xcl]2.5[/constant.numeric.xcl]`)
}

func TestTextLabelsTrueAsConstantLanguage(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[constant.language.xcl]true[/constant.language.xcl]`)
}

func TestTextLabelsFalseAsConstantLanguage(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]=[/keyword.operator.xcl] [constant.language.xcl]false[/constant.language.xcl]`)
}

func TestTextLabelsNullAsConstantLanguage(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[constant.language.xcl]null[/constant.language.xcl]`)
}

func TestTextLabelsLenAsSupportFunctionBuiltin(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[support.function.builtin.xcl]len[/support.function.builtin.xcl](`)
}

func TestTextLabelsEnvAsSupportFunctionBuiltin(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[support.function.builtin.xcl]env[/support.function.builtin.xcl](`)
}

func TestTextLabelsOtherFunctionAsEntityNameFunction(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[entity.name.function.xcl]my_function[/entity.name.function.xcl](`)
}

func TestTextLabelsReferenceRootAsSupportClassReference(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[support.class.reference.xcl]resource[/support.class.reference.xcl][punctuation.accessor.xcl].`)
}

func TestTextLabelsReferenceToRegisteredBlockTypeAsSupportClassReference(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[support.class.reference.xcl]deployment[/support.class.reference.xcl][punctuation.accessor.xcl].`)
}

func TestTextLabelsReferenceDotAsPunctuationAccessor(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[/support.class.reference.xcl][punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]network`)
}

func TestTextLabelsReferenceMembersAsVariableOtherMember(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[variable.other.member.xcl]network[/variable.other.member.xcl][punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]onprem[/variable.other.member.xcl][punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]subnet[/variable.other.member.xcl]`)
}

func TestTextLabelsEqualsAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[variable.other.property.xcl]subnet[/variable.other.property.xcl] [keyword.operator.xcl]=[/keyword.operator.xcl] `)
}

func TestTextLabelsNotAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]![/keyword.operator.xcl][constant.language.xcl]false`)
}

func TestTextLabelsAndAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]&&[/keyword.operator.xcl]`)
}

func TestTextLabelsPlusAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]+[/keyword.operator.xcl]`)
}

func TestTextLabelsMultiplyAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]*[/keyword.operator.xcl]`)
}

func TestTextLabelsGreaterOrEqualAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]>=[/keyword.operator.xcl]`)
}

func TestTextLabelsOrAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]||[/keyword.operator.xcl]`)
}

func TestTextLabelsModuloAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]%[/keyword.operator.xcl]`)
}

func TestTextLabelsNotEqualAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]!=[/keyword.operator.xcl]`)
}

func TestTextLabelsEqualityAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]==[/keyword.operator.xcl]`)
}

func TestTextLabelsConditionalQuestionMarkAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]?[/keyword.operator.xcl] [string.quoted.double.xcl]"yes"`)
}

func TestTextLabelsConditionalColonAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]:[/keyword.operator.xcl] [string.quoted.double.xcl]"no"`)
}

func TestTextLabelsMinusAsKeywordOperator(t *testing.T) {
	output := markedSample(t)

	require.Contains(t, output, `[keyword.operator.xcl]-[/keyword.operator.xcl][constant.numeric.xcl]3[/constant.numeric.xcl]`)
}

func TestTextLabelsReferenceRootInShownReferences(t *testing.T) {
	output := markedFile(t, "testdata/references.xcl")

	require.Contains(t, output, `[support.class.reference.xcl]resource[/support.class.reference.xcl][punctuation.accessor.xcl].[/punctuation.accessor.xcl][variable.other.member.xcl]b[/variable.other.member.xcl]`)
}

func TestTextLabelsReferenceRootInsideInterpolationInShownReferences(t *testing.T) {
	output := markedFile(t, "testdata/references.xcl")

	require.Contains(t, output, `${[/punctuation.section.interpolation.begin.xcl][support.class.reference.xcl]variable[/support.class.reference.xcl][punctuation.accessor.xcl].`)
}

func TestTokenizeMergesLabelQuotesIntoOneToken(t *testing.T) {
	tokens := highlight.Tokenize([]byte(`variable "cpu" {}`))

	require.Contains(t, tokens, highlight.Token{Kind: highlight.NameLabel, Text: `"cpu"`})
}

func TestTokenizeLeavesBracesAndNewlinesAsText(t *testing.T) {
	tokens := highlight.Tokenize([]byte("variable \"cpu\" {\n}\n"))

	require.Equal(t, highlight.Token{Kind: highlight.Text, Text: " {\n}\n"}, tokens[len(tokens)-1])
}

func TestTokenizeLeavesWhitespaceAsText(t *testing.T) {
	tokens := highlight.Tokenize([]byte("variable \"cpu\" {\n}\n"))

	require.Equal(t, highlight.Token{Kind: highlight.Text, Text: " "}, tokens[1])
}

func TestTokenizeLeavesUnlabelledTextAsText(t *testing.T) {
	tokens := highlight.Tokenize([]byte("variable \"cpu\" {\n}\n"))

	expected := []highlight.Token{
		{Kind: highlight.BlockType, Text: "variable"},
		{Kind: highlight.Text, Text: " "},
		{Kind: highlight.NameLabel, Text: `"cpu"`},
		{Kind: highlight.Text, Text: " {\n}\n"},
	}
	require.Equal(t, expected, tokens)
}
