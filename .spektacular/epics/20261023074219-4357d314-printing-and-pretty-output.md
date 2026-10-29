---
created_date: "2026-10-23"
document_status: draft
specs:
    - name: 20261023074223-e06f835f-event-data-level-enum
      depends_on: []
    - name: 20261023074627-cc23fc08-encode-saved-entity-during-operation
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
    - name: 20261023074926-eaa51527-printing-and-highlighting-core
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
        - 20261023074627-cc23fc08-encode-saved-entity-during-operation
    - name: 20261023075136-42c51a7d-printing-tokens-over-the-engine
      depends_on:
        - 20261023074926-eaa51527-printing-and-highlighting-core
    - name: 20261023075302-bad71237-printing-and-highlighting-python
      depends_on:
        - 20261023074926-eaa51527-printing-and-highlighting-core
        - 20261023075136-42c51a7d-printing-tokens-over-the-engine
    - name: 20261023075328-741138ee-python-prettylog
      depends_on:
        - 20261023074223-e06f835f-event-data-level-enum
        - 20261023074627-cc23fc08-encode-saved-entity-during-operation
        - 20261023075302-bad71237-printing-and-highlighting-python
---

## Overview

Reading xcl's output is harder than it should be, and in Python much of it is not possible at all. Since v0.8.0 a configuration error prints as one line with no source. Highlighting needs TextMate scope strings, raw escape codes and a VS Code theme blob. Python gets finished ANSI bytes or nothing. A Python program cannot show what each resource became as an apply runs, because encoding is refused while an operation is in progress.

This epic makes xcl's output readable by default and easy to restyle in every language. The design is fixed by `design/printing-and-highlighting.md`: the core lexes and lays out tokens, each language formats them through themes and formatters, errors carry their report, and the usual loggers need little or no adapter. Around it sit three smaller changes that a pretty Python logger needs: event data levels as an enum, encoding a saved record while an operation runs, and the logger itself. Documentation for each change ships with it.

## Specs

| # | Spec | Scope |
|---|---|---|
| 1 | 20261023074223-e06f835f-event-data-level-enum | Python event data levels become a top-level enum, accepted only as the enum. |
| 2 | 20261023074627-cc23fc08-encode-saved-entity-during-operation | Saved records can be rendered while an operation runs, so Python can show resources as they are created. |
| 3 | 20261023074926-eaa51527-printing-and-highlighting-core | Go: one-call printing of errors (with source context), resources and diffs; tokens, themes and formatters (plain, terminal, HTML classes, HTML inline, SVG). |
| 4 | 20261023075136-42c51a7d-printing-tokens-over-the-engine | The engine sends other languages labelled reports on every error, labelled resource and diff text, and converted editor themes. |
| 5 | 20261023075302-bad71237-printing-and-highlighting-python | Python: the same printing, formats and themes as Go, reports in tracebacks and logs, logging bridge. |
| 6 | 20261023075328-741138ee-python-prettylog | Python pretty logger modelled on Go's example, showing resources as they are created. |

## Release plan

**Covers:** 20261023074223-e06f835f-event-data-level-enum, 20261023074627-cc23fc08-encode-saved-entity-during-operation, 20261023074926-eaa51527-printing-and-highlighting-core, 20261023075136-42c51a7d-printing-tokens-over-the-engine, 20261023075302-bad71237-printing-and-highlighting-python, 20261023075328-741138ee-python-prettylog (all implemented and merged locally)

**Repos:**
- xcl: branch `f-python-prettylogger`, 13 commits not pushed (5 spec merges, plus the spec and plan commits)
- xcl-python: local `main`, 8 commits not pushed (4 spec merges)
- xcl-website: local `main`, 10 commits not pushed (5 spec merges)

**Versions:** xcl **v0.12.0** (`minor`, after v0.11.1) · xcl-python **v0.14.0** (`minor`, after v0.13.0) · xcl-website deploys, no version. Confirm the latest tags with `git fetch --tags` before Step 3; the auto-tag takes the next number from the tags on GitHub.

