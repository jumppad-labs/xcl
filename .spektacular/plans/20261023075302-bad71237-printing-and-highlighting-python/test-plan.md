---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Test plan: 20261023075302-bad71237-printing-and-highlighting-python

The spec defines no post-delivery success metric; every acceptance criterion that can be automated is covered by the xcl-python unit, integration, reference-output and parity tests and by the Go parity program's tests. What follows are the manual reviews the plan's Testing Approach lists, plus the release work descoped to a person after the merge.

Setup used by several procedures (in an xcl-python checkout carrying this change, with an xcl checkout carrying it at `<xcl>`):

```shell
uv venv -p 3.12 .venv && uv pip install -p .venv/bin/python -e '.[test]'
make protos engine plugins XCL_SRC=<xcl> PYTHON=.venv/bin/python
mkdir -p /tmp/xcl-manual/config && cat > /tmp/xcl-manual/config/main.xcl <<'EOF'
app "web" {
  image = "nginx"
}

app "api" {
  image = "nginx"
  timeout {
    seconds = 5
  }
}
EOF
cat > /tmp/xcl-manual/app.py <<'EOF'
from dataclasses import dataclass
import xcl

@dataclass
class App:
    image: str
    meta: xcl.Meta | None = None

local = xcl.LocalRegistry()
local.register_type(App, "app")
cfg = xcl.Config(registries=[local])
EOF
```

## 1. Package documentation (manual review)

- **Who / when**: a maintainer, at review of the xcl-python pull request.
- **Where**: xcl-python `README.md` sections "Printing and highlighting" (Printing errors, Printing resources and change summaries, Output formats, Themes, On a website, Writing a format), "Logging", "Migrating from highlight=", "Errors"; `PARITY.md` sections "Lookups", "Printing", "Logging", "Not applicable".
- **Look for**: Python examples of error printing, how reports appear on unhandled and logged errors (exception notes), the five output formats and `for_stream`, themes (`DEFAULT_THEME | {...}`, `validate`, `load_vscode_theme`), stylesheet generation (`css`), custom formats (the `Formatter` protocol), and logging (`logging_handler`, `LogFormatter`); the migration note names `print_entity(..., formatter=xcl.highlight.TERMINAL)`, `print_saved_entity` and `entity_tokens` as the replacement for `highlight=`; PARITY.md pairs each Go printing, theme, format and logging function with its Python form and no longer lists `highlight=`.
- **Pass**: every topic above is present with a correct, runnable Python example, and nothing describes `highlight=` as a current option.

## 2. Website (manual review)

- **Who / when**: a maintainer, at review of the xcl-website pull request, after the xcl-python release it documents (see 7).
- **How**: in the xcl-website checkout, `npm ci && npm run build` (must finish with "23 page(s) built"), then `npm run preview` and open `/configuration-text/`, `/problems/`, `/diff/`, `/events/`, `/examples/plugins/` and `/examples/configuration-only/`.
- **Look for**: each page's Python tabs show Python printing, formats, themes, stylesheet generation, custom formats and logging (problems: `print_error`, `format_error`, `width=`, "In a traceback", "In logs" with `LogFormatter`; configuration-text: `print_entity`, formats, themes, `validate`, `load_vscode_theme`, `css`, a custom format, `entity_tokens`; diff: `print_diff` with colour, themes, HTML and `diff.tokens`; events: `xcl.logging_handler`; the plugins example's `run`, `plan` and `inspect` listings print through the package). Run `grep -rni "not yet available\|is coming\|highlight=" src/pages`: only custom registries, custom maskers and in-process plugins may remain.
- **Pass**: the site builds, every existing page URL still opens, and no page says the report, rendered diffs, editor themes or custom formats are unavailable from Python.

## 3. Terminal colour and NO_COLOR (manual check)

