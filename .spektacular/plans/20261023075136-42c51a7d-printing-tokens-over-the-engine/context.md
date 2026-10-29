---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Context: 20261023075136-42c51a7d-printing-tokens-over-the-engine

## Current State Analysis

- `proto/xcl/core/v1/core.proto:1-329` defines CoreService. `Error` has code, message, details and causes (`:57-66`). `OperationFinished` carries `diff_json = 3` (`:245-249`). `EncodeSavedEntityRequest` and `EncodeEntityRequest` carry `bool highlight = 2` (`:306-325`), and both responses carry only `text = 1`. There is no token, kind or theme message.
- `coreservice/errors.go:84-331` `toProto` converts every outgoing error. It is called from `service.go:96,102` (call results), `operations.go:61` (finished task) and `events.go:30` (event errors). A diagnostic's parts already cross as `details` (severity, summary, detail, entity, field, file, line, column, end_line, end_column, source_start, source), and a `ConfigError`'s problems cross as `causes` (`:127-131`).
- `coreservice/lookups.go:121-197`: both encode handlers apply `colour(text, req.GetHighlight())`. After the printing-core plan lands, `colour` formats `highlight.Tokenize(text)` with `highlight.Terminal` and `highlight.DefaultTheme`.
- `coreservice/operations.go:29-66`: `start(work func(ctx) (diffJSON []byte, err error))`. `startDiff` (`:135-152`) marshals the diff inside `work`.
- `coreservice/service.go:205-227`: the handler table. `GetVersion` and `CreateConfig` are the handle-0 methods. `service_test.go:299-315` holds the table to the descriptor.
- `proto/buf.yaml:19-21`: `breaking: use: [FILE]`. Reserving a removed field fails under it (probe in research.md).
- `.github/workflows/go.yml:30-55`: CI runs `buf lint`, and on pull requests `buf breaking` against main (buf 1.50.0).
- After the printing-core plan: `xcl.ErrorTokens`, `xcl.PrintError`, `xcl.PrintDiff`, `highlight.Kind`/`Kinds()`/`Parent()`/`String()`, `highlight.Token`, `highlight.Tokenize`, `(*diff.Diff).Tokens()`, `highlight.LoadVSCodeTheme`, `highlight.Plain`/`Terminal`/`DefaultTheme`.

Requirement → repo and files:
- Every error carries its report; Reports match Go's; Errors without a configuration problem → xcl: `coreservice/errors.go`, `proto/xcl/core/v1/core.proto` (`Error.report`), tests in `coreservice/errors_test.go`, `coreservice/operations_test.go`, `coreservice/lookups_test.go`.
- Rendered resources come as labelled text → xcl: `coreservice/lookups.go`, `core.proto` (encode messages), `coreservice/lookups_test.go`.
- Change summaries come as labelled text → xcl: `coreservice/operations.go`, `core.proto` (`OperationFinished`), `coreservice/operations_test.go`.
- Editor themes convert without a configuration → xcl: `coreservice/theme.go`, `coreservice/service.go`, `coreservice/errors.go`, `core.proto`, `coreservice/theme_test.go`.
- Kinds agree across languages → xcl: `core.proto` (`TokenKind`), `coreservice/tokens.go`, `coreservice/tokens_test.go`.
- Existing callers keep working → xcl: `core.proto` (reserved field), `proto/buf.yaml`, `.github/workflows/go.yml` (comment), `coreservice/lookups.go`, `coreservice/lookups_test.go`.
- Protocol documentation → xcl: `core.proto` comments, `design:libxcl.md` (through the design CLI), `CHANGELOG.md`.

## Per-Task Technical Notes

### Task: Add token kinds, labelled text and the error report to the protocol

Requirement attribution: "Kinds agree across languages", plus the groundwork for "Every error carries its report". Constraints: the protocol changes additively; lexing stays in the core. Repo xcl. Prerequisite: plan `20261023074926-eaa51527-printing-and-highlighting-core` has landed (`highlight.Kind`, `Kinds()`, `Token`, `Terminal`, `DefaultTheme`).

