---
created_date: "2026-10-06"
document_status: final
closed_date: "2026-10-06"
---

# Context: 20261006112108-17623cda-encoder-syntax-highlighting

## Current State Analysis

- `encode.go:27-39`: encoding options are a closed `encodeOptions` struct set by functional `EncodeOption`s: `IncludeComputed`, `IncludeEmpty`, `RevealSensitive` and `ShowReferences`.
- `encode.go:180-230`: `encodeEntity` is the single path behind `EncodeEntity` (`:123`) and `EncodeSavedEntity` (`:149`). It builds an `hclwrite` block, applies `ShowReferences` (`:281`), and returns the formatted `file.Bytes()`. Output is plain text only.
- `internal/xcl/hclsyntax/public.go:159`: `LexConfig` is available, and its tokens carry byte ranges (`token.go:16-20`). Nothing in the library tokenises for display today.
- `example/prettylog/highlight.go`: a 122-line regex, line-based highlighter built on lipgloss styles, the only configuration highlighter in the repo. `prettylog.go:73-104` wires it in, deciding colour per writer through `lipgloss.NewRenderer(w)`.
- Dependency boundary: no non-example package depends on charmbracelet, and prettylog does. This is a rule kept by review; no test inspects imports or `go.mod`.
- `config.go:76-93`: re-exports the encode sentinels from `errors/encode_errors.go`.
- `xcl-vscode:syntaxes/xcl.tmLanguage.json`: the grammar defining the scope names. Its working tree has uncommitted edits, and this plan uses the working-tree version.
- `xcl-website:src/pages/configuration-text.mdx`: the encoding page, with sections from line 23 to 180 and nothing on highlighting.

## Per-Task Technical Notes

### Task: Add the highlight package with the tokeniser and renderer interface

Requirement attribution: "Tokens use editor-standard names", "Output formats are pluggable", "Highlighting never changes the text", "All encoder output can be highlighted" (tokeniser side). All of these are in repo xclconfig. The xcl-vscode grammar is read only.

**File changes**:
- `highlight/doc.go` (new): the package comment. It explains that `highlight` labels xcl configuration text with the TextMate scopes the xcl-vscode grammar assigns and hands every piece to a `Renderer`, that the text is never changed, and that the library never checks for a terminal.
- `highlight/highlight.go` (new):
  - `type Renderer interface { Render(scope, text string) string }`, `type RendererFunc func(scope, text string) string` with its `Render` method, and `func Text(text []byte, renderer Renderer) []byte`.
  - `Text` returns the input unchanged when `renderer` is nil.
  - It also holds the exported `Scope*` constants (full names with the `.xcl` suffix) for every scope listed under the grammar in research.md.
