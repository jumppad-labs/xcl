---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023075328-741138ee-python-prettylog

The spec's one success metric ("the same apply run through Go's example and through the Python pretty logger gives the same event lines and configuration, apart from timestamps and the event-attribute order") is covered by an automated behavioural test: the `pretty_log` parity stage, recorded by xcl's `bindings/libxcl/cmd/parity` and by xcl-python's `tests/parity/run_python.py`, and compared on every shared fixture by `tests/parity/test_parity.py`. It is not repeated here.

The plan lists four manual checks. Each is below.

## 1. README review (spec acceptance criterion "Docs")

- **Who / when**: the reviewer of the xcl-python pull request, before merge.
- **Where**: xcl-python `README.md`, section "## Logging" > "### Pretty logging", and section "## Asyncio"; `PARITY.md`, the table after the "## Logging" table.
- **What to look for**:
  - the one-line setup on `xcl.Config` (`on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env())` with `event_data=xcl.EventDataLevel.PROCESSED`) and on `xcl.aio.Config`;
  - a sample of the output: a `create success` line with the resource's configuration indented beneath it, including `output = "one-a"`;
  - the level (`logging.INFO` by default) and `XCL_LOG_LEVEL` (`debug`, `info`, `warn`, `error`, any case, info otherwise);
  - that resource configuration needs processed event data, and the once-only line saying so;
  - reports beneath configuration mistakes at every level, and only event lines and reports above info;
  - colour only on a terminal with `NO_COLOR` unset;
  - the logger form `handler(logger=logging.getLogger("app"))`, with `%(message)s` giving identical lines, and records carrying the event's fields;
  - the Asyncio section saying events reach `on_event` one at a time, in order;
  - PARITY.md pairing `prettylog.Handler`, `LevelFromEnv`, `LevelEnv` with `xcl.prettylog.handler`, `level_from_env`, `LEVEL_ENV`.
- **Pass**: every item above is present and accurate against `xcl/prettylog.py`.

## 2. Website events page review (spec acceptance criterion "Docs")

- **Who / when**: the reviewer of the xcl-website pull request, before merge, after the xcl-python release it documents is published (see check 4).
- **How**: in the xcl-website checkout, `npm ci && npm run dev`, then open `http://localhost:4321/events/` (or `npm run build` and open `dist/events/index.html`).
- **What to look for**: in "A pretty receiver", the Python tab shows `xcl.prettylog`'s setup and a sample of its output; no hand-written Python receiver, `RANK`/`COLOUR` table or `\033` escape remains; the second Python tab shows the one-line `xcl.Config(on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env()))`; the "Ordered, one at a time" guarantee mentions both Python clients; every link on the page opens, and the other pages' URLs are unchanged.
- **Pass**: `npm run build` exits 0 and every item above holds.

## 3. Look in a real terminal

- **Who / when**: the xcl-python pull request reviewer, before merge, on Linux or macOS (and once on Windows Terminal if available).
- **Setup**: in an xcl-python checkout with the engine and plugins built (`make engine plugins XCL_SRC=<xcl checkout>`), save this as `/tmp/pretty_demo.py`, with a configuration directory `/tmp/pretty-demo/main.xcl` holding one `resource "recording" "one" { log = "/tmp/pretty-demo/log.jsonl"  value = "a" }`, and a second file `/tmp/pretty-bad/main.xcl` adding a nested `nope { a = 1 }` block inside a second recording resource:

  ```python
  import sys, xcl
  reg = xcl.LocalRegistry(); reg.register_external_plugin("build/plugins/xcl-plugin-recording")
  cfg = xcl.Config(registries=[reg], state_path="/tmp/pretty-demo/state",
                   on_event=xcl.prettylog.handler(sys.stderr, xcl.prettylog.level_from_env()),
                   event_data=xcl.EventDataLevel.PROCESSED)
  cfg.apply(sys.argv[1])
  ```

- **How**: run `python /tmp/pretty_demo.py /tmp/pretty-demo` in a terminal; then `python /tmp/pretty_demo.py /tmp/pretty-bad` (it ends with a `ConfigError` traceback); then each again with `NO_COLOR=1`, and with `2> /tmp/out.txt`; then with `XCL_LOG_LEVEL=debug` and `XCL_LOG_LEVEL=warn`.
- **Expected**: in the terminal the level labels are coloured (DEBU blue, INFO cyan, WARN yellow, ERRO red, all bold), the `source`, `operation` and `resource` keys and values are magenta, yellow and cyan (values bold), other keys and `=` are dim, the configuration block is coloured by the default theme (block type bold magenta, attributes blue, strings yellow), and the unsupported block's report sits indented beneath its `ERRO` line, coloured, once. With `NO_COLOR=1` or redirected to a file there are no escape codes. `debug` adds `DEBU` lines; `warn` shows no `INFO` lines and no configuration, but still the report.
- **Pass**: all of the above, and the layout matches a run of xcl's own `example/plugin` with the same `XCL_LOG_LEVEL`.

## 4. Release (descoped task, done by a person after the merge)

- **Who / when**: the xcl and xcl-python maintainer, after this spec's branches are merged.
- **What to do**:
  1. Make sure the xcl release xcl-python will bundle contains the parity program's `pretty_log` stage (`bindings/libxcl/cmd/parity`), the printing core, engine and parity work, and saved-record rendering during an operation. If the printing release already went out without the stage, merge the xcl change with its release label so a new xcl release is cut.
  2. In xcl-python, set `xcl-ref.txt` to that release tag (on this work's branch or on the automated engine-bump pull request), confirm CI passes including the parity suite (`tests/parity`), and merge with the `minor` label so the next minor version is tagged and published.
  3. Once the release is published, add this line to its generated GitHub release notes: "The asyncio client now delivers events to `on_event` one at a time and in order, as the synchronous client and Go do; a handler that relied on running concurrently may slow an operation."
  4. Merge the xcl-website change after the release is published.
- **Pass**: the published release's notes contain the asyncio ordering line; `python -c "import xcl; print(xcl.engine_version())"` from the PyPI install reports the pinned xcl release; `python -c "import xcl.prettylog"` works from the PyPI install; xcl.dev's events page shows the new Python tabs.