### Summary

One xcl release (v0.12.0) carries every engine, protocol, Go-library and parity-program change of the epic. Then one xcl-python release (v0.14.0) carries the four Python specs with the engine pin moved to v0.12.0. The website goes last, once v0.14.0 is on PyPI.

The Python work lands through our own pull request, not the automatic engine-bump pull request:
- **The bump would fail.** v0.12.0 reserves the protocol's `highlight` field, and xcl-python's CI regenerates its stubs from the pin. The bump pull request, which carries only the pin change, therefore fails against the old Python code.
- **Our own pull request can be tested first.** It runs xcl-python's full CI matrix against the unreleased xcl commit before anything is released.

### Preconditions

- [ ] **Every spec is merged.** Check with `spektacular status 20261023074219-4357d314-printing-and-pretty-output`, which should show 6/6 implemented.
- [ ] **Spektacular's records are committed in each repo.** These are the specs, plans, changelogs, knowledge entries and design, so the pull requests carry them. They are uncommitted today:
  - xcl: the modified `.spektacular/` files.
  - xcl-python and xcl-website: the untracked `.spektacular/changelog/xclconfig/` and `.spektacular/knowledge/` files.

  The commands are in Step 1. Decide whether `.spektacular/knowledge/.usage-index.json` is committed or ignored.
- [ ] **No uncommitted code in any repo.** Check with `git -C <repo> status --short`, which should list only `.spektacular/` files until Step 1.
- [ ] **Local tools.**
  - Go 1.27, buf, protoc, uv, Python 3.11 or later, Node and npm.
  - A Docker engine, for the plugin example.
  - `gh`, authenticated with rights to label and merge in all three repos.
  - `gpg`, to verify signatures.
- [ ] **The release automation's secrets are still configured.** They worked for xcl v0.11.1 and xcl-python v0.13.0. Nothing to do unless a step fails:
  - xcl: `RELEASE_APP_ID`, `RELEASE_APP_PRIVATE_KEY`, `XCL_RELEASE_GPG_KEY`, `XCL_RELEASE_GPG_PASSPHRASE`, `XCL_PYTHON_DISPATCH_TOKEN`.
  - xcl-python: the release App secrets, and PyPI Trusted Publishing for the `pypi` environment.

### Pre-release testing (before anything is pushed)

#### Automated

Run these from `/home/nicj/code/github.com/jumppad-labs`.

- [ ] **xcl Go module, against itself:**
  ```shell
  cd xcl
  go test ./... -race
  go vet ./...
  ```
  Expected: all pass.
- [ ] **xcl protocol checks:**
  ```shell
  cd xcl
  git fetch origin main:main      # buf breaking compares with the local main branch
  make buf-lint
  make buf-breaking
  ```
  Expected: both exit 0. This was last checked with buf 1.27.2; CI uses 1.50.0 (Step 3).
- [ ] **xcl examples and shared library:**
  ```shell
  cd xcl
  make test-examples          # the plugin example's smoke tests need Docker
  make test-libxcl
  (cd bindings/libxcl && go test ./...)
  ```
  Expected: all pass.
- [ ] **xcl-python's full suite, against the local xcl on `f-python-prettylogger`:**
  ```shell
  cd xcl-python
  uv venv -p 3.12 .venv && uv pip install -p .venv/bin/python -e '.[test]'
  make protos engine plugins libxcl XCL_SRC=../xcl PYTHON=.venv/bin/python
  git status --short xcl/_core          # must be empty: committed stubs match the protocol
  GITHUB_TOKEN=$(gh auth token) .venv/bin/python -m pytest -rs
  GITHUB_TOKEN=$(gh auth token) XCL_TRANSPORT=inprocess .venv/bin/python -m pytest -rs
  ```
  Expected:
  - The default run gives 958 passed and 1 skipped, the skip being an in-process-only load test.
  - The in-process run passes with only route skips listed.
  - `GITHUB_TOKEN` keeps the end-to-end tests within GitHub's API rate limit.
