---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023075136-42c51a7d-printing-tokens-over-the-engine

<!-- Metadata -->
<!-- Created: 2026-10-23T08:42:59Z -->
<!-- Commit: ae3b7b6 -->
<!-- Branch: f-python-prettylogger -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Programs in languages other than Go get readable xcl output from the engine without asking it twice. Every error that holds a configuration problem now carries Go's full report as labelled text, including errors delivered to an event handler mid-operation. Rendered resources and change summaries come as labelled text too, and a VS Code theme can be converted into styles keyed by kind with no configuration. This replaces the engine's one-line errors and finished terminal colour codes. Binding authors, starting with the Python package, can then print the same reports, resources and diffs as Go and style them their own way. Older callers keep working: their colour request now gets plain text.

## Conventions

- **Testing & Mocking (testify `require`, never table-driven, positive and negative tests apart, verbose over abstract, tests beside their source, no tests that inspect repository files)** — every new core-service test (report on errors, labelled encode and diff text, ConvertTheme, kind identities, old-caller requests) is one behaviour per function in `coreservice/*_test.go`. The kind-identity test reads the generated descriptor, which is code, not the `.proto` file. The changelog, proto comments, buf config and design revision are checked by review.
- **Go Code Style & Standards (gofmt/vet, descriptive names, explicit error handling, stdlib style, `any`)** — applies to the new `coreservice` helpers (token conversion, ConvertTheme handler, the finished-task change) and the error-code mapping.
- **Shared error types live in the `errors` package** — the new `invalid_theme` code maps the existing `errors.InvalidThemeError` and adds no new error type. A `codedError` stays for coreservice's own errors, as today.
- **Dependencies (prefer the standard library, pin and justify third-party modules)** — no new module. `google.golang.org/protobuf` (already present) supplies `protowire` for the old-caller test and `protoreflect` for the descriptor test.
- **Project structure (public packages at module top level, private code in `/internal`, API definitions under `/api`-style proto dirs)** — the protocol stays in `proto/xcl/core/v1`, its Go types in `coreservice/proto`, and no new package is created.

Deliberately dropped: database and external services, config parser options builder, hclsyntax.Variables references, graph-parent ordering, real-apply test state (the tests apply through the service as existing tests do), provider Changed/Update testing, shared test helpers (every new helper is used only by `coreservice` tests), never-modify-dependencies (no dependency is touched), patterns & architecture (no new service), development standards/logging (no new logging).

## Architecture & Design Decisions

The work follows the design `design/printing-and-highlighting.md` (source `design`) as written. The core lexes and lays out, the engine sends labelled text (tokens), and each language formats them. All of it lands in the xcl repo (`/home/nicj/code/github.com/jumppad-labs/xcl`): the protocol in `proto/xcl/core/v1/core.proto` and its generated Go types in `coreservice/proto`, the core service in `coreservice/`, the breaking-change rules in `proto/buf.yaml`, the changelog, and the other-language design `design:libxcl.md`, revised through the design CLI. No language package changes here. The Python package regenerates its stubs, drops `highlight=` and gains printing in the sibling spec `20261023075302-bad71237-printing-and-highlighting-python`, which depends on this one. The protocol gains, additively and with the design's numbers: a `TokenKind` enum mirroring `highlight.Kind` (`TOKEN_KIND_TEXT = 0` … `TOKEN_KIND_DETAIL = 1404`), `Token { kind, text }`, `Error.report = 5`, `bool tokens = 4` on both encode requests, `repeated Token tokens = 2` on both encode responses, `OperationFinished.diff_tokens = 4`, and a handle-0 `ConvertTheme` method. The legacy `highlight = 2` on both encode requests is reserved by number and name.

**The report is attached in one place.** Every error the engine sends passes through `toProto` in `coreservice/errors.go`: call results (`service.go`), finished operations (`operations.go`) and event errors delivered while an operation runs (`events.go`). `toProto` sets `report` from the dependency plan's `xcl.ErrorTokens(err)` whenever the error tree holds a `*errors.Diagnostic`, found with `errors.As`, so a provider failure reported at its block carries a report as well as a validation problem does. Otherwise `report` is left empty and the message is unchanged. Because `toProto` already recurses into a `ConfigError`'s causes, each collected problem carries its own report, and the top-level error carries all of them in order. The report is laid out once, by the same function `xcl.PrintError` uses, and sent unwrapped. Its plain text therefore equals Go's plain report at width 0 by construction, and a host prints it with no further call, including from an event handler mid-operation. Building the report at each call site instead would mean three places to keep in step and would miss causes.