- `highlight/tokenize.go` (new): `func tokenize(src []byte) []piece`, where `piece{scope string; text []byte}`.
  - Lex with `hclsyntax.LexConfig(src, "", hcl.InitialPos)` (`internal/xcl/hclsyntax/public.go:159`) and ignore the diagnostics. Walk the tokens with a byte cursor (`token.Range.Start.Byte` and `End.Byte`, `internal/xcl/hclsyntax/token.go:16-20`). Emit `src[cursor:start]` as an unscoped piece before each token, and the tail after the last token. Skip `TokenEOF`.
  - Labelling functions, one per grammar rule, each with a comment quoting the grammar rule it mirrors (`xcl-vscode:syntaxes/xcl.tmLanguage.json`):
    - `TokenIdent` first on its line, followed by one or two `OQuote/QuotedLit/CQuote` labels and then `OBrace`: `storage.type.xcl`. With two labels, the first label's quotes and literal are `entity.name.type.xcl` and the second's are `entity.name.tag.xcl`. With one label, it is `entity.name.tag.xcl`. A label's `"` quotes take the label scope, because the grammar captures `"[^"]*"` whole.
    - The keywords `resource|module|variable|output|local` not followed by `.`: `storage.type.xcl`, with their labels as above, even when `{` is not on the same line.
    - `TokenIdent` followed directly by `OBrace`, and not preceded by `.`: `entity.name.function.block.xcl`.
    - `TokenIdent` followed by `TokenEqual` (not `==`): `variable.other.property.xcl`.
    - `TokenIdent` followed by `OParen`: `support.function.builtin.xcl` when it is in the grammar's built-in list (copy the list verbatim into a `builtinFunctions` set), and `entity.name.function.xcl` otherwise.
    - `true|false|null`: `constant.language.xcl`.
    - `TokenNumberLit`: `constant.numeric.xcl`.
    - `TokenIdent` followed by `TokenDot` and an identifier, and not preceded by `.`: `support.class.reference.xcl`. Each `TokenDot` in the chain is `punctuation.accessor.xcl` and each later identifier is `variable.other.member.xcl`. An index `[0]` or a splat is unscoped, as the grammar has no rule for it.
    - Operator tokens (`= == != >= <= && || + - * / % < > ! ? :`): `keyword.operator.xcl`.
    - `TokenOQuote`, `TokenCQuote` and `TokenQuotedLit`: `string.quoted.double.xcl`. Split a `QuotedLit` so that `\n \r \t \" \\ \uXXXX` become `constant.character.escape.xcl`.
    - Inside a string, `TokenTemplateInterp` (`${`) is `punctuation.section.interpolation.begin.xcl`, and its matching `TokenTemplateSeqEnd` (`}`) is `punctuation.section.interpolation.end.xcl`. Tokens between them are labelled as expressions.
    - `TokenOHeredoc` (`<<-EOF\n`): split into `<<` or `<<-` as `keyword.operator.heredoc.xcl`, the marker as `keyword.control.heredoc.xcl`, and the newline unscoped. `TokenStringLit` inside a heredoc is `string.unquoted.heredoc.xcl`. Within it, `#{{ … }}` is cut out as `meta.interpolation.template.xcl`, with its `#{{`/`}}` as interpolation begin and end and `.name` as accessor and member. `TokenCHeredoc`: the marker is `keyword.control.heredoc.xcl`, and its leading spaces and trailing newline are unscoped.
    - `TokenComment`: `//` is `comment.line.double-slash.xcl`, `#` is `comment.line.number-sign.xcl` and `/*` is `comment.block.xcl`. The trailing newline the scanner includes in a line comment is split off as unscoped.
    - Everything else (`{ } [ ] ( ) ,`, newlines, `TokenInvalid`, …) is unscoped.
  - Track the brace depth only as far as needed for "first on its line". Use the token's `Range.Start.Column == 1`, or "only whitespace since the last newline token".
- `highlight/testdata/sample.xcl` (new): a copy of `xcl-vscode:test/fixtures/sample.xcl`, plus a line with escapes (`"a\tb\"c"`), a `/* */` comment on one line, and an operator expression. This keeps the package self-contained.
- `highlight/testdata/references.xcl` (new): text in the shape `ShowReferences` writes, for example `x = resource.b.one.y` and `msg = "hello ${variable.cpu} world"`.
- `highlight/tokenize_test.go` (new):
  - One test function per scope, for example `TestTextLabelsBlockTypeAsStorageType` and `TestTextLabelsFirstOfTwoLabelsAsEntityNameType`. Each uses a `[scope]text[/scope]` marker renderer over `testdata/sample.xcl` and `require.Contains` on the expected wrapped text.
  - `TestTextLabelsReferenceRootInShownReferences` uses `references.xcl`.
  - `TestTextPassesUnlabelledTextWithEmptyScope`.
- `highlight/highlight_test.go` (new):
  - `TestTextWithIdentityRendererReturnsInput`, for both testdata files.
  - `TestTextWithNilRendererReturnsInput`.
  - `FuzzTextKeepsText`, seeded from both testdata files and from a few malformed strings such as an unterminated string, a stray `${`, `<<EOF` with no end, and invalid UTF-8. It asserts that the identity join equals the input.