- [ ] **xcl-website build:**
  ```shell
  cd xcl-website
  npm ci
  npm run build
  npx astro check
  ```
  Expected: "23 page(s) built" and no errors from `astro check`.

#### Manual

Each check's full procedure is in its spec's test plan: `spektacular plan file read <spec> test-plan`, check N. All of them run on local builds.

**xcl** (best done while the Step 3 pull request is open, before merge):
- [ ] **Breaking list.** Every removed `highlight` and `xcl` function has a correct one-line replacement in `CHANGELOG.md`. Confirm the names with `go doc`. *(printing-and-highlighting-core, check 1)*
- [ ] **Reference outputs.**
  - The SVG, HTML with `default.css` and inline HTML in `highlight/testdata/reference/` look right in a browser.
  - The `*.terminal.txt` files look right in a terminal.
  - `highlight.For` gives colour on a terminal and plain text with `NO_COLOR=1` or when piped.

  *(printing-and-highlighting-core, check 3)*
- [ ] **Example reports.** `configonly`, `plugin` (needs Docker) and `remoteplugin` (needs the network) each print a coloured report for `testdata/broken`. An application error still prints as one line. *(printing-and-highlighting-core, check 4)*
- [ ] **Protocol comments.** The comments in `proto/xcl/core/v1/core.proto` describe the new output, and nothing still says the engine colours text. *(printing-tokens-over-the-engine, review 2)*
- [ ] **Changelog sections.**
  - The printing-tokens-over-the-engine section: what other languages receive, and its three Breaking items. *(printing-tokens-over-the-engine, review 4)*
  - The encode-saved-entity-during-operation section: what is allowed and refused during an operation, and "Breaking: None". *(encode-saved-entity-during-operation, check 4)*
- [ ] **The `libxcl.md` design.** Read it with `spektacular design read --data '{"source":"design","path":"libxcl.md"}'`. It should:
  - say the saved-record render is allowed during an operation and every other lookup is refused;
  - describe the report on errors, labelled text, `ConvertTheme` and the buf rule exactly as the protocol does.

  *(encode-saved-entity-during-operation, check 5; printing-tokens-over-the-engine, review 3)*

**xcl-python** (on the local build from the automated step):
- [ ] **pyright.** It reports exactly two `reportArgumentType` errors for `event_data="processed"` and none for `xcl.EventDataLevel.PROCESSED`. *(event-data-level-enum, check 1)*
- [ ] **No bare-string event levels in docs.** No bare-string level is documented in the README or on the site, and the README migration note says strings are refused. *(event-data-level-enum, check 2)*
- [ ] **PARITY.md event data row.** It pairs Go's `WithEventData` and `xcl.EventDataLevel` with the Python enum. *(event-data-level-enum, check 3)*
- [ ] **Encoding docstrings and README.**
  - The `encode_saved_entity` and `encode_entity` docstrings in `xcl/config.py` and `xcl/aio.py` are correct.
  - The README's "Encoding entities as configuration" section prints each created resource on its success event.

  *(encode-saved-entity-during-operation, check 6)*
- [ ] **Printing and logging docs.** The README's "Printing and highlighting", "Logging", "Migrating from highlight=" and "Errors" sections, and the PARITY.md printing and logging tables, are complete and runnable. *(printing-and-highlighting-python, check 1)*
- [ ] **Terminal colour and `NO_COLOR`.** `print_error` with `for_stream(sys.stderr)` is coloured in a terminal, and has no escape codes with `NO_COLOR=1` or when redirected. *(printing-and-highlighting-python, check 3)*
- [ ] **HTML, inline HTML and SVG.** The three files from `format_error` render legibly in a browser. *(printing-and-highlighting-python, check 4)*
- [ ] **Third-party tracebacks and loggers.**
  - rich and loguru tracebacks show the plain report.
  - structlog's JSON lines carry `operation`, `phase` and `resource` with no adapter.

  *(printing-and-highlighting-python, check 5)*
