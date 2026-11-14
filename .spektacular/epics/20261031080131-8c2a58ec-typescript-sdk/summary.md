---
created_date: "2026-11-10"
---

# Planning summary: 20261031080131-8c2a58ec-typescript-sdk

## Decisions
- **A reference link to a name that does not exist.** Plans: Python SDK reference (failed the site build) and TypeScript SDK reference (warned, failed only in a check workflow). Outcome, settled with the user 2026-11-10: both warn in a normal build and fail only under `XCL_REFERENCE_STRICT=1`. A hard failure would stop a release that renames something from updating the reference. The Python plan gains `reference-check.yml`.

## Order added for shared files
- Added: `20261110084343-9ebf34a1-python-sdk-reference` now depends on `20261031080136-264e0eab-typescript-core-sdk`. Both change `xcl-python:xcl/_version.py`, `xcl-python:xcl/registry.py`, `xcl-python:xcl/_types.py`, `xcl-python:xcl/aio.py`, `xcl-python:xcl/mask.py`, `xcl-python:.github/workflows/test.yml`, `xcl-python:.github/workflows/engine-bump.yml`, `xcl-python:.github/workflows/release.yml`.
- Added: `20261110084343-9ebf34a1-python-sdk-reference` now depends on `20261110083940-53be9068-typescript-printing`. Both change `xcl-python:xcl/highlight/_formatters.py`, `xcl-python:xcl/_print.py`, `xcl-python:xcl/_logbridge.py`, `xcl-python:xcl/prettylog.py`, `xcl-python:xcl/aio.py`.
- Removed at review: `20261110084343-9ebf34a1-python-sdk-reference` no longer depends on `20261031080136-264e0eab-typescript-core-sdk`; they may be implemented side by side.
- Removed at review: `20261110084343-9ebf34a1-python-sdk-reference` no longer depends on `20261110083940-53be9068-typescript-printing`; they may be implemented side by side.
- Removed at review: `20261110084722-9e301981-typescript-sdk-reference` no longer depends on `20261110084343-9ebf34a1-python-sdk-reference`; they may be implemented side by side.

## 20261031080136-264e0eab-typescript-core-sdk
### Approach
`xcl-lang`, built to the three designs: byte transport, typed client and hand-written API, with a fake transport for tests. One engine per module graph (per process and per worker thread), frames read on the event loop, and the child and pipes `ref`'d or `unref`'d by outstanding calls. One poll loop per operation; function and state tasks run concurrently, events go through one ordered queue, and an abort sends `Cancel` at once. Decorators in both compiler modes fill one per-class field table; standard mode fills it through `addInitializer` when registration constructs the class once.

### Milestones and tasks
23 tasks: (1) a Node program runs configuration with Go plugins, (2) types and functions in TypeScript, (3) long-running work, (4) releases, including the xcl dispatch and bindings gate and the `typescript-sdk.md` correction.

### Tasks a person must do
Set up npm publishing and GitHub secrets: claim the nine npm names with a one-off first publish, set trusted publishers, install the release bot App, create `XCL_TYPESCRIPT_DISPATCH_TOKEN`.

### Out of scope
Printing and the pretty logger, the xcl.dev reference, TypeScript providers and plugins, plugin type generation, Bun and Deno guarantees, CommonJS, a synchronous API, browsers, a name-mapping escape hatch, SIGTERM in `interruptSignal()`, object-typed attributes, any Go or Python change.

### Drafting assumptions
- `entities()` returns registered class instances, otherwise generic `Entity`; `find(address)` with no class stays generic; `typescript-sdk.md` is corrected. Decided by the orchestrator from the approved spec while the user was away.
- Platform engine packages do not count as separate packages. Decided by the orchestrator from the approved packaging.
- npm version equals the pin, so only engine bumps release; a package-only fix ships with the next xcl release.
- After a cancel, the rejection does not wait for the program's own handlers or tasks. A failing event handler becomes a warning.
- `Sensitive` is one export serving as decorator and constructor, split by `new.target`.
- Small Go-parity additions: `GitHubRegistry.register`, array-form external plugins, event buffer and mask options, `revealSensitive` on diff, an `Any` attribute type.
- Tests use Node's built-in runner compiled with `tsc`; Ctrl-C tests are automated on Linux and macOS, manual on Windows. macOS x86-64 is tested on `macos-15-intel`.
- Open for implementation, each stop and ask: whether the engine sees end of input on Windows when its host is killed; whether the nine npm names are free.