File changes:
- `proto/xcl/core/v1/core.proto:54-66`: after `Error`, add the `TokenKind` enum with every value from the design's table. Each name is `TOKEN_KIND_` plus the upper snake case of the Go `Kind.String()` name, and each number is the Go number (see plan.md Data Structures). Comment it: "TokenKind labels a piece of text. Values are numbered in blocks of 100; a value's category is `kind / 100 * 100`, so a value a binding does not know still falls into its category. Numbers are Go's highlight.Kind and are never renumbered." Add `message Token { TokenKind kind = 1; string text = 2; }` with the comment "a piece of labelled text; the texts joined in order are the plain text". Add `repeated Token report = 5;` to `Error` with the comment: "report is the error's report as Go prints it with xcl.PrintError, labelled, laid out once and unwrapped (each language wraps the detail when it prints). It is set when the error holds a configuration problem, on this error and on each cause that holds one, and empty otherwise."
- `coreservice/proto/core.pb.go`: regenerate with `make core-protos` (`buf generate`). Never edit by hand.
- `coreservice/tokens.go` (new): `func tokensToProto(tokens []highlight.Token) []*corepb.Token`, which returns nil for an empty input and otherwise `&corepb.Token{Kind: corepb.TokenKind(token.Kind), Text: token.Text}` per token, in order. Add a doc comment saying kind numbers cross unchanged.
- `coreservice/tokens_test.go` (new), one behaviour per function, package `coreservice`:
  - `TestTokenKindsMatchGoKinds`: for every `k` in `highlight.Kinds()`, `corepb.File_xcl_core_v1_core_proto.Enums().ByName("TokenKind").Values().ByNumber(protoreflect.EnumNumber(k))` is non-nil and its name equals `"TOKEN_KIND_" + upperSnake(k.String())`. Write `upperSnake` as a local helper that inserts `_` before each interior upper-case letter and upper-cases the result.
  - `TestEveryTokenKindIsAGoKind`: every enum value's number is in `highlight.Kinds()`.
  - `TestTokenKindCategoryMatchesGoParent`: for every Go kind, `int32(corepb.TokenKind(k)) / 100 * 100 == int32(k.Parent())`.
  - `TestTokensToProtoKeepsKindAndText`: `[{Attribute,"image"},{Text," = "},{String,"\"nginx\""}]` converts to the same numbers and texts.
  - `TestTokensToProtoEmptyIsNil`.
  - `TestUnknownTokenKindSurvivesTheWire`: marshal `&corepb.Token{Kind: corepb.TokenKind(1199), Text: "# x"}`, unmarshal it into a new `corepb.Token`, and require `int32(GetKind()) == 1199`.
  - `TestUnknownTokenKindPrintsInItsCategoryStyle`: `highlight.Kind(1199).Parent() == highlight.Comment`, and `highlight.Terminal` with `highlight.DefaultTheme` formats `{Kind(1199), "# x"}` to the same bytes as `{Comment, "# x"}`. Go is the reference behaviour other languages are held to.
- Run `buf lint` and `make buf-breaking`. Both must be clean, since this change is purely additive.

Complexity: Medium
Token estimate: ~20k tokens
Agent strategy: Single agent, sequential execution: proto, generate, helper, tests.

### Task: Attach the full report to every error that holds a configuration problem

Requirement attribution: "Every error carries its report", "Reports match Go's", "Errors without a configuration problem". Constraint: the report is sent unwrapped and laid out in the core. Technical approach: lay the report out once, when an error is converted. Success metric: printing with zero extra engine calls. Repo xcl.

File changes:
- `coreservice/errors.go:84-90`: after building `out`, and before the code switch or after it (order is irrelevant), add:
  `if holdsProblem(err) { out.Report = tokensToProto(xcl.ErrorTokens(err)) }`
  with `func holdsProblem(err error) bool { var d *xclerrors.Diagnostic; return errors.As(err, &d) }`. Document that a diagnostic anywhere in the tree counts, through `ConfigError.Unwrap() []error`, `Diagnostics.Unwrap() []error` and `%w` wrapping, so a provider failure reported at its block (code `provider`) is covered. Update the `toProto` doc (`errors.go:81-83`) to mention the report. Causes already recurse through `toProto` (`errors.go:127-131`), so each diagnostic cause gets its own report.