- [ ] **Pretty logging docs.** The README's "Pretty logging" and "Asyncio" sections, and the PARITY.md prettylog table, are accurate against `xcl/prettylog.py`. *(python-prettylog, check 1)*
- [ ] **The pretty logger in a real terminal.**
  - The level labels, keys and configuration are coloured.
  - The unsupported block's report appears once, beneath its `ERRO` line.
  - There is no colour with `NO_COLOR=1` or in a file.
  - `XCL_LOG_LEVEL=debug` adds `DEBU` lines, and `warn` hides `INFO` lines and configuration but keeps the report.
  - The layout matches xcl's `example/plugin`.

  *(python-prettylog, check 3)*

**xcl-website** (on `npm run preview` from the automated step):
- [ ] **Event levels on the site.** The events, configuration-text, configuration-only and plugins pages use `xcl.EventDataLevel` in their Python tabs. *(event-data-level-enum, check 2)*
- [ ] **Saved-record rendering.** `configuration-text.mdx` ("Converting saved data") and `events.mdx` say saved records render during an operation, and that `cfg.encode_entity` is refused then. *(encode-saved-entity-during-operation, check 7)*
- [ ] **Go printing pages.**
  - The Go printing, format, theme, stylesheet, custom-format and logging examples are on every listed page.
  - No removed function (`Pieces`, `Renderer`, `NewANSI`, `WithThemeFile`, `Scope*`, `DiagnosticPieces`) is documented.
  - Every navigation link and the `#printing-and-highlighting` and `#showing-problems-to-a-person` anchors work.

  *(printing-and-highlighting-core, check 2)*
- [ ] **Python printing pages.**
  - The Python tabs show printing, formats, themes, `css`, custom formats and logging.
  - `grep -rni "not yet available\|is coming\|highlight=" src/pages` finds only the custom registries, custom maskers and in-process plugins.

  *(printing-and-highlighting-python, check 2)*
- [ ] **Events page.** "A pretty receiver" shows `xcl.prettylog`'s setup and output, with no hand-written receiver left. "Ordered, one at a time" names both Python clients. *(python-prettylog, check 2)*

### Release steps

#### Step 1: All repos, commit Spektacular's records

- **Who:** a person.
- **Do:**
  ```shell
  cd /home/nicj/code/github.com/jumppad-labs
  git -C xcl add .spektacular && git -C xcl commit -m "Record the printing and pretty output epic's specs, plans and changelogs"
  git -C xcl-python add .spektacular && git -C xcl-python commit -m "Record the printing and pretty output epic's changelogs and knowledge"
  git -C xcl-website add .spektacular && git -C xcl-website commit -m "Record the printing and pretty output epic's changelogs"
  ```
  Leave out `.usage-index.json` if you decided to ignore it.
- **What happens next by itself:** nothing.
- **Check before moving on:** `git -C <repo> status --short` is empty in all three.
- **If it fails:** nothing is pushed; amend the commits.

#### Step 2: Run the pre-release testing above

- **Who:** a person.
- **Do:** every automated check, and the manual checks that need only local builds.
- **Check before moving on:** every automated check passes.
- **If it fails:** fix it on the branch and repeat. Nothing has been pushed yet.

#### Step 3: xcl, open the release pull request (do not merge yet)

- **Who:** a person.
- **Do:**
  ```shell
  cd /home/nicj/code/github.com/jumppad-labs/xcl
  git push -u origin f-python-prettylogger
  gh pr create --base main --head f-python-prettylogger \
    --title "Readable output: error reports, highlight formatters and labelled text over the engine" \
    --body "Implements epic printing-and-pretty-output (see CHANGELOG.md sections for each spec). Breaking: see CHANGELOG.md."
  gh pr edit --add-label minor --add-label breaks-bindings
  git rev-parse HEAD          # note this SHA for Step 4
  ```
  **Why `breaks-bindings`:** the Bindings check runs xcl-python's **current `main`** suite against this commit. That suite still sends `highlight=`, so it fails against the reserved field. The label records that the break is intended, and that it is fixed by the xcl-python pull request in Step 4. Step 4 runs the real cross-language test.
