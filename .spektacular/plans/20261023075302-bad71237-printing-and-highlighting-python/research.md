---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Research: 20261023075302-bad71237-printing-and-highlighting-python

## Alternatives considered and rejected

- **Render formats in the engine and send finished bytes to Python** (today's `highlight=True` model). Rejected: the design `design:printing-and-highlighting.md` ("Decisions": built-in formatters are native, themes never cross the wire) and the spec constraint "formats the labelled text returned by the bundled xcl engine" fix formatting in Python. Custom formats and themes could not work over finished bytes (`xcl-python:xcl/config.py:251-257` shows the current model).
- **Lex configuration in Python** (a Python port of `highlight.Tokenize`). Rejected by the spec constraint "must not lex configuration or lay out reports itself" and the design's "What lives where" table (lexing stays in the core). So `xcl.highlight` has no `tokenize`, and no `Kind.scope()` either: Python's VS Code import goes through the engine's `ConvertTheme`, the only use of scopes (design, "Token kinds").
- **Python's `textwrap` for detail wrapping.** Rejected: `textwrap.fill` collapses runs of whitespace, treats `\x1c-\x1f` as space and breaks long words differently from `github.com/mitchellh/go-wordwrap` v1.0.1 (`~/go/pkg/mod/github.com/mitchellh/go-wordwrap@v1.0.1/wordwrap.go:15-84`), which Go's printing uses (`xcl:internal/report` per the core plan). Byte-identical reports need a port of that function, with Go's `unicode.IsSpace` and the no-break-space exception.
- **Put the report in `str(err)`.** Rejected by the spec requirement "Ordinary messages stay one line" and the design ("`str(err)` stays one line"). The report goes in an exception note (`BaseException.add_note`, Python 3.11+, the package minimum `xcl-python:pyproject.toml:10`).
- **Make `str(err)` collapse a multi-problem message to one line.** Rejected: the engine's message for a configuration error holding several problems is Go's `ConfigError.Error()`, one line per problem (`xcl:errors/config_error.go:41-50`, sent as `Message: err.Error()` at `xcl:coreservice/errors.go:84`). The design author describes that same `str(err)` as "stays one line", so the plan keeps `Error.__str__` as the engine's message (`xcl-python:xcl/_errors.py:50-51`), identical to Go's `Error()`. **Recorded decision:** the user confirmed during epic planning that "Ordinary messages stay one line" means "the report never goes into `str(err)`", and that `str(err)` matches Go, one line per problem for a multi-problem `ConfigError`.
- **Literal expected strings in Python tests for "the same as Go" criteria.** Rejected in favour of extending the existing Go/Python parity harness (`xcl:bindings/libxcl/cmd/parity/main.go:202-258`, `xcl-python:tests/parity/test_parity.py`), which already exists so that "the two give the same results on a shared set of fixtures" (`xcl-python:PARITY.md:3-7`). Literals only prove agreement with Go at the moment they were written.
- **A dedicated adapter for structlog or loguru.** Rejected: spec Non-Goal; the design relies on their stdlib integration.
- **Return the VS Code theme merged with the default.** Rejected: Go's `highlight.LoadVSCodeTheme` returns only the kinds the theme styles (core plan, `highlight/vscode.go` notes). Python mirrors Go, and the docs show `DEFAULT_THEME | load_vscode_theme(path)` for a fallback.
- **An `ExceptionGroup` (or returned list) from theme validation.** Rejected for one `xcl.InvalidStyleError` that names every unknown word: catchable with a plain `except`, and still lists all words as Go's `errors.Join` does.
- **Accept bare callables as formatters** (a `FormatterFunc` analogue). Rejected: the design fixes the Python seam as a `Formatter` protocol with `format(out, theme, tokens)`; one shape keeps every print function's annotation exact under `py.typed`.

## Chosen approach — evidence

- Engine payloads this plan consumes (plan `20261023075136-42c51a7d-printing-tokens-over-the-engine`, "Data Structures & Interfaces"): `TokenKind` enum, `Token{kind,text}`, `Error.report = 5` on every error and cause that holds a configuration problem (unwrapped detail), `tokens = 4` on `EncodeEntityRequest`/`EncodeSavedEntityRequest` with `repeated Token tokens = 2` on the responses, `OperationFinished.diff_tokens = 4`, handle-0 `ConvertTheme` (`vscode_theme_json` → `map<int32,string> styles`), new error code `invalid_theme` (`source`, `reason` details), legacy `highlight = 2` reserved.
- Formatter contract (plan `20261023074926-eaa51527-printing-and-highlighting-core`, context "Rebuild highlighting…"): style words and palette, `Style` resolution (kind, then parent `k // 100 * 100`, then `Text`), Terminal SGR order and per-line reset, HTML class contract and escaping, HTMLInline declaration order, SVG geometry, CSS rule order and resets, `DefaultTheme`. Published as byte-exact files under `xcl:highlight/testdata/reference/` (`report|entity|diff` `.tokens.json`, `.plain.txt`, `.terminal.txt`, `.html`, `.inline.html`, `.svg`, plus `default.css`, `classes.txt`).
- Python reaches the xcl checkout through one fixture: `xcl-python:tests/conftest.py:24-29` (`xcl_src`) and `xcl-python:tests/xcl_source.py:28-34,72-79`; CI checks out xcl at the pinned ref and sets `XCL_SRC` with strict mode (`xcl-python:.github/workflows/test.yml:69-110`).
- Stubs are generated by `make protos` from the xcl checkout and committed (`xcl-python:Makefile:13-22`, `git ls-files xcl/_core`); CI regenerates them before testing (`test.yml:93-96`), so once the pin moves to an engine with the reserved field, `EncodeEntityRequest(highlight=…)` stops constructing — the removal must land with the stub regeneration.
- Every engine error becomes a Python exception in one place: `xcl-python:xcl/_errors.py:458-464` (`from_proto`), used by calls (`_client`), finished operations (`xcl-python:xcl/_poll.py:124-132`) and events (`xcl-python:xcl/events.py:184`). Adding `tokens` and the note there covers every path.
- The finished diff is built in one place: `xcl-python:xcl/_poll.py:131-132` → `Diff._from_json` (`xcl-python:xcl/diff.py:67-101`); both clients go through `Operation.finish` (`xcl-python:xcl/aio.py:148`).
- Encode calls: `xcl-python:xcl/config.py:213-286` and `xcl-python:xcl/aio.py:258-296` (the aio flavour imports `_encode_options` from `config`, `aio.py:29`).
- Go's logging bridge to mirror: `xcl:events/slog.go:9-118` (levels, `"<operation> <phase>"` messages, attribute names source, operation, phase, resource, type, file, duration, error, then meta keys in order, level/message excluded for log events, `Enabled` checked first, event time used).
- Option pattern and typing: plan `20261023074223-e06f835f-event-data-level-enum` (top-level `StrEnum` named after the Go type, refused by type with `TypeError`; ships `xcl/py.typed`, so public annotations are read by users' type checkers).
- Test harness: fake transport `xcl-python:tests/fakes.py:13-57`, unit style `xcl-python:tests/unit/test_config.py:305-330`, docstring walk `xcl-python:tests/unit/test_docstrings.py:17-56` (extend its module list with `xcl.highlight`).

## Files examined

- `xcl-python:xcl/_errors.py:25-51` — `Error(message, *, code, details, causes)`, `__str__` returns the message; add `tokens` and the note here.
- `xcl-python:xcl/_errors.py:206-238` — `ConfigError.problems` from causes' details; unchanged.
- `xcl-python:xcl/_errors.py:380-393` — `RemovedNameError`, the precedent for binding-raised errors with their own code.
- `xcl-python:xcl/_errors.py:418-464` — `_BY_CODE` registry and `from_proto`; `invalid_theme` is added here.
- `xcl-python:xcl/config.py:213-286` — `encode_entity`/`encode_saved_entity` with `highlight=`, to remove; add the print and tokens methods beside them.
- `xcl-python:xcl/config.py:302-309` — `_encode_options`, shared with aio.
- `xcl-python:xcl/aio.py:258-296` — async encode twins.
- `xcl-python:xcl/aio.py:131-148` — async drive ends in `op.finish()`.
- `xcl-python:xcl/_poll.py:124-132` — `Operation.finish` builds the `Diff`.
- `xcl-python:xcl/diff.py:59-101` — frozen `Diff` with `raw` (compare=False); gains `tokens`.
- `xcl-python:xcl/events.py:138-209` — `Event` fields; its `error` comes through `from_proto`.
- `xcl-python:xcl/__init__.py:12-115` — top-level exports and `__all__`.
- `xcl-python:xcl/_client.py` — `default_client()`; handle-0 calls (`CreateConfig`) are the pattern for `ConvertTheme`.
- `xcl-python:Makefile:13-22` — `make protos`.
- `xcl-python:pyproject.toml:27-33` — only runtime dependency is protobuf.
- `xcl-python:xcl/_version.py:14-16`, `xcl-python:xcl-ref.txt` (`v0.11.1`) — engine pin.
- `xcl-python:.github/workflows/engine-bump.yml:1-20` — the automated pin bump opens a PR that stays open if checks fail.
- `xcl-python:.github/workflows/test.yml:69-110` — CI checks out xcl at the pin, runs `make protos engine plugins`, strict tests.
- `xcl-python:tests/xcl_source.py:28-79`, `xcl-python:tests/conftest.py:24-29` — how tests reach the xcl checkout.
- `xcl-python:tests/fakes.py:13-57` — scripted fake transport for unit tests.
- `xcl-python:tests/unit/test_config.py:305-330,412-421,483-519` — tests asserting `request.highlight`, to remove.
- `xcl-python:tests/integration/test_sync.py:788-800,1234-1265` and `xcl-python:tests/integration/test_aio.py:715-740` — colour tests to remove.
- `xcl-python:tests/unit/test_docstrings.py:17-63` — public-docstring walk.
- `xcl-python:tests/parity/test_parity.py:1-107`, `xcl-python:tests/parity/run_python.py:1-60,214` — the Python parity program and comparison.
- `xcl-python:PARITY.md:22-50` — Lookups table and the `highlight=True` paragraph to rewrite.
- `xcl-python:README.md:257,291` — `highlight=True` examples to replace.
- `xcl:bindings/libxcl/cmd/parity/main.go:202-258` — the Go parity record stages; new printing and logging stages go here.
- `xcl:bindings/libxcl/cmd/parity/main.go:310-370` — `outcome`/`problem` normalisation, `n.text` path replacement.
- `xcl:bindings/libxcl/testdata/parity/README.md` — fixture layout; documents each record section.
- `xcl:bindings/libxcl/testdata/parity/invalid/main.xcl` — existing failing fixture (unsupported argument, missing required field).
- `xcl:bindings/libxcl/go.mod:1-16` — separate module requiring the root module at the same tag, `replace ../..` in development.
- `xcl:events/slog.go:9-118` — Go logging bridge to mirror.
- `xcl:errors/config_error.go:41-50`, `xcl:errors/diagnostic.go:131-150` — `Error()` strings.
- `xcl:coreservice/errors.go:79-140` — `toProto`: message is `err.Error()`, causes recursed.
- `~/go/pkg/mod/github.com/mitchellh/go-wordwrap@v1.0.1/wordwrap.go:15-84` — the wrapping algorithm to port.
- `xcl-website:src/pages/problems.mdx:72-144` — Python tab and "The report is not yet available from Python" (:132-134).
- `xcl-website:src/pages/configuration-text.mdx:326-479` — Python tabs using `highlight=True`, "not yet available" for editor themes (:423) and renderers (:470).
- `xcl-website:src/pages/diff.mdx:52-67,287-289` — "not yet available from Python" for rendered diffs.
- `xcl-website:src/pages/events.mdx:27-84` — Python tab hand-writing a logging handler.
- `xcl-website:src/pages/examples/plugins.mdx:1296-1319,1409-1440` — Python `plan`/`inspect` listings say diff rendering is unavailable.
- `xcl-website:src/pages/examples/configuration-only.mdx:638-660` — Python program lets errors propagate.

## External references

- PEP 678 / `BaseException.add_note` (Python 3.11) — exception notes, printed by `traceback`, `logging`'s default `formatException`, rich and loguru after the message; the mechanism for reports on unhandled and logged errors.
- `traceback.TracebackException` (3.11: captures `__notes__` at construction) — lets `LogFormatter` format a traceback without the plain note and append the formatted report instead.
- `logging.Logger.makeRecord` / `LogRecord` reserved attributes — `extra` keys may not overwrite `message`, `asctime` or any `LogRecord` attribute (KeyError), which shapes the bridge's attribute names.
- no-color.org — any non-empty `NO_COLOR` disables colour; matches Go's `highlight.For`.
- `types.MappingProxyType.__or__` (3.9+) — `MappingProxyType | dict` returns a new dict, so an immutable `DEFAULT_THEME` still extends with `|`, as the design shows.
- `mitchellh/go-wordwrap` v1.0.1 — the wrap algorithm Go uses for report details.
- structlog (`ProcessorFormatter`, `ExtraAdder`) and loguru (`InterceptHandler` / `logger.add(handler)`) stdlib integrations — why no adapter is needed; also why the bridge does not use an `extra` key named `event` (structlog's message key).

## Prior plans / specs consulted

- `20261023074926-eaa51527-printing-and-highlighting-core` (plan, final) — Go formatter rules, default theme, reference-output file set and JSON shape, class contract; left the website Python tabs and "not available from Python" statements for this spec.
- `20261023075136-42c51a7d-printing-tokens-over-the-engine` (plan, final) — wire shapes and codes; leaves to this spec the stub regeneration, `highlight=` removal and colour tests, `err.tokens`, `load_vscode_theme`, unknown kinds in Python; warns that an engine bump fails colour tests until this lands.
- `20261023074223-e06f835f-event-data-level-enum` (plan, final) — option pattern, `py.typed`, no changelog file in xcl-python (README migration note + human-added release-note line, `minor` label), mypy test pattern.
- `20261023074627-cc23fc08-encode-saved-entity-during-operation` (plan, final) — dev against an xcl branch with `xcl-ref.txt` temporarily pointing at it; release order xcl → auto-tag → engine-bump PR → package release → website; overlapping docstrings/README/tests in xcl-python.
- `20261023075328-741138ee-python-prettylog` (spec, downstream) — renders each created resource from the event's saved record with this spec's printing support; motivates `print_saved_entity`/`saved_entity_tokens`.

## Open assumptions

- The core and engine plans land as written: `xcl:highlight/testdata/reference/` exists with the file set described, Go's `PrintError`/`PrintEntity`/`PrintDiff` exist, and the engine sends `report`, `tokens`, `diff_tokens` and `ConvertTheme` with the numbers above. If a file name, field number or JSON shape differs, the implementer must STOP and ask.
- The engine's report on a configuration error equals Go's `ErrorTokens` for the same error (engine plan guarantees "laid out once, by the same function `xcl.PrintError` uses"), so Python's plain output differs from Go's only through wrapping.
- Go and Python applies of the same parity fixture emit the same multiset of events (same engine code path). If a fixture's events or log messages turn out nondeterministic or different between the two sides, the implementer must STOP and ask rather than weaken the comparison.
- The xcl parity-program change can merge into xcl before (or with) the xcl release that xcl-python pins; if that release is cut first, a further xcl release is needed (a person decides the label).
- For an unknown kind, Python's HTML class and style fallback follow Go's landed `highlight/html.go` and `Theme.Style`; the reference outputs contain no unknown kind, so the Python tests assert only the category's style and class.

## Drafting assumptions

### Chosen direction: `xcl.highlight` package, reports as notes from `from_proto`, parity proven against Go (architecture)
- **Decision**: build the design's Python surface as a public `xcl.highlight` package of small private modules plus top-level `print_error`/`format_error`/`print_diff`/`logging_handler`; attach `err.tokens` and the plain-report note eagerly in `_errors.from_proto`; wrap detail at print time with a go-wordwrap port; prove formats against xcl's reference files, kinds against the protocol descriptor, and report/resource/diff/logging output against Go through the extended parity harness.
- **Rationale**: one conversion point covers calls, finished operations and events; eager notes mean any traceback shows the report; the parity harness compares with Go's real output on every run.
- **Rejected**: a single flat `xcl/highlight.py` (mixes formatting, logging and engine calls in one large file); lazy notes (missing when no xcl code touches the error); literal expected strings.

### "Same as Go" proven through the parity harness (discovery)
- **Decision**: extend xcl's Go parity program and xcl-python's Python parity program with printed-report, printed-entity, printed-diff and logging stages, plus a new fixture with an unsupported block on line 7.
- **Rationale**: the harness exists to prove Python and Go agree on shared fixtures (PARITY.md); it compares against Go's real output on every run.
- **Rejected**: literal expected strings in Python tests (agreement only at authoring time).

### Detail wrapping is a port of go-wordwrap (discovery)
- **Decision**: port `mitchellh/go-wordwrap` v1.0.1 `WrapString` exactly (Go's `unicode.IsSpace`, no-break space not a space), with unit tests whose expected strings were produced by Go.
- **Rationale**: reports must be byte-identical to Go's; `textwrap` differs.
- **Rejected**: `textwrap.fill`.

### VS Code theme returns only converted kinds (discovery)
- **Decision**: `load_vscode_theme` returns only the kinds the theme styles, like Go's `LoadVSCodeTheme`; docs show `DEFAULT_THEME | load_vscode_theme(path)`.
- **Rationale**: same result as Go for the same file.
- **Rejected**: merging with the default inside the function.

### One `InvalidStyleError` lists every unknown word (discovery)
- **Decision**: `highlight.validate(theme)` returns nothing for a valid theme and raises one `xcl.InvalidStyleError` whose `words` lists each `(kind, style, word)` for an invalid one.
- **Rationale**: Pythonic single `except`, still reports every word as Go's joined errors do.
- **Rejected**: `ExceptionGroup`, returning a list.

### Saved-record printing added beside entity printing (discovery)
- **Decision**: add `print_saved_entity` and `saved_entity_tokens` to both configuration flavours, beside the design's `print_entity`/`entity_tokens`.
- **Rationale**: the engine offers tokens on both encode calls; removing `highlight=` from `encode_saved_entity` needs a replacement; the downstream pretty-logger spec renders created resources from saved records with this spec's printing.
- **Rejected**: leaving saved-record printing to the pretty-logger spec (it would add printing API there, against its constraint to use the package's printing support).

### Formatters are objects with `format`, not bare callables (discovery)
- **Decision**: the seam is the design's `Formatter` protocol only.
- **Rationale**: one shape, exact annotations under `py.typed`.
- **Rejected**: also accepting a bare callable.

### Report note text (architecture)
- **Decision**: the note is `format_error(err)` with the default `PLAIN` and width 80, trailing newlines removed (traceback adds its own); added for every `xcl.Error` that carries tokens, causes and event errors included.
- **Rationale**: matches Go's `%+v` (plain, wrapped at 80); no stray blank lines at the end of a traceback.
- **Rejected**: an unwrapped note (long detail lines in tracebacks); notes on the top-level error only (an event error would lose its report).

### Logging bridge record shape (architecture)
- **Decision**: `extra` uses Go's attribute names plus the event's details, skipping a detail whose name would overwrite a `LogRecord` attribute, and always adds the `Event` as `xcl_event`; the record time is the event's time; the error is passed as the `xcl.Error` object (one-line `str`), with no `exc_info`.
- **Rationale**: mirrors Go's `SlogHandler` attributes; `logging` raises on overwriting reserved attributes; `event` is structlog's message key.
- **Rejected**: nesting all fields under one key (loses flat fields for JSON handlers); `exc_info` on error events (Go writes only the one-line error).

### Non-xcl exceptions print as their string (architecture)
- **Decision**: `print_error`/`format_error` accept any exception; one without tokens prints `str(exc)` and a newline.
- **Rationale**: Go prints any other error as `err.Error()` and a newline.
- **Rejected**: printing `Type: message` like `traceback` (diverges from Go's output).

### Conventions selected (architecture)
- **Decision**: keep the testing, dependencies and structured-logging conventions, and Go code style for the parity program; drop the Go-library layout, errors-package, testutil, parser, DAG, state-fixture and provider conventions.
- **Rationale**: only those bear on a Python formatting layer and a small Go parity-program change.
- **Rejected**: listing every convention.

### Kind helpers for unknown kinds (data_structures)
- **Decision**: `Token.kind` is an `int` (a `Kind` member when known); `parent_of` and `css_class_of` work on any int, and `Kind` has `parent` and `css_class` properties. No `scope()` in Python.
- **Rationale**: a newer engine's kind must not fail conversion and must fall back to its category; scopes are only needed for VS Code import, which the engine does.
- **Rejected**: converting unknown kinds to `TEXT` (loses the category); a `Kind` pseudo-member per unknown number.

### `print_diff` of a diff with no labelled text raises (data_structures)
- **Decision**: a `Diff` built by hand (no tokens) makes `print_diff` raise `ValueError` naming `Config.diff` as the source of printable diffs.
- **Rationale**: Python cannot lay out a diff (the core does); silently printing nothing would hide the mistake.
- **Rejected**: printing nothing; laying the diff out in Python.

### `width=None` and `0` both mean no wrapping (data_structures)
- **Decision**: `width: int | None = 80`; `0` or `None` never wraps; a negative width is treated as `0`, as Go does.
- **Rationale**: Go's `WithWidth(0)` never wraps; `None` is the Python idiom for "no limit".
- **Rejected**: raising on a negative width (Go accepts it).

### Stub regeneration and keyword removal are one task (tasks)
- **Decision**: regenerate the stubs, remove `highlight=` and delete the colour tests in one change, pinned to the xcl branch during development.
- **Rationale**: regenerated stubs reject the field, so either half alone breaks the build; the engine plan warns about exactly this.
- **Rejected**: removing the keyword first (the old engine still accepts it, and nothing tests the new stubs).

### One human task for the release (tasks)
- **Decision**: a single human task covers making sure the xcl release holds the parity change, pinning it, merging with `minor`, adding the release-notes line and holding the website until the package is out.
- **Rationale**: every step is a release action outside the repos; reviews of docs are manual checks, not tasks.
- **Rejected**: an agent task for the pin change (it cannot be done until a person cuts the release).

### Unknown-kind tests assert category style and class only (tasks)
- **Decision**: Python asserts an unknown kind (1199) prints in the comment style and carries `xcl-comment`; exact class strings for unknown kinds follow Go's landed `html.go`.
- **Rationale**: the reference files hold no unknown kind; the category fallback is the behaviour the design requires.
- **Rejected**: inventing a class rule that Go might not share.

## Rehydration cues

- `spektacular spec file read 20261023075302-bad71237-printing-and-highlighting-python`
- `spektacular design read --data '{"source":"design","path":"printing-and-highlighting.md"}'`
- `spektacular plan file read 20261023074926-eaa51527-printing-and-highlighting-core context` (formatter byte rules) and `… 20261023075136-42c51a7d-printing-tokens-over-the-engine plan` (wire shapes)
- `spektacular repo list` → xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`, xcl `/home/nicj/code/github.com/jumppad-labs/xcl`, xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`
- Re-read `xcl-python:xcl/_errors.py`, `xcl/config.py`, `xcl/aio.py`, `xcl/_poll.py`, `xcl/diff.py`, `tests/fakes.py`, `tests/parity/run_python.py`; `xcl:events/slog.go`; `xcl:bindings/libxcl/cmd/parity/main.go`.
