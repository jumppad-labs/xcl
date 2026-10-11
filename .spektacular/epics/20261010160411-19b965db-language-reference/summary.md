---
created_date: "2026-10-11"
---

# Planning summary: 20261010160411-19b965db-language-reference

## Decisions
- **Plugin function clash timing** — language-surface. Clashes involving a plugin fail the first operation, when plugins load; application-function clashes fail `NewConfig`, as type clashes do today.
- **Registering xcl-python before it exists** — python-support. xcl-python is registered with Spektacular at `~/code/github.com/jumppad-labs/xcl-python` so its tasks can be stored; creating the GitHub repository stays a human task.
- **Home page links** — language-reference, using-xcl-with-go. Both changes stand: the reference adds "Learn the language" and a "Documentation" section after "Two examples" with the Language reference first, and the guides join it after the reference; the guides also rewrite the closing banner with Go and Python buttons. Whichever spec lands first creates the section.
- **xcl README Documentation order** — language-reference, using-xcl-with-go. Language reference, then "Using XCL with Go", then "Using XCL with Python".
- **Tabs check exemption** — language-reference, using-xcl-with-go. The per-language guides are exempt from the "no Go box without a Python tab" check, alongside the plugin authoring pages.
- **Release versions** — language-surface, python-support, language-reference, using-xcl-with-go. The latest tag is v0.2.1: language-surface ships as v0.3.0 and python-support as v0.4.0; the docs are verified against v0.4.0.
- **Home page hero** — language-reference. The hero badge shows the verified release (expected v0.4.0) and the tagline says XCL is available for Go and Python.

## Order added for shared files
- Added: `20261011084948-c995ed25-using-xcl-with-go` now depends on `20261010160411-19b965db-language-reference`. Both change `xcl-website:src/config/site.ts`, `xcl-website:src/pages/state-masking.mdx`, `xcl-website:src/pages/index.mdx`, `xcl-website:main.go`, `xcl-website:main.py`, `xcl-website:database.postgres.main`, `xcl-website:app.web`, `xcl-website:src/components/Nav.astro`, `xcl:README.md`, `xcl:CHANGELOG.md`.

## 20261011095738-fad05e41-language-surface
### Approach

Functions are registered on a registry, plugin functions are hosted by providers and reached over two new gRPC calls, and the language rules are implemented in XCL's forked parser (`internal/xcl`). Applications register plain Go funcs with `registry.Local.RegisterFunction(name, fn)`; `registry.Registry` gains `Functions()`; the catalog owns the function set and checks names. Plugin functions build on the existing `ResourceProvider.Functions()` with a reshaped typed `ProviderFunction`, a `plugins.NewFunction(goFunc)` helper, and `GetFunctions`/`CallFunction` RPCs added to `PluginService` (handshake stays at version 1; older plugins answer Unimplemented and offer no functions). The fork rejects `%{ }` template directives, collects named child blocks into `map[string]T` (errors for missing or duplicate names), turns every type-triggered gohcl panic into an error, and honours `optional` inside object and map values. Dependency extraction is rebuilt on `hclsyntax.Variables`, so references inside `for`, index and parenthesis expressions are found. `XCL_VAR_` is set through one shared parser-options builder.

### Milestones and tasks

- M1, configuration is read the way the language says (xcl): `XCL_VAR_` prefix; references in every expression kind, plus a variable default built from an entity; optional fields in object and map values; template directives rejected.
- M2, named blocks by name, no crashes (xcl): named child blocks into maps and lists; unsupported field types return errors; named block maps through apply, diff and state.
- M3, applications and plugins add functions (xcl): application functions; plugin providers offer functions; plugin functions called over gRPC.
- M4, ecosystem: xcl docs and CHANGELOG; release xcl (human); xcl-plugin-template example `note_filename` function; xcl-plugin-docker builds against the release; xcl-website claim corrections; xcl-vscode tokenize regression test.

### Tasks a person must do

- Release xcl with the language surface changes (expected `v0.3.0`).

### Out of scope

No `for_each` or dynamic blocks, no `local` block, no templating function, no removal of map or object attributes, no other expression changes, no docs pages beyond claim fixes, no API renames, no editor formatting, no plugin protocol v2, schema v1 or Python/Node functions, no function namespacing, no module boundary change. The provider `Init` `functions` parameter still receives nil.

### Drafting assumptions