- **What happens next by itself:** CI runs Go, Examples, Release label and Bindings (skipped by the label). The PR title becomes the line in v0.12.0's generated release notes.
- **Check before moving on:**
  - The Go workflow is green, including "Lint the protos" and "Check the protos for breaking changes" on buf 1.50.0. If buf 1.50.0 reports the reserved-field removal as breaking, stop and raise it; do not loosen the buf rules further. *(printing-tokens-over-the-engine, review 1)*
  - Examples and Release label are green.
  - The xcl manual checks above are done.
- **If it fails:** fix it on `f-python-prettylogger` and push. The PR re-runs; nothing is released until it merges.

#### Step 4: xcl-python, open the release pull request against the unreleased engine (do not merge yet)

- **Who:** a person.
- **Do:**
  ```shell
  cd /home/nicj/code/github.com/jumppad-labs/xcl-python
  git switch -c release/printing-and-pretty-output main
  echo "<SHA from Step 3>" > xcl-ref.txt      # temporary: CI builds the engine from this xcl commit
  git commit -am "Test against the unreleased xcl engine (temporary pin)"
  git push -u origin release/printing-and-pretty-output
  gh pr create --draft --base main --head release/printing-and-pretty-output \
    --title "Print like Go, pretty logging, EventDataLevel enum, in-order asyncio events" \
    --body "Implements epic printing-and-pretty-output for Python. Pins xcl v0.12.0 before merge."
  gh pr edit --add-label minor
  ```
- **What happens next by itself:** CI runs the full test matrix against the xcl commit: every platform and supported CPython, musl, and the in-process route, all strict, plus the parity suite against the Go parity program built from that commit.
- **Check before moving on:** the CI `result` job is green, and the Release label check passes.
- **If it fails:**
  - **A real failure:** fix it on this branch, or on the xcl branch with a new SHA, and push.
  - **CI refuses a non-tag pin:** skip this dry run and rely on the local suite from Step 2. Pin `v0.12.0` in Step 7 as planned.

#### Step 5: xcl-website, open the pull request (do not merge yet)

- **Who:** a person.
- **Do:**
  ```shell
  cd /home/nicj/code/github.com/jumppad-labs/xcl-website
  git switch -c release/printing-and-pretty-output main
  git push -u origin release/printing-and-pretty-output
  gh pr create --draft --base main --head release/printing-and-pretty-output \
    --title "Document readable output, Python printing and pretty logging" \
    --body "Merge only after xcl-lang v0.14.0 is on PyPI: merging deploys xcl.dev."
  ```
- **What happens next by itself:** nothing. The site has no PR checks; it deploys on a push to `main`.
- **Check before moving on:** the website manual checks above are done.
- **If it fails:** fix it on the branch.

#### Step 6: xcl, merge and release v0.12.0

- **Who:** a person merges; the automation does the rest.
- **Do:** `gh pr merge <xcl PR> --merge -R jumppad-labs/xcl`
- **What happens next by itself:**
  1. `tag.yml` pushes `v0.12.0`.
  2. `release.yml` builds `xcl-core` for eight targets with cgo disabled, signs the checksums, and creates the GitHub release with generated notes.
  3. It dispatches xcl-python, whose `engine-bump.yml` opens `engine/v0.12.0` with the `minor` label and auto-merge on.