- `internal/testutil/ansi.go` (new): `func StripANSI(text string) string` using `\x1b\[[0-9;]*m`, the same pattern as `example/prettylog/highlight_test.go:27`. It is shared by the `highlight` tests and the root encode tests, per the shared-test-helpers convention. It is created here, because the identity tests do not need it but the next tasks do, so it lands with the package. It imports nothing new.

**Complexity**: High
**Token estimate**: ~60k tokens
**Agent strategy**: Parallel analysis, sequential integration. One agent maps every grammar rule to its scanner token pattern, by lexing `sample.xcl` with `LexConfig` and dumping the tokens, while another writes the package skeleton, `Text` and the fuzz test. Then a single agent implements the labelling functions and the per-scope tests one rule at a time.

### Task: Add the invalid-theme error

Requirement attribution: "Bad themes are reported", in repo xclconfig.

**File changes**:
- `errors/highlight_errors.go` (new):
  - `var ErrInvalidTheme = errors.New("invalid theme")`, with a doc comment saying that `highlight.NewANSIRenderer` returns it and that it is matched with `errors.Is`.
  - `type InvalidThemeError struct { Source, Reason string; Err error }` with pointer receivers. `Error()` gives `theme <Source> is invalid: <Reason>[: <Err>]`. `Unwrap() []error` returns `ErrInvalidTheme` and `Err` when it is non-nil.
  - Model it on `errors/encode_errors.go:50-79` (`InvalidSavedDataError`).
- `errors/highlight_errors_test.go` (new):
  - `TestInvalidThemeErrorMatchesSentinel`.
  - `TestInvalidThemeErrorMatchesUnderlyingError`.
  - `TestInvalidThemeErrorMessageNamesSourceAndReason`.
  - `TestInvalidThemeErrorIsRecoveredWithAs`.
- `config.go:76-93` (the encode-errors `var` block): add `ErrInvalidTheme = xclerrors.ErrInvalidTheme` with a doc comment, after `ErrNotEncodable`.

**Complexity**: Low
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Read VS Code colour themes and match scopes against them

Requirement attribution: "Editor themes work in the terminal" (theme side), "Unmatched tokens keep the default colour", "Bad themes are reported", and "Default theme follows the terminal" (rule form). All are in repo xclconfig.

**File changes**:
- `highlight/jsonc.go` (new): `func stripJSONC(src []byte) []byte`. It removes `//` line comments, `/* */` block comments and trailing commas before `]` or `}`, outside string literals and honouring `\"` escapes. Byte offsets do not need preserving.
- `highlight/theme.go` (new):
  - `type rule`, `type colour` and `type fontStyle`, as in Data Structures.
  - `type theme struct { rules []rule; cache map[string]style }`.
  - `func parseTheme(source string, r io.Reader) (*theme, error)` reads everything, runs `stripJSONC`, and decodes into `struct { Include *string; TokenColors json.RawMessage }`. Then:
    - A present `include` is rejected.
    - `tokenColors` must be an array. A string, which would be a .tmTheme path, is rejected.
    - Each entry decodes `scope` as a string or `[]string`. A string is split on `,`, each selector is trimmed, and the last space-separated element is taken. Anything with a leading `-`, or after ` -`, is dropped.
    - `settings.foreground` is parsed as `#RGB`, `#RRGGBB` or `#RRGGBBAA`.
    - `settings.fontStyle` is space-split into `bold`, `italic`, `underline` and `strikethrough`. An empty string is an explicit empty style. Any other word is an error.
    - An entry with no scope is skipped.
    - Every error is a `*xclerrors.InvalidThemeError{Source: source, Reason: "tokenColors[i]…"}`.
  - `func (t *theme) style(scope string) style` returns the best foreground and the best font style, picked independently. A selector `s` matches a scope `c` when `c == s` or `strings.HasPrefix(c, s+".")`. The best match has the most segments, and a later `order` wins a tie. The result is cached per scope.
  - `func defaultTheme() *theme` uses basic rules taken from `example/prettylog/highlight.go:42-53`:
    - `storage.type` 35 bold
    - `entity.name.type` 36
    - `entity.name.tag` 32
    - `variable.other.property` 34
    - `string` 33
    - `constant` 95
    - `support.class.reference` 36
    - `comment` 90 italic