- `coreservice/events.go:30`, `coreservice/operations.go:61`, `coreservice/service.go:96,102`: no change. They already call `toProto`.
- `coreservice/errors_test.go` (add after `:353`), unit tests:
  - `TestErrorReportOnDiagnostic`: `toProto(unsupportedDiagnostic(hcl.DiagError))` has a non-empty report. Its joined text equals the output of `xcl.PrintError(&buf, d, xcl.WithWidth(0))`, its first token is `TOKEN_KIND_ERROR` "Error", and it holds a `TOKEN_KIND_GUTTER` token.
  - `TestErrorReportOnConfigErrorAndEachCause`: a `ConfigError` with two diagnostics. The top-level report joins to `PrintError(..., WithWidth(0))` of the whole error, and each cause's report joins to that diagnostic's own plain report.
  - `TestErrorReportOnProviderDiagnostic`: reuse the `TestErrorCodeProviderForADiagnosticCausedByAProvider` setup (`:304-317`). The report is non-empty and the code is still `provider`.
  - `TestErrorReportDetailIsUnwrapped`: a diagnostic with a detail over 120 characters. Its `TOKEN_KIND_DETAIL` token holds no newline.
  - `TestNoErrorReportOnNotFound`: `toProto(&xclerrors.NotFoundError{Address: "app.nope"})` has an empty report and the same message as `Error()`.
  - `TestNoErrorReportOnInternal`: `toProto(errors.New("x"))` has an empty report.
- `coreservice/operations_test.go` (new tests after `:932`), integration through `Call`:
  - Add the fixture `unsupportedBlockConfig`, an `app "web"` of nine or more lines with an unsupported nested block `bogus { }` on line 7 and the other lines filled with comments or attributes (`image = "nginx"`), so the report shows two lines of context on each side.
  - `TestApplyErrorCarriesGoReport`: create a config with `hostRequest(t)`, write the fixture once with `writeConfig`, and `StartApply` and `drive` with `&fakeStore{}`. Then build a Go config the way `TestApplyMatchesGoApply` does (`:296-335`, `registry.TypeFromSchema([]byte(appSchema))` on a local registry, `xcl.WithStatePath(t.TempDir())`), apply the same file path, and `xcl.PrintError(&want, goErr, xcl.WithWidth(0))`. Require `finished.GetError().GetCode() == CodeConfig` and the joined `finished.GetError().GetReport()` texts equal to `want.String()`.
  - `TestApplyErrorEventCarriesTheSameReport`: the same run, collecting events in `onEvent`. The `apply/error` event (`eventName` = `"apply/error"`, emitted by `config.go:658-666`) has a report whose joined text equals the finished error's.
  - `TestValidateErrorEventsCarryEachProblemsReport`: with the same fixture, every event whose `Error` holds `details["summary"]` has a non-empty report containing that summary.
  - `TestHostPrintsReportWithoutAnotherCall` (success metric): wrap the test client so it counts `Call`s (a local counter incremented in a small helper that calls `c.s.Call`). Drive the failing apply. Record the count when the finished task arrives. Convert the received report back with `highlight.Token{Kind: highlight.Kind(t.GetKind()), Text: t.GetText()}` and format it with `highlight.Plain`. Require that the output holds "Error: Unsupported block type", the location line naming the file and line 7, and gutters for lines 5 to 9. Require that the call count did not change after the finished task.
- `coreservice/lookups_test.go` (after `:83`): `TestFindUnknownAddressHasNoReport`. `c.fail(... "Find" ... "app.nope")` has an empty report, `CodeNotFound`, and the message `xcl.NotFoundError{Address: "app.nope"}.Error()`.
- `proto/xcl/core/v1/core.proto`: the report comment is written in the previous task. Check that it matches the behaviour.

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: 2-3 parallel agents: one writes the `toProto` change and the unit tests, one writes the operation and lookup integration tests. Then integrate and run `go test ./coreservice/... -race`.

### Task: Return labelled resource text and retire the colour request

Requirement attribution: "Rendered resources come as labelled text", "Existing callers keep working". Constraints: additive protocol, the legacy field reserved and never reused, the breaking-change check passes. Repo xcl.