- **Check before moving on:**
  - `gh run list -R jumppad-labs/xcl --workflow release.yml -L 1` shows success.
  - `gh release view v0.12.0 -R jumppad-labs/xcl` lists the assets:
    - eight archives: linux amd64 and arm64, their two `_musl` copies, darwin amd64 and arm64, windows amd64 and arm64;
    - `xcl-core_0.12.0_checksums.txt`;
    - `xcl-core_0.12.0_checksums.txt.sig`.
  - The signature verifies:
    ```shell
    cd "$(mktemp -d)"
    gh release download v0.12.0 -R jumppad-labs/xcl -p 'xcl-core_0.12.0_checksums.txt*' -p 'xcl-core_0.12.0_linux_amd64.tar.gz'
    gpg --import /home/nicj/code/github.com/jumppad-labs/xcl-python/tests/keys/jumppad-labs.asc
    gpg --verify xcl-core_0.12.0_checksums.txt.sig xcl-core_0.12.0_checksums.txt
    sha256sum --ignore-missing -c xcl-core_0.12.0_checksums.txt
    tar xzf xcl-core_0.12.0_linux_amd64.tar.gz && ./xcl-core --version      # prints v0.12.0
    ```
  - The tag holds the epic's changes: `git fetch --tags && git tag --contains <merge commit>` lists `v0.12.0`. At the tag, `coreservice/lookups.go` has `encodeSavedEntity` calling `requireEntity` and not `lookupAllowed`. *(encode-saved-entity-during-operation, check 1)*
  - The xcl release notes from "Release notes" below have been added: `gh release edit v0.12.0 -R jumppad-labs/xcl --notes-file <file>`.
- **If it fails:**
  - **The release workflow fails before publishing:** re-run it by hand for the tag with `gh workflow run release.yml -R jumppad-labs/xcl -f tag=v0.12.0 -f publish=true`.
  - **The code is wrong:** see Rollback.

#### Step 7: xcl-python, stop the automatic bump and pin v0.12.0 on the release pull request

- **Who:** a person.
- **Do:** as soon as `engine/v0.12.0` appears in `gh pr list -R jumppad-labs/xcl-python`:
  ```shell
  gh pr merge engine/v0.12.0 -R jumppad-labs/xcl-python --disable-auto
  gh pr close engine/v0.12.0 -R jumppad-labs/xcl-python --comment "Superseded by the printing-and-pretty-output release PR, which moves the pin with the Python changes it needs."
  cd /home/nicj/code/github.com/jumppad-labs/xcl-python
  echo v0.12.0 > xcl-ref.txt
  git commit -am "Use xcl engine v0.12.0"
  git push
  gh pr ready
  ```
- **What happens next by itself:** CI runs the full matrix against v0.12.0. This is the "Python suites against the released engine" check. The parity suite runs against the released parity program, including the `pretty_log` stage. *(encode-saved-entity-during-operation, check 2; printing-and-highlighting-python, check 7; python-prettylog, check 4)*
- **Check before moving on:**
  - CI `result` is green.
  - The PR diff to `main` changes `xcl-ref.txt` to exactly `v0.12.0`.
  - The xcl-python manual checks above are done.
- **If it fails:**
  - **The bump merged before you disabled it:** that means its CI passed with the old Python code. Check whether a v0.14.0 was released (see Rollback). Otherwise continue; this PR then only adds the Python changes.
  - **CI fails:** fix it on this branch. Nothing is released until it merges.

#### Step 8: xcl-python, merge and release v0.14.0

- **Who:** a person merges; the automation does the rest.
- **Do:** `gh pr merge <xcl-python PR> --merge -R jumppad-labs/xcl-python`
- **What happens next by itself:**
  1. `tag.yml` pushes `v0.14.0`.
  2. `release.yml`:
     - downloads and verifies the v0.12.0 engine for seven platforms;
     - builds the wheels and sdist, and runs the parity suite against each wheel;
     - publishes `xcl-lang` 0.14.0 to PyPI;
     - installs it on every platform and runs `install_check.py`;
     - creates the GitHub release with generated notes that name the bundled engine.
- **Check before moving on:**
  - `gh run list -R jumppad-labs/xcl-python --workflow release.yml -L 1` shows success, including every install-check job.
  - <https://pypi.org/project/xcl-lang/0.14.0/> exists.
- **If it fails:**
  - **It fails before the PyPI publish:** fix it, then re-run with `gh workflow run release.yml -R jumppad-labs/xcl-python -f tag=v0.14.0`.
  - **It fails after the publish:** see Rollback.

#### Step 9: xcl-python, add the release notes

- **Who:** a person.
- **Do:** `gh release edit v0.14.0 -R jumppad-labs/xcl-python --notes-file <file>`, putting the xcl-python lines from "Release notes" above the generated list.
- **Check before moving on:** the release page shows all three lines. *(event-data-level-enum, check 4; printing-and-highlighting-python, check 6; python-prettylog, check 4)*