### Project-wide rules
- Package version equals the xcl tag; pin is `xcl-ref.txt`, starting at `v0.13.1`.
- xcl's workflow changes merge after `xcl-typescript`'s test workflow is on its `main` and the dispatch token exists.
- Tests: one behaviour per test, no table-driven tests, positive and negative apart; no test reads the README, parity record, workflows or source.
- The plan adds no changelog entries; breaking-change notes go by hand on the release.

### Manual checks
- Install to a working apply against a Go plugin from the README alone.
- Every Go operation and lookup has a TypeScript equivalent in the parity record, at each release.
- No open issue reports an orphaned engine after release.
- First release end to end: tag to nine packages at one version with provenance, and the engine reports the tag.
- A deliberately parity-breaking change to xcl, and to the package, fails its run.
- Fresh macOS and Windows machines, network disconnected after install: import, create and close on Node 22, 24 and 26.
- Ctrl-C on Windows, with and without opting in.
- The npm page shows the README, and its example matches the checked example.
- Every public export has a doc comment.
- npm, pnpm, yarn and bun pick the right platform package, including on Alpine, and skipping optional dependencies gives the named error.

## 20261110083940-53be9068-typescript-printing
### Approach
All work is in `xcl-typescript` on top of the core plan. `src/highlight/` ports Python's formats and themes, held to `xcl:highlight/testdata/reference`. `printError` and `formatError` take any format, `console.error` uses `util.inspect.custom` with Node's colour option, and an uncaught error shows the plain report held in `err.stack`, with no process listener. The pretty logger is an async `onEvent` handler that renders saved records, writing each group in one `write` and never awaiting the stream.

### Milestones and tasks
14 tasks: (1) output formats and themes, (2) one-call printing, including the printing design correction in xcl, (3) logging, (4) proven against Go and documented.

### Tasks a person must do
None.

### Out of scope
The xcl.dev reference and tabs, browser use, new formats or themes, engine or protocol changes, colour in Node's uncaught output, any process listener, `printError` following `cause`, public saved-entity printing, a `LogFormatter` equivalent.

### Drafting assumptions
- The uncaught report is plain text in `err.stack`. Decided by the orchestrator while the user was away, because a listener on import goes against the user's recorded objection to process-wide side effects; the spec's constraint and technical approach were amended.
- `report` is an own enumerable accessor; `problems` stays a data property.
- Multi-line engine messages are joined with `; `.
- Exports are flat, with `events` and `prettylog` namespaces; `prettylog.handler` takes one options object.
- Unverified: Node 22 and 24 uncaught behaviour was read from source only; pino and winston carrying `problems` and `report`. Tests settle both, and the implementer stops and asks if they fail.

### Project-wide rules
- `xcl:highlight/testdata/reference` is a cross-language contract; this package only reads it.
- The only runtime dependency stays `@bufbuild/protobuf`; `pino` and `winston` are pinned dev dependencies.
- The parity record's printing keys are added here.
- The package adds no process listeners.

### Manual checks
- No open issue reports colour codes in output the user did not ask to colour.
- The README covers every required topic with TypeScript examples.
- The parity record maps each Go printing and logging item to its TypeScript form.
- Every new public export has a doc comment.
- A real terminal shows a pretty-logged apply and a `console.error` report in readable colour, and an uncaught report as plain text.
- A site linking the shipped `xcl.css` and showing HTML output renders xcl's colours.

## 20261110084343-9ebf34a1-python-sdk-reference
### Approach
One Griffe tool in `xcl-python` (CI only) defines "public", fails naming each undocumented name, and writes neutral JSON attached to each release as `xcl-lang-<version>-reference.json`. The site pins a tag in `xcl-python-ref.txt`, fetches the asset in `prebuild`/`predev`, and renders `/reference/python/` inside `Shell.astro`. A release dispatches `xcl-website`, whose `reference-bump.yml` moves the pin, builds as a gate and pushes to `main` as the release bot. `PyRef` links warn on an unknown name and fail only in the new `reference-check.yml`.

### Milestones and tasks
15 tasks: (1) doc comments and check, (2) published reference, (3) guide and reachability.

### Tasks a person must do
- Give the website the release bot's access.
- Release the package with the reference asset.
- Set the site's first pin.

