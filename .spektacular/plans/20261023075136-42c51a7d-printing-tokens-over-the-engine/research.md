---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023075136-42c51a7d-printing-tokens-over-the-engine

## Alternatives considered and rejected

### Option A: Deprecate the legacy `highlight` field instead of reserving it

Keep `bool highlight = 2 [deprecated = true]`, ignored by the engine. It passes `buf breaking` under the current `FILE` category with no config change.

**Rejected**: the spec's constraint says the field "is reserved and never reused", and the design (`design/printing-and-highlighting.md`, Protocol section) says "The old `highlight` field is reserved, never reused". A kept field would do nothing in every generated stub.

### Option B: Reserve the field and leave `proto/buf.yaml` on `breaking: use: [FILE]`

**Rejected** on evidence. A probe in a scratch copy of `proto/` (2026-10-23, buf 1.27.2) replaced `bool highlight = 2;` with `reserved 2; reserved "highlight";`, and `buf breaking --against <old>` failed with `Previously present field "2" with name "highlight" on message "EncodeSavedEntityRequest" was deleted.` (exit 100). `FIELD_NO_DELETE` is in the FILE category and fires even when number and name are reserved. The spec requires both reservation and a passing check.

### Option C: Switch the breaking category to `WIRE_JSON`

It allows reserved deletions.

**Rejected**: it drops FILE-only checks (for example `FILE_SAME_PACKAGE` and `FILE_SAME_GO_PACKAGE`). The narrower change (FILE minus `FIELD_NO_DELETE`, plus the two `_UNLESS_*_RESERVED` rules) keeps them.

### Option D: A separate call that returns an error's report

**Rejected** by the design's decision "The report travels on every `Error`, not through a separate call", and by the spec's success metric (zero extra engine calls).

### Option E: Lay the report out in `coreservice` from the existing `details`

**Rejected**: the spec keeps layout in the core, done once when an error is converted. The dependency plan's `xcl.ErrorTokens` is what `xcl.PrintError` uses, so equality with Go holds by construction. A second layout could drift.

### Option F: Return labelled text on encode calls always, with no request flag

**Rejected**: the design's message shapes add `bool tokens = 4` to both encode requests, so an old caller that never asks pays nothing.

### Option G: Use `xcl.EntityTokens` in the encode handlers

**Rejected**: the handlers hold a record (bytes), not a Go entity. `Config.EncodeSavedEntity` and `EncodeEntityRecord` return text, and `highlight.Tokenize(text)` labels it, which is what `EntityTokens` does internally (dependency plan context: "`EntityTokens` = `EncodeEntity` then `highlight.Tokenize`").

### Option H: Map an unusable theme to `internal`

**Rejected**: every other user-input failure has a stable code, and a binding should raise a specific exception. A new `invalid_theme` code is additive (the proto header and `coreservice/errors.go:14-15` say codes are only added).

### Option I: Build reports at each call site (results, finished task, events)

**Rejected**: that is three places to keep in step, and it would miss causes. `toProto` (`coreservice/errors.go:84`) is already the single conversion point and already recurses into causes.

## Chosen approach — evidence

