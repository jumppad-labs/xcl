---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Research: 20261011084948-c995ed25-using-xcl-with-go

## Alternatives considered and rejected

- **One "Using XCL" page with Go|Python tabs.** Rejected: the spec constraint says each guide shows only its own language, with no tabs. The site-wide tab component belongs to the sibling spec `20261010160411-19b965db-language-reference`.
- **Guides under a new `/guides/` path (`/guides/go/`, `/guides/python/`).** Rejected: every existing page is a flat slug at the site root (`xcl-website:src/pages/*.mdx`) apart from `examples/`, and the site has no redirects (`xcl-website` knowledge `gotchas/no-redirects-on-page-removal.md`), so the slug is chosen once and should match the page titles the spec names: `/using-xcl-with-go/`, `/using-xcl-with-python/`.
- **A new top-level nav item per guide.** Rejected: the nav already has a `Guides` dropdown whose label links to its first child (`xcl-website:src/components/Nav.astro:22-36,59-70`). Putting both guides first in it makes the label open the Go guide and keeps the bar short on phones (`max-sm` rules at `Nav.astro:51`).
- **A plugin in the Go guide.** Rejected: Go applications can declare types with `registry.Local.RegisterType` and need no plugin (`xcl:registry/local.go:75-106`). A plugin would need Docker and a release download, adding failure modes to an "empty program to working" walkthrough. The guide links to the registries and plugin pages instead.
- **No Go plugin in the Python guide.** Rejected: the spec acceptance criterion requires the Python program to use a Go plugin. Python apps cannot write providers (python-support plan, Out of Scope).
- **The plugin template's `notes` plugin for the Python guide.** Rejected: it is a GitHub template repository, not the published example remote plugin. `xcl-plugin-docker` is the registered "example remote plugin, published as signed GitHub releases", and the python-support README uses it too.
- **The method form of lookups (`c.Find[T](...)`) in the Go guide.** Rejected: it needs Go 1.27 and xcl supports Go 1.25.0 (`xcl:go.mod:3`); every runnable example uses the function form (xcl knowledge `architecture/ux-flow.md`).
- **Copying the removed README sections.** Rejected: `Lifecycle Callbacks` (Processable, ParseCallback, `Config.Process`) describes API that no longer exists (`git show bdba483^:README.md` lines 2061-2268; no such symbols in `xcl:*.go`). Only "Querying a configuration" and "State and Destroy" (lines 600-842) are raw material, rechecked against the code.
- **Linking the PyPI page as the Python "generated API docs".** Rejected: PyPI shows the README, not generated API docs. No hosted Python API reference exists: python-support ships docstrings and `PARITY.md` only. This plan adds pdoc-generated docs published to the xcl-python GitHub Pages site.

## Chosen approach — evidence

- `xcl-website:src/components/Nav.astro:22-36`: an existing `Guides` dropdown; the label links to `item.children[0].href` (`:61`).
- `xcl-website:src/pages/index.mdx:220-226`: the closing `CtaBanner` with a primary and a secondary `Button`. This is where the home page links to the guides; they are also listed in the "Documentation" section the language-reference spec adds after "Two examples", after its Language reference entry.
- `xcl-website:src/pages/state-masking.mdx`: the page pattern: front matter with `layout`, `title "<Title> - xcl"` and `description`; `Hero` with a heading and sub; `Prose` body; closing `CtaBanner`.
- `xcl-website:src/config/site.ts`: shared links, including `goReferenceURL = https://pkg.go.dev/github.com/jumppad-labs/xcl`. The Python reference URL is added beside it.
- `xcl-website:README.md:35-43`: the page table, which gains rows for the two guides.
- `xcl-website:ec.config.mjs`: expressive-code with Shiki, which highlights `go`, `python`, `hcl` and `bash` fences.
- `xcl:README.md:166-177`: the Documentation list, which gains the two guides.
- `xcl:CHANGELOG.md:1-11`: entries are `## <spec name>`, prose, then `**Breaking:**` and `None.`.
- Go API in the release: `xcl:query.go:29,62,180,190,351` (`Find`, `As`, `FindByType`, `FindOne`, `All`); `xcl:config.go:227,262,309,337,371,438,489` (`NewConfig`, `Entities`, `Outputs`, `Validate`, `Apply`, `Destroy`, `Load`); `xcl:options.go:22,50,106,126` (`WithRegistry`, `WithStatePath`, `WithEventHandler`, `WithVariables`); sentinels at `xcl:config.go:42-68`. Additive in the release: `RegisterFunction` (language-surface), `ApplyContext` and the other context forms, and `FindEntitiesByType`/`FindOneEntity` (python-support).
- Python API: python-support plan "Python public API": `@xcl.entity`, `xcl.Resource`, `LocalRegistry.register_type/register_function`, `GitHubRegistry(...).register_plugin`, `xcl.Config(registries=, state_path=, variables=, on_event=)` as a context manager, `validate/apply/diff/destroy/load`, `find/find_by_type/find_one/all/outputs`, `xcl.aio.Config` with the same names, awaitable; exceptions `xcl.Error`, `NotFoundError`, `NotUniqueError` and `ConfigError` (with causes carrying file and line); `ForkedProcessError`.
- `xcl-plugin-docker:examples/basic/*.xcl`: the `docker "network"` / `docker "container"` configuration the Python guide adapts.