- **Who / when**: a maintainer, before release, in a real terminal (not an IDE's output pane).
- **How**: from `/tmp/xcl-manual`:
  ```shell
  PYTHONPATH=. <xcl-python>/.venv/bin/python -c "
  import sys, xcl; from app import cfg
  try: cfg.apply('config')
  except xcl.Error as err: xcl.print_error(err, formatter=xcl.highlight.for_stream(sys.stderr))"
  ```
  then the same command with `NO_COLOR=1` set, and once with stderr redirected to a file (`2> out.txt`).
- **Expected**: in the terminal the report is coloured (bold red "Error", bold summary, dim line-number gutter, the faulty `timeout` in bold red), and no colour bleeds past a line end; with `NO_COLOR=1` and in `out.txt` the same report has no escape codes.
- **Pass**: all three match.

## 4. HTML, inline HTML and SVG in a browser (manual check)

- **Who / when**: a maintainer, before release.
- **How**: from `/tmp/xcl-manual`:
  ```shell
  PYTHONPATH=. <xcl-python>/.venv/bin/python -c "
  import xcl; from app import cfg
  from xcl.highlight import HTML, HTML_INLINE, SVG, css, DEFAULT_THEME
  try: cfg.apply('config')
  except xcl.Error as err:
      open('report.html','w').write('<style>'+css(DEFAULT_THEME)+'</style>'+xcl.format_error(err, formatter=HTML))
      open('report.inline.html','w').write(xcl.format_error(err, formatter=HTML_INLINE))
      open('report.svg','w').write(xcl.format_error(err, formatter=SVG))"
  ```
  and open the three files in a browser.
- **Expected**: each shows the line-7 report with the source lines aligned in a monospace font and the same colours as the terminal; the SVG's box fits the longest line.
- **Pass**: all three render legibly with colours applied and no markup visible as text.

## 5. Third-party tracebacks and loggers (manual check)

- **Who / when**: a maintainer, before release; rich, loguru and structlog are not dependencies, so install them in a scratch venv (`uv pip install -p .venv/bin/python rich loguru structlog`).
- **How**:
  - rich: a script that calls `rich.traceback.install()` then `cfg.apply('config')` uncaught.
  - loguru: `from loguru import logger`, then `try: cfg.apply('config')` / `except xcl.Error: logger.exception("apply failed")`.
  - structlog: configure the stdlib root logger with a handler whose formatter is `structlog.stdlib.ProcessorFormatter(processors=[structlog.stdlib.ExtraAdder(), structlog.processors.JSONRenderer()])`, then apply a valid configuration with `xcl.Config(registries=[local], on_event=xcl.logging_handler(logging.getLogger("xcl")))`.
- **Expected**: rich's traceback and loguru's logged exception both show the plain report (the `Error: Unsupported block type` block) after the exception message; structlog's JSON lines carry `operation`, `phase` and `resource` fields for events such as `create success`.
- **Pass**: all three show what is expected with no adapter code.

## 6. Release notes name the replacement for highlight= (descoped release task; manual check)

- **Who / when**: the person releasing xcl-python, right after the release is published (see 7).
- **How**: open the GitHub release page of the new xcl-python minor version.
- **Expected**: its notes contain the added line: "Breaking: `highlight=` was removed from `encode_entity` and `encode_saved_entity`; use `cfg.print_entity(entity, formatter=xcl.highlight.TERMINAL)` (or `print_saved_entity`) to print coloured configuration text, or `entity_tokens` for labelled text."
- **Pass**: the line is present.

## 7. Release the engine and the package (descoped task, done by a person after the merge)

- **Who / when**: the maintainer releasing, after this spec's branches are merged.
- **Steps**:
  1. In xcl, confirm the printing core, the engine labelled-text change and this spec's parity-program change (`bindings/libxcl/cmd/parity`, fixture `testdata/parity/unsupported_block`) are merged, and release them through the release label on their pull requests (a separate `patch` release if the parity change merges after the other two are released).
  2. In xcl-python, set `xcl-ref.txt` (still `v0.11.1` on this branch) to that release tag, on this work's branch or by pushing this work onto the automated `engine/<tag>` pull request, closing the other. CI regenerates the stubs from the pin, so the pin bump and this change must land together.
  3. Merge the xcl-python pull request with the `minor` label so the next minor version is published, then add the release-notes line in 6.
  4. Merge the website pull request after the package release is on PyPI.
- **Expected**: `python -c "import xcl; print(xcl.engine_version())"` from the released package reports the xcl release tag; xcl-python's checks pass on main, including the parity suite against the released parity program; the release page shows the line in 6; xcl.dev shows the new Python tabs.
- **Pass**: all four hold.
