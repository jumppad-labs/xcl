---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Context: 20261023075328-741138ee-python-prettylog

## Current State Analysis

- **Decisions the user confirmed during epic planning**: (1) the one `EncodeSavedEntity` render per created resource inside the handler is allowed, because the constraint forbids waiting on the operation, not this call; (2) configuration-mistake reports show beneath their error line at every level, with the once-per-problem rule, while configuration, render warnings and guidance show only at info and below. This differs from Go's example, which writes no reports. The parity stage runs at info on successful fixtures, so it never sees reports.
- **Go reference**: `xcl:example/prettylog/prettylog.go:37-178`. An example, not API. It wraps `events.SlogHandler` (`xcl:events/slog.go:20-120`) around charmbracelet/log v0.4.2, with Kitchen time and coloured `source`/`resource`/`operation`. It writes a created entity's configuration beneath `create success` at info and below, warns `unable to show configuration`, skips `ErrNotEncodable`, and reads `XCL_LOG_LEVEL`. After the printing-core plan it renders with `xcl.PrintEntity`. Every Go example sets it up in one line (`xcl:example/plugin/main.go:219`, `example/configonly/main.go:55`, `example/remoteplugin/main.go:122`).
- **Python today**: xcl-python has no pretty logger. The website's Python tab (`xcl-website:src/pages/events.mdx:363-397`) shows a hand-written receiver with its own ANSI table. After the printing plan, the package has `xcl.highlight` (formatters, `DEFAULT_THEME`, `for_stream`), `xcl.format_error`, `err.tokens`, `saved_entity_tokens`/`print_saved_entity` on both flavours, and `xcl.logging_handler` in `xcl/_logbridge.py`.
- **Event delivery**: the synchronous driver calls `on_event` on the calling thread, one event at a time (`xcl-python:xcl/_poll.py:177-201`). The asyncio driver makes one asyncio task per event (`xcl-python:xcl/aio.py:158-172`), and `_call_maybe_async` runs a plain handler on the default executor (`xcl/aio.py:299-307`), so events can be handled concurrently and out of order. Asyncio lookups go through a single-worker executor shared with `Poll` (`xcl/aio.py:66,118-121,216-218`).
- **Events**: `xcl-python:xcl/events.py:16-90`. `_classes` is attached by the delivering configuration and kept out of fields and equality. `data` holds the saved record at processed event data. `duration` is a microsecond `timedelta`. `meta` is JSON-decoded.
- **Engine**: after plan cc23fc08, `EncodeSavedEntity` is allowed during an operation and `EncodeEntity` is not. The transport matches responses by id, so calls from different threads do not wait on each other (`xcl-python:xcl/_transport/subprocess.py:1-30`).
- **Error events**: an unsupported block emits a `parse error` event carrying its one diagnostic (`xcl:internal/parser/parser.go:860-940`), then the operation's closing `apply error` event carrying the whole `ConfigError` (`xcl:config.go:655-668`).
- **Parity**: `xcl:bindings/libxcl/cmd/parity/main.go` and `xcl-python:tests/parity/run_python.py` write comparable records, and `tests/parity/test_parity.py` reports the first differing path. xcl-python's Makefile builds the parity program from `XCL_SRC` (`xcl-python:Makefile:41-45`).

Requirement-to-repo attribution:

| Requirement / criterion | Repo | Files |
|---|---|---|
| One line of setup; level default info | xcl-python | `xcl/prettylog.py`, `xcl/__init__.py` |
| Works with existing logging | xcl-python | `xcl/prettylog.py`, `xcl/_logbridge.py` |
| Readable event lines | xcl-python | `xcl/_prettyline.py` |
| Created resources shown as they happen | xcl-python | `xcl/prettylog.py`, `xcl/events.py`, `xcl/_poll.py`, `xcl/config.py`, `xcl/aio.py` |
| Same lines on the asyncio client | xcl-python | `xcl/aio.py` |
| Quiet at higher levels | xcl-python | `xcl/prettylog.py` |
| Configuration mistakes shown with their source | xcl-python | `xcl/prettylog.py` |
| Colour follows the stream | xcl-python | `xcl/prettylog.py` (through `xcl.highlight.for_stream`) |
| Level from the environment | xcl-python | `xcl/prettylog.py` |
| Guidance when resource data is off | xcl-python | `xcl/prettylog.py`, `xcl/config.py`, `xcl/aio.py` |
| Render failures warn and continue; builtins silent | xcl-python | `xcl/prettylog.py` |
| Documentation | xcl-python, xcl-website | `README.md`, `PARITY.md`; `src/pages/events.mdx` |
| Success metric (same as Go) | xcl, xcl-python | `bindings/libxcl/cmd/parity/main.go`, `bindings/libxcl/go.mod`, `bindings/libxcl/testdata/parity/README.md`; `tests/parity/run_python.py` |

## Per-Task Technical Notes

### Task: Deliver asyncio events in order

Requirement attribution: AC "Asynchronous client" (same lines in the same order); constraint "must not wait on the operation inside the handler" (no lock held across a render). Repo xcl-python (`/home/nicj/code/github.com/jumppad-labs/xcl-python`).