## Files examined

- xcl-website:src/components/Nav.astro:1-110 — nav items array; the `Guides` dropdown; the label links to its first child; the first item is hidden on small screens.
- xcl-website:src/pages/index.mdx:1-226 — hero install block (`go get`), the Go snippets, the feature cards (stale `HCL_VAR_` and "Around sixty" are fixed by language-surface), the closing CtaBanner.
- xcl-website:src/components/sections/CtaBanner.astro — heading, body or body slot, button slot.
- xcl-website:src/pages/state-masking.mdx — page template to copy.
- xcl-website:src/pages/{registries,events,github-registry,plugin-template}.mdx, examples/plugins.mdx — the topic pages the guides link to; no single "state" page exists, and state is covered by `/state-masking/` (plus the guide's own section).
- xcl-website:src/config/site.ts — `goReferenceURL`.
- xcl-website:README.md:35-45 — page table.
- xcl-website:ec.config.mjs — code block config.
- xcl:README.md:160-199 — Documentation and Examples lists.
- xcl:CHANGELOG.md:1-40 — entry format.
- xcl:registry/local.go:75-106 — `RegisterType(prototype any, name ...string)`, which panics on a malformed declaration.
- xcl:query.go — the typed lookups; function form only below Go 1.27.
- xcl:config.go:20-170 — re-exported sentinels and detail types.
- xcl:options.go — config options.
- xcl:go.mod:3 — go 1.25.0.
- xcl git `bdba483^:README.md`:600-842 — the old "Querying a configuration" and "State and Destroy" text, accurate in substance against `query.go`/`config.go`; :2061-2268 "Lifecycle Callbacks", stale.
- xcl-plugin-docker:entities/{container,network}.go, examples/basic — plugin entity fields; tags v0.1.0-v0.2.0 today, with a newer release made by language-surface.

## External references

- pdoc (https://pdoc.dev) — generates HTML API docs from Python docstrings and type hints with no configuration. It suits a package whose docstrings are complete (python-support enforces this with a test).
- GitHub Pages with `actions/upload-pages-artifact` + `actions/deploy-pages` — hosts the generated docs at `https://jumppad-labs.github.io/xcl-python/` with no extra infrastructure.
- pkg.go.dev for `github.com/jumppad-labs/xcl` — the Go generated API docs, already linked as `goReferenceURL`.

## Prior plans / specs consulted

- `20261011095738-fad05e41-language-surface` (final plan) — `registry.Local.RegisterFunction(name, fn)`, `XCL_VAR_`, named-block maps (`map[string]T` with `xcl:"listener,block"`), released as v0.3.0. Its Out of Scope says this guide is written elsewhere (here).
- `20261011130709-aa3eb933-python-support` (final plan and context) — the Python API, the context-taking Go operations, `FindEntitiesByType`/`FindOneEntity`, and the release v0.4.0 with matching `pip install xcl==0.4.0`. Python docs on xcl.dev are left to the epic's docs specs (this one and language-reference). The package README uses the Docker plugin through `GitHubRegistry`.
- `20261010160411-19b965db-language-reference` (spec only, planned in parallel) — owns the reference pages, the Go/Python type mapping, the site-wide tab component, and adding Python to existing pages. Its "No Go-only examples" criterion must exempt these per-language guides, which our spec forbids from having tabs.
- The previous Go-only plan for this spec (deleted; known only from the orchestrator's brief) — slug, the function-form lookups, dropping Lifecycle Callbacks, the Guides dropdown and the CtaBanner, all reused.

## Open assumptions

- The release containing language-surface and python-support is v0.4.0, and the Python package is `xcl==0.4.0`. If the tag differs, use the actual tag. If either feature set is missing from the tag, STOP and ask.
- A Docker plugin release built against xcl ≥ v0.3.0 exists on `jumppad-labs/xcl-plugin-docker` when the guide is written; the implementer pins the latest such tag. If none loads under v0.4.0, STOP and ask.
- The jumppad-labs signing public key is published somewhere a reader can fetch it (for `trusted_keys`). If not, the guide shows the registry without keys and links the GitHub registry page's signing section.
- pdoc renders the `xcl` package (cffi-backed, lazy engine load) without loading libxcl, since `import xcl` does not load the engine (python-support acceptance criterion).
- The sibling language-reference plan publishes its reference under a slug the guides can link to; until then the guides link the type-mapping sentence to wherever that plan lands. The implementer resolves the actual URL at implementation time.

## Drafting assumptions

### Slugs and files (discovery)
- **Decision**: `src/pages/using-xcl-with-go.mdx` at `/using-xcl-with-go/` and `src/pages/using-xcl-with-python.mdx` at `/using-xcl-with-python/`.
- **Rationale**: the slugs match the page titles the spec names and follow the site's flat slugs. The site has no redirects, so a slug is chosen once.
- **Rejected**: `/guides/go/` (a new path convention for two pages).

### Python generated API docs via pdoc on GitHub Pages (discovery)
- **Decision**: add pdoc-generated Python API docs to xcl-python, published to GitHub Pages on each release, and link them from the Python guide as its "generated API docs".
- **Rationale**: the spec requires each guide to link its language's generated API docs, and none exist for Python (python-support ships docstrings only). pdoc needs no configuration, and the docstrings are already enforced.
- **Rejected**: linking PyPI (README, not API docs); generating into xcl.dev (couples the site build to a Python toolchain).

### The Python guide uses the Docker plugin from the GitHub registry (discovery)
- **Decision**: the Python guide's Go plugin is `jumppad-labs/xcl-plugin-docker` through `xcl.GitHubRegistry`, with a Docker engine as a stated prerequisite.
- **Rationale**: it is the published example remote plugin, and the python-support README uses the same path.
- **Rejected**: the plugin template (a template repository); a local plugin path (needs a Go build, which Python readers may not have).

### Chosen direction: two standalone per-language pages (architecture)
- **Decision**: two MDX pages on the existing page pattern; both first in the `Guides` dropdown; the home page's closing CtaBanner points at both; the xcl README and the website README list both; pdoc-generated Python API docs on xcl-python's GitHub Pages.
- **Rationale**: this is the only option meeting the no-tabs, own-language and xcl.dev constraints together. It reuses existing nav and banner patterns.
- **Rejected**: one tabbed page (breaks a constraint); README walkthroughs in the language repos (the site is the home of the docs).

### The Go guide uses registered types, no plugin, and the v0.4.0 additions (architecture)
- **Decision**: the Go guide registers types and one application function, and uses no plugin. It uses `ApplyContext` with `signal.NotifyContext`, the function-form lookups, the untyped `FindEntitiesByType`, `Load` and `Destroy`.
- **Rationale**: the program runs with nothing else installed. The spec asks for current behaviour of the release containing both dependency specs. The function form works on Go 1.25.
- **Rejected**: a Docker plugin in the Go guide (adds Docker as a prerequisite); method-form lookups (need Go 1.27).

### The Python asyncio example uses only the standard library (architecture)
- **Decision**: the asyncio section shows an `asyncio` program in which a ticker or a minimal `asyncio.start_server` handler keeps running while `await config.apply(...)` runs. Web frameworks are named in prose only.
- **Rationale**: the example runs after `pip install xcl` with nothing else, and readers of any framework can map it.
- **Rejected**: a FastAPI/aiohttp example (an extra dependency to verify and pin).

### Conventions selected (architecture)
- **Decision**: apply no-repository-file tests, the entity glossary, code style for snippets, the root error re-exports and public-packages-only. Drop the testing, database, dependency and architecture conventions.
- **Rationale**: the work is documentation plus one CI workflow; no library code changes.
- **Rejected**: listing every convention.

### The Python package README links to the guide and the API docs (components)
- **Decision**: the xcl-python README gains links to the generated API docs and the xcl.dev Python guide.
- **Rationale**: readers often arrive from PyPI. This is a two-line change in the repo that hosts the docs workflow.
- **Rejected**: leaving the README untouched (the docs site would be discoverable only from xcl.dev).

### Example names and values (data_structures)
- **Decision**: both guides use `database "postgres" "main"`, `app "web"`, a `slugify` application function, a variable `environment` and an output `database_location`. The Python guide adds `docker "network" "guide"` (subnet `10.74.0.0/24`) and `docker "container" "guide"`.
- **Rationale**: these are the site's running examples, and the spec's `slugify` precedent in language-surface. A distinct subnet and name avoid clashing with the Docker plugin's own samples (`10.73.0.0/24`).
- **Rejected**: a new domain example (readers would meet two vocabularies across the site).

### Python API docs URL (data_structures)
- **Decision**: `https://jumppad-labs.github.io/xcl-python/`, the default GitHub Pages URL for the repository.
- **Rationale**: it needs no DNS or custom domain setup.
- **Rejected**: a custom subdomain of xcl.dev (DNS work outside the repos).

### Describing references in strings without the forbidden word (implementation_detail)
- **Decision**: call `"${variable.environment}"` "a reference in a string", or say the value is "built from" a reference.
- **Rationale**: the spec bans "interpolation". This is also the sibling spec's wording ("references in strings").
- **Rejected**: "string templating" (template directives are rejected by the language, so the term misleads).

### No link checker or snippet test (testing_approach)
- **Decision**: rely on the existing site build plus manual reviews, and add no automated link or snippet checks.
- **Rationale**: the spec non-goal and the knowledge convention forbid tests that read pages.
- **Rejected**: a CI link checker over `dist/` (a test that reads pages).

### The cross-plan release gate is recorded in context, not as a dependency (tasks)
- **Decision**: the guide tasks name python-support's release task (`75d985e2-…`) as a precondition in their summaries and in context, not under **Depends on**.
- **Rationale**: the CLI accepts dependencies only within this plan.
- **Rejected**: a duplicate human "cut the release" task here (it would tag twice).

### The home page's closing banner is repurposed for the guides (tasks)
- **Decision**: the banner becomes "Start building with XCL", with Go (primary) and Python (secondary) buttons. The examples stay linked from the hero and the "Two examples" section.
- **Rationale**: it puts the guides at the end of the page, where a reader decides what to do next, without adding a new section.
- **Rejected**: changing the hero buttons (the sibling reference spec is likelier to edit the hero); adding a third banner (clutter).

### The Go config lookup error type is imported under an alias (tasks)
- **Decision**: the Go guide matches validation failures with `*xclerrors.ConfigError` from `github.com/jumppad-labs/xcl/errors`, since the root package does not re-export `ConfigError`.
- **Rationale**: the guides document current behaviour. Adding a re-export is a library change, and the spec says gaps are reported, not fixed.
- **Rejected**: adding `xcl.ConfigError` (out of scope; reported as a gap instead).

## Rehydration cues

- `spektacular spec file read 20261011084948-c995ed25-using-xcl-with-go`
- `spektacular plan file read 20261011130709-aa3eb933-python-support plan` (sections "Python public API", Milestone 2-3 acceptance criteria) and `... context` ("Python classes describe entity types", "The synchronous API…", "The asyncio API…").
- `spektacular plan file read 20261011095738-fad05e41-language-surface plan` ("Data Structures & Interfaces").
- `git -C /home/nicj/code/github.com/jumppad-labs/xcl show bdba483^:README.md | sed -n 600,842p`
- Read `xcl-website:src/components/Nav.astro`, `src/pages/index.mdx`, `src/pages/state-masking.mdx`, `src/config/site.ts`.
- `spektacular knowledge read --data '{"tier":"repo","name":"xcl","path":"architecture/ux-flow.md"}'`