### Out of scope
TypeScript reference or guide, a Go guide, older versions, a README rewrite, package behaviour changes, names outside the export lists, stdlib-inherited members, a site-wide link checker.

### Drafting assumptions
- Public names are `xcl.__all__`, each exported module's `__all__`, plus `xcl.aio`, widening the spec's literal "export list" constraint.
- The doc comment check is a CI lint replacing `tests/unit/test_docstrings.py`.
- The bump pushes to `main` after a passing build, because the website has no required checks.
- `:param:`, `:returns:` and `:raises:` go on the clients, registries and module functions only.
- `config-files-are-xcl` was judged not to cover the reference JSON.
- `xcl-python-ref.txt` starts at `v0.14.1`, which has no asset, so branch builds fail until the first pin or `XCL_PYTHON_REFERENCE` is set.

### Project-wide rules
- Reference data: language-neutral JSON with `schema`, `language`, `package`, `version` and `sections`, delivered as a release asset.
- Navigation: a Reference dropdown; new guides go below the first Guides entry.
- Anchors: full dotted name.
- Unknown reference link names warn, and fail only under `XCL_REFERENCE_STRICT=1`.
- Docs merge only after the release they describe.

### Manual checks
- A person can answer what a method takes and returns from the site alone.
- A new developer reaches a working apply against a Go plugin by following the guide alone.
- The reference shows the latest version within one hour of a real release, with no hand commit.
- The reference page shows nav and footer, labels both clients, shows the version, and every export has an entry with members.
- A doc comment changed in a local package appears in a site built against regenerated data.
- Every guide example behaves as described against the latest release.
- The guide meets the wording rules.
- Navigation, home page and both READMEs link to the reference and guide.

## 20261110084722-9e301981-typescript-sdk-reference
### Approach
One TypeDoc tool in `xcl-typescript` (`.github/scripts/api-reference.mjs`, TypeDoc 0.28.20) defines "public", fails on undocumented names and writes the neutral JSON. The site reuses the Python reference's pin, fetch, page template and bump at `/reference/typescript/`. Each TypeScript tab is a real file in `xcl-website:typescript-examples/`, shown through `TsExample` and compiled by `typescript-examples.yml`, so a release that breaks an example still updates the reference.

### Milestones and tasks
23 tasks: (1) doc comments and data, (2) reference published, (3) TypeScript tabs across 107 boxes in five subject tasks, with the tab itself arriving last, (4) guide and links.

### Tasks a person must do
- Release the package with the reference asset.
- Set the site's first TypeScript pin.

### Out of scope
The Python reference or guide, a Go guide, older versions, tabs on XCL-only examples, other languages, package behaviour changes, running the tabs, a site-wide link checker, automatic issues for broken examples, stdlib-inherited members.

### Drafting assumptions
- The Python reference is delivered first; recorded as a dependency in the epic.
- A failed check run on `main` is the report for a broken example, with no automatic issue. Worth a look, because a red run on a bot's commit is easy to miss.
- Interface and object-type members are checked like class members; enum members are listed only.
- Tabs are compiled, not run; only the guide is run, by hand.

### Project-wide rules
- Reference data adds optional `overloads` and more `kind` values; `schema` stays 1.
- Pin `xcl-typescript-ref.txt`; the fetch script is renamed `scripts/fetch-reference.mjs`, run once per language.
- `reference-bump.yml` gains event `xcl-typescript-release` and a manual `package` input.
- Language preference: `localStorage` key `xcl-lang`, with `typescript` as a new value.
- Unknown reference link names warn, and fail only under `XCL_REFERENCE_STRICT=1`.

### Manual checks
- A TypeScript developer can answer what a method takes and returns from the site alone.
- A new developer reaches a working apply against a Go plugin by following the guide alone.
- The reference shows the latest version within one hour of a real release, with no hand commit, and a release that breaks an example still updates it.
- No open issue reports a TypeScript example that does not work.
- Every reference page shows nav, footer and version; every export has an entry, with constructors, methods and properties.
- Removing a public method's doc comment fails the checks naming it.
- Every guide example behaves as described against the latest release, and the guide meets the wording rules.
- Every code box has a TypeScript tab, and each "no equivalent" statement is true of the release.
- Choosing TypeScript persists across pages and reloads; Go stays the default and a stored Python choice still shows Python, with no other language first.
- The home page shows `npm install xcl-lang`, and every page listing the languages names TypeScript.