File changes:
- `xcl-python:xcl/aio.py:131-148` (`_drive`): create `events: asyncio.Queue[Optional[Event]] = asyncio.Queue()` and, when `self._on_event` is set, one consumer task `consumer = asyncio.create_task(self._deliver_events(events))` before the loop. After the loop, `events.put_nowait(None)`, then `await consumer` (inside the existing `try`), then the existing `gather(*pending, return_exceptions=True)`. On `asyncio.CancelledError`, pass the consumer to `_stop` along with `pending` so it is cancelled and awaited.
- `xcl-python:xcl/aio.py:150-156` (`_stop`): accept and cancel the consumer, as it cancels `pending`. Simplest: add the consumer to `pending` after creating it, and keep `_stop` unchanged. In that case `_drive` awaits the consumer through the final `gather`, and the sentinel must be put before that `gather`.
- `xcl-python:xcl/aio.py:158-168` (`_dispatch`): for `kind == "event"`, build the `Event` (`Event._from_proto(response.event, self._classes)`, plus the configuration attached by the next task) and `put_nowait` it on the queue, returning `None`. Keep state and function tasks as they are. `_dispatch` needs the queue, so pass it as an argument or hold it on the operation's local scope (e.g. `self._dispatch(op, response, events)`).
- `xcl-python:xcl/aio.py:170-172` (`_handle_event`): replace with `async def _deliver_events(self, events)`. It loops `event = await events.get()` and, on `None`, returns. Otherwise it runs `try: await _call_maybe_async(self._on_event, event) except asyncio.CancelledError: raise except Exception: pass`. Comment that a handler's failure must not affect the operation, matching today's `gather(return_exceptions=True)`.
- `xcl-python:xcl/aio.py:1-9` (module docstring) and `:37-42` (class docstring): say that events reach `on_event` one at a time, in the order the engine sent them, a plain handler on the default executor and an `async def` one on the loop, while state-store methods and functions still run concurrently.
- Tests, `xcl-python:tests/unit/test_aio_events.py` (new, fake transport, `Poll` scripted with five events then `finished`):
  - `test_async_plain_handler_receives_events_in_order`: a plain handler that sleeps 50 ms on the first event and records `resource_id`; the order equals the script's.
  - `test_async_plain_handler_called_one_at_a_time`: a plain handler that increments a counter under a lock, asserts it is 1, sleeps, decrements; the maximum seen is 1.
  - `test_async_coroutine_handler_receives_events_in_order`: an `async def` handler that awaits `asyncio.sleep` with decreasing delays; the order equals the script's.
  - `test_async_operation_returns_after_last_event_handled`: the last event's handler sleeps; when `apply` returns, all five are recorded.
  - `test_async_handler_exception_does_not_stop_later_events`: the handler raises on the second event; all five are seen, and `apply` returns normally.
  - `test_async_cancel_while_handling_events_stops_operation`: a handler awaiting an `asyncio.Event` that is never set; cancel the apply task; `CancelledError` propagates and `Cancel` was called on the transport (as the existing cancel tests check).
- The existing tests `tests/unit/test_events.py:195-230` and `tests/integration/test_aio.py:252-290` must still pass unchanged.

Complexity: Medium
Token estimate: ~20k tokens
Agent strategy: Single agent, sequential execution. Read the existing asyncio cancel tests first, to reuse how they script the fake transport for `Cancel`.

### Task: Let a handler reach the configuration that delivered its event