- Plugin function clashes fail the first operation, when plugins load; application-function clashes fail `NewConfig` (decided by the user).
- Application functions are plain Go funcs; `ParserOptions.CustomFunctions` is removed; `InvalidFunctionError` for unconvertible funcs.
- Function signatures are JSON with cty type strings; supported types are string, bool, numeric kinds, and slices and string-keyed maps of those.
- Sensitive arguments reach plugin functions unmarked; the result is re-marked on the host.
- The optional name label applies to map fields and list fields whose element has zero or one label field; duplicate names are errors in maps and lists.
- Bad block field types are checked when a type is declared or a plugin loads (`InvalidTypeError`).
- The vscode grammar has no `%{` rule today, so its change is a regression test only. The site function card states 57 built-ins.

### Project-wide rules

- CHANGELOG.md: one `## <spec name>` entry, prose then a `**Breaking:**` list.
- `registry.Registry` gains `Functions() []registry.Function` (breaking for every implementation).
- `plugins.PluginHost` and `PluginEntityProvider` gain `GetFunctions` and `CallFunction`; `plugin.proto` only gains RPCs.
- `FunctionSignature` is JSON with cty JSON type strings.
- Named block maps are `map[string]T` with `xcl:"name,block"`.
- `FunctionNameClashError`, `InvalidFunctionError`, `InvalidTypeError` live in `errors/` and are re-exported from the root package.
- Fork edits keep the MPL header, add a modification notice and are logged in UPSTREAM.md.
- Plugin repos pin a released xcl, never a `replace` directive.

### Manual checks

- xcl.dev and the README contain no `HCL_VAR_` or "around sixty", and the function count and wording are right.
- In VS Code, `%{ … }` is not coloured as template syntax and `${ … }` is.
- The CHANGELOG `**Breaking:**` list matches the changed public surface.
- Following the template README from a fresh copy produces a working plugin function.
- Every claim on xcl.dev and in the README matches the latest release when tried.

## 20261011130709-aa3eb933-python-support
### Approach

Built on the binding `libxcl.md` design, limited to what this spec needs: a pure-Go `coreservice` holding all binding logic behind `Service.Call`; `proto/xcl/core/v1/core.proto` generated with buf; a nested cgo module `bindings/libxcl` with four C exports; and a Python SDK in three layers (cffi transport with a ctypes fallback, a typed client, the hand-written API). Python functions are application functions: coreservice builds each as a Go func from its declared `FunctionSignature` and offers it through its registry's `Functions()`, so clashes and redaction behave as for Go functions. One Python poll engine has a sync driver and an asyncio driver; cancellation goes through new context-taking operations. An xcl tag builds, signs and publishes eight libxcl archives and dispatches the xcl-python release, which verifies them, builds eight `py3-none-<platform>` wheels and publishes to PyPI with Trusted Publishing.

### Milestones and tasks

- M1, the xcl engine can be driven from another language: context-taking operations; untyped lookups (`FindEntitiesByType`, `FindOneEntity`); schema v1 reader and `registry.TypeFromSchema`; coreservice CreateConfig and lookups; coreservice operations through the poll loop plus cancel; host-language functions; libxcl shared library and CI.
- M2, a Python program can configure, apply and query: Go fixture plugins and the Go parity program; create the xcl-python repository (human); safe engine loading (packaging, transport, client, errors, version and ABI checks, fork guard); Python classes describe entity types; the sync API; Python functions; Go plugins from a path, a directory and the GitHub registry with a tamper test; parity with Go.
- M3, async without blocking: the asyncio API never blocks and can be cancelled.
- M4, every xcl release ships a matching PyPI package: xcl release workflow for 8 signed archives; signing key and dispatch token (human); xcl-python CI matrix and release workflow; PyPI project and Trusted Publishing (human); README, docstrings and PARITY.md; CHANGELOG entry; cut the first release with Python support (human).

### Tasks a person must do

- Create the GitHub repository jumppad-labs/xcl-python and connect the registered local directory to it.
- Add the release signing key and dispatch token to xcl.
- Set up the PyPI project and Trusted Publishing.
- Cut the first xcl release with Python support (expected `v0.4.0`).

### Out of scope

In-process Python providers (proto numbers reserved), out-of-process Python plugins and protocol v2, distributing Python plugins through the GitHub registry, Node, xcl.dev Python docs, `DescribeTypes` and codegen, the shared `types.proto` move, switching the plugin schema producer to v1, the xcl-core transport and Wasm, the sync `max_workers` option, Python `decode`/`EncodeEntity`, platforms beyond the eight.

### Drafting assumptions