**Labelled text rides on the existing calls.** `EncodeSavedEntity` and `EncodeEntity` render text as today. When `tokens` is set, they also return `highlight.Tokenize(text)`, so the labels come from the one lexer and join back to the text exactly. The legacy `colour()` path that the dependency plan left on `highlight.Terminal` is deleted. A request from an old caller that still sets field 2 parses, because unknown fields are skipped, and gets plain text, as the spec requires. For diffs, the operation runner's work function returns the `*diff.Diff` instead of its JSON. The finished task then builds both `diff_json`, unchanged, and `diff_tokens` from `(*diff.Diff).Tokens()`, so both always describe the same diff. `ConvertTheme` takes handle 0, so no configuration is needed. It runs `highlight.LoadVSCodeTheme` over the bytes, which reuses the core's TextMate selector rules as the spec's technical approach asks, and returns `map<int32, string>` from kind number to style string. An unusable theme maps to a new, additive error code `invalid_theme` with `source` and `reason` details. Token conversion is one small helper (`highlight.Token` → `corepb.Token`, kind number unchanged). Because proto3 enums are open, a kind an older SDK does not know still arrives as its number, and its category is `kind / 100 * 100` in every language.

**Keeping the contract check honest.** The spec requires the legacy field to be reserved and `buf breaking` to pass. Under the repo's current `breaking: use: [FILE]` rule they cannot both hold. A probe showed that `FIELD_NO_DELETE` refuses even a fully reserved deletion. `proto/buf.yaml` therefore keeps every FILE rule except `FIELD_NO_DELETE` and adds `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`. A field can now be removed only by reserving both its number and its name, which is the proto header's existing rule ("added and never renamed or reused") made exact. The comments in `buf.yaml` and the CI workflow are updated to say so. A Go test holds the `TokenKind` enum to `highlight.Kinds()` in both directions, number and name, so the proto and Go cannot drift (`conventions/testing-and-mocking.md`: a behaviour test over the generated descriptor, not a scan of the `.proto` file). The proto's own comments, the changelog (`## <spec name>`, prose, then a `**Breaking:**` list naming the reserved field, the removed generated `Highlight` accessors and the plain text old callers now get) and `design:libxcl.md` (error model, lookups and encode messages, `OperationFinished`, the method list) describe the report, labelled text and theme conversion. This beats deprecating the field (it contradicts the spec's "reserved"), switching to the looser `WIRE_JSON` category (it drops FILE checks), a separate report call (the design's "report travels on every `Error`" decision), and laying the report out in `coreservice` from `details` (a second layout that could drift). The evidence is in `research.md#alternatives-considered-and-rejected`.

**Landing order.** The dependency plan `20261023074926-eaa51527-printing-and-highlighting-core` must land first, since it provides `ErrorTokens`, `Kind`, `Tokenize`, `Diff.Tokens` and `LoadVSCodeTheme`. The sibling plan `20261023074627-cc23fc08-encode-saved-entity-during-operation` changes the same encode handlers in `coreservice/lookups.go`, the `EncodeSavedEntity` comment in `core.proto`, `CHANGELOG.md` and `design:libxcl.md`. Whichever of the two lands second rebases onto the other and regenerates the protocol's generated Go types from the merged protocol, rather than merging the generated file by hand.

## Component Breakdown

- **CoreService protocol (xcl, changed)**: owns the wire contract between the engine and every other language. It gains the token-kind enum, numbered exactly as Go's kinds, and the labelled-text message. It adds a report to the error message, a labelled-text request flag and labelled text to both encode calls, labelled text beside the structured diff on a finished diff, and the theme-conversion call that needs no configuration. The legacy terminal-colour request field is reserved by number and name. Its comments are the protocol's own documentation of all of this. Everything else in the engine reads and writes these messages. Nothing is renamed or renumbered.
- **Generated Go protocol types (xcl, regenerated)**: the message and enum types the core service builds and parses, regenerated from the protocol with the existing generation target. The generated accessors for the reserved field disappear, and new ones appear for the new fields.
- **Breaking-change rules (xcl, changed)**: own the guarantee that the protocol only grows. They keep every file-level rule except plain field deletion. In its place they allow a deletion only when both the field's number and its name are reserved, which is the protocol's existing "never reused" promise. The CI step that runs them is unchanged apart from its comment.
- **Error conversion (xcl core service, changed)**: owns turning any Go error into the error message a host receives, with its stable code, message, details and causes. It now also attaches the labelled report, built by the Go library's report builder, whenever the error holds a configuration problem, at every level of causes. It maps an unusable editor theme to a new stable code. Every path that sends an error uses it: call results, finished operations and event errors during an operation.
- **Token conversion (xcl core service, new)**: a small helper that turns the Go library's labelled text into protocol tokens, with the kind number unchanged. Error conversion, the encode calls and the diff result share it, so labels cross the wire one way everywhere.
- **Encode calls (xcl core service, changed)**: own rendering a saved record, or a looked-up or edited entity's record, as configuration text. When asked, they now also return that text labelled by the core's one lexer. The legacy terminal colouring is removed, so a request from an older caller that still asks for colour gets plain text. Rendering itself, and the rules on when each call may run, are unchanged here. The sibling saved-record plan changes those rules.
- **Operation runner (xcl core service, changed)**: owns running one operation and posting its finished task after its last event. Its work now hands back the diff itself rather than its JSON, so the finished task derives both the structured diff and its labelled text from the one value.
- **Theme conversion call (xcl core service, new)**: owns converting a VS Code colour theme into styles keyed by kind number, with no configuration needed. It delegates entirely to the Go library's VS Code theme importer, which holds the TextMate selector rules, and reports an unusable theme through error conversion.
- **Go library printing core (xcl root and `highlight`, unchanged here, from the dependency plan)**: the report builder, entity and diff labelling, the lexer, the kind list and the VS Code theme importer. This plan consumes them and never re-implements lexing or layout.
- **Core service tests (xcl, changed)**: own proof at the protocol boundary. They cover reports on an apply's finished error and on the event error during the apply, equal to Go's plain report, and no report on other errors. They show labelled encode and diff text joining to the plain text and to Go's change summary, theme conversion with no configuration, and kind numbers and names matching Go in both directions. They also show an unrecognised kind keeping its number and category, and an old caller's colour request getting plain text. The four legacy colour tests are replaced.
- **Release notes and other-language design (xcl, docs)**: the changelog gets this spec's section, with a Breaking list for the reserved field, the removed generated accessors and the colour request now answered with plain text. The other-language design document gains the report on errors, labelled text, the diff's labelled text and theme conversion in its error model, method list and message sections, revised through the design CLI.

## Data Structures & Interfaces

All wire shapes come from the design's Protocol section. They are the contract between the engine and every language binding. Numbers not shown are unchanged.

**`TokenKind` (new enum)**: the label vocabulary on the wire. It has one value per `highlight.Kind`, with the same number. The name is `TOKEN_KIND_` plus the Go name in upper snake case. A value's category is `number / 100 * 100` in every language. proto3 enums are open, so a number the receiver does not know still arrives as an integer and falls into its category.

```protobuf
enum TokenKind {
  TOKEN_KIND_TEXT = 0;
  TOKEN_KIND_BLOCK = 100;  TOKEN_KIND_BLOCK_TYPE = 101;  TOKEN_KIND_NESTED_BLOCK = 102;
  TOKEN_KIND_LABEL = 200;  TOKEN_KIND_TYPE_LABEL = 201;  TOKEN_KIND_NAME_LABEL = 202;
  TOKEN_KIND_ATTRIBUTE = 300;
  TOKEN_KIND_STRING = 400; TOKEN_KIND_STRING_ESCAPE = 401; TOKEN_KIND_HEREDOC = 402;
                           TOKEN_KIND_HEREDOC_MARKER = 403; TOKEN_KIND_HEREDOC_OPERATOR = 404;
  TOKEN_KIND_NUMBER = 500;
  TOKEN_KIND_CONSTANT = 600;
  TOKEN_KIND_FUNCTION = 700; TOKEN_KIND_BUILTIN_FUNCTION = 701;
  TOKEN_KIND_REFERENCE = 800; TOKEN_KIND_MEMBER = 801;
  TOKEN_KIND_OPERATOR = 900;
  TOKEN_KIND_PUNCTUATION = 1000; TOKEN_KIND_INTERPOLATION_BEGIN = 1001; TOKEN_KIND_INTERPOLATION_END = 1002;
                                 TOKEN_KIND_TEMPLATE_DIRECTIVE = 1003; TOKEN_KIND_ACCESSOR = 1004;
  TOKEN_KIND_COMMENT = 1100; TOKEN_KIND_LINE_COMMENT = 1101; TOKEN_KIND_HASH_COMMENT = 1102; TOKEN_KIND_BLOCK_COMMENT = 1103;
  TOKEN_KIND_DIFF = 1200; TOKEN_KIND_INSERTED = 1201; TOKEN_KIND_DELETED = 1202; TOKEN_KIND_CHANGED = 1203;
  TOKEN_KIND_PROBLEM = 1300; TOKEN_KIND_ERROR = 1301; TOKEN_KIND_WARNING = 1302;
  TOKEN_KIND_REPORT = 1400; TOKEN_KIND_SUMMARY = 1401; TOKEN_KIND_LOCATION = 1402; TOKEN_KIND_GUTTER = 1403; TOKEN_KIND_DETAIL = 1404;
}

message Token { TokenKind kind = 1; string text = 2; }   // joined texts are the plain rendering
```

**`Error` (changed)**: gains the report. It is set whenever the error holds a configuration problem, including on each cause that is a problem and on errors carried by events. Otherwise it is empty and the message is unchanged. It is unwrapped: the detail is one line, and wrapping it is left to the language.

```protobuf
message Error { string code = 1; string message = 2; map<string, string> details = 3;
                repeated Error causes = 4; repeated Token report = 5; }
```

**Encode calls (changed)**: field 2 is reserved by number and name on both requests. `tokens` asks for labelled text alongside the text, and an old caller never sets it.

```protobuf
message EncodeSavedEntityRequest  { bytes entity = 1; reserved 2; reserved "highlight"; EncodeOptions options = 3; bool tokens = 4; }
message EncodeSavedEntityResponse { bytes text = 1; repeated Token tokens = 2; }   // tokens empty unless asked
message EncodeEntityRequest       { bytes entity = 1; reserved 2; reserved "highlight"; EncodeOptions options = 3; bool tokens = 4; }
message EncodeEntityResponse      { bytes text = 1; repeated Token tokens = 2; }
```

**`OperationFinished` (changed)**: a finished diff carries labelled text for the same diff as `diff_json`. Other operations leave it empty.

```protobuf
message OperationFinished { uint64 operation_id = 1; Error error = 2; bytes diff_json = 3; repeated Token diff_tokens = 4; }
```

**`ConvertTheme` (new method, handle 0)**: a VS Code theme (JSONC) in, style strings keyed by kind number out. Only kinds the theme styles appear in the map. An unusable theme fails with the new code `invalid_theme`, carrying `source` and `reason` details. An empty request fails with `invalid_argument`.

```protobuf
rpc ConvertTheme(ConvertThemeRequest) returns (ConvertThemeResponse);
message ConvertThemeRequest  { bytes vscode_theme_json = 1; }
message ConvertThemeResponse { map<int32, string> styles = 1; }   // TokenKind number → "bold #ff8800"
```

**Core service internals (package-private Go)**: these are not part of any public API.

```go
const CodeInvalidTheme = "invalid_theme"                        // exported beside the other codes
func tokensToProto(tokens []highlight.Token) []*corepb.Token   // kind number unchanged; nil for none
func (c *configuration) start(work func(ctx context.Context) (*diff.Diff, error)) (uint64, error)
func (s *Service) convertTheme(_ *configuration, req *corepb.ConvertThemeRequest) (*corepb.ConvertThemeResponse, error)
```

**Breaking-change rules (`proto/buf.yaml`)**: they keep FILE, except `FIELD_NO_DELETE`, and add `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`.

**Generated Go API (`coreservice/proto`, breaking for Go importers of the generated package)**: the change removes `EncodeSavedEntityRequest.Highlight`, `EncodeEntityRequest.Highlight` and their `GetHighlight` accessors. It adds `TokenKind`, `Token`, `Error.Report`, the request and response `Tokens` fields, `OperationFinished.DiffTokens`, `ConvertThemeRequest` and `ConvertThemeResponse`.

## Implementation Detail

**No new pattern in the core service; the existing seams take the new data.** The core service already funnels every outgoing error through one conversion function and every method through one handler table that a test holds to the protocol's descriptor. This plan uses both seams as they are. The report is one more field filled in by the error conversion, next to the details it already fills. Theme conversion is one more handle-0 entry in the table, next to the version and configuration-creation methods. A reader of the error conversion sees the same switch over codes as today, followed by one step that adds the report when the error holds a configuration problem. A reader of the handler table sees one new line.

**One converter for labelled text.** Labelled text crosses the wire in exactly one way: a small helper copies each Go token's kind number and text into a protocol token. Reports, encode responses and diff results all use it, so nothing in the service builds tokens by hand, and a kind added to the Go library later crosses with no service change. The kind-parity test then fails until the enum gains the value.

**The operation runner hands back what it computed, not a serialisation of it.** Today a diff operation serialises its result inside its work function, and the runner posts the bytes. After this change, the work function returns the diff, and the runner derives every representation the finished task carries from it: the structured JSON and the labelled text. Operations without a result return nothing, as before. The ordering guarantee is untouched: the finished task still follows the operation's last event.

**The legacy colour path is deleted, not adapted.** The encode handlers lose their colouring step. Labelled text is opt-in through the new request flag, so an old caller pays nothing and a new caller gets labels from the same lexer as Go. Old requests that still carry the reserved field are accepted, because protobuf skips unknown fields, and are answered with plain text. A test pins this down by sending the old field's bytes directly.

**The protocol's contract check is made exact rather than loosened.** The breaking-change rules swap one blanket rule ("no field is ever deleted") for the precise one the protocol header already states ("a deleted field's number and name are reserved forever"). Every other file-level check stays. The comments in the rules file, the CI workflow and the protocol header say the same thing, so a future contributor reads one consistent rule.

**Kinds are held to Go by a descriptor test, not by convention.** The protocol's enum is written by hand to mirror the Go kinds, as the design asks. A test in the core service walks the generated enum descriptor and the Go kind list in both directions, comparing numbers and names. A missing, extra, renumbered or misspelt kind fails the build's tests. This follows the existing pattern of holding the handler table to the service descriptor.

**Docs follow existing shapes.** Protocol comments sit beside each new field and method in the style of the existing ones. The changelog gets this spec's section in the usual form. The other-language design document is revised in place through the design CLI: its error-model block, method list, encode-message block and finished-task block gain the new fields, with a short paragraph each on the report, labelled text and theme conversion.

## Dependencies

- **Design documents this plan was built on:** `printing-and-highlighting.md` from the `design` design source. It settles what the engine sends, the kind numbering, the message shapes, the report on every `Error`, the reserved `highlight` field and `ConvertTheme`, which this plan implements.
- **Design document revised (not built on): `libxcl.md` from the `design` design source.** This is the other-language design, read as current documentation. The spec's documentation requirement makes this plan revise it through the design CLI.
- **Plan `20261023074926-eaa51527-printing-and-highlighting-core` (must land first).** It provides `xcl.ErrorTokens`, `xcl.PrintError` (used by tests for Go's plain report), `xcl.PrintDiff`, `highlight.Kind` with `Kinds()`, `String()` and `Parent()`, `highlight.Token`, `highlight.Tokenize`, `(*diff.Diff).Tokens()`, `highlight.LoadVSCodeTheme`, `highlight.Terminal` and `highlight.DefaultTheme`. It leaves the core service's `highlight=true` on the Terminal formatter, which this plan removes. Nothing in it needs to change.
- **Plan `20261023074627-cc23fc08-encode-saved-entity-during-operation` (independent, overlapping).** It edits the same encode handlers and tests in the core service, the `EncodeSavedEntity` comment in the protocol, the generated Go types, `CHANGELOG.md` and `libxcl.md`. Neither must land first. Whichever lands second rebases and regenerates the protocol's Go types, rather than merging them by hand.
- **Sibling spec `20261023075302-bad71237-printing-and-highlighting-python` (downstream).** It depends on this plan. It regenerates the Python stubs from this protocol, removes Python's `highlight=` parameter and the tests that expect colour, and does the Python printing. Until it lands, an xcl-python build against this engine still sends field 2 and gets plain text, so its colour tests fail on the engine bump. That is expected, and it is that spec's work.
- **`google.golang.org/protobuf` (existing).** It provides generated types, `protowire` (to build an old caller's request bytes in a test) and `protoreflect` (the enum descriptor for the kind-parity test). No change.
- **`buf` (existing tooling, CI pins 1.50.0).** It runs `buf generate`, `buf lint` and `buf breaking`. Only the breaking rules in `proto/buf.yaml` change.
- **Go library errors (`errors` package, existing).** `ConfigError` and `Diagnostic` unwrap so `errors.As` finds a problem anywhere, and `InvalidThemeError` carries `Source` and `Reason`. No change.
- **`github.com/stretchr/testify` (existing).** `require` is used in every new test.
- **No change to xcl-python, xcl-website, xcl-vscode or the plugin repos.** The website does not document the protocol.

## Testing Approach

Testing follows the repo's conventions: testify `require`, one behaviour per test function, positive and negative cases in separate functions, no table-driven tests, and tests beside the code they test. Every new test lives in the core service's own test files. Tests drive the service the way a binding does: marshal a request, pass the bytes through the service's call entry point and unmarshal the reply, using the existing in-package test client, polling loop and fixtures. Documentation, the changelog, the breaking-change rules and the design revision are checked by review. No test reads them.

**Integration tests at the protocol boundary (most of the coverage).** These are the load-bearing guarantees:
- Applying a configuration with an unsupported block through the engine finishes with an error whose report, joined as plain text, equals Go's plain report (width 0) for the same file applied directly through the Go library. This covers the requirements "every error carries its report" and "reports match Go's".
- While that apply runs, the error event delivered for the failed apply carries the same report as the finished error. Each collected problem in the finished error's causes carries its own report.
- A not-found lookup returns an error with no report and the same code, message and details as today. A plain `internal` error carries no report either.
- Rendering a created resource returns labelled text that joins to the plain text returned beside it. This holds from its saved record, from its looked-up record, and from an edited record (a changed attribute value). Without the flag, no labelled text comes back.
- A diff through the engine returns labelled text, beside the unchanged structured diff, that joins to Go's plain change summary for the same diff run directly. Other operations return none.
- Converting a VS Code theme that colours properties orange, on handle 0 with no configuration created, returns the attribute kind's style containing that orange. An unusable theme fails with the new theme code and its reason. An empty request fails as an invalid argument. A configuration handle given to the call is refused as a handle mismatch.
- A request built as an older caller would send it, with the reserved colour field set, succeeds on both encode calls and returns plain text with no terminal codes.

**Contract tests (kinds).** The generated token-kind enum and the Go kind list are compared both ways. Every Go kind has an enum value with the same number and the `TOKEN_KIND_` upper-snake name, and every enum value is a Go kind. Tokens sent on the wire keep their kind numbers, so the category a language computes (number / 100 × 100) equals Go's parent. A token whose kind number is unrecognised but inside a known category (for example 1199) survives encoding and decoding with its number, falls into the comment category, and is printed in the comment style by Go's terminal formatter. The existing descriptor test keeps the handler table and the service in step, so the new method must have a handler.

**Unit tests.** The token converter copies kind numbers and text unchanged and returns nothing for nothing. The error conversion attaches a report to a diagnostic, to a configuration error holding one, and to a diagnostic caused by a provider failure. It attaches none to an error without a configuration problem.

**Regression.** The four existing tests that expected terminal colour from the encode calls are replaced by the plain-text-for-old-callers tests above. Every other existing core-service test passes unchanged, including the structured diff matching Go's diff and every error-code test. The race detector stays on, as CI runs it.

**Success metrics.**
- *A Python program prints a configuration problem with its source using only what the engine already returned, with zero extra engine calls.* **Behavioural test** (engine half): a host-shaped test receives the failed apply's finished error and prints its report through Go's plain formatter from the received tokens alone. It asserts the summary, location and numbered source lines, and asserts that no further engine call was made after the finished task arrived. The Python half (printing from `err.tokens` in Python) is verified by the sibling Python spec, which depends on this one.

**Manual reviews.**
- **Manual — captured in the implementation test plan**: `buf lint` and `buf breaking` against the main branch report nothing for this change. This runs locally and as CI's breaking check on the pull request.
- **Manual — captured in the implementation test plan**: the protocol's comments describe the report on errors, labelled text on encode and diff results, the reserved colour field and `ConvertTheme`.
- **Manual — captured in the implementation test plan**: the revised other-language design (`libxcl.md`) describes the report on errors, labelled text and theme conversion consistently with the protocol.
- **Manual — captured in the implementation test plan**: the changelog section describes the change and its Breaking list names the reserved field, the removed generated accessors and the plain text old colour requests now get.

**Deliberate gaps.** There are no Python tests here: Python's stubs, printing and its handling of unknown kinds belong to the Python spec. There is no test of the C shared library or the stdio engine program for the new fields, because both pass bytes through the same service unchanged and their existing smoke tests cover the transport.

## Milestones & Tasks

### Milestone 1: Every configuration problem reaches other languages with its full report

**What changes**: When a program in another language applies, validates, diffs or loads a configuration that has a mistake, every error the engine returns now carries the full report, the same one Go prints. That includes the error its event handler receives while the operation is still running. The report is labelled text: the summary, location, numbered source lines with the fault marked, and the detail, each piece tagged with what it is, using the same kind numbers as Go. A language can print it, plain or styled, without asking the engine again. Errors that are not configuration problems, such as a lookup of an address that does not exist, are unchanged and carry no report. The protocol only gains fields here, so existing callers are unaffected.

**Validation point**: The core service's tests pass with the race detector on. That includes a failed apply whose finished error and error event both carry a report equal to Go's plain report, a not-found lookup with no report, the kind numbers and names matching Go's in both directions, an unrecognised kind keeping its category, and the host-shaped test that prints a report with no further engine call. `buf lint` passes.

#### - [ ] Task: Add token kinds, labelled text and the error report to the protocol
**Id:** 33d2f2d9-106b-4995-a686-242ded6baea3
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Add the kind enum, numbered exactly as Go's kinds, and the labelled-text message to the engine's protocol, plus an empty report field on the error message, then regenerate the Go types. Add the one helper that turns Go's labelled text into protocol tokens, so every later task sends labels the same way. Hold the protocol's kinds to Go's with a test in both directions, so the two can never drift. This task requires the printing-core plan to have landed.

*Technical detail:* [context.md#task-add-token-kinds-labelled-text-and-the-error-report-to-the-protocol](./context.md#task-add-token-kinds-labelled-text-and-the-error-report-to-the-protocol)

**Acceptance criteria**:
- [ ] Every Go kind has a protocol kind with the same number and matching name, and the protocol has no kind Go lacks
- [ ] Labelled text crosses the wire with each piece's kind number and text unchanged
- [ ] A kind number the receiver does not recognise keeps its number on the wire and falls into its category, printing in that category's style
- [ ] The protocol only gains fields and passes its lint and breaking-change checks

#### - [ ] Task: Attach the full report to every error that holds a configuration problem
**Id:** 425934f7-6c4d-4fc3-9b80-589d2565b498
**Repo:** xcl
**Depends on:**
- 33d2f2d9-106b-4995-a686-242ded6baea3 — Add token kinds, labelled text and the error report to the protocol
**Execution:** agent

Make the engine attach the labelled report Go prints to every error it sends that holds a configuration problem, at every level of collected problems. This covers errors returned from calls, the error a finished operation reports, and errors carried by events while the operation runs. Errors with no configuration problem keep their message and get no report. The protocol comment on the report says it is laid out once, unwrapped, and present only for configuration problems.

*Technical detail:* [context.md#task-attach-the-full-report-to-every-error-that-holds-a-configuration-problem](./context.md#task-attach-the-full-report-to-every-error-that-holds-a-configuration-problem)

**Acceptance criteria**:
- [ ] Applying a configuration with an unsupported block through the engine finishes with an error whose report, as plain text, equals Go's plain report for the same configuration
- [ ] The error event delivered while that apply runs carries the same report
- [ ] Each collected problem in the error carries its own report
- [ ] A provider failure reported at its block carries a report
- [ ] Looking up an address that does not exist returns a not-found error with no report and the same message as before
- [ ] A host can print the report from the error it received, with no further engine call

### Milestone 2: Labelled resource text, labelled change summaries and editor themes over the engine

**What changes**: Programs in other languages can ask for a rendered resource as labelled text as well as plain text, from its saved record or from a looked-up or edited entity. A finished diff now carries labelled text for its change summary beside the structured diff it already carries. A VS Code colour theme can be converted into styles keyed by kind, with no configuration created, using the core's own TextMate matching. The old terminal-colour request is retired: its field is reserved and never reused, and a caller built against the previous protocol that still asks for colour gets plain text instead of an error. The protocol's breaking-change check is made exact, so it accepts a removed field only when its number and name are both reserved.

**Validation point**: The core service's tests pass with the race detector on. That includes labelled encode text joining to the plain text for saved, looked-up and edited records, a diff's labelled text equal to Go's plain change summary, the orange-properties theme giving an orange attribute style with no configuration, and an old caller's colour request returning plain text. `buf lint` passes and `buf breaking` against the main branch reports no breaking change.

#### - [ ] Task: Return labelled resource text and retire the colour request
**Id:** ed5f53d3-49e6-4552-8e67-fbb89f704972
**Repo:** xcl
**Depends on:**
- 33d2f2d9-106b-4995-a686-242ded6baea3 — Add token kinds, labelled text and the error report to the protocol
**Execution:** agent

Let both rendering calls, the one for saved records and the one for looked-up or edited entities, return the text labelled by the core's lexer when the caller asks. Reserve the old terminal-colour field by number and name, and remove the engine's colouring, so a caller built against the previous protocol that still asks for colour gets plain text. Make the protocol's breaking-change check accept a removed field only when its number and name are both reserved, and update the comments that describe that rule.

*Technical detail:* [context.md#task-return-labelled-resource-text-and-retire-the-colour-request](./context.md#task-return-labelled-resource-text-and-retire-the-colour-request)

**Acceptance criteria**:
- [ ] Rendering a created resource from its saved record, from its looked-up record and from an edited record, with labelled text requested, returns labelled text that joins to the plain text returned beside it
- [ ] Without the request, no labelled text is returned and the text is unchanged
- [ ] A request built as an older caller sends it, asking for colour, succeeds and returns plain text with no colour codes
- [ ] The breaking-change check against the main branch reports nothing, and still refuses a field removed without reserving its number or name

#### - [ ] Task: Send labelled change summaries with a finished diff
**Id:** 77761c9d-d20a-409e-a788-112f74787197
**Repo:** xcl
**Depends on:**
- 33d2f2d9-106b-4995-a686-242ded6baea3 — Add token kinds, labelled text and the error report to the protocol
- ed5f53d3-49e6-4552-8e67-fbb89f704972 — Return labelled resource text and retire the colour request
**Execution:** agent

Give a finished diff its change summary as labelled text, beside the structured diff it already carries, both derived from the same diff. The operation runner hands back the diff itself instead of its serialised form, so the two can never disagree. Other operations carry no labelled text. This task follows the rendering task only so the two do not edit the protocol and its generated code at the same time.

*Technical detail:* [context.md#task-send-labelled-change-summaries-with-a-finished-diff](./context.md#task-send-labelled-change-summaries-with-a-finished-diff)

**Acceptance criteria**:
- [ ] A diff through the engine returns labelled text whose plain rendering equals Go's plain change summary for the same diff
- [ ] The structured diff is unchanged and still equals Go's diff
- [ ] Apply, validate, destroy and load finish with no labelled diff text

#### - [ ] Task: Convert editor themes without a configuration
**Id:** 5eb6e9df-52f9-4339-87ab-66988b2844e9
**Repo:** xcl
**Depends on:**
- 77761c9d-d20a-409e-a788-112f74787197 — Send labelled change summaries with a finished diff
**Execution:** agent

Add a call that converts a VS Code colour theme into styles keyed by kind number. It needs no configuration and uses the core's own TextMate matching through the Go library's theme importer. An unusable theme is reported with a new, stable error code carrying the reason, so a binding can raise a specific error. This task follows the diff task only so the two do not edit the protocol and its generated code at the same time.

*Technical detail:* [context.md#task-convert-editor-themes-without-a-configuration](./context.md#task-convert-editor-themes-without-a-configuration)

**Acceptance criteria**:
- [ ] Converting a theme that colours properties orange, with no configuration created, returns a style for the attribute kind containing that orange
- [ ] An unusable theme fails with the theme error code and says what was wrong
- [ ] An empty request fails as an invalid argument
- [ ] The call refuses a configuration handle, as the other configuration-free calls do

### Milestone 3: The protocol, the other-language design and the release notes describe the new output

**What changes**: Anyone building or maintaining a language binding can read, in the other-language design, how errors carry their report, how resources and diffs come as labelled text, and how themes are converted. The protocol's own comments, written with the fields in the earlier milestones, say the same thing. The changelog gets this change's section, with a Breaking list that names the reserved field, the removed generated accessors and the plain text old colour requests now receive. Nothing changes in behaviour. This milestone is documentation only, and it is worth its own milestone because bindings are built from the design and the release notes.

**Validation point**: The manual reviews of the protocol comments, the revised design and the changelog pass.

#### - [ ] Task: Describe reports, labelled text and theme conversion in the other-language design
**Id:** dcb90fc8-a2f8-4f29-ad0b-0127d8b39781
**Repo:** xcl
**Depends on:**
- 425934f7-6c4d-4fc3-9b80-589d2565b498 — Attach the full report to every error that holds a configuration problem
- ed5f53d3-49e6-4552-8e67-fbb89f704972 — Return labelled resource text and retire the colour request
- 77761c9d-d20a-409e-a788-112f74787197 — Send labelled change summaries with a finished diff
- 5eb6e9df-52f9-4339-87ab-66988b2844e9 — Convert editor themes without a configuration
**Execution:** agent

Revise the other-language design document, through the design CLI, so its error model, method list, encode messages and finished-operation message show the report, labelled text, the reserved colour field and theme conversion. Add a short explanation of each: why the report travels on every error, how kinds and their categories work, and how a theme is converted with no configuration. Check that the protocol's comments say the same.

*Technical detail:* [context.md#task-describe-reports-labelled-text-and-theme-conversion-in-the-other-language-design](./context.md#task-describe-reports-labelled-text-and-theme-conversion-in-the-other-language-design)

**Acceptance criteria**:
- [ ] The design's error model shows the report and says when it is present and that it is unwrapped
- [ ] The design shows labelled text on both rendering calls and on a finished diff, with the kinds and their categories explained
- [ ] The design lists theme conversion as a configuration-free call and describes its result and its error
- [ ] The design no longer describes terminal colouring by the engine

#### - [ ] Task: Write the changelog entry for labelled output over the engine
**Id:** 4d27c6ce-3ffb-456b-ae8d-826fb46ea8dd
**Repo:** xcl
**Depends on:**
- 425934f7-6c4d-4fc3-9b80-589d2565b498 — Attach the full report to every error that holds a configuration problem
- ed5f53d3-49e6-4552-8e67-fbb89f704972 — Return labelled resource text and retire the colour request
- 77761c9d-d20a-409e-a788-112f74787197 — Send labelled change summaries with a finished diff
- 5eb6e9df-52f9-4339-87ab-66988b2844e9 — Convert editor themes without a configuration
**Execution:** agent

Add this change's section to the changelog. It describes the report on every error, labelled resource and diff text, theme conversion, the new error code and the exact breaking-change rule. Its Breaking list names the reserved colour field, the removed generated accessors for it, and the plain text an old colour request now receives.

*Technical detail:* [context.md#task-write-the-changelog-entry-for-labelled-output-over-the-engine](./context.md#task-write-the-changelog-entry-for-labelled-output-over-the-engine)

**Acceptance criteria**:
- [ ] The changelog has a section for this change describing what other languages now receive
- [ ] Its Breaking list names the reserved field, the removed generated accessors and the plain-text answer to old colour requests

## Open Questions

None. The one uncertainty found in research, whether `buf breaking` accepts a reserved field under the repo's rules, was settled by a probe against the current protocol: it does not under `FILE` alone, and the narrow rule change in this plan makes it pass while still refusing an unreserved deletion. Every other choice is decided and recorded in the drafting assumptions. If CI's buf (1.50.0) behaves differently from the locally probed 1.27.2, the implementer must STOP and ask, as research.md's open assumptions say.

## Out of Scope

- **Formatting labelled text in any language.** Terminal colour, HTML and SVG output from tokens are not done here (spec Non-Goal). Go's formatters come from the printing-core plan `20261023074926-eaa51527-printing-and-highlighting-core`, and Python's from the sibling spec `20261023075302-bad71237-printing-and-highlighting-python`.
- **Any change to the Python package.** Regenerating its protocol stubs, removing its `highlight=` parameter and the tests that expect colour, exposing `err.tokens`, `diff.tokens` and `load_vscode_theme`, and handling unknown kinds in Python all belong to the Python spec, which depends on this plan.
- **Structured problem fields beyond what errors carry today** (spec Non-Goal). The existing `details` on a configuration error are unchanged, and nothing new is added beside the report.
- **A TypeScript client** (spec Non-Goal). No TypeScript package exists yet.
- **Changing when the encode calls may run during an operation.** That is the sibling plan `20261023074627-cc23fc08-encode-saved-entity-during-operation`. This plan leaves the guard and the record-presence check alone.
- **The host-provider and type-description parts of the other-language design** (`Log`, `StateGet`, `StateFind`, `DescribeTypes`, provider tasks). They remain as the design describes, for later releases.
- **Website pages.** xcl.dev does not document the core protocol. Python's pages are rewritten by the Python spec.
- **Cutting a release.** Tagging the version that carries the reserved field follows the repo's release process.