Requirement attribution: Technical Approach "Render each created resource from the event's saved record"; requirement "Guidance when resource data is off" (the configuration's level); constraint "must not wait on the operation … inside the event handler" (the blocking render bypasses the asyncio poll executor); Technical Approach "Build on the package's logging bridge". Repo xcl-python.

File changes:
- `xcl-python:xcl/events.py:39-43`: beside `_classes = None`, add `_config = None` with a comment. It holds the configuration (`xcl.Config` or `xcl.aio.Config`) that delivered the event, so a handler can have its data rendered. It is not a field, so it takes no part in fields, equality or repr, and an event built by hand has none.
- `xcl-python:xcl/events.py:45-66` (`_from_proto`): add the parameter `config: Any = None` and set `event._config = config` next to `_classes`.
- `xcl-python:xcl/_poll.py:62-70` (`Handlers`): add `config: Optional[Any] = None`.
- `xcl-python:xcl/_poll.py:196-201` (`_dispatch`): `Event._from_proto(response.event, handlers.classes, handlers.config)`.
- `xcl-python:xcl/config.py:70`: `Handlers(..., classes=self._classes, config=self)`.
- `xcl-python:xcl/config.py:47-87`: store `self._event_data = event_data` (the `xcl.EventDataLevel` member, after the event-data-level plan's check).
- `xcl-python:xcl/config.py` (near the printing plan's `saved_entity_tokens`): add `_saved_entity_tokens_now(self, data, *, include_computed=False, include_empty=False, reveal_sensitive=False, show_references=False) -> list[Token]`. It builds the same `EncodeSavedEntityRequest(entity=bytes(data), options=_encode_options(...), tokens=True)` as `saved_entity_tokens` and calls `self._client.call(self._open(), "EncodeSavedEntity", request)`. Have `saved_entity_tokens` delegate to it. The docstring says it is private, blocking and safe from an `on_event` handler during an operation.
- `xcl-python:xcl/aio.py:44-79`: store `self._event_data = event_data`.
- `xcl-python:xcl/aio.py` (next to the printing plan's async `saved_entity_tokens`): add the same `_saved_entity_tokens_now(...)`, a plain method. It calls `self._client.call(self.handle, "EncodeSavedEntity", request)` directly on the calling thread, never through `self._in_executor` (the single-worker poll executor). If the handle is 0, it raises `RuntimeError("the configuration is not open")`. Share the request building with the sync flavour, as `_encode_options` is shared today.
- `xcl-python:xcl/aio.py` (`_dispatch`, as changed by the previous task): `Event._from_proto(response.event, self._classes, self)`.
- `xcl-python:xcl/_logbridge.py` (from the printing plan): extract the rules `logging_handler` already applies into module-level private functions, without changing what `logging_handler` writes:
  - `event_level(event) -> int`
  - `event_message(event) -> str`
  - `event_extra(event) -> dict[str, Any]`, the bridge's extras, including `xcl_event`
  - `event_record(logger, event, level, message) -> logging.LogRecord`, using `makeRecord` with `extra=event_extra(event)` and setting `created`, `msecs` and `relativeCreated` from the event's time

  `logging_handler` becomes a call to these.
- Tests:
  - `xcl-python:tests/unit/test_events.py` (add): `test_sync_config_attaches_itself_to_events`; `test_asyncio_config_attaches_itself_to_events`; `test_event_built_by_hand_has_no_config`; `test_events_delivered_by_different_configs_are_equal` (same proto, two configs); `test_event_repr_does_not_show_config`.
  - `xcl-python:tests/unit/test_config.py` (add, fake transport): `test_saved_entity_tokens_now_asks_for_tokens` (request `tokens is True` with the options); `test_aio_saved_entity_tokens_now_calls_on_calling_thread` (record `threading.get_ident()` inside the fake transport's `call` for `EncodeSavedEntity`, and assert it equals the test thread's); `test_config_remembers_event_data_level`; `test_aio_config_remembers_event_data_level`.
  - The printing plan's `tests/unit/test_logbridge.py` passes unchanged.

Complexity: Medium
Token estimate: ~20k tokens
Agent strategy: 2 parallel agents: the event and configuration plumbing with its tests, and the logging-bridge helper extraction. Integrate and run the unit suite. Requires the printing plan's `saved_entity_tokens` and `_logbridge` on the branch. If either is shaped differently from its plan, follow the landed code. If a capability is missing, STOP and ask.

### Task: Lay out event lines as Go's example writes them

Requirement attribution: requirement "Readable event lines" (time, level, message, source, operation, phase, resource, remaining attributes as key=value, highlighted); success metric "same event lines as Go's example". Repo xcl-python.

File changes:
- `xcl-python:xcl/_prettyline.py` (new, private, standard library only). It ports charmbracelet/log v0.4.2 `textFormatter` (`~/go/pkg/mod/github.com/charmbracelet/log@v0.4.2/text.go:178-272`, `escapeStringForOutput` :73-125, `needsQuoting` :127-157, `writeIndent` :19-48) with the attribute rules of `xcl:events/slog.go:83-120`.
  - `Styler = Callable[[str, str], str]` takes `(text, style)` and returns the styled text. The identity is used for plain text.
  - `STYLES`: a mapping of part to style string:
    - levels: `"DEBU": "bold blue"`, `"INFO": "bold cyan"`, `"WARN": "bold yellow"`, `"ERRO": "bold red"`;
    - `"key": "dim"`, `"separator": "dim"`;
    - `"key.source": "magenta"`, `"value.source": "bold magenta"`;
    - `"key.resource": "cyan"`, `"value.resource": "bold cyan"`;
    - `"key.operation": "yellow"`, `"value.operation": "bold yellow"`.

    Timestamp, message and other values are unstyled.
  - `kitchen(t: datetime) -> str`: `t.astimezone()`, then `f"{(t.hour % 12) or 12}:{t.minute:02d}{'AM' if t.hour < 12 else 'PM'}"`.
  - `level_label(level: int) -> str`: `logging.DEBUG`→`DEBU`, `INFO`→`INFO`, `WARNING`→`WARN`, `ERROR`→`ERRO`. Any other int maps to the label of the highest standard level not above it, with `DEBU` below DEBUG.
  - `attributes(event) -> list[tuple[str, Any]]`:
    - `("source", …)`, `("operation", …)`, `("phase", …)`, `("resource", resource_id)`, `("type", resource_type)` and `("file", …)`, each when non-empty;
    - `("duration", event.duration)` when non-zero;
    - `("error", str(event.error))` when set;
    - then `sorted(event.meta)`, skipping `level` and `message` when `phase == "log"`.
  - `go_value(value) -> str`, Go's `fmt.Sprintf("%+v", slog.Value)` for JSON-decoded values:
    - `str` as is; `bool` as `true`/`false`; `None` as `<nil>`; `int` in decimal;
    - `float` by Go's `%v`, which is `strconv.FormatFloat(f, 'g', -1, 64)`: take the shortest round-trip digits (from `repr`, through `decimal.Decimal`), let `exp` be the decimal exponent of the first digit, and use exponent form exactly when `exp < -4 or exp >= 6` (strconv sets the precision for this decision to 6 when formatting shortest). Exponent form is `d[.ddd]e±XX`, with at least two exponent digits; otherwise plain decimal with no trailing `.0`. Verified by running Go: `100.0`→`100`, `123456.0`→`123456`, `1234567.0`→`1.234567e+06`, `1e6`→`1e+06`, `1.5`→`1.5`, `0.0001`→`0.0001`, `1e-05`→`1e-05`. NaN and infinities print as Go prints them (`NaN`, `+Inf`, `-Inf`);
    - `list` as `[` + space-joined `go_value` + `]`;
    - `dict` as `map[` + space-joined `k:v` in sorted key order + `]`;
    - `timedelta` as a port of Go's `time.Duration.String()` on the microsecond value (`0s`, `1.5ms`, `5ms`, `1m1s`, `2h0m0s`, `999µs`).
  - `format_line(event, level: int, message: str, style: Styler) -> str`. It writes `kitchen(event.time)`, a space, `style(label, STYLES[label])`, then the message (when non-empty, unstyled, after a space), then each attribute as in `text.go:208-269`:
    - `val = go_value(v)`; `raw = val == ""`, which writes `""`;
    - the key is styled with `key.<name>` or `key`; the separator `=` with `separator`;
    - when `"\n" in val`: `"\n  " + key + sep + "\n"`, then for each line but the last `"  │ "` (separator style) + the escaped line + `"\n"`, and for the last line the indent + the escaped line + (`"\n"` only if more attributes follow);
    - elif `not raw and needs_quoting(val)`: `" " + key + sep + style('"' + escape(val, quotes=True) + '"', value style)`;
    - else `" " + key + sep + style(val, value style)`.

    The line ends with `"\n"`. The space before an attribute follows `writeSpace` (none before the first item; the time is always first). Port `needs_quoting` (ASCII `"`, `=`, Unicode spaces, non-printables, invalid runes) and `escape` (`\a \b \f \n \r \t \v`, `\xHH` below space, `\uHHHH`, `\UHHHHHHHH` for non-printables, `\"` when quoting) with `str.isprintable` per character as Go's `unicode.IsPrint`. Go's IsPrint treats U+0020 as printable and other spaces as not. Port that exactly.
- `xcl-python:tests/unit/test_prettyline.py` (new). Expected strings are produced once by a small Go program running charmbracelet/log v0.4.2 `TextFormatter` with `TimeFormat: time.Kitchen`, through `events.SlogHandler`, on the same events at a fixed time in UTC, with the test setting `TZ=UTC` (`monkeypatch.setenv` plus `time.tzset()` where available, and skipping the time-zone-sensitive tests on Windows). Paste them as literals with a comment naming the versions. One test each:
  - `test_kitchen_midnight` (`12:04AM`), `test_kitchen_noon`, `test_kitchen_afternoon`;
  - `test_level_labels` is split into four tests, `test_level_debug_label` and so on;
  - `test_message_omitted_when_empty`;
  - `test_attribute_order`, `test_unset_attributes_left_out`, `test_details_sorted`, `test_log_event_level_and_message_not_repeated`;
  - `test_duration_formatting_ms`, `test_duration_formatting_minutes`, `test_zero_duration_left_out`;
  - `test_int_value`, `test_integral_float_value`, `test_exponent_float_value`, `test_fractional_float_value`, `test_bool_value`, `test_none_value`, `test_list_value`, `test_map_value`;
  - `test_value_with_space_quoted`, `test_value_with_equals_quoted`, `test_value_with_quote_escaped`, `test_value_with_tab_escaped`, `test_empty_value_written_as_quotes`;
  - `test_multiline_value_last`, `test_multiline_value_followed_by_attribute`;
  - `test_error_value_is_str_of_error`;
  - with a recording styler: `test_source_key_and_value_styles`, `test_operation_key_and_value_styles`, `test_resource_key_and_value_styles`, `test_other_key_dim_value_plain`, `test_level_style`.

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: Parallel analysis, sequential integration. One agent writes and runs the throwaway Go program (in a scratch directory outside both repos) that prints the expected lines, including the float and duration cases. A second ports the formatter. Then compare. The Go output is the authority: where this note's rule and Go disagree, follow Go.

### Task: Add the pretty logger

Requirement attribution: requirements "One line of setup", "Works with existing logging", "Created resources shown as they happen", "Quiet at higher levels", "Configuration mistakes shown with their source", "Colour follows the stream", "Level from the environment", "Guidance when resource data is off", "Render failures warn and continue", "Items without a configuration form are skipped silently"; constraints `XCL_LOG_LEVEL`, standard library only, take colours, formats and reports from the printing support, no waiting in the handler; Technical Approach "a small module … with a handler factory and a level-from-environment helper". Repo xcl-python.

File changes:
- `xcl-python:xcl/prettylog.py` (new, public). The module docstring shows the one-line setup for both flavours and a sample of the output, and names Go's `example/prettylog` as the model.
  - `LEVEL_ENV = "XCL_LOG_LEVEL"`.
  - `level_from_env() -> int`: `os.environ.get(LEVEL_ENV, "").lower()` maps `"debug"`→`logging.DEBUG`, `"warn"`→`logging.WARNING`, `"error"`→`logging.ERROR`, and anything else to `logging.INFO`. This mirrors `xcl:example/prettylog/prettylog.go:164-178`.
  - `handler(file=None, level=logging.INFO, *, logger=None, formatter=None) -> Callable[[Event], None]` checks its arguments first:
    - `type(level) is bool or not isinstance(level, int)` raises `TypeError("level must be a logging level such as logging.INFO, not {level!r} ({type})")`;
    - `file is not None and logger is not None` raises `ValueError("give file or logger, not both")`;
    - a `logger` that is not a `logging.Logger` raises `TypeError`.

    Without a logger, `file = file or sys.stderr` (looked up now). The formatter is `formatter or (highlight.PLAIN if logger else highlight.for_stream(file))`. It returns `_PrettyLog(file, level, logger, formatter)`.
  - `class _PrettyLog` holds `_lock = threading.Lock()`, `_guided = False` and `_shown_problems = 0`. Its `__call__(self, event)` wraps everything in `try/except Exception` and ignores the exception, so the handler never raises into the operation.
    1. `lvl = _logbridge.event_level(event)`. If `lvl < self._level`, return.
    2. `line = _prettyline.format_line(event, lvl, _logbridge.event_message(event), self._style)`.
    3. `groups = [(lvl, line + block)]`. `block` is the report block below at every level. The configuration block, render warnings and guidance are added only when `self._level <= logging.INFO`; above info, skip the render entirely. The user confirmed during epic planning that reports show at every level and that "Quiet at higher levels" means no configuration blocks. This differs from Go's example.
    4. Write the groups under `self._lock`.
  - Configuration block. For `event.operation == "create" and event.phase == "success"`:
    - If `event.data` is empty: when `event._config is not None and event._config._event_data is EventDataLevel.NONE and not self._guided`, set `_guided` and add an INFO group, `format_line` of a synthetic line with message `configuration not shown: events carry no resource data` and the single attribute `hint="create the configuration with event_data=xcl.EventDataLevel.PROCESSED"`. Build that line with a helper of `_prettyline` that takes explicit attributes rather than an event. Otherwise nothing.
    - Else, if `event._config is None`: add a WARN group `unable to show configuration` with `resource=<id>` and `error="the event was not delivered by a configuration"`. This matches Go's hand-built-event warning, `prettylog_test.go:449-462`.
    - Else `tokens = event._config._saved_entity_tokens_now(event.data, include_computed=True)`:
      - `xcl.NotEncodableError` means nothing;
      - any other `Exception` adds a WARN group `unable to show configuration resource=<id> error=<str(exc)>`;
      - on success, `block = _indent(self._render(tokens))`.
  - Report block. For `event.phase == "error"` with `isinstance(event.error, xcl.Error) and event.error.tokens`:
    - `problems = len(event.error.problems) if isinstance(event.error, xcl.ConfigError) and event.error.problems else 1`;
    - `closing = event.operation != "parse" and not event.resource_id`.
    - If not closing: write the report and add `problems` to `_shown_problems`.
    - If closing: write it only if `problems > _shown_problems`, then reset `_shown_problems = 0`.
    - The report is `xcl.format_error(event.error, formatter=self._formatter, theme=highlight.DEFAULT_THEME, width=80)` through `_indent`.

    Update `_shown_problems` under the lock.
  - `_style(text, style)`: `buf = io.StringIO(); self._formatter.format(buf, {highlight.Kind.TEXT: style}, [highlight.Token(highlight.Kind.TEXT, text)]); return buf.getvalue()`. An empty style returns the text.
  - `_render(tokens)`: formats with `highlight.DEFAULT_THEME` into a `StringIO`.
  - `_indent(text)`: a port of `prettylog.go:136-147`. Strip trailing `"\n"`, prefix every non-empty line with two spaces, join with `"\n"` and add a final `"\n"`. An empty text gives `""`.
  - Writing with a stream: `self._file.write(text)` and `self._file.flush()` if present, inside `try/except (OSError, ValueError): pass`.
  - Writing with a logger, for each group: `record = _logbridge.event_record(self._logger, event, group_level, text.rstrip("\n"))`, and if `self._logger.isEnabledFor(group_level)`, `self._logger.handle(record)`.
- `xcl-python:xcl/__init__.py:54`: add `from xcl import prettylog` next to `from xcl import mask`, and `"prettylog"` to `__all__`. Check the import order against the printing plan's `highlight` imports to avoid a cycle. `prettylog` imports `xcl.highlight`, `xcl._logbridge`, `xcl._prettyline` and `xcl._errors`. Import `xcl.EventDataLevel` from its module, not from the package.
- `xcl-python:tests/unit/test_prettylog.py` (new). These tests use hand-built events, `StringIO`, and configurations over the fake transport whose `Poll` script delivers the events, with `EncodeSavedEntity` scripted to return tokens or an error. One test each:
  - Level from the environment: `test_level_from_env_debug`, `_warn`, `_error`, `_upper_case_info`, `_unset`, `_unknown` (`"loud"`).
  - Arguments: `test_handler_refuses_string_level`, `test_handler_refuses_bool_level`, `test_handler_refuses_file_and_logger`, `test_handler_defaults_to_stderr` (capsys).
  - Levels: `test_info_handler_skips_debug_event`, `test_debug_handler_writes_debug_event`, `test_warn_handler_writes_no_info_line`, `test_warn_handler_writes_no_configuration` (a scripted create success with data).
  - Configuration: `test_configuration_follows_create_success` (lines after the success line equal the indented joined tokens); `test_configuration_asks_for_computed_values` (the request carries `include_computed`); `test_no_configuration_for_create_start`; `test_no_configuration_for_hand_built_event_without_data` (no warning either); `test_hand_built_event_with_data_warns_once`; `test_later_events_written_after_warning`; `test_not_encodable_skipped_silently`; `test_two_render_failures_warn_twice_and_apply_completes` (a full `Config.apply` over the fake transport).
  - Guidance: `test_guidance_written_once_when_data_off` (two create successes from a config with `EventDataLevel.NONE`); `test_no_guidance_when_data_on_and_event_has_no_data`; `test_no_guidance_above_info`.
  - Reports: `test_report_beneath_parse_error`; `test_report_not_repeated_beneath_closing_error`; `test_report_beneath_closing_error_when_not_shown_before`; `test_no_report_for_error_without_tokens`; `test_report_indented`; `test_warn_handler_writes_report_beneath_parse_error` (`level=logging.WARNING`, a `parse error` event with tokens: the `ERRO` line is followed by the indented report); `test_error_handler_writes_report_beneath_closing_error`; `test_warn_handler_writes_no_guidance_or_render_warning`.
  - Colour: `test_plain_stream_has_no_colour` (`StringIO`); `test_terminal_formatter_colours_source_operation_resource` (`formatter=highlight.TERMINAL`, assert the codes the printing support writes for `bold magenta`, `bold yellow` and `bold cyan` wrap the values); `test_terminal_formatter_colours_configuration`.
  - Logger: `test_logger_record_message_is_line_and_block`; `test_logger_record_level_and_time_are_events`; `test_logger_record_carries_bridge_fields`; `test_logger_warning_is_its_own_record`; `test_logger_level_filters_records`.
  - Robustness: `test_failing_stream_does_not_raise`; `test_handler_works_for_aio_config` (asyncio over the fake transport, lines in order with the block after the success line).
- `xcl-python:tests/unit/test_docstrings.py`: the existing public docstring walk covers `xcl.prettylog`. Check that it does, and add the module if the walk lists modules explicitly.

Complexity: High
Token estimate: ~45k tokens
Agent strategy: Parallel analysis, sequential integration. One agent writes the handler and its argument and level tests. A second writes the fake-transport scripts for configuration, report and guidance cases. Integrate, then run the unit suite with type checking of the new module (`py.typed` makes its annotations public).

### Task: Prove the pretty logger's behaviour against the real engine

Requirement attribution: every acceptance criterion except "Level from the environment" (unit), "Docs" (manual) and the render-failure half of "Unrenderable items" (unit, fake transport). Repo xcl-python.

File changes:
- `xcl-python:tests/integration/test_prettylog.py` (new, sync) and `xcl-python:tests/integration/test_aio.py` (add the asyncio cases). The shared configuration is one `resource "recording" "one" { log = "<tmp>/log.jsonl", value = "a" }` written to `tmp_path`, with the `recording_plugin` fixture registered through `local.register_external_plugin`, and `event_data=xcl.EventDataLevel.PROCESSED` unless stated. A helper `normalised(text)` replaces `^\d{1,2}:\d{2}(AM|PM) ` with `<time> ` and `duration=\S+` with `duration=<duration>`. One test each:
  - `test_prettylog_setup_writes_one_line_per_event`: `handler(buf)` with no level. Every line not indented starts with a time and a level label. Lines for `apply start`, `create start`, `create success` and `apply success` exist, and the `create success` line has `source=core`, `operation=create`, `phase=success` and `resource=resource.recording.one`.
  - `test_prettylog_configuration_follows_creation`: the indented lines directly after the `create success` line start with `  resource "recording" "one" {` and contain `output = "one-a"` (regex `output\s+=\s+"one-a"`), and the next unindented line is a later event.
  - `test_async_prettylog_same_lines_as_sync` (in `test_aio.py`): `normalised` of the asyncio run equals `normalised` of a sync run of the same configuration in a fresh state directory.
  - `test_prettylog_through_application_logger`: a `logging.Logger("app")` with `propagate=False` and a `StreamHandler(buf2)` formatted `%(message)s`. `normalised` equals that of a stream run.
  - `test_prettylog_warn_level_is_terse`: no indented lines and no ` INFO ` lines.
  - `test_prettylog_warn_level_still_shows_report`: the unsupported-block configuration at `level=logging.WARNING`. The `ERRO` line is followed by indented lines containing `Error: Unsupported block type` and the gutter `     7 | `, and no ` INFO ` lines appear.
  - `test_prettylog_debug_level_adds_debug_events`: at least one ` DEBU ` line, for example the core's `decided every entity`.
  - `test_prettylog_configuration_mistake_shows_report`: a nine-line configuration with an unsupported nested block on line 7, the same shape as the printing plan's `unsupported_block` fixture, using a Python-registered type. The apply raises `xcl.ConfigError`. The output has an `ERRO` line followed by indented lines containing `Error: Unsupported block type`, a location naming line 7, and the gutter `     7 | `.
  - `test_prettylog_colours_pseudo_terminal` (`pytest.importorskip("pty")`): `pty.openpty()`, wrap the controlling end with `os.fdopen(fd, "w")`, set `NO_COLOR` unset (`monkeypatch.delenv`), and run. Read the other end in a thread until the apply ends, then assert the presence of `\x1b[` and of the codes for `bold magenta`, `bold yellow` and `bold cyan` around `core`, `create` and `resource.recording.one`.
  - `test_prettylog_file_has_no_colour`: an `open(tmp_path / "log.txt", "w")`, with no `\x1b`.
  - `test_prettylog_pseudo_terminal_with_no_color_has_no_colour` (`importorskip("pty")`, `NO_COLOR=1`).
  - `test_prettylog_guidance_once_without_data`: two recording resources with `event_data=EventDataLevel.NONE`. Exactly one line contains `configuration not shown`, and there are no indented lines.
  - `test_prettylog_builtins_silent`: a configuration with a `variable`, an `output` and a `module` (a module source directory under `tmp_path` holding one variable) plus one recording resource. No `unable to show configuration` line appears, and no configuration is written for the variable, output or module addresses.
  - `test_async_prettylog_configuration_follows_creation` (in `test_aio.py`).

Complexity: Medium
Token estimate: ~30k tokens
Agent strategy: 2-3 parallel agents: the sync event-line, level and logger tests; the mistake, colour and pseudo-terminal tests; and the asyncio tests. Merge and run the integration suite against the engine built from the xcl branch.

### Task: Record the pretty log in the Go parity program

Requirement attribution: success metric "same event lines and configuration as Go's example" (Go side). Repo xcl (`/home/nicj/code/github.com/jumppad-labs/xcl`), module `bindings/libxcl`.

File changes:
- `xcl:bindings/libxcl/go.mod`: `require github.com/jumppad-labs/xcl/example/prettylog v0.0.0` with `replace github.com/jumppad-labs/xcl/example/prettylog => ../../example/prettylog`, beside the existing replace at :50. Add charmbracelet/log v0.4.2, lipgloss v1.1.0 and termenv v0.16.0 at the versions `xcl:example/prettylog/go.mod:6-9` pins, via `go mod tidy`. Add a comment: "example/prettylog is required only by cmd/parity, which compares the Python pretty logger with Go's example; the shared library does not import it."
- `xcl:bindings/libxcl/cmd/parity/main.go:159` (`newConfig`): add a trailing `extra ...xcl.ConfigOption` appended to the options, so the stage can add its handler and event data.
- `xcl:bindings/libxcl/cmd/parity/main.go:228-258` (`run`): after the existing stages, `if applyErr == nil { record["pretty_log"] = n.prettyLog(f, plugins, tmp) } else { record["pretty_log"] = nil }`.
  - `prettyLog` makes a fresh state directory `filepath.Join(tmp, "pretty-state")` and a `bytes.Buffer`, then calls `newConfig(f, plugins, dir, tmp, xcl.WithEventHandler(prettylog.Handler(&buf, slog.LevelInfo)), xcl.WithEventData(xcl.EventDataProcessed))` and `Apply(f.main)`. If that apply fails, it returns the outcome map, so a difference shows.
  - It then normalises:
    - strip `\x1b\[[0-9;]*m`;
    - split into lines;
    - group, where a line that does not start with a space starts a new group and the others join the current group;
    - in each group's first line, replace `^\d{1,2}:\d{2}(AM|PM) ` with `<time> ` and every `duration=\S+` with `duration=<duration>`;
    - apply `n.text` (run paths) to the whole group;
    - join each group's lines with `\n` and sort the groups.

    Set `os.Setenv("CLICOLOR_FORCE", "")` before the stage so lipgloss writes plain text into the buffer.
- `xcl:bindings/libxcl/cmd/parity/main.go:1-20` (package doc): list the `pretty_log` stage.
- `xcl:bindings/libxcl/testdata/parity/README.md`: document `pretty_log`, its normalisation, and `null` for a fixture whose apply fails.
- `xcl:bindings/libxcl/cmd/parity/main_test.go` (testify `require`; one behaviour each):
  - `TestParityRecordCoversEveryStage` (:146) gains `pretty_log`.
  - `TestParityPrettyLogHasCreateSuccessWithConfiguration`: on `plugin_entities`, a group starts `<time> INFO create success` and holds an indented `resource "recording"` line.
  - `TestParityPrettyLogNullForFailingFixture`: on `invalid`.
  - `TestParityPrettyLogHasNoTimesDurationsOrColour`.
  - `TestParityPrettyLogHasNoTempPaths`.
  - The existing `TestParityRecordIsDeterministic` (:132) and `TestParityRecordIsDeterministicForPluginEntities` (:139) cover the stage's determinism.
- No xcl `CHANGELOG.md` section: the parity program is a test tool, as in the printing plan's parity task.

Complexity: Medium
Token estimate: ~20k tokens
Agent strategy: Single agent, sequential execution. Requires the printing-core plan on the branch (its prettylog uses `xcl.PrintEntity`). Run `go test ./cmd/parity/...` in `bindings/libxcl`. If the determinism test fails because of the stage, for example concurrent resources writing different lines rather than a different order, STOP and ask rather than filtering lines.

### Task: Compare the pretty log with Go in the Python parity suite

Requirement attribution: success metric (Python side). Repo xcl-python.

File changes:
- `xcl-python:tests/parity/run_python.py:90-116` (`new_config`): accept `**extra` keyword arguments passed through to `xcl.Config`, so the stage can pass `on_event` and `event_data`.
- `xcl-python:tests/parity/run_python.py:133-172` (`run`): when the apply succeeded, `record["pretty_log"] = pretty_log(fixture, plugins, tmp, n)`; otherwise `None`. `pretty_log` mirrors the Go stage:
  - a fresh `pretty-state` directory and a `StringIO`;
  - `new_config(..., on_event=xcl.prettylog.handler(buf, logging.INFO), event_data=xcl.EventDataLevel.PROCESSED)`, then apply;
  - the same normalisation (ANSI strip, grouping, `<time>`, `<duration>`, `n.text`, sorted groups).

  Update the module docstring (:1-17).
- `xcl-python:tests/parity/test_parity.py`: no new test function. Every per-fixture test already compares the whole record, so `pretty_log` is compared on every fixture, including the printing plan's `test_parity_unsupported_block`, where both sides record `null`.

Complexity: Low
Token estimate: ~10k tokens
Agent strategy: Single agent, sequential execution. Build the Go parity program from the xcl branch carrying the previous task (`make plugins XCL_SRC=…`). If the records differ only in `pretty_log`, find the first differing group. Differences in the line layout are fixed in the layout task's module. Differences in which events appear are a STOP and ask.

### Task: Document the pretty logger in the README and parity list

Requirement attribution: requirement and AC "Documentation"/"Docs" (package README). Repo xcl-python.

File changes:
- `xcl-python:README.md:221-247` ("Variables, state and events"): after the event paragraph, add a `#### Pretty logging` subsection. Show:

  ```python
  cfg = xcl.Config(
      registries=[local],
      on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env()),
      event_data=xcl.EventDataLevel.PROCESSED,   # puts each created resource on its event
  )
  ```

  - Show a sample of the output with a create-success line and its indented configuration, as in this plan's Data Structures.
  - Explain `XCL_LOG_LEVEL` (debug, info, warn, error; info by default), colour only on a terminal without `NO_COLOR`, the configuration beneath creation lines at info and below, reports beneath configuration mistakes, and the guidance line.
  - Show the logger form: `handler(logger=logging.getLogger("app"))`, with a `%(message)s` handler to get identical lines, and say that records carry the event's fields.
- `xcl-python:README.md:308-346` ("Asyncio"): show `on_event=xcl.prettylog.handler(sys.stderr)` working unchanged. Replace "each task it posts runs as an asyncio task" with wording that state-store calls and functions run as asyncio tasks while events reach `on_event` one at a time, in the order they were sent, a plain handler on the default executor and an `async def` one on the loop.
- `xcl-python:PARITY.md:56-70`: add a section "Examples", or rows under "Configuration options", pairing `prettylog.Handler(w, level)` (Go's `example/prettylog`) with `xcl.prettylog.handler(file, level)` and `prettylog.LevelFromEnv()` with `xcl.prettylog.level_from_env()`, and `prettylog.LevelEnv` with `xcl.prettylog.LEVEL_ENV`. Note that Go's is an example while Python's is part of the package.

Complexity: Low
Token estimate: ~10k tokens
Agent strategy: Single agent, sequential execution. Rebase on the printing plan's README rewrite if it landed first. Review only, no tests.

### Task: Document the pretty logger on the website's events page

Requirement attribution: requirement and AC "Documentation"/"Docs" (website events page). Repo xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`).

File changes:
- `xcl-website:src/pages/events.mdx:332-342`: keep the Go description, as the core plan rewrote it. Replace "The Python tab shows the same receiver written with the standard library alone, colouring its lines with ANSI escapes." with: Python ships the same receiver as `xcl.prettylog`, part of the package and written with the standard library alone, coloured by the package's own printing support.
- `xcl-website:src/pages/events.mdx:363-397` (Python tab): replace the hand-written receiver with:

  ```python
  import sys
  import xcl

  cfg = xcl.Config(
      on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env()),
      event_data=xcl.EventDataLevel.PROCESSED,
  )
  ```

  Add a sentence that `xcl.aio.Config` takes the same handler and that `logger=` sends the same lines through a standard logger.
- `xcl-website:src/pages/events.mdx:401-409`: add a Python sentence. The Python receiver renders each created resource from the event's saved record, which works during the operation, colours it through `xcl.highlight` and also shows a configuration mistake's report beneath its error line.
- `xcl-website:src/pages/events.mdx:423-429` (Python one-line tab): `on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env())`.
- `xcl-website:src/pages/events.mdx:433-442`: after the Go sample output, add a short Python sample, a create-success line with its indented configuration, or say the Python output is the same.
- `xcl-website:src/pages/events.mdx:277-280` ("Ordered, one at a time"): add that both Python clients deliver events the same way.
- Build the site (`npm run build`, as the repo's README says) and check internal links.

Complexity: Low
Token estimate: ~10k tokens
Agent strategy: Single agent, sequential execution. Rebase on the core and Python printing plans' edits to this page if they landed first. Review only, no tests.

### Task: Release the parity stage and the package with the pretty logger

Requirement attribution: delivery of every requirement to users (the package bundles only released engines). Repo xcl-python; action outside the repo.

What the person does:
- Check that the xcl release xcl-python will pin contains three things: the parity program's `pretty_log` stage (task "Record the pretty log in the Go parity program"), the printing core, engine and parity changes from the plans this one builds on, and saved-record rendering during an operation. If the printing release has already been cut without the stage, merge the xcl PR with its release label so a new xcl release is cut.
- In xcl-python, set `xcl-ref.txt` to that release tag, on this work's branch or by pushing this work onto the automated engine-bump pull request. Confirm CI passes, including the parity suite, and merge with the `minor` label so the next minor version is tagged and published.
- After the release is published, add a line to its generated GitHub release notes: the asyncio client now delivers events to `on_event` one at a time and in order, so a handler that relied on running concurrently may slow an operation (decided during epic planning).
- After the release is published, merge the website PR.

How the result is checked: the published release's GitHub notes contain the asyncio ordering line; the published xcl-python version's `xcl.engine_version()` reports the pinned xcl release; `import xcl.prettylog` works from the PyPI install; the website shows the new tabs.

## Testing Strategy

Per task:

- **Deliver asyncio events in order**: unit tests in `xcl-python:tests/unit/test_aio_events.py` over the fake transport (order with a slow plain handler and with an awaiting coroutine handler, one at a time, returns after the last event, exceptions dropped, cancellation). The existing asyncio event tests are the regression check.
- **Let a handler reach the configuration that delivered its event**: unit tests in `tests/unit/test_events.py` and `tests/unit/test_config.py`. The printing plan's `tests/unit/test_logbridge.py` must pass unchanged.
- **Lay out event lines as Go's example writes them**: `tests/unit/test_prettyline.py`, literal events in and Go-produced literal lines out, one rule per test, with a recording styler for styles.
- **Add the pretty logger**: `tests/unit/test_prettylog.py`, with hand-built events and fake-transport configurations for every handler decision, including asyncio.
- **Prove the pretty logger's behaviour against the real engine**: `tests/integration/test_prettylog.py` and additions to `tests/integration/test_aio.py`, with the recording plugin and pseudo-terminals where available.
- **Record the pretty log in the Go parity program**: testify tests in `xcl:bindings/libxcl/cmd/parity/main_test.go`. The determinism tests cover the new stage.
- **Compare the pretty log with Go in the Python parity suite**: the existing per-fixture comparisons in `tests/parity/test_parity.py`.
- **Documentation tasks**: reviewed by a person (manual items in plan.md's Testing Approach). No test reads docs.
- **Release task**: checked by the published version and its engine version.

Conventions: one behaviour per test function, no `pytest.mark.parametrize` tables, positive and negative cases in separate functions, Python tests with plain `assert`, Go tests with testify `require`, and no test reading README, PARITY.md, the website or the changelog. Manual checks are carried into the implementation test plan: the README review, the website review with a site build, and the look in a real terminal with and without `NO_COLOR`.

## Project References

- Spec: `20261023075328-741138ee-python-prettylog`, epic `20261023074219-4357d314-printing-and-pretty-output`.
- Design: `printing-and-highlighting.md` (source `design`).
- Plans built on: `20261023075302-bad71237-printing-and-highlighting-python`, `20261023074627-cc23fc08-encode-saved-entity-during-operation`, `20261023074223-e06f835f-event-data-level-enum`, `20261023074926-eaa51527-printing-and-highlighting-core`.
- Knowledge: `xcl:conventions/testing-and-mocking.md`, `xcl:conventions/dependencies.md`, `xcl:conventions/development-standards.md`, `xcl:conventions/code-style.md`.
- Repo roots:
  - xcl `/home/nicj/code/github.com/jumppad-labs/xcl`
  - xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`
  - xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`
- Reference source: `~/go/pkg/mod/github.com/charmbracelet/log@v0.4.2/text.go`, `styles.go`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The high-complexity task is "Add the pretty logger". The layout task is held to Go-produced expected strings, so start it with the throwaway Go program. Run unit tests before the integration suites, which need the engine and fixture plugins built from the xcl branch.

## Migration Notes

- No public API is removed, and every existing call keeps its signature.
- `xcl.aio.Config` now delivers events to `on_event` one at a time and in order. A handler that relied on overlapping calls is serialised. The README notes it. The release is a `minor` (pre-1.0), and no release-notes line is required beyond the generated one.
- Development pins `xcl-ref.txt` to the xcl branch carrying the parity stage, and main pins only a released tag.

## Performance Considerations

- Each created resource costs one extra engine round trip (`EncodeSavedEntity`) inside the handler, on the handler's thread. The synchronous driver polls the next event only after the handler returns, as the spec requires (configuration before the next line). The asyncio render bypasses the poll executor, so it never waits up to the 200 ms poll slice.
- Asyncio's ordered delivery means a slow handler delays later events, not the operation. The engine never waits on the host for events, and state and function tasks stay concurrent.
- Each styled piece of a line is formatted through the formatter separately, a few small `StringIO` writes per line, which is negligible next to the engine round trip.