- xcl-python is registered with Spektacular before its GitHub repository exists (decided by the user).
- Python functions are reflected Go funcs; cty `number` maps to `float64`, so integers beyond 2^53 lose precision.
- Schema v1 is extended with a named-block map form and a `label` property.
- The sync API runs tasks serially; cancellation raises `xcl.CancelledError`, and under asyncio `asyncio.CancelledError` is re-raised after the engine stops.
- Plugin entities are read as `xcl.Entity`; PARITY.md lives in xcl-python.
- Python CI builds libxcl from source; releases use only the signed archives. Musl archives carry an `_musl` suffix.
- Open at implementation: whether language-surface's conversion accepts `MakeFunc` funcs, whether a cancelled Apply wraps `context.Canceled`, and whether the PyPI name `xcl` is free (stop and ask if not).

### Project-wide rules

- CHANGELOG.md gets one entry with `**Breaking:** None.`
- The language-surface function contract is used unchanged.
- Schema v1's named-block map form and `label` are what later protocol v2 or Python-provider work reuses.
- The CoreService error-code table is append-only; proto field numbers are reserved for provider work.
- The root module stays `CGO_ENABLED=0`; cgo lives only in `bindings/libxcl`.
- `buf lint` and `buf breaking` run in CI for `proto/`.
- xcl gets its first release workflow; libxcl assets reuse the plugin asset, checksum and signature contract and key.
- The PyPI version equals the xcl tag without the `v`, checked on load.
- Root API additions are additive only.

### Manual checks

- Follow the README alone from `pip install xcl` to a working apply against a Go plugin.
- Review PARITY.md against the Go API of the released tag.
- Review the CHANGELOG entry: additions described, Breaking None.
- Review the PyPI project page: install, sync getting started, then the asyncio section.
- Watch the first tagged release end to end: 8 signed archives, 8 wheels plus an sdist, install checks green on every platform.

## 20261010160411-19b965db-language-reference
### Approach

A hand-written `CodeTabs.astro` component in xcl-website takes one fenced block per language through `go` and `python` slots, so Expressive Code still highlights each. A head script sets `<html data-lang>` from `localStorage["xcl-lang"]` (default Go) before paint; CSS shows the matching panel in every box; clicks update the attribute and the stored value, with storage access guarded. The language reference is eight new pages under `/language/`: overview, entities, attributes and blocks, references, expressions, variables and outputs, modules, functions. Rejected: moving to Starlight (restructures every page with no redirects) and a third-party tabs package with one long page (choices not shared or remembered).

### Milestones and tasks

- M1, every code example speaks Go and Python (xcl-website): tabbed code boxes; Python tabs on the registries, GitHub registry and events pages; on the sensitive values, state masking, configuration text and diff pages; on the home page and the configuration-only example; plugin authoring pages say plugins are Go and tab the application code.
- M2, XCL has a language reference (xcl-website): overview, entities, and attributes and blocks; references and expressions; variables and outputs, and modules; functions (all 57 built-ins).
- M3, the reference is reachable and matches the release: nav and home page links, plus the hero badge and tagline (xcl-website); xcl README and changelog (xcl); try every example against the release and correct the pages (xcl-website), stopping if v0.4.0 and the `xcl` PyPI package aren't published.

### Tasks a person must do

None.

### Out of scope

Automated tests of examples or tabs; the highlighter; library fixes; prose rewrites of existing pages; the Go and Python guides; Python plugin authoring; Python forms of Go-only features (custom registry, custom masker, `EncodeEntity`, custom highlight renderers, in-process plugin registration); Python example programs in the xcl repo; restructuring or renaming pages; API renames; locals.

### Drafting assumptions

- The plugin authoring pages are plugin-template, examples/plugins, replacement and plugin-logging: plugin code stays Go-only, application boxes get Python tabs, and each states plugins are Go for now and usable from Go and Python.
- Where an application feature has no Python form, the Python tab says so and links the xcl-python parity list.
- The home page hero badge shows the verified release (expected v0.4.0) and the tagline says XCL is available for Go and Python (decided by the user at epic review).
- Where Go registers a plugin in process, the Python tab loads it as a separate program and says why.
- Tab behaviour is checked by hand; the site's type check and build are the automated gates.

### Project-wide rules

- Nav: a new "Language" dropdown between "Features" and "Examples", overview first.
- Home page: a "Learn the language" link in "What is XCL?" and a new "Documentation" section after "Two examples" with the Language reference first; the guides follow it there.
- xcl README: "Language reference" is the first item of the Documentation list.
- Slugs under `/language/` are chosen once (no redirects).
- `CodeTabs` with `go` and `python` slots, `<html data-lang>`, storage key `xcl-lang`; the per-language guides use no tabs.
- `site.ts` gains `pythonRepoURL`, `pythonParityURL` and `pypiURL`.
- CHANGELOG entry with `**Breaking:** None`.
- Type mapping lives in the reference; API walkthroughs live in the guides.