#### Step 10: Verify the published package (see Post-release verification)

- **Who:** a person.
- **Check before moving on:** every post-release check for the package passes.
- **If it fails:** see Rollback. Do not publish the website.

#### Step 11: xcl-website, merge and deploy

- **Who:** a person merges; the automation deploys.
- **Do:** `gh pr ready <website PR> -R jumppad-labs/xcl-website && gh pr merge <website PR> --merge -R jumppad-labs/xcl-website`
- **What happens next by itself:** `deploy.yml` builds the site and publishes it to GitHub Pages.
- **Check before moving on:** `gh run list -R jumppad-labs/xcl-website --workflow deploy.yml -L 1` shows success, and the live pages pass the website checks in Post-release verification.
- **If it fails:** revert the merge on `main`, which redeploys the previous site.

#### Step 12: Sync local checkouts

- **Who:** a person.
- **Do:**
  ```shell
  cd /home/nicj/code/github.com/jumppad-labs
  git -C xcl switch main && git -C xcl pull --ff-only && git -C xcl branch -d f-python-prettylogger
  git -C xcl-python switch main && git -C xcl-python pull --ff-only && git -C xcl-python branch -d release/printing-and-pretty-output
  git -C xcl-website switch main && git -C xcl-website pull --ff-only && git -C xcl-website branch -d release/printing-and-pretty-output
  ```
- **Check before moving on:** each `main` matches `origin/main`.

### Release notes

**xcl v0.12.0**: put these above the generated list.

> Readable output in every language. Configuration errors print as full reports again, with two lines of source either side of the fault. They are printed by `xcl.PrintError`, and by `%+v` on the unwrapped error. `xcl.PrintEntity` and `xcl.PrintDiff` print resources and change summaries. The `highlight` package is rebuilt around tokens, themes (with `LoadVSCodeTheme`) and the Plain, Terminal, HTML, HTMLInline and SVG formatters, with reference outputs in `highlight/testdata/reference/`. The core service sends error reports, resource text and diffs as labelled tokens, adds `ConvertTheme`, and allows `EncodeSavedEntity` while an operation runs.
>
> **Breaking:**
> - The old `highlight` API (`Piece`, `Pieces`, `Renderer`, `ANSI`, `NewANSI`, `WithThemeFile`, the `Scope*` constants), `xcl.DiagnosticPieces` and `(*Diff).Pieces` are removed. CHANGELOG.md lists a replacement for each one.
> - `Diagnostic.Source` now includes context lines.
> - Protocol: `highlight` (field 2) on both encode requests is reserved. Set `tokens` instead. The generated `corepb` `Highlight`/`GetHighlight` are removed.
> - Example programs no longer prefix non-configuration errors with `error:`.

**xcl-python v0.14.0**: put these above the generated list.

> - **Breaking:** `event_data` no longer accepts a string; pass an `xcl.EventDataLevel` member. `event_data="processed"` becomes `event_data=xcl.EventDataLevel.PROCESSED`.
> - **Breaking:** `highlight=` was removed from `encode_entity` and `encode_saved_entity`. Use `cfg.print_entity(entity, formatter=xcl.highlight.TERMINAL)` (or `print_saved_entity`) to print coloured configuration text, or `entity_tokens` for labelled text.
> - The asyncio client now delivers events to `on_event` one at a time and in order, as the synchronous client and Go do. A handler that relied on running concurrently may slow an operation.
> - New:
>   - errors carry their report, shown in tracebacks;
>   - `print_error`, `print_entity` and `print_diff`;
>   - `xcl.highlight` themes and formats;
>   - `load_vscode_theme`;
>   - `xcl.logging_handler`;
>   - `xcl.prettylog`;
>   - saved records can be rendered from an event handler during an operation.

**xcl-website:** none.

### Post-release verification