File changes:
- `proto/xcl/core/v1/core.proto:306-329`: in `EncodeSavedEntityRequest` and `EncodeEntityRequest`, replace `bool highlight = 2;` with `reserved 2;` and `reserved "highlight";`, commented "2 was highlight, terminal colour applied by the engine; a request that still sets it is answered with plain text". Add `bool tokens = 4;` with the comment "tokens asks for the text labelled as well, in the response's tokens". In both responses, add `repeated Token tokens = 2;` with the comment "the text labelled by the core's lexer when the request asked; the texts joined are text". Update the `EncodeEntity` rpc comment (`:48-50`) if it mentions colour.
- `proto/buf.yaml:1-4,19-21`: set `breaking: use: [FILE, FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED, FIELD_NO_DELETE_UNLESS_NAME_RESERVED]` and `except: [FIELD_NO_DELETE]`. Comment it: "a field may be removed only by reserving both its number and its name, so neither is reused". Rewrite the header comment's "removes a field or method" to "removes a method, or a field without reserving its number and name".
- `.github/workflows/go.yml:33-35`: align the comment ("refuses any change that renumbers, retypes or removes a field without reserving it, or removes a method"). The steps are unchanged.
- `proto/xcl/core/v1/core.proto:7-10`: the header contract comment says field numbers and names are reserved when a field is retired, never reused.
- `coreservice/proto/core.pb.go`: regenerate with `make core-protos`.
- `coreservice/lookups.go:121-152`: both handlers build the response with `Text: text`. When `req.GetTokens()` is set, they also set `Tokens: tokensToProto(highlight.Tokenize(text))`. Delete `colour` (`:189-197`) and update the handler doc comments (`:121-123`, `:137-140`) so they no longer mention colouring. Keep the `highlight` import (for `Tokenize`). Leave `encodeAllowed` alone, because the sibling plan cc23fc08 changes it.
- `coreservice/lookups_test.go`: delete `TestEncodeSavedEntityHighlightAddsColour` (`:167-176`), `TestEncodeEntityHighlightAddsColour` (`:424-434`), `TestEncodeSavedEntityHighlightStripsToPlainText` (`:436-446`) and `TestEncodeEntityHighlightMatchesSavedRecord` (`:448-458`). Delete `ansiEscape`/`stripColour` (`:186-193`) if nothing else uses them, otherwise keep them for the plain-text assertion. Add:
  - `TestEncodeSavedEntityReturnsTokensJoiningToText`: `appliedApp`, the `appRecord` with `Tokens: true`. Tokens are non-empty and their joined text equals `GetText()`. A `TOKEN_KIND_BLOCK_TYPE` token "app" is present.
  - `TestEncodeEntityReturnsTokensForLookedUpRecord`: the same with `c.find(handle, "app.web")`.
  - `TestEncodeEntityReturnsTokensForEditedRecord`: unmarshal the found record into `map[string]any`, set `image` to `"caddy"`, marshal it, and encode with `Tokens: true`. The joined tokens equal the text, and the text holds `"caddy"`.
  - `TestEncodeSavedEntityWithoutTokensReturnsNone` and `TestEncodeEntityWithoutTokensReturnsNone`.
  - `TestEncodeSavedEntityOldColourRequestReturnsPlainText`: build the request bytes as an older caller would. Marshal `&corepb.EncodeSavedEntityRequest{Entity: record}`, then append `protowire.AppendTag(nil, 2, protowire.VarintType)` and `protowire.AppendVarint(..., 1)`. Call `c.s.Call(handle, "EncodeSavedEntity", body)` and require `StatusOK`, text equal to the plain encode, and no `"\x1b["`.
  - `TestEncodeEntityOldColourRequestReturnsPlainText`: the same for `EncodeEntity` with the found record.
- Run `buf lint` and `make buf-breaking` (against `main`). Both must be clean. To be sure the rule still bites, check once by hand in a scratch copy that dropping `reserved "highlight";` makes `buf breaking` fail. That check is not a committed test.

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: Single agent, sequential execution: proto and buf rules, generate, handlers, tests, buf checks.

### Task: Send labelled change summaries with a finished diff

Requirement attribution: "Change summaries come as labelled text". Repo xcl.

