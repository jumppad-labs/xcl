---
created_date: "2026-10-23"
---

# Planning summary: 20261023074219-4357d314-printing-and-pretty-output

## Decisions
- **Release order for the saved-record render.** Plans: 20261023074627-cc23fc08-encode-saved-entity-during-operation, 20261023075136-42c51a7d-printing-tokens-over-the-engine, 20261023075302-bad71237-printing-and-highlighting-python. The encode plan relied on xcl-python's automatic engine bump merging cleanly, while the tokens plan reserves the protocol's `highlight` field, which breaks that bump until the Python printing work lands. Outcome: the encode plan's xcl release is cut before the tokens change merges to xcl main; if that is missed, its Python tests and docs ship with the Python printing release instead.
- **Release notes for xcl-python behaviour changes.** Plans: 20261023074223-e06f835f-event-data-level-enum, 20261023075328-741138ee-python-prettylog. Event-data-level adds a hand-written GitHub release-notes line for a user-visible change; prettylog recorded asyncio's in-order event delivery in the README only. Outcome: prettylog's release task also adds a release-notes line about the asyncio event order.
- **`str(err)` for a multi-problem configuration error.** Plan: 20261023075302-bad71237-printing-and-highlighting-python. Outcome: `str(err)` stays Go's `Error()` text, one line per problem; the criterion "Ordinary messages stay one line" is read as "the report never goes into `str(err)`".
- **Rendering inside the pretty logger's handler.** Plan: 20261023075328-741138ee-python-prettylog. Outcome: one `EncodeSavedEntity` render per created resource inside the handler is allowed; the constraint forbids waiting on the operation.
- **Reports above info.** Plan: 20261023075328-741138ee-python-prettylog. Outcome: above info the pretty logger writes no resource configuration, but configuration-error reports still show beneath their error line at every level (a deliberate difference from Go's example).

## Order added for shared files
- Added: `20261023074627-cc23fc08-encode-saved-entity-during-operation` now depends on `20261023074223-e06f835f-event-data-level-enum`. Both change `xcl-python:xcl/config.py`, `xcl-python:xcl/aio.py`, `xcl-python:tests/integration/test_sync.py`, `xcl-python:tests/integration/test_aio.py`, `xcl-python:xcl-ref.txt`, `xcl-python:README.md`, `xcl-python:PARITY.md`, `xcl-website:src/pages/events.mdx`, `xcl-website:src/pages/configuration-text.mdx`.
- Added: `20261023074926-eaa51527-printing-and-highlighting-core` now depends on `20261023074223-e06f835f-event-data-level-enum`. Both change `xcl-website:src/pages/events.mdx`, `xcl-website:src/pages/configuration-text.mdx`, `xcl-website:src/pages/examples/plugins.mdx`.
- Added: `20261023074926-eaa51527-printing-and-highlighting-core` now depends on `20261023074627-cc23fc08-encode-saved-entity-during-operation`. Both change `xcl:coreservice/lookups.go`, `xcl:CHANGELOG.md`, `xcl-website:src/pages/configuration-text.mdx`, `xcl-website:src/pages/events.mdx`.

## 20261023074223-e06f835f-event-data-level-enum
### Approach

Add `xcl.EventDataLevel` as an `enum.StrEnum` (`NONE`, `RAW`, `PROCESSED`; values "none", "raw", "processed") declared in `xcl/events.py` beside `Event` and exported from the top level. A private check in `xcl/config.py` replaces the public bare-word map `EVENT_DATA_LEVELS`: it accepts a value only when `isinstance(value, EventDataLevel)` and otherwise raises a plain `TypeError` naming `xcl.EventDataLevel` (a type test is required because a StrEnum member compares and hashes equal to its string). Both `xcl.Config` and `xcl.aio.Config` run the check first, so a refused value creates no configuration and starts nothing. The signature becomes `event_data: EventDataLevel = EventDataLevel.NONE`. The package ships `xcl/py.typed` so mypy reads its annotations (verified during planning: without it mypy reports `import-untyped`). Integer enums, re-exporting the protobuf enum, a string deprecation period, `ValueError` and an `xcl.Error` subclass were rejected.

### Milestones and tasks

- Milestone 1: Python programs choose event data from `xcl.EventDataLevel`
  - Add the EventDataLevel enum and accept only its members (xcl-python): enum, check, both signatures and docstrings, existing tests switched to the member, unit tests for members, wire values, default, and refusal of "processed", "verbose", an integer and None for both flavours with no CreateConfig call.
  - Prove each level against the engine (xcl-python): integration tests for raw, the option left out, and none.
  - Declare the package typed and check the option with mypy (xcl-python): `py.typed`, `mypy` in the `test` extra, `tests/unit/test_typing.py` running mypy over two snippets.
- Milestone 2: Documentation, parity list and release notes show the enum
  - Show the enum in the package README (with a migration note) and PARITY.md (xcl-python).
  - Show the enum on the xcl website pages `events.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx`, `examples/plugins.mdx` (xcl-website).
  - Release xcl-python with the breaking change in its notes (human task).

The first task must land before the others, which can then run in parallel.

### Tasks a person must do

- Release xcl-python: merge with the `minor` label (expected v0.14.0), then add a line to the generated GitHub release notes saying strings are refused, with the example `event_data="processed"` → `event_data=xcl.EventDataLevel.PROCESSED`.

### Out of scope

- New levels, turning other options into enums, TypeScript, and any engine, protocol or Go change.
- A deprecation period or string fallback.
- Changing `release.yml` to take notes from a file.
- An entry in xcl's CHANGELOG.
- Auditing other annotations that `py.typed` now exposes.
- Moving the `state_path`/`state_store` check or changing its exception type.

### Drafting assumptions

- A StrEnum in `xcl/events.py` with one shared type check, run first in both constructors, plus `py.typed`.
- xcl's testing rules apply to the Python suite; Go-only conventions were dropped.
- A refused value raises a plain `TypeError`, not an `xcl.Error` subclass.
- `mypy` joins the `test` extra and the typing criterion is automated; only pyright is checked by hand.
- Release notes come from a README migration note (no bare-string literal) plus a person editing the GitHub release; no xcl CHANGELOG entry.
- `xcl.config.EVENT_DATA_LEVELS` becomes private and its old name is not kept.
- "Raw matches what the string delivered before" is proved through the wire value plus an engine test.

### Project-wide rules

- Changelog: no entry in xcl's CHANGELOG.md for a Python-only change; xcl-python keeps no changelog file, so breaks are recorded in the README migration note and a hand-added line in the GitHub release notes.
- Releases: an xcl-python breaking change before 1.0 gets the `minor` release label; a person adds prose to the generated notes.
- Testing: xcl's Testing & Mocking conventions apply to the Python suite; no test reads README, PARITY.md or website pages; a mypy subprocess over snippets counts as testing behaviour.
- Typed package: xcl-python ships `py.typed`, so later Python specs keep their public annotations accurate for mypy.
- Python option pattern: an option with a fixed set of values is a top-level `StrEnum` named after the Go type, checked by type, refused with `TypeError`.
- Website overlap: edits `xcl-website:src/pages/examples/configuration-only.mdx` and `plugins.mdx`, which the prettylog and Python printing specs may also touch; whichever lands second rebases.

### Manual checks

- pyright over the same two snippets flags the string and accepts the member.
- The README and every xcl.dev page showing the event data option use `xcl.EventDataLevel` members, with no bare-string level left, and the README migration note says strings are refused and names the members.
- The PARITY.md event data row names Go's `xcl.EventDataLevel` / `WithEventData` and the Python enum.
- The GitHub release notes of the xcl-python version that ships this contain the line saying strings are refused, with the `event_data="processed"` → `event_data=xcl.EventDataLevel.PROCESSED` example.

## 20261023074627-cc23fc08-encode-saved-entity-during-operation
### Approach

The engine's saved-record render (`encodeSavedEntity` in `xcl:coreservice/lookups.go`) stops refusing calls while an operation runs; it keeps its check that a record was actually sent. `EncodeEntity` and every other lookup stay refused. The guard's stated reason ("the entity slice is replaced when an operation ends") does not apply, because Go's `Config.EncodeSavedEntity` never reads those entities. A comment is added on `rpc EncodeSavedEntity` in the protocol and the Go stubs are regenerated; no messages change, and there is no Go library or Python package code change. No new locking is needed: events reach the host fire-and-forget, and the plugin catalog counts its users. Buffering renders in Python, changing Go's encoder, and relaxing every encode or lookup were rejected.

### Milestones and tasks

- Milestone 1: the engine renders saved records while an operation runs (xcl)
  - Allow the saved-record render while an operation runs: handler change, proto comment, one test per operation (each held at its first host state-store task), error tests.
  - Prove rendering from an event handler leaves the apply unchanged.
  - Verify plugins keep running when a handler renders during an apply (external fixture plugin, one-slot event buffer; no code change expected).
  - Describe the change in the xcl changelog and in `design:libxcl.md` (revised through the design CLI).
- Milestone 2: Python programs show each resource as it is created (xcl-python)
  - Test rendering from Python event handlers during an apply, sync and asyncio, using a Python state store.
  - Document when Python programs can render configuration text (docstrings and README).
  - Release xcl with the saved-record render allowed during operations (human task).
  - Run the Python suites against the released engine.
- Milestone 3: xcl.dev says when rendering is allowed (xcl-website)
  - Update the configuration-text page, plus one sentence on the events page.

### Tasks a person must do

- Release xcl: merge the xcl PR with the `minor` release label (expected v0.12.0), which publishes the release. It must be cut before printing-tokens-over-the-engine merges to xcl main; if that is missed, the Python tests and docs ship with the Python printing release instead of the automatic engine bump.

### Out of scope

- Changing Go's encode API, events carrying rendered text, a TypeScript package.
- Allowing any other call during an operation.
- The pretty logger itself (python-prettylog spec).
- Labelled output and changes to `highlight` (printing-tokens-over-the-engine spec).
- Stopping a render near an operation's own catalog use from briefly starting and stopping external plugins.
- The version badge on xcl.dev's home page.

### Drafting assumptions

- Remove the guard at the one call site, with a comment, rather than adding a per-method policy flag to the dispatch table.
- A comment-only proto change with regenerated Go stubs.
- Core-service tests hold each operation at its first host state-store task (a blocking registry was rejected because it would hang on the catalog lock).
- The xcl release is labelled `minor` (v0.11.1 → v0.12.0).
- The xcl.dev events page is aligned too, beyond the page the spec names.
- "Every operation allows it" is tested in Go across all five operations; Python tests cover apply only.
- Python tests use a Python state store so the render is guaranteed mid-operation.
- The engine pin bump is left to xcl-python's existing automation, followed by an agent task that rebases onto the released pin.
- The plugin test is independent with no code change; if it fails, the implementer stops and asks.
- Open for implementation (the implementer stops and asks): whether the race detector reports a race on the render path; whether the external plugin is ever seen stopped during a render from a handler.

### Project-wide rules

- Decision (epic planning): this plan's xcl release is cut before printing-tokens-over-the-engine merges to xcl main.

- xcl `CHANGELOG.md`: one section per spec, headed by the spec name, ending in `**Breaking:**`.
- `design:libxcl.md` is revised only through the design CLI.
- Release order: xcl PR merged with its release label, then auto-tag and the release workflow, then the automated engine-bump PR in xcl-python merges and releases the package, then the website.
- xcl-python development runs its tests against an engine built from the xcl branch, with `xcl-ref.txt` temporarily pointing at the branch; main only pins a released tag.
- Testing: testify `require`, no table-driven tests, positive and negative cases separate, no tests that read docs, changelog or CI files, race detector on for new Go tests.
- Core service: `EncodeSavedEntity` is the only lookup allowed during an operation; `EncodeEntity`, `Entities`, `Find`, `FindByType`, `FindOne` and `Outputs` stay refused.
- Cross-spec overlap: printing-tokens-over-the-engine also edits the encode handlers in `coreservice/lookups.go`; whichever lands second rebases. Python tests pass `event_data` in whatever form the package accepts at the time.

### Manual checks

- After the release and engine bump, a fresh PyPI install with no local engine passes the sync render-from-handler check, and `xcl.engine_version()` reports the released xcl version containing the change.
- Review the xcl changelog entry.
- Review the revised `libxcl.md`: saved-record render allowed during an operation, entity-record render refused, no "every render is refused" wording left.
- Review the Python docstrings and README for both rules.
- Review the xcl.dev configuration-text and events pages for both rules, and confirm the site builds.

## 20261023074926-eaa51527-printing-and-highlighting-core
### Approach

Builds the design `printing-and-highlighting.md` (source `design`) as written. `highlight` gets `Kind`, `Token`, `Tokenize`, `Theme` (style strings, `With`, `Validate`), `DefaultTheme`, `Formatter`/`FormatterFunc`, the formats Plain, Terminal, HTML, HTMLInline and SVG, plus `WriteCSS`, `For` and `LoadVSCodeTheme`. The root package gets `PrintError`, `PrintEntity` and `PrintDiff`, the options `WithFormatter`/`WithTheme`/`WithWidth`, and `ErrorTokens`/`EntityTokens`; `EncodeOption` also works as a `PrintOption`. `diff.(*Diff).Tokens` replaces `Pieces`. The old API is deleted outright, with a one-line replacement for each in the changelog.

Because `highlight` imports `errors` and the shared-errors-package knowledge entry keeps `errors`' imports thin, the error report is laid out once, without tokens, in a new `internal/report` package: `errors` uses it for the plain `%+v` report (picked up by zap through `errorVerbose`), and the root package builds tokens from the same layout by lexing the source lines. Internal type aliases, an `init()` hook and a second plain layout were rejected. Two context lines either side of the fault are captured in `errors.NewDiagnostic`; detail is wrapped only when printing.

### Milestones and tasks

- Milestone 1: Errors show surrounding lines and print in full with `%+v`
  - Lay out error reports in one private package
  - Capture two lines of context around each fault
  - Print the full report through detailed error formatting (adds zap)
- Milestone 2: One-call printing in every format, styled by themes
  - Capture today's plain output of resources and change summaries
  - Rebuild highlighting around kinds, themes and formatters
  - Label change summaries with kinds
  - Add one-call printing of errors, resources and change summaries
  - Move every remaining caller to the new printing (core service, tests, examples' resource and diff output)
- Milestone 3: Reference outputs
  - Publish reference outputs for every built-in format
- Milestone 4: Examples, release notes and website
  - Make the example programs print errors as reports
  - Write the release notes and repository docs
  - Document the new printing on the website (xcl-website)

12 tasks: 11 in xcl, 1 in xcl-website.

### Tasks a person must do

None.

### Out of scope

- Python printing (sibling Python spec) and TypeScript.
- Proto and engine token transport, the `ConvertTheme` call, and reserving the old `highlight` field (sibling engine spec).
- Structured slog fields for configuration problems.
- Changing what a report says, beyond the restored lines.
- Changes to `SlogHandler` or the xcl-vscode extension.
- Cutting a release.
- New website pages or navigation.

### Drafting assumptions

- The design as written, with the report layout in a token-free `internal/report` package.
- Reference outputs live in `highlight/testdata/reference/` (token JSON inputs, byte-exact outputs, a default stylesheet, `classes.txt`), rewritten with an `-update` test flag.
- Examples that already decide colour with lipgloss (plugin, prettylog) keep that decision; configonly and remoteplugin use `highlight.For`.
- The core service's `highlight=true` keeps working through the Terminal format until the engine spec reserves the field.
- zap v1.27.0 is a pinned, test-only dependency of the root module (the no-new-dependencies constraint read as applying to highlighting).
- Theme validation returns a new `ErrInvalidStyle`/`InvalidStyleError` in `errors`, re-exported from the root package.
- A kind's style replaces its parent's entirely; `DefaultTheme` styles categories, plus TypeLabel and StringEscape where they must differ.
- Style words are exactly the design's set; VS Code's strikethrough is dropped and `#rgb` short hex is not accepted.
- `Style`, `Colour`, `ParseStyle` and `Theme.Style` are exported so custom formats resolve styles like the built-ins.
- `LoadVSCodeTheme` returns only what the VS Code theme styles; callers combine it with `DefaultTheme.With`.
- An unknown kind takes its parent's class and scope.
- The CSS validity check uses a small test-local parser.
- `For`'s terminal case is tested through `/dev/null`, skipped where absent.
- Milestone 2 passes through a non-compiling state between "Rebuild highlighting" and "Move every remaining caller"; each task verifies its own packages.
- The error walk keeps a plain wrapper's own message; a wrapper around configuration problems contributes only the reports.
- Website Python tabs are left for the Python spec.
- The reference report input is built from `errors.NewDiagnostic`, so no file paths leak into it.

### Project-wide rules

- Changelog: one `## <spec name>` section at the top of `CHANGELOG.md`, prose, then a `**Breaking:**` list; here with a one-line replacement for every removed function.
- Shared error types live in `errors` and are re-exported from the root package; this plan adds `ErrInvalidStyle`/`InvalidStyleError`.
- `errors`' import list stays thin: only `internal/report` is added, and `errors` never imports `highlight`.
- Cross-language contract: reference outputs in `highlight/testdata/reference/`; token JSON is `[{"kind":<int>,"text":"…"}]`; every built-in format's output, `default.css` and `classes.txt` are byte-exact. The Python spec consumes them.
- Kind numbering and HTML class names (`xcl` and `xcl-<kebab-kind>`, parent class first) are a public contract held by the reference files; the engine spec's proto enum must match the numbering.
- Go functions the engine spec builds on: `ErrorTokens(err)`, `EntityTokens`, `Diff.Tokens`, `highlight.LoadVSCodeTheme`. `ErrorTokens` returns a single message token for an error with no configuration problem.
- The core service's legacy `highlight=true` is formatted by Terminal with the default theme until the engine spec reserves it.
- Website pages are edited in place, never renamed or removed; Python tabs stay untouched for the Python spec.
- No tests read docs, the changelog or website files.
- zap is a test-only, pinned dependency of the root module.

### Manual checks

- The changelog gives a correct one-line replacement for every removed function.
- The website covers printing errors, resources and change summaries, output formats, themes, class names, stylesheet generation, custom formats and logging, each with a Go example; no page documents a removed function, and the site builds.
- The reference HTML (with the generated stylesheet), inline HTML and SVG look right in a browser; Terminal output looks right in a real terminal, including with `NO_COLOR` set through `highlight.For`.
- Running each example against a broken configuration in a real terminal shows a readable, coloured report.

## 20261023075136-42c51a7d-printing-tokens-over-the-engine
### Approach

Follows `design/printing-and-highlighting.md` as written; every change is in xcl. The protocol (`proto/xcl/core/v1/core.proto`) gains, additively: a `TokenKind` enum (`TOKEN_KIND_` plus the Go kind name, same numbers as `highlight.Kind`), a `Token { kind, text }` message, `Error.report = 5`, `bool tokens = 4` on both encode requests with `repeated Token tokens = 2` on both responses, `OperationFinished.diff_tokens = 4`, and a `ConvertTheme` call that takes no configuration. The legacy `highlight = 2` is reserved by number and name.

The report is attached in one place, `toProto` in `coreservice/errors.go`, using `xcl.ErrorTokens(err)` whenever `errors.As` finds a `*Diagnostic`, at every level of causes; that covers call results, finished operations and event errors, and matches `xcl.PrintError` by construction. Encode calls return `highlight.Tokenize(text)` when `tokens` is set; the engine's `colour()` is deleted, so an old caller still sending field 2 gets plain text. The operation runner's work function returns the `*diff.Diff`, and the finished task builds both `diff_json` and `diff_tokens`. `ConvertTheme` delegates to `highlight.LoadVSCodeTheme`; an unusable theme gets a new `invalid_theme` code. Deprecating the field, a separate report call, a second report layout built from `details`, and per-call-site reports were rejected.

### Milestones and tasks

- Milestone 1: Every configuration problem reaches other languages with its full report
  - Add token kinds, labelled text and the error report to the protocol (with a test holding the enum to `highlight.Kinds()` both ways, and the token converter).
  - Attach the full report to every error that holds a configuration problem.
- Milestone 2: Labelled resource text, labelled change summaries and editor themes over the engine
  - Return labelled resource text and retire the colour request (reserves the field, changes the buf rules).
  - Send labelled change summaries with a finished diff.
  - Convert editor themes without a configuration.
  - These three run in sequence because each regenerates `core.pb.go`.
- Milestone 3: The protocol, the other-language design and the release notes describe the new output
  - Describe reports, labelled text and theme conversion in `design:libxcl.md` (through `spektacular design author`).
  - Write the changelog entry.

### Tasks a person must do

None.

### Out of scope

- Formatting tokens in any language.
- Any change to the Python package (stubs, removing `highlight=`, `err.tokens`, `load_vscode_theme`, unknown kinds in Python) — the Python spec.
- Structured problem fields beyond what errors carry today.
- TypeScript.
- When the encode calls may run during an operation (encode-saved-entity-during-operation).
- The host-provider and DescribeTypes parts of the design.
- Website pages (xcl.dev does not document the protocol).
- Cutting a release.

### Drafting assumptions

- buf rules change: under the current `breaking: use: [FILE]` rule, reserving the field and passing the breaking check cannot both hold (tested with buf 1.27.2: `FIELD_NO_DELETE` fails even when number and name are reserved). The plan keeps FILE, excepts `FIELD_NO_DELETE`, and adds `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`; deleting a field without reserving it still fails. Flagged for the user's review.
- A report is attached whenever the error tree holds a `*Diagnostic`, at every level including causes (covers provider failures reported at a block); other errors carry no report.
- New error code `invalid_theme` for ConvertTheme, with `source` and `reason` details; an empty request gets `invalid_argument`.
- The "unknown kinds" criterion and the Python success metric are tested at the engine boundary in Go; the Python half belongs to the Python spec.
- Enum names are `TOKEN_KIND_` plus the Go `Kind.String()` name in upper snake case; no `true`/`false`/`null` kinds under Constant.
- `reserved "highlight"` as well as `reserved 2`.
- The three protocol-editing tasks run in sequence.
- The old-caller test is built from raw request bytes (field 2 added with `protowire`).

### Project-wide rules

- Changelog: one `## <spec name>` section at the top of `CHANGELOG.md`, prose, then a `**Breaking:**` list (here: the reserved `highlight` field, the removed `corepb` `Highlight`/`GetHighlight` accessors, plain text for old colour requests).
- Protocol breaking-change rule (`proto/buf.yaml`, with matching comments in `.github/workflows/go.yml` and the proto header): a field may be removed only if both its number and name are reserved.
- Kind numbering on the wire: proto `TokenKind` mirrors Go `highlight.Kind` exactly, kept in step by a descriptor test; a kind's category is `kind / 100 * 100`.
- Error codes are only ever added; this plan adds `invalid_theme`.
- `coreservice/proto/core.pb.go` is always regenerated, never merged or edited by hand.
- Overlap with encode-saved-entity-during-operation: both edit `coreservice/lookups.go`, `core.proto`, `core.pb.go`, `CHANGELOG.md` and `design:libxcl.md`; whichever lands second rebases and regenerates.
- Design revisions go through `spektacular design author`.
- Python effect: once an engine with this change is bumped into xcl-python, its tests expecting colour from `highlight=True` fail until the Python spec lands.

### Manual checks

- `buf lint` and `buf breaking` against main report nothing, locally and in the PR's CI check.
- The proto comments describe the report on errors, labelled text on encode and diff results, the reserved field, and ConvertTheme.
- The revised `libxcl.md` describes the report on errors, labelled text and theme conversion, consistent with the protocol.
- The changelog section and its Breaking list are correct.

## 20261023075302-bad71237-printing-and-highlighting-python
### Approach

Python formats the labelled text (tokens) the engine sends; it never lexes and never lays out a report. A new public package, `xcl.highlight`, holds `Kind` (an `IntEnum` mirrored by hand from the protocol and tested against it both ways) and `Token`; style strings, theme resolution and an immutable `DEFAULT_THEME` extended with `|`; `validate`, which raises `xcl.InvalidStyleError` naming every unknown word; the built-in formatters `PLAIN`, `TERMINAL`, `HTML`, `HTML_INLINE` and `SVG` behind a `Formatter` protocol; and `css`, `for_stream`, `load_vscode_theme` (through the engine's `ConvertTheme`; adds `xcl.InvalidThemeError`) and `LogFormatter`. The top-level package gains `print_error`, `format_error`, `print_diff` and `logging_handler`; both configuration flavours gain `print_entity`, `entity_tokens`, `print_saved_entity` and `saved_entity_tokens`.

`_errors.from_proto` sets `err.tokens` and adds the plain report (width 80) as an exception note, so tracebacks and `logging.exception` show the report with no extra code. Detail wrapping is an exact port of go-wordwrap v1.0.1. `highlight=` is removed from both encode calls in the same change that regenerates the stubs. Parity with Go is checked against Go's real output on every run: every format byte for byte against xcl's `highlight/testdata/reference/` files, and printed reports, entities, diffs and event logs through the existing Go/Python parity harness, extended with new stages and an `unsupported_block` fixture (the only xcl change). One flat module, lazily added notes, `textwrap` and literal "same as Go" strings were rejected.

### Milestones and tasks

- M1: formats identical to Go's, old keyword gone
  - Regenerate the protocol stubs and remove the colour keyword (xcl-python)
  - Add token kinds and themes (xcl-python)
  - Add the built-in formats and the stylesheet (xcl-python)
- M2: one-call printing, reports on unhandled and logged errors
  - Keep reports on errors and print them in one call (xcl-python)
  - Print resources and change summaries in one call (xcl-python)
  - Load VS Code themes through the engine (xcl-python)
- M3: logging, coloured log reports, parity with Go
  - Send events to standard logging (xcl-python)
  - Render logged errors in a chosen format (xcl-python)
  - Record printing and logging in the Go parity program (xcl)
  - Compare printing and logging with Go in the Python parity suite (xcl-python)
- M4: docs and release
  - Document Python printing in the README and parity list (xcl-python)
  - Document Python printing on the website (xcl-website)
  - Release the engine and the package with the printing work (human task)

13 tasks.

### Tasks a person must do

- Release the engine and the package: make sure the xcl release contains the core, engine and parity changes, pin it, merge xcl-python with `minor`, add the release-notes line naming the replacement for `highlight=`, and hold the website until the package is out.

### Out of scope

- The Python pretty logger (python-prettylog), including the website's pretty-receiver Python tab.
- Adapters for structlog or loguru.
- TypeScript.
- Any change to the engine, protocol or Go library, except the parity program.
- Lexing in Python; `Kind.scope()` / `tokenize`.
- Changing `str(err)` or `err.message`.
- A deprecation period for `highlight=`.
- Structured problem fields on log records.
- Workflow changes, new website pages, and fixing unrelated typing issues `py.typed` may surface.

### Drafting assumptions

- "Same as Go" is proven by extending the parity harness (cmd/parity plus a new fixture in xcl).
- Detail wrapping is an exact go-wordwrap port, with unit-test expectations produced by Go.
- `load_vscode_theme` returns only the kinds the theme styles; the docs show `DEFAULT_THEME | load_vscode_theme(p)`.
- `validate` raises one `InvalidStyleError` whose `.words` lists every `(kind, style, word)`.
- `print_saved_entity` and `saved_entity_tokens` are added beyond the design; they replace `encode_saved_entity(highlight=)` and the pretty logger needs them.
- The formatter seam is the `Formatter` protocol only; bare callables are not accepted.
- The note is `format_error(err)` (plain, width 80) with trailing newlines removed; every `xcl.Error` with tokens gets one, causes and event errors included.
- `logging_handler` records: `extra` uses Go's attribute names plus the event's details; details clashing with `LogRecord` attributes are skipped but reachable through `xcl_event`; the record time is the event's time; `error` is the `xcl.Error` object, with no `exc_info`.
- A non-xcl exception prints as `str(exc)` plus a newline.
- `Token.kind` is an `int`; `parent_of` and `css_class_of` handle unknown kinds; Python has no scopes.
- `print_diff` raises `ValueError` for a diff with no labelled text.
- `width` of `0` or `None` never wraps; a negative width is treated as `0`.
- Stub regeneration and keyword removal are one task, developed against the xcl branch pin.

### Project-wide rules

- Decision (user, epic planning): `str(err)` stays Go's `Error()` text, one line per problem for a multi-problem `ConfigError`; the criterion "Ordinary messages stay one line" is read as "the report never goes into `str(err)`".

- xcl-python keeps no changelog file; breaking changes get a README migration note plus a hand-added line in the GitHub release notes, and a pre-1.0 break gets the `minor` label.
- Development runs against an xcl branch with `xcl-ref.txt` pointing at it temporarily; main pins only a released tag. Order: xcl release, then the engine-bump PR (this work is pushed onto it, or replaces it), then the package release, then the website.
- The xcl parity program change must be in the xcl release that xcl-python pins; if core and engine are released first, the parity PR needs its own `patch` label.
- Python option and enum pattern: `Kind` is an `IntEnum` mirrored by hand and tested against the protocol descriptor.
- `py.typed` makes the new public annotations part of users' type checking; no new mypy test is added.
- Testing: no parametrize tables, positive and negative tests separate, no tests that read README, PARITY.md or the website; reference files and parity fixtures count as test input.
- Website pages are edited in place; this plan edits only the Python tabs, on top of the core plan's rewrite.
- Shared-file overlaps (whichever lands second rebases): `xcl/config.py`, `xcl/aio.py`, README and PARITY.md (with event-data-level and encode-saved-entity); website `examples/configuration-only.mdx`, `examples/plugins.mdx`, `events.mdx` (with event-data-level and prettylog); `configuration-text.mdx`, `problems.mdx`, `diff.mdx` (with core).
- No new runtime or test dependency is added (protobuf only).

### Manual checks

- The README and PARITY.md cover printing, reports on unhandled and logged errors, formats, themes, stylesheets, custom formats and logging, and the README has the migration note for `highlight=`.
- The website pages show Python examples for the same topics, no page still says something is "unavailable from Python", and the site builds.
- The GitHub release notes of the shipping xcl-python version contain the line naming the replacement for `highlight=`.
- In a real terminal, `for_stream` output looks right in colour and is plain with `NO_COLOR` set; HTML (with `css(DEFAULT_THEME)`), inline HTML and SVG look right in a browser.
- An uncaught configuration error shows its report in a rich traceback and through loguru's `logger.exception`, and `logging_handler` records reach structlog's `ProcessorFormatter` with their attributes.

## 20261023075328-741138ee-python-prettylog
### Approach

A new public module `xcl.prettylog`, standard library only, ported from Go's `example/prettylog`. `handler(file=None, level=logging.INFO, *, logger=None, formatter=None)` returns an ordinary `on_event` callable for both clients; `level_from_env()` reads `LEVEL_ENV = "XCL_LOG_LEVEL"`. Lines reproduce charmbracelet/log v0.4.2's text layout exactly, with Go's `SlogHandler` attribute order and Go's value formatting (checked by running Go). Each styled piece is one `Kind.TEXT` token passed through an `xcl.highlight` formatter with a one-entry theme, so colour comes from the printing support and the formatter defaults to `for_stream(file)`. Configuration comes from the delivering configuration's labelled text for the saved record, with computed values; reports come from `xcl.format_error`.

Plumbing: `Event` keeps a private `_config`; both configuration flavours get a private blocking `_saved_entity_tokens_now` that skips the asyncio poll executor, plus a stored `_event_data`; the logging bridge's level, message and record rules become shared private helpers. `xcl.aio.Config` now delivers events to `on_event` one at a time, in order (today each event is its own concurrent task). Above info it writes event lines and configuration-error reports, but no resource configuration, render warnings or guidance. A pretty `logging.Formatter` over the bridge's records, an `async def` handler with a lock, hand-written ANSI codes and literal-only parity tests were rejected.

### Milestones and tasks

- M1: asyncio handlers get events in order; handlers can reach their configuration
  - Deliver asyncio events in order (xcl-python)
  - Let a handler reach the configuration that delivered its event (xcl-python)
- M2: one line turns on pretty logging
  - Lay out event lines as Go's example writes them (xcl-python)
  - Add the pretty logger (xcl-python)
  - Prove the pretty logger's behaviour against the real engine (xcl-python)
- M3: Python's output proven to match Go's
  - Record the pretty log in the Go parity program (xcl)
  - Compare the pretty log with Go in the Python parity suite (xcl-python)
- M4: docs and ship
  - Document the pretty logger in the README and parity list (xcl-python)
  - Document the pretty logger on the website's events page (xcl-website)
  - Release the parity stage and the package with the pretty logger (human task)

### Tasks a person must do

- Release the parity stage and the package: merge the labelled PRs and publish the releases; if the printing release went out without the parity stage, this includes cutting a new xcl release. After publishing, add a line to the GitHub release notes about the asyncio client's in-order event delivery.

### Out of scope

- Changing Go's example, JSON output, log rotation.
- Configurable line styles or themes; only a `formatter=` override is offered.
- Resource configuration or guidance above info.
- Configuration beneath update or read events.
- Nanosecond durations.
- A TypeScript pretty logger, new website pages, and type errors `py.typed` surfaces elsewhere.
- An xcl changelog entry.

### Drafting assumptions

- A stdlib port of Go's example built on the printing support.
- Decision (user, epic planning): one `EncodeSavedEntity` render per created resource inside the handler is allowed; the constraint forbids waiting on the operation, which deadlocks.
- Lines reproduce charmbracelet/log's text format, with the time in local time.
- asyncio delivers events in order through one consumer per operation; handler exceptions are still dropped.
- The level is a `logging` int; anything else, including bools and level names, raises `TypeError`.
- Colours are named style strings: levels bold blue, cyan, yellow and red; source, resource and operation magenta, cyan and yellow with bold values; other keys and `=` dim.
- A report is shown once per problem: beneath parse or resource error events, and on the operation's closing error event only if it carries more problems than were already shown.
- Decision (user, epic planning): above info, no resource configuration blocks, but configuration-error reports still show beneath their error line at every level (differs from Go's example).
- The guidance line is info level, written once per handler, only when a `create success` has no data and the delivering configuration was created with `EventDataLevel.NONE`.
- With `logger=`, each event becomes one record whose message is the full pretty text (line plus block), at the event's level and time, carrying the bridge's fields; passing both `file` and `logger` raises `ValueError`; the default formatter in logger mode is `PLAIN`.
- The handler never raises: render failures become a warning line and stream write errors are swallowed.
- The success metric is checked by a new `pretty_log` parity stage.
- Durations are normalised like timestamps whenever two runs are compared.
- The "unrenderable resources warn" half of that criterion is tested through the fake transport.
- Layout unit tests compare against Go-produced expected lines pasted in as literals.
- The website gets one sentence on asyncio ordering under "Ordered, one at a time".
- Open for implementation (the implementer stops and asks): whether Go in-process and the Python engine path emit the same events and value formatting for the parity fixtures; whether the landed printing support matches its plan's shapes (the TERMINAL formatter must write a one-token, one-style stream as that style's codes followed by a reset).

### Project-wide rules

- xcl CHANGELOG: no section for this spec; the only xcl change is a parity-program test stage, following the printing plan's precedent.
- xcl-python release: `minor` label; the asyncio ordering change is described in the README and in a hand-added line in the GitHub release notes (decided during epic planning).
- `bindings/libxcl/go.mod` gains charmbracelet/log v0.4.2, lipgloss v1.1.0 and termenv v0.16.0, required only by `cmd/parity`, with a comment saying why; the parity program imports `example/prettylog` through a replace.
- The parity record gains a `pretty_log` key: sorted line-plus-block groups with time, duration, colour and paths normalised, and `null` for fixtures whose apply fails.
- xcl-python dev pins `xcl-ref.txt` to the xcl branch; main pins only a released tag. Order: xcl release (with the parity stage) → engine bump → package release → website.
- Option pattern: the level uses stdlib `logging` ints, not a new StrEnum; wrong types raise `TypeError`.
- Shared-file overlaps (whichever lands second rebases): xcl-python `xcl/config.py`, `xcl/aio.py`, `xcl/_poll.py`, `xcl/events.py`, `xcl/_logbridge.py`, `xcl/__init__.py`, README.md, PARITY.md, `tests/parity/run_python.py`; xcl `bindings/libxcl/cmd/parity/main.go`, `main_test.go`, `testdata/parity/README.md`; xcl-website `src/pages/events.mdx`.
- Testing: no parametrize tables, positive and negative tests in separate functions, no tests that read README, PARITY.md or the website.

### Manual checks

- README: one-line setup for both clients, the logger form, `XCL_LOG_LEVEL`, the processed event data needed, a sample of the output, and the asyncio ordering note.
- Website events page: the Python tabs show `xcl.prettylog`'s setup and a sample; no hand-written receiver remains; URLs still work; the site builds.
- Real terminal: levels coloured, source, operation and resource highlighted, configuration and report coloured; plain with `NO_COLOR` set or when redirected to a file.