- [ ] **The published package, in a clean venv with no xcl checkout:**
  ```shell
  python3 -m venv /tmp/xcl-check
  /tmp/xcl-check/bin/pip install 'xcl-lang==0.14.0'
  /tmp/xcl-check/bin/python -c "import xcl, xcl.prettylog; print(xcl.engine_version())"   # prints v0.12.0
  ```
  *(printing-and-highlighting-python, check 7; python-prettylog, check 4)*
- [ ] **Rendering from an event handler during an apply.** Run the program from encode-saved-entity-during-operation check 3 against the installed package. It should use `event_data=xcl.EventDataLevel.PROCESSED` and an in-memory `state_store`, with an `on_event` handler that prints `cfg.encode_saved_entity(e.data, include_computed=True)` on create success. The handler prints configuration text, the apply succeeds, and the text equals the render after the apply. *(encode-saved-entity-during-operation, check 3)*
- [ ] **A report and the pretty logger from the installed package.** Run the python-prettylog check 3 demo with `/tmp/xcl-check/bin/python`, using a fixture plugin built from xcl v0.12.0. The `create success` lines appear with configuration beneath them, and the broken configuration's report appears once.
- [ ] **Release pages.** The v0.12.0 and v0.14.0 GitHub release pages show the notes above. The v0.14.0 page names the bundled engine v0.12.0.
- [ ] **Live site on xcl.dev.** Each of these opens with its new Python tabs and Go examples: `/events/`, `/configuration-text/`, `/problems/`, `/diff/`, `/sensitive-values/`, `/examples/viper/`, `/examples/plugins/` and `/examples/configuration-only/`. The `#printing-and-highlighting` and `#showing-problems-to-a-person` anchors resolve. *(printing-and-highlighting-core, check 2; printing-and-highlighting-python, checks 2 and 7; python-prettylog, check 2)*

### Rollback

- **xcl, before Step 6:** close the PR. Nothing is released.
- **xcl, after Step 6:**
  - **What can't be undone:** the tag is pushed and the Go module proxy caches `v0.12.0`, so the version can never be reused.
  - **Before xcl-python moves its pin (Step 7):** fix forward by merging a `patch`-labelled fix, which gives v0.12.1. Use that tag in Step 7.
  - **To withdraw the release:** mark it a pre-release or edit its notes. Deleting the GitHub release does not remove the module version.
- **xcl-python, before Step 8:** close the PR. Nothing is released. If `engine/v0.12.0` merged by itself, revert that merge with a `no-release` PR, or leave it and land the release PR on top.
- **xcl-python, after Step 8:**
  - **What can't be undone:** a published PyPI version can never be re-uploaded.
  - **If 0.14.0 is broken:** yank it on PyPI ("Manage project", then "Releases", then "Yank"), which keeps it for exact pins and hides it from new installs. Then merge a `patch`-labelled fix, which gives 0.14.1.
- **xcl-website, after Step 11:** revert the merge commit on `main`. The deploy workflow republishes the previous site.

### Alternatives considered

- **The release order recorded in planning: two xcl releases.**
  - **The plan:** cut the encode-saved-entity release first, before the printing-tokens change reached xcl `main`. To do that now, PR a branch cut at `ff2e846` (the encode-saved-entity merge) as `minor`, giving v0.12.0, and let its automatic engine bump pass on its own. Land xcl-python's event-data-level-enum and encode-saved-entity work. Then PR the rest of `f-python-prettylogger` as v0.13.0, and land the remaining Python work as in Steps 7 and 8.
  - **Not chosen because:** the specs were merged onto one branch, so this needs a second branch, a second round of CI and an extra package release. It reaches the same end state. The planned order existed only so the encode-saved-entity release could ride an automatic bump before `highlight` was reserved. Landing all the Python work with the pin bump makes that unnecessary.
- **Pushing the Python work onto the automatic `engine/v0.12.0` branch** instead of our own PR.
  - **Not chosen because:** it gives no CI run against the engine before the release (Step 4). The bump has auto-merge on, so the package publishes the moment checks pass, with the bot's PR title as the release-notes line.