File changes:
- `proto/xcl/core/v1/core.proto:245-249`: add `repeated Token diff_tokens = 4;` to `OperationFinished`, commented "StartDiff: the change summary Go prints with xcl.PrintDiff, labelled, for the same diff as diff_json".
- `coreservice/proto/core.pb.go`: regenerate.
- `coreservice/operations.go:25-66`: change `start`'s parameter to `work func(ctx context.Context) (*diff.Diff, error)`. In the goroutine, after `work` returns, build the finished task. If `found != nil`, set `diffJSON, err2 := json.Marshal(found)`. If that fails and `err == nil`, set `err = err2`; otherwise set `DiffJson` and `DiffTokens: tokensToProto(found.Tokens())`. Update the doc comment.
- `coreservice/operations.go:113-162`: `startValidate`, `startApply`, `startDestroy` and `startLoad` return `nil, conf.cfg.XContext(...)`. `startDiff` returns `conf.cfg.DiffContext(ctx, req.GetPaths(), options...)` directly, removing its own `json.Marshal`.
- `coreservice/operations_test.go` (after `:654`):
  - `TestDiffFinishedCarriesDiffTokens`: `applyThenDiffNetwork`. `GetDiffTokens()` is non-empty and holds a `TOKEN_KIND_CHANGED` token.
  - `TestDiffTokensMatchGoChangeSummary`: as `TestDiffMatchesGoDiff` (`:635-654`), build the Go diff directly and `xcl.PrintDiff(&want, direct)`. The joined `GetDiffTokens()` texts equal `want.String()`.
  - `TestApplyFinishedCarriesNoDiffTokens`: a successful apply's finished task has empty `diff_json` and `diff_tokens`.
  - The existing `TestDiffFinishedCarriesDiffJSON` and `TestDiffMatchesGoDiff` pass unchanged.

Complexity: Low
Token estimate: ~15k tokens
Agent strategy: Single agent, sequential execution.

### Task: Convert editor themes without a configuration

Requirement attribution: "Editor themes convert without a configuration". Technical approach: reuse the core's TextMate selector rules. Repo xcl.

File changes:
- `proto/xcl/core/v1/core.proto:17-24`: add `rpc ConvertTheme(ConvertThemeRequest) returns (ConvertThemeResponse);` under GetVersion and CreateConfig, commented "ConvertTheme turns a VS Code colour theme into a style string per TokenKind, through the core's TextMate selector rules. It takes handle 0: no configuration is needed." Add the messages after `GetVersionResponse`: `ConvertThemeRequest { bytes vscode_theme_json = 1; }`, commented "the theme file's bytes; comments and trailing commas are allowed", and `ConvertThemeResponse { map<int32, string> styles = 1; }`, commented "TokenKind number to style string, such as \"bold #ff8800\"; only kinds the theme styles appear, and every other kind keeps the language's default".
- `coreservice/proto/core.pb.go`: regenerate.
- `coreservice/errors.go:14-44`: add `CodeInvalidTheme = "invalid_theme"`. In `toProto`, add a case before the sentinel cases: `case errors.As(err, &invalidTheme)` (`*xclerrors.InvalidThemeError`) sets the code and details `source` and `reason`. Add a fallback `case errors.Is(err, xclerrors.ErrInvalidTheme)`.
- `coreservice/theme.go` (new): `func (s *Service) convertTheme(_ *configuration, req *corepb.ConvertThemeRequest) (*corepb.ConvertThemeResponse, error)`. An empty `vscode_theme_json` returns `invalid_argument("a theme is required", "argument", "vscode_theme_json")`. Otherwise call `highlight.LoadVSCodeTheme(bytes.NewReader(...))`, return its error as is, and build `styles[int32(kind)] = style` for each entry.
- `coreservice/service.go:205-209`: add `"ConvertTheme": unary(false, (*Service).convertTheme),`. The descriptor tests (`service_test.go:299-315`) then pass.
- `coreservice/theme_test.go` (new):
  - `TestConvertThemeColoursAttributeOrange`: a theme JSON whose `tokenColors` sets scope `variable.other.property` to foreground `#ff8800`. Call it on handle 0 with no `CreateConfig` made. `styles[300]` contains `#ff8800`.
  - `TestConvertThemeAcceptsComments`: the same theme with a `//` comment and a trailing comma.
  - `TestConvertThemeInvalidThemeIsInvalidTheme`: a foreground of `"#zz0000"` gives `CodeInvalidTheme`, a non-empty `reason` and `source` `"theme"`.
  - `TestConvertThemeNotJSONIsInvalidTheme`.
  - `TestConvertThemeEmptyIsInvalidArgument`.
  - `TestConvertThemeWithConfigHandleIsMismatch`: create a config, then call ConvertTheme with its handle and get `StatusHandleMismatch`.
- `coreservice/errors_test.go`: `TestErrorCodeInvalidTheme`. `toProto(&xclerrors.InvalidThemeError{Source: "theme", Reason: "r"})` gives the code and both details.