- `highlight/theme_test.go` (new): positive and negative tests in separate functions, for example:
  - `TestThemePrefixSelectorMatchesDeeperScope`
  - `TestThemeMoreSpecificSelectorOverridesGeneral`
  - `TestThemeResolvesColourAndFontStyleIndependently`
  - `TestThemeLaterRuleWinsTie`
  - `TestThemeReadsCommaSeparatedScopes`
  - `TestThemeReadsScopeArray`
  - `TestThemeMatchesDescendantSelectorOnLastElement`
  - `TestThemeLoadsCommentsAndTrailingCommas`
  - `TestThemeLeavesUnmatchedScopeUnstyled`
  - `TestThemeIgnoresEntryWithoutScope`
  - `TestParseThemeRejectsMalformedJSON`
  - `TestParseThemeRejectsMalformedColour`
  - `TestParseThemeRejectsUnknownFontStyle`
  - `TestParseThemeRejectsTokenColorsPath`
  - `TestParseThemeRejectsInclude`
  
  Each negative test asserts `errors.Is(err, xcl.ErrInvalidTheme)`. Since `highlight` cannot import root `xcl`, assert against `xclerrors.ErrInvalidTheme` (the same value) and `errors.As` to `*xclerrors.InvalidThemeError`.
- `highlight/testdata/reference-theme.json` (new): a small VS Code theme (JSONC) with one rule per default-theme scope family, a deliberately general `entity.name` rule plus a specific `entity.name.tag` override, bold, italic and underline styles, and no rule for `keyword.operator`, which is used by the unmatched test.

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: 2-3 parallel agents for independent changes: one for the JSONC stripper and its tests, one for theme parsing and validation, and one for matching and the default theme. A final pass integrates them.

### Task: Add the ANSI terminal renderer

Requirement attribution: "Terminal colour ships built in", "Default theme follows the terminal", "Editor themes work in the terminal", "Unmatched tokens keep the default colour" and "Bad themes are reported" (constructor). All are in repo xclconfig.

**File changes**:
- `highlight/ansi.go` (new):
  - `type ansiOptions struct { source string; reader io.Reader; path string }` and `type ANSIOption func(*ansiOptions)`.
  - `WithTheme(r io.Reader)` sets the source to "theme". `WithThemeFile(path string)` sets the source to the path. Reading the file happens in the constructor, and an `os.Open` error becomes `*InvalidThemeError{Source: path, Reason: "cannot be read", Err: err}`.
  - `NewANSIRenderer(options ...ANSIOption) (*ANSIRenderer, error)` uses `defaultTheme()` when no theme is given. On any error it returns a nil renderer.
  - `(*ANSIRenderer).Render(scope, text string) string`. It returns the text unchanged when `scope == ""` or when the style is empty. Otherwise it builds SGR parameters:
    - basic colour `30–37` or `90–97`, or truecolour `38;2;r;g;b`
    - `1` bold, `3` italic, `4` underline, `9` strikethrough
    - It splits the text on `\n` and wraps each non-empty segment as `\x1b[<params>m` + segment + `\x1b[0m`, leaving the newlines between them bare.
  - Use only `strconv` and `strings`.