- `coreservice/errors.go:84-331` — `toProto` is the single place every error crosses (`service.go:96,102` call results, `operations.go:61` finished task, `events.go:30` event errors). Adding the report there covers responses, finished operations and event-handler errors at once. Causes recurse through `toProto` (`errors.go:129-131`), so each collected diagnostic gets its own report too.
- `errors/config_error.go` — `ConfigError.Unwrap() []error` and `errors/diagnostic.go:162,218` (`Diagnostic.Unwrap`, `Diagnostics.Unwrap() []error`) mean `errors.As(err, &*Diagnostic)` finds a diagnostic anywhere in the tree; that is the "holds a configuration problem" test.
- Dependency plan `20261023074926-eaa51527-printing-and-highlighting-core` (final): `xcl.ErrorTokens(err)` returns the unwrapped report built from `internal/report.Layout`, `nil` for nil, and a single `Text` message token for an error with no diagnostic; `PrintError` = `ErrorTokens` + detail wrap + formatter. So `ErrorTokens` joined equals `PrintError(..., WithWidth(0))` plain.
- Dependency plan: `highlight.Kind` numbering is fixed (Text 0 … Report 1400, Summary 1401 … Detail 1404), `highlight.Kinds()` lists every known kind in numeric order, `Kind.String()` is the Go name, `Kind.Parent()` is `k/100*100`; `highlight.Tokenize`, `(*diff.Diff).Tokens()`, `highlight.LoadVSCodeTheme(io.Reader) (Theme, error)` returning `*errors.InvalidThemeError` exist. `coreservice/lookups.go:189-197` `colour` is left formatting through `highlight.Terminal`, for this plan to remove.
- `coreservice/service.go:202-227` — handlers map, `unary(false, …)` for handle-0 methods (GetVersion, CreateConfig); `service_test.go:284-315` holds the map to the descriptor, so `ConvertTheme` must be added to both.
- `coreservice/operations.go:29-66,135-152` — `start(work func(ctx) (diffJSON []byte, err error))` posts `OperationFinished{Error, DiffJson}`; `startDiff` marshals the `*diff.Diff` inside `work`. Diff tokens need the `*diff.Diff`, so `work` returns the diff instead of its JSON.
- `config.go:658-666` — an operation's final event carries the operation's error (`PhaseError`), and `internal/parser/parser.go:1690-1710` emits a validate error event per problem with the `*Diagnostic`; both reach `eventToProto` → `toProto`.
- buf probe (scratch copy, buf 1.27.2): with `breaking: use: [FILE, FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED, FIELD_NO_DELETE_UNLESS_NAME_RESERVED]`, `except: [FIELD_NO_DELETE]`, reserving `2` and `"highlight"` passes (exit 0); dropping the name reservation fails ("deleted without reserving the name"), dropping the number fails ("without reserving the number"). CI (`.github/workflows/go.yml:30-55`, buf 1.50.0) runs `buf lint` and `buf breaking --against …#branch=main,subdir=proto` from the branch, which reads the branch's `proto/buf.yaml`.
- `proto/buf.yaml` lint excepts `ENUM_VALUE_PREFIX` and `ENUM_ZERO_VALUE_SUFFIX`; the design's `TOKEN_KIND_*` names pass `ENUM_VALUE_UPPER_SNAKE_CASE`, and the `TOKEN_KIND_` prefix keeps enum values unique in the package (proto3 C++ scoping: `EVENT_DATA_*`, nested `StateTask.Kind`).
- proto3 enums are open: a `TokenKind` number the receiver does not know is kept as its integer when decoded (Go: `protoreflect.EnumNumber`; Python: plain int), so an older SDK still gets the number and can compute the parent.
- Unknown proto fields are kept and ignored on decode, so an old caller's request with field 2 set to `true` still parses as `EncodeSavedEntityRequest` once 2 is reserved; `protowire.AppendTag/AppendVarint` builds such bytes in a test.

## Files examined

- `proto/xcl/core/v1/core.proto:1-329` — protocol: header contract comment, `Error` (fields 1-4), `OperationFinished` (diff_json = 3), encode requests with `highlight = 2`, responses `text = 1`, "Lookups, between operations." comment.
- `proto/buf.yaml` — lint DEFAULT with excepts; `breaking: use: [FILE]`; comment says buf refuses removing a field.
- `buf.gen.yaml`, `buf.work.yaml`, `Makefile:21-33` — `make core-protos` (`buf generate`), `make buf-lint`, `make buf-breaking` (`--against '.git#branch=main'`).
- `.github/workflows/go.yml:30-55` — CI lint and breaking steps; comment on what breaking refuses.
- `coreservice/errors.go:14-331` — codes, `codedError`, `toProto`.
- `coreservice/errors_test.go:211-380` — style of error tests; `unsupportedDiagnostic` fixture; `TestConfigErrorListsCausesWithFileAndLine` applies a broken config directly.
- `coreservice/lookups.go:121-197` — encode handlers, `encodeAllowed`, `encodeOptions`, `colour`.
- `coreservice/lookups_test.go:150-215,410-460` — highlight tests (`TestEncodeSavedEntityHighlightAddsColour`, `TestEncodeEntityHighlightAddsColour`, `TestEncodeSavedEntityHighlightStripsToPlainText`, `TestEncodeEntityHighlightMatchesSavedRecord`), helpers `encodeEntity`, `encodeSavedEntity`, `stripColour`, `appRecord`.
- `coreservice/operations.go:29-152` — `start`, `startDiff`, finished task.
- `coreservice/operations_test.go:32,262-276,614-654,920-932` — `bogusConfig`, `applyThenDiffNetwork`, `TestDiffMatchesGoDiff`, `TestValidateInvalidConfigFinishedHasConfigError`.
- `coreservice/events.go:20-45` — `eventToProto` uses `toProto`.
- `coreservice/service.go:72-227` — `Call`, handler map, `unary`.
- `coreservice/service_test.go:1-300` — `testClient` (`call`, `ok`, `fail`, `start`, `drive`), `appRequest`, `appliedApp`, `writeConfig`, descriptor tests.
- `errors/config_error.go`, `errors/diagnostic.go:22-221` — unwrap behaviour.
- `errors/highlight_errors.go` — `ErrInvalidTheme`, `InvalidThemeError{Source, Reason, Err}`.
- `config.go:630-690`, `internal/parser/events.go:160-200`, `internal/parser/parser.go:1690-1710` — events carrying errors.
- `CHANGELOG.md:1-20` — `## <spec name>` section, prose, `**Breaking:**` list.
- `docs/overview.md:130`, `docs/README.md:53` — highlight mentions (the dependency plan rewrites them; nothing protocol-specific).
- `xcl-python:Makefile:13-22` — Python stubs generated from `$(XCL_SRC)/proto` with protoc into `xcl/_core/`.
- `xcl-python:xcl/config.py:221-288`, `xcl/aio.py:266-296`, `tests/integration/test_sync.py:788-1262`, `tests/integration/test_aio.py:715-737`, `tests/unit/test_config.py:305-416` — Python's `highlight=` parameter and tests that expect colour; they belong to the Python spec.
- `xcl-website` — no page documents the CoreService protocol (grep for CoreService, core.proto, EncodeSavedEntityRequest, xcl-core, libxcl in `src`: none).
- `design:libxcl.md` (draft) — Error model (lines ~143-191), Threading, CoreService listing, Lookups/encode messages (~461-491), OperationFinished (~393-397), code generation (~503-641).