Complexity: Low
Token estimate: ~15k tokens
Agent strategy: Single agent, sequential execution.

### Task: Describe reports, labelled text and theme conversion in the other-language design

Requirement attribution: "Protocol documentation" (design half). Repo xcl (design source `design`, reached only through the CLI).

File changes:
- `.spektacular/design/libxcl.md` (reach it only through the CLI, never with file tools): read it with `spektacular design read --data '{"source":"design","path":"libxcl.md"}' > .spektacular/tmp/libxcl.md`, edit the staged copy, and write it back with `spektacular design author --data '{"source":"design","path":"libxcl.md"}' --spec 20261023075136-42c51a7d-printing-tokens-over-the-engine --from .spektacular/tmp/libxcl.md`. Remove the staged file afterwards. Keep the document's status. If the sibling plan cc23fc08 has already revised it, build on that version.
  - "Error model" (around design lines 143-191): add `repeated Token report = 5;` to the `Error` block, and a paragraph saying the report is laid out once, when the error is converted, unwrapped, present on every error and cause holding a configuration problem, empty otherwise, and printable with no engine call, including from an event handler. Add `invalid_theme` (`*errors.InvalidThemeError`, `source`, `reason`) to the code table.
  - "The CoreService" method list (around 203-249): add `rpc ConvertTheme` under Library with "handle 0".
  - "Polling" `OperationFinished` block (around 393-397): add `diff_tokens = 4`.
  - "Lookups" encode blocks and paragraph (around 475-491): show `reserved 2; reserved "highlight";`, `bool tokens = 4` and `repeated Token tokens = 2`. Replace the sentence about `highlight` applying terminal colouring with labelled text on request, with old colour requests answered with plain text.
  - Add a short "Labelled text" subsection after Lookups. It covers `TokenKind` (numbered in blocks of 100, the category by arithmetic, open enum so unknown values still fall into their category, numbers mirrored from Go's `highlight.Kind` and held by a test), `Token`, `ConvertTheme`, and a pointer to `printing-and-highlighting.md` for formatting in each language.
- `proto/xcl/core/v1/core.proto`: re-read the comments written in the earlier tasks against the design, and correct any mismatch.

Complexity: Low
Token estimate: ~15k tokens
Agent strategy: Single agent, sequential execution.

### Task: Write the changelog entry for labelled output over the engine

Requirement attribution: release notes (the project's changelog rule). Repo xcl.

File changes:
- `CHANGELOG.md:1-3`: insert a new top section `## 20261023075136-42c51a7d-printing-tokens-over-the-engine` after `# Changelog`. Its prose covers: every error a host receives carries `report`, Go's report as labelled tokens, unwrapped, on each cause too and on event errors mid-operation, for errors holding a configuration problem only; the `TokenKind` enum and `Token`, with kinds numbered as Go's and categories by arithmetic; `tokens` on `EncodeSavedEntity` and `EncodeEntity`; `diff_tokens` on a finished diff; the handle-0 `ConvertTheme` call; the new `invalid_theme` code; and the breaking-change rule now allowing a field's removal only when its number and name are reserved. Then `**Breaking:**` with:
  - `highlight` (field 2) on `EncodeSavedEntityRequest` and `EncodeEntityRequest` is reserved. A request that still sets it gets plain text. Ask for `tokens` and format them in the language.
  - The generated Go `corepb.EncodeSavedEntityRequest.Highlight`, `corepb.EncodeEntityRequest.Highlight` and their `GetHighlight` methods are removed. Set `Tokens` instead.
- If the sibling plan cc23fc08's section is already at the top, put this section above it (newest first).

Complexity: Low
Token estimate: ~5k tokens
Agent strategy: Single agent, sequential execution.

## Testing Strategy

Every test follows the repo's conventions: testify `require`, one behaviour per function, positive and negative cases in separate functions, no table-driven tests, tests in `coreservice/*_test.go` beside the code, and the race detector on. Tests drive `Service.Call` with marshalled requests through the existing `testClient` (`coreservice/service_test.go:31-200`). No test reads the changelog, the design, `buf.yaml` or the `.proto` file.

- **Add token kinds, labelled text and the error report to the protocol**: the generated enum descriptor against `highlight.Kinds()` in both directions (number and `TOKEN_KIND_` name); categories by arithmetic; `tokensToProto` keeps kinds and text and returns nil for nothing; an unknown kind (1199) survives a marshal/unmarshal round trip and formats like a Comment through Go's Terminal formatter.
- **Attach the full report to every error that holds a configuration problem**: unit tests on `toProto` (a diagnostic, a ConfigError and each cause, a provider diagnostic, the detail unwrapped, no report on not-found and internal); integration through `StartApply` with the unsupported-block fixture (the finished error's report equals Go's `PrintError` at width 0 for the same file, the `apply/error` event carries the same report, validate events carry each problem's report); the host-shaped success-metric test (print from received tokens, no further call); a not-found `Find` with no report.
- **Return labelled resource text and retire the colour request**: tokens join to the text for a saved record, a looked-up record and an edited record; no tokens unless asked; raw old-caller bytes with field 2 set get plain text on both calls. The four colour tests are removed. `buf lint` and `buf breaking` are run by hand, not as tests.
- **Send labelled change summaries with a finished diff**: the finished diff carries tokens with a Changed kind; the joined tokens equal Go's `PrintDiff` for the same diff; an apply carries none; the existing diff JSON tests are unchanged.
- **Convert editor themes without a configuration**: orange properties give an orange attribute style on handle 0; comments and trailing commas are accepted; a bad colour and non-JSON give `invalid_theme` with a reason; an empty request gives `invalid_argument`; a config handle gives `StatusHandleMismatch`; `toProto` maps `InvalidThemeError`.
- **Describe reports, labelled text and theme conversion in the other-language design** and **Write the changelog entry for labelled output over the engine**: manual review only (captured in the implementation test plan).

Success metric: "A Python program prints a configuration problem with its source using only what the engine already returned, with zero extra engine calls" is a behavioural test (engine half) in the report task. The Python half is the Python spec's.

Manual — captured in the implementation test plan: `buf lint` and `buf breaking` against main are clean; the protocol comments; the revised `libxcl.md`; the changelog section.

## Project References

- Spec: `20261023075136-42c51a7d-printing-tokens-over-the-engine` (epic `20261023074219-4357d314-printing-and-pretty-output`).
- Design built on: `printing-and-highlighting.md` (source `design`). Design revised: `libxcl.md` (source `design`).
- Plans: `20261023074926-eaa51527-printing-and-highlighting-core` (upstream, must land first) and `20261023074627-cc23fc08-encode-saved-entity-during-operation` (overlapping files; the second to land rebases).
- Downstream spec: `20261023075302-bad71237-printing-and-highlighting-python`.
- Knowledge (repo xcl): `conventions/testing-and-mocking.md`, `conventions/code-style.md`, `conventions/shared-errors-package.md`, `conventions/dependencies.md`, `conventions/project-structure.md`.
- Repo root: xcl at `/home/nicj/code/github.com/jumppad-labs/xcl`. No other repo is changed.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

Tasks: proto groundwork (Medium, ~20k), error reports (Medium, ~30k), encode tokens and reserved field (Medium, ~30k), diff tokens (Low, ~15k), ConvertTheme (Low, ~15k), design revision (Low, ~15k), changelog (Low, ~5k). The protocol-editing tasks run in sequence because each regenerates `coreservice/proto/core.pb.go`.

## Migration Notes

- Bindings: a binding built against the previous protocol keeps working. Its `highlight=true` requests now return plain text, and it sees none of the new fields until it regenerates. The Python package regenerates its stubs, drops `highlight=` and removes its colour tests in the Python spec. Until then, its integration tests that expect colour fail against an engine built from this change.
- Go importers of `coreservice/proto`: the `Highlight` field and `GetHighlight` methods on both encode requests are removed. Set `Tokens` and format the returned tokens.
- Rebasing with plan cc23fc08: resolve `core.proto` and `coreservice/lookups.go` by hand (keep both changes), and regenerate `core.pb.go` with `make core-protos` rather than merging it. Put the later changelog section on top. Revise `libxcl.md` from the version already in the store.

## Performance Considerations

- The report is built for every error that holds a configuration problem, once per level (top-level error and each cause), so the report text is sent roughly twice for a configuration error. Reports are small (a few lines per problem), and errors are not a hot path.
- Encode tokens are computed only when asked. Diff tokens are computed once per finished diff, by the same lexer `PrintDiff` uses.
- `ConvertTheme` runs TextMate matching once per kind, as `LoadVSCodeTheme` does in Go. It is not on any operation path.