- `highlight/ansi_test.go` (new):
  - `TestANSIRendererDefaultUsesOnlyBasicColours`. Over `testdata/sample.xcl`, every SGR parameter list contains only 0, 1, 3, 4, 9, 30–37 and 90–97, never `38`, and at least one colour code appears.
  - `TestANSIRendererThemeColoursEachTokenKind`. With `testdata/reference-theme.json`, it asserts the exact 24-bit sequence for each kind.
  - `TestANSIRendererThemeAppliesFontStyles`.
  - `TestANSIRendererSpecificRuleOverridesGeneral`.
  - `TestANSIRendererLeavesUnmatchedTokensUncoloured`. Operators carry no codes.
  - `TestANSIRendererLeavesUnlabelledTextUncoloured`.
  - `TestANSIRendererStylesMultiLineTokenPerLine`.
  - `TestANSIRendererOutputStripsToInput`, using `testutil.StripANSI`.
  - `TestNewANSIRendererFailsForUnreadableThemeFile`.
  - `TestNewANSIRendererFailsForInvalidTheme`.
  - `TestNewANSIRendererReadsThemeFromReader`.
  - `TestNewANSIRendererReadsThemeFromFile`.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add the Highlight encode option

Requirement attribution: "Highlighting is opt-in" and "All encoder output can be highlighted", plus the constraint "Existing encoding is unchanged". All are in repo xclconfig.

**File changes**:
- `encode.go:29-34`: add `renderer highlight.Renderer` to `encodeOptions`.
- `encode.go:36-39`: update the `EncodeOption` doc comment so it lists `Highlight` too.
- `encode.go:~103` (after `ShowReferences`): add `func Highlight(renderer highlight.Renderer) EncodeOption`. Its doc comment says:
  - Each token is passed to the renderer, labelled with the xcl-vscode scope.
  - The text is unchanged apart from what the renderer adds.
  - Highlighting combines with every other option, including `ShowReferences`.
  - xcl never checks for a terminal, so the caller decides when to ask for colour.
  - It names `highlight.NewANSIRenderer` for terminals.
  - A nil renderer means no highlighting.
- `encode.go:226-230` (end of `encodeEntity`): take `text := file.Bytes()`, and when `opts.renderer != nil`, `text = highlight.Text(text, opts.renderer)`. Then return it.
- `encode.go:3-21` imports: add `github.com/jumppad-labs/xcl/highlight`.
- `encode_highlight_test.go` (new, root package): reuse the encode fixture setup from `encode_test.go:60` (`internal/test_fixtures/config/encode/main.xcl`) and its helpers. Tests:
  - `TestEncodeEntityWithoutHighlightHasNoColourCodes`
  - `TestEncodeEntityWithHighlightPassesEveryTokenToRenderer` (marker renderer)
  - `TestEncodeEntityWithHighlightStripsToPlainText`
  - `TestEncodeSavedEntityWithHighlightStripsToPlainText`
  - `TestEncodeEntityWithHighlightAndShowReferencesLabelsReferenceRoots`
  - `TestEncodeEntityWithNilRendererMatchesPlainText`

  The stripping tests use the marker renderer and strip its markers. The ANSI path through the encoder is covered by the prettylog task.

  The existing `encode_test.go`, `encode_references_test.go` and `encode_sensitive_test.go` must pass unchanged.
- No dependency test is added or relied on. Review confirms that `highlight` imports only the standard library, `internal/xcl/hclsyntax` and the `errors` package.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Switch the logging example to library highlighting

Requirement attribution: "The logging example uses it" and the success metric. Both are in repo xclconfig. Note the sibling spec `20261006071142-506b8289-e2e-suite-and-real-world-examples` also edits `example/prettylog/`. If prettylog has become its own module by then, make these changes in that module, and make sure it resolves an xcl that contains `highlight`, through its development `replace` or a published version. If it pins a published xcl without `highlight`, STOP and ask.