## External references

- buf breaking rules and categories (buf docs, "Rules and categories"): `FIELD_NO_DELETE` is in FILE/PACKAGE; `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED` are in WIRE_JSON (and WIRE for the number). Why: decides the buf.yaml change; confirmed by the local probe.
- Protocol Buffers language guide, "Reserved fields" and "Enum: unrecognised values" (proto3 enums are open). Why: old callers' field 2 is ignored, unknown kinds survive decoding.

## Prior plans / specs consulted

- `20261023074926-eaa51527-printing-and-highlighting-core` (plan, final) — provides `ErrorTokens`, `EntityTokens`, `Diff.Tokens`, `Kind`, `Kinds()`, `Tokenize`, `LoadVSCodeTheme`, reference outputs; leaves coreservice's `highlight=true` on Terminal; changelog form.
- `20261023074627-cc23fc08-encode-saved-entity-during-operation` (plan, final) — splits `encodeAllowed` in `coreservice/lookups.go`, adds a comment to the `EncodeSavedEntity` rpc, regenerates `core.pb.go`, adds a changelog section and revises `design:libxcl.md` (Threading, error table, encode paragraph). Overlaps this plan in `lookups.go`, `core.proto`, `core.pb.go`, `CHANGELOG.md`, `libxcl.md`; whichever lands second rebases.
- `20261016064106-8931613a-python-encode-entity` (changelog entry only) — introduced `EncodeEntity` with `highlight`; shows changelog prose style.

## Open assumptions

- The dependency plan lands first and its API matches its plan (`ErrorTokens` returns nil for nil and a message token for non-diagnostic errors; `Kinds()` exists and lists every kind; `LoadVSCodeTheme` returns `*errors.InvalidThemeError`). If a name differs, the implement workflow adapts to the shipped name; if behaviour differs (for example `ErrorTokens` wraps detail), STOP and ask.
- `buf` 1.50.0 in CI behaves as 1.27.2 did locally for the reserved-field rules. If CI's breaking check still fails with the adjusted rules, STOP and ask.
- `Config.ApplyContext` returns the `*ConfigError` unwrapped (the dependency plan verified this for Apply/Validate), so the engine's finished error and Go's direct `Apply` error are the same tree.
- The Python package regenerates its stubs only in the Python spec; until then the Python package keeps sending `highlight` (field 2), which the engine ignores, and its integration tests that expect colour will fail against an engine built from this change. That is the Python spec's work, not this plan's.

## Drafting assumptions

### Chosen direction: report in toProto, tokens on existing calls, narrow buf rule change (architecture)
- **Decision**: attach `Error.report` once in `coreservice/errors.go` `toProto` from `xcl.ErrorTokens`; return `highlight.Tokenize(text)` on the encode responses when `tokens` is set; have the operation runner's work return `*diff.Diff` so the finished task carries `diff_json` and `diff_tokens`; add handle-0 `ConvertTheme` over `highlight.LoadVSCodeTheme`; reserve `highlight = 2` by number and name, with `proto/buf.yaml` allowing only reserved deletions.
- **Rationale**: one conversion point covers responses, finished operations, events and causes; the labels come from the one lexer and report layout, so plain renderings equal Go's by construction; the design fixes the message shapes.
- **Rejected**: building reports at each call site (three places, misses causes); a separate report call (design decision); a `{diffJSON, tokens}` pair computed inside `startDiff` (works, but the runner then carries two derived values instead of the one diff they come from).

### Conventions selected (architecture)
- **Decision**: apply testing & mocking, code style, shared errors package, dependencies, project structure; drop the rest as listed in conventions.md.
- **Rationale**: these are the conventions the touched surfaces (proto, coreservice, tests, errors mapping) bear on.
- **Rejected**: listing every convention (noise).