### Manual checks

- Every language topic removed from the README has a home on xcl.dev; locals map to defaults built from other values.
- Every acceptance area has its own heading anchor.
- No page outside the four plugin authoring pages and the two per-language guides has a Go box without a Python tab.
- Picking Python switches every box and carries to the next page; reload, keyboard switching and blocked storage work.
- The reference is in the nav, linked from the home page and the README, and every link opens it.
- The reference covers each acceptance area of the spec.
- Terminology review of the reference pages.
- The four plugin authoring pages carry the "plugins are Go for now" statement.
- The CHANGELOG entry reads as prose then `**Breaking:** None`.

## 20261011084948-c995ed25-using-xcl-with-go
### Approach

Two standalone MDX pages on xcl.dev built from the site's existing parts (Hero, Prose, CtaBanner): `/using-xcl-with-go/` and `/using-xcl-with-python/`. Each grows one program section by section and ends with the full listing, using the same configuration (a variable, `database "postgres" "main"`, `app "web"` calling a `slugify` application function, an output). The Go guide uses registered types and no plugin: function-form lookups, `ApplyContext` with Ctrl-C handling, `Find`/`FindByType`/`FindOne`/`All`, `Entities` and `FindEntitiesByType`, outputs, `ErrNotFound` and `NotUniqueError`, `Load` in a later run, and `Destroy`. The Python guide uses `@xcl.entity` dataclasses, `LocalRegistry` with `register_type` and `register_function`, the Go Docker plugin through `GitHubRegistry`, the sync `xcl.Config`, then `xcl.aio.Config` with when to use it, cancellation and fork safety. Python's generated API docs are added with pdoc, published to xcl-python's GitHub Pages.

### Milestones and tasks

- M1, the Go guide: write the guide (xcl-website); nav and home page (xcl-website); link from the xcl README (xcl).
- M2, Python API docs: generate and publish with pdoc, a docs workflow and README links (xcl-python); turn on GitHub Pages (human).
- M3, the Python guide: write the guide plus `pythonReferenceURL` in `site.ts` (xcl-website); nav and home page (xcl-website); link from the xcl README and record the guides in the changelog (xcl).
- All guide work waits on python-support's release (expected v0.4.0), recorded as a gate in the context document.

### Tasks a person must do

- Turn on GitHub Pages for xcl-python and publish the API docs.

### Out of scope

Automated tests and link checkers; xcl-vscode; library fixes; changes to existing topic pages beyond nav, home and links; tabs, the type mapping and Python on other pages (the reference spec); a plugin in the Go guide; Python plugins or providers; Node; changes to the python-support API or PARITY.md; a custom domain for the API docs; redirects.

### Drafting assumptions

- Python's generated API docs are pdoc output at `https://jumppad-labs.github.io/xcl-python/`.
- The Python guide uses xcl-plugin-docker through `GitHubRegistry`, so Docker is a prerequisite.
- The asyncio example uses only the standard library; web frameworks are named in prose.
- The home page's closing banner becomes "Start building with XCL" with Go (primary) and Python (secondary) buttons.
- The Go guide imports `xclerrors` for `ConfigError`, which the root package doesn't re-export.

### Project-wide rules

- Nav: both guides are the first two children of the `Guides` dropdown, so its label opens the Go guide.
- Home page: the closing CtaBanner points to both guides, and both guides are listed in the "Documentation" section right after the Language reference.
- xcl README: Language reference, then "Using XCL with Go", then "Using XCL with Python".
- Website README: a page-table row for each guide.
- The per-language guides have no Go|Python tabs and are exempt from the reference's tabs check.
- Guides link to the language reference for type mapping and don't repeat it.
- CHANGELOG entry with `**Breaking:** None`.
- `site.ts` gains `pythonReferenceURL` beside `goReferenceURL`.
- Go examples use function-form lookups.

### Manual checks

- Every Go API topic removed from the README has a home on xcl.dev.
- Getting-started questions can be answered with a link to the matching guide.
- Go run-through in an empty directory against the release tag; output matches the guide.
- Python run-through in a fresh virtualenv with Docker, including the asyncio apply.
- The nav, the home page (banner and Documentation section) and the xcl README link both guides in the agreed order.
- Each guide's link-outs resolve, including its language's API docs.
- Features marked not yet supported are really unsupported in the release.
- Terminology review; each guide shows only its own language; CHANGELOG entry reads prose then Breaking None.