**File changes**:
- `example/prettylog/highlight.go`: delete.
- `example/prettylog/highlight_test.go`: delete.
- `example/prettylog/prettylog.go:56-80` (`Handler`):
  - Replace `h := newHighlighter(lipgloss.NewRenderer(w))` with a colour decision, `colour := lipgloss.NewRenderer(w).ColorProfile() != termenv.Ascii`.
  - When `colour` is true, build `renderer, _ := highlight.NewANSIRenderer()`. The default theme cannot fail, so assert that with a comment and handle the error by falling back to plain.
  - Pass the encode options `[]xcl.EncodeOption{xcl.IncludeComputed()}`, plus `xcl.Highlight(renderer)` when colour is on, into `writeConfiguration`.
  - Update the package and `Handler` doc comments (`prettylog.go:1-10`, `33-48`) to say the configuration is coloured by xcl's `highlight` package.
- `example/prettylog/prettylog.go:85-110` (`writeConfiguration`): drop the `highlighter` parameter, take the options, call `xcl.EncodeSavedEntity(reg, e.Data, options...)`, and write `indent(string(text))`. Indenting after highlighting is safe, because the ANSI renderer leaves newlines outside escape codes.
- `example/prettylog/prettylog_test.go`:
  - Add `TestHandlerColoursConfigurationWhenColourIsForced`. It uses `t.Setenv("CLICOLOR_FORCE", "1")`, which lipgloss honours through `termenv.EnvColorProfile`, and the real apply helper `applyEncodeFixture` (`prettylog_test.go:234`). It asserts that the configuration lines contain SGR codes and that stripping them with the local `ansiCodes` (`prettylog_test.go:24`) gives the plain encoding.
  - Add `TestHandlerWritesPlainConfigurationToNonTerminal`. It asserts no codes are written to a `bytes.Buffer` with no forcing.
  - Keep the existing tests passing.
  - prettylog keeps its local strip regex rather than `internal/testutil`, because it is becoming a self-contained module that cannot import `internal/`.
- prettylog keeps its existing charmbracelet dependencies; no test checks them.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Document highlighting on the website encoding page

Requirement attribution: "Highlighting is documented", in repo xcl-website.

**File changes**:
- `xcl-website:src/pages/configuration-text.mdx:180`: insert a `## Highlighting` section before `## For reading, not reprocessing`. It covers:
  1. Turning highlighting on: `xcl.EncodeEntity(entity, xcl.Highlight(renderer))`, with `renderer, err := highlight.NewANSIRenderer()`. The default theme uses the terminal's 16 colours. xcl never checks for a terminal, so show a caller deciding, for example with an `isTerminal` check in application code.
  2. Using a theme: `highlight.NewANSIRenderer(highlight.WithThemeFile("dark-plus.json"))`. Explain that it reads VS Code colour theme JSON, comments allowed, that tokens the theme does not cover keep the default colour, and that a bad theme returns `xcl.ErrInvalidTheme` from the constructor (`errors.Is` example).
  3. Writing a renderer: a `highlight.RendererFunc` that wraps tokens, for example `<span class="…">`-style markers. It notes that every byte reaches the renderer, unlabelled text with scope `""`, and that scope names are the xcl-vscode TextMate scopes, for example `variable.other.property.xcl`, exported as `highlight.Scope*` constants.
  
  It mentions that highlighting combines with `ShowReferences`. Follow the existing page style: `<Prose>` wrapper, fenced `go` blocks rendered by expressive-code, and short paragraphs.
- `xcl-website:src/pages/configuration-text.mdx:3-22` (Hero/intro): add one sentence mentioning highlighting, if the intro lists the page's topics.

**Complexity**: Low
**Token estimate**: ~10k tokens
**Agent strategy**: Single agent, sequential execution. Confirm the final API names against `highlight/*.go` before writing, and build the site to check it.

## Testing Strategy

The per-task test lists are in each task's section above. Across the plan:

- **Add the highlight package with the tokeniser and renderer interface.** One parity test per grammar scope over `highlight/testdata/sample.xcl`, a reference-root test over `highlight/testdata/references.xcl`, identity and nil-renderer tests, and `FuzzTextKeepsText`. These carry the "standard token names", "custom renderer" and "text unchanged" criteria.
- **Add the invalid-theme error.** Detail-type tests in `errors/highlight_errors_test.go`, covering `Is` against the sentinel and against the cause, `As`, and the message.
- **Read VS Code colour themes and match scopes against them.** Matching tests and one negative test per invalid-theme cause in `highlight/theme_test.go`.
- **Add the ANSI terminal renderer.** Default-palette, reference-theme, font-style, override, unmatched, unlabelled, multi-line and strip-to-input tests, plus constructor failure and success tests, in `highlight/ansi_test.go`.
- **Add the Highlight encode option.** Encoder integration tests in `encode_highlight_test.go`, against the real encode fixture and a real apply. The existing encode tests must pass unchanged, which guards "Existing encoding is unchanged".
- **Switch the logging example to library highlighting.** Forced-colour and plain-writer handler tests in `example/prettylog/prettylog_test.go`. Review confirms the library gained no charmbracelet dependency.
- **Document highlighting on the website encoding page.** No automated test. The site build and a manual review go in the test plan.

Every test follows the project conventions: testify `require`, no table-driven tests, positive and negative cases in separate functions, and each test file beside the code it tests.

Manual items for the implementation test plan:
- A sweep of every `.xcl` file in the examples and test fixtures, checking that the stripped output equals the input.
- VS Code "Inspect Editor Tokens and Scopes" parity spot-check.
- The logging example in a real terminal, with the default and a dark theme.
- Review of the website section.
- Confirming that no other example prints configuration with its own highlighter.
- Review confirming that the library gains no charmbracelet or other new module dependency.

## Project References

- Spec: `20261006112108-17623cda-encoder-syntax-highlighting` (epic `20261006071139-7b266535-examples-and-output`, source issue jumppad-labs/xcl#6).
- Design documents: none referenced by the spec.
- Knowledge entries (xclconfig, conventions): `testing-and-mocking.md`, `shared-test-helpers.md`, `shared-errors-package.md`, `dependencies.md`, `code-style.md`, `project-structure.md`, `never-modify-dependencies.md`.
- Repo roots:
  - xclconfig: `/home/nicj/code/github.com/jumppad-labs/xcl`, where the code changes land.
  - xcl-website: `/home/nicj/code/github.com/jumppad-labs/xcl-website`, where the docs change lands.
  - xcl-vscode: `/home/nicj/code/github.com/jumppad-labs/xcl-vscode`, the grammar reference, read only.
- Requirement-to-repo resolution:
  - Opt-in, standard names, pluggable renderers, terminal renderer, default theme, editor themes, unmatched tokens, bad themes, all encoder output, and text unchanged: xclconfig (`highlight/`, `encode.go`, `errors/`, `config.go`).
  - Logging example: xclconfig (`example/prettylog/`).
  - Documented: xcl-website (`src/pages/configuration-text.mdx`).
- Prior plan precedent: `20261003153421-9fa72edd-masking`, for a top-level public package with errors re-exported from `xcl`.
- Sibling spec: `20261006071142-506b8289-e2e-suite-and-real-world-examples`, which also edits `example/prettylog/`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The only High task is the tokeniser. Split its analysis, which maps scanner tokens to grammar rules, from the implementation, and implement the labelling one rule at a time with its test.

## Migration Notes

No data or API migration is needed, because the change is purely additive and existing encode output is unchanged. prettylog's private `highlighter` type and its tests are deleted, not migrated. If the sibling e2e spec has moved prettylog into its own module first, apply the prettylog change there; see the open question in plan.md.

## Performance Considerations

Highlighting runs only when asked for, and costs one extra lex of text that is already formatted, which is small: an entity's configuration is usually a few hundred bytes. Theme parsing happens once, in the constructor. The ANSI renderer caches the resolved style per scope, so rendering does no theme matching per token after the first time a scope is seen. No allocations are added to the plain encode path.