### Adjust buf breaking rules so a reserved field may be removed (discovery)
- **Decision**: `proto/buf.yaml` keeps the FILE category, excepts `FIELD_NO_DELETE`, and adds `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`. The legacy `highlight` field is reserved by number and name.
- **Rationale**: the spec requires the field to be reserved and the breaking check to pass; a probe showed FILE's `FIELD_NO_DELETE` fails on a reserved deletion. The narrow change still refuses any deletion that does not reserve both number and name, which is the repo's stated rule ("never renamed or reused").
- **Rejected**: deprecating the field instead (contradicts the spec and design); switching to WIRE_JSON (drops FILE checks).

### Report attached when the error tree holds a diagnostic (discovery)
- **Decision**: `toProto` sets `report = xcl.ErrorTokens(err)` when `errors.As(err, *Diagnostic)` succeeds, at every level, including causes; otherwise `report` stays empty.
- **Rationale**: the spec: errors with a configuration problem carry the report, others carry none; `ErrorTokens` returns a message token for other errors, so the engine decides.
- **Rejected**: attaching only for code `config` (misses provider diagnostics, code `provider`).

### New error code `invalid_theme` for ConvertTheme (discovery)
- **Decision**: an unusable VS Code theme returns code `invalid_theme` with `source` and `reason` details; an empty request returns `invalid_argument` (`argument=vscode_theme_json`).
- **Rationale**: codes are only added; a binding can raise a specific error; the design names no error.
- **Rejected**: `internal` (not actionable), `invalid_argument` for a malformed theme (loses the reason as a detail).

### Unknown-kind and Python acceptance criteria tested at the engine boundary (discovery)
- **Decision**: this plan tests that kind numbers cross unchanged and an unrecognised number survives decoding and resolves to its category's style in Go; Python's own handling is the Python spec's.
- **Rationale**: no language package is changed here (Non-Goals; the Python spec depends on this one).
- **Rejected**: changing xcl-python in this plan (the Python spec owns stubs, `highlight=` removal and printing).

### Enum value names follow the Go kind names (data_structures)
- **Decision**: `TOKEN_KIND_` + the Go `Kind.String()` name in upper snake case, one value per `highlight.Kinds()` entry and nothing else; a test holds both directions.
- **Rationale**: the design shows `TOKEN_KIND_TEXT`, `TOKEN_KIND_BLOCK`, `TOKEN_KIND_BLOCK_TYPE`; deriving the rest the same way makes the parity test mechanical.
- **Rejected**: hand-picked names (drift), adding values for `true`/`false`/`null` under Constant (the dependency plan defines no such kinds).

### Reserve the name as well as the number (data_structures)
- **Decision**: `reserved 2; reserved "highlight";` on both encode requests.
- **Rationale**: the adjusted buf rules require both; reserving the name also stops JSON-mapping reuse.
- **Rejected**: number only (the design's literal `reserved 2;`), which the adjusted rules refuse.

### Protocol-editing tasks run one after another (tasks)
- **Decision**: the encode, diff and theme tasks are chained (encode → diff → theme) even though their logic is independent.
- **Rationale**: each edits `core.proto` and regenerates `core.pb.go`; running them in parallel would conflict in the generated file.
- **Rejected**: parallel tasks with a merge step (generated-file conflicts).

### Old-caller compatibility proved with raw request bytes (tasks)
- **Decision**: the old-caller tests append field 2 (varint 1) to a marshalled request with `protowire` and call the service with those bytes.
- **Rationale**: once the field is reserved, the generated type cannot set it; raw bytes are exactly what an old binding sends.
- **Rejected**: keeping a copy of the old generated types for the test (extra generated code).

## Rehydration cues

- `spektacular spec file read 20261023075136-42c51a7d-printing-tokens-over-the-engine`
- `spektacular design read --data '{"source":"design","path":"printing-and-highlighting.md"}'` (Protocol, Token kinds, Decisions)
- `spektacular design read --data '{"source":"design","path":"libxcl.md"}'` (Error model, Lookups, OperationFinished)
- `spektacular plan file read 20261023074926-eaa51527-printing-and-highlighting-core plan` and `context` (sections "Add one-call printing…", "Rebuild highlighting…", "Move every remaining caller…")
- `spektacular plan file read 20261023074627-cc23fc08-encode-saved-entity-during-operation plan`
- Re-read `coreservice/errors.go`, `coreservice/lookups.go`, `coreservice/operations.go:29-152`, `coreservice/service.go:202-227`, `proto/xcl/core/v1/core.proto`, `proto/buf.yaml`.
- Re-run the buf probe: copy `proto/` twice into a scratch dir, reserve field 2 in one, `buf breaking --against <other>`.
