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
- **Linking the PyPI page as the Python "generated API docs".** Rejected: PyPI shows the README, not generated API docs. No hosted Python API reference exists: python-support ships docstrings and `PARITY.md` only. This plan generates one into xcl.dev at `/api/python/`.
- **pdoc docs built and published to xcl-python's own GitHub Pages site (`https://jumppad-labs.github.io/xcl-python/`).** The previous choice, superseded on 2026-10-12 (decision log). Rejected: the user prefers both references on xcl.dev and accepts a Python toolchain in the site build. It would also have needed a person to enable Pages on xcl-python, and a second site with its own look and URL.
- **Linking pkg.go.dev as the only Go reference.** Rejected (decision log, 2026-10-12): the user wants the Go reference on xcl.dev as well, next to the guides and in the site's look. pkg.go.dev stays linked from the Go reference's heading (`goPkgDevURL`) and from doc2go's links to undocumented packages.
- **gomarkdoc (Markdown) for Go.** Rejected: its last release is v1.1.0 from 2023-06-17 (Go proxy), so it is effectively unmaintained. Its Markdown would need either MDX, where generics' `[T any]` and braces in signatures break parsing, or a Markdown page per package plus anchor and cross-package link rewriting. doc2go v0.12.2 (2026-05-17) emits HTML fragments with directory-style links directly.
- **pkgsite or godoc self-hosted for Go.** Rejected: both are servers, not static generators; GitHub Pages serves only static files.
- **doc2go's and pdoc's standalone sites under `/api/go/` and `/api/python/`.** Kept as the fallback, not chosen: each brings its own page frame, navigation and CSS, so the references would look like separate sites without xcl.dev's header and nav. The embed modes cost one route and one stylesheet, which the trial runs showed is little work.
- **Sphinx with autodoc for Python.** Rejected: it needs a `conf.py`, a theme to match the site, and hand-written `.rst` stubs, and its HTML is a full site. pdoc needs no configuration and supports content-only output.
- **griffe (`griffe dump` to JSON) with a custom Astro renderer for Python.** Rejected as disproportionate: it would give the most native look, but the plan would have to write and maintain a renderer for signatures, docstring sections, cross-references and inheritance. pdoc's embedded HTML plus a stylesheet gets close to the same look with a fraction of the code.
- **pydoc-markdown or another Markdown generator for Python.** Rejected: pydoc-markdown's last release is 4.8.2 from 2023-06-26 (PyPI), and Markdown has the same MDX and link-rewriting problems as gomarkdoc.
- **Generating the references from the main branches.** Rejected (user default): the site would document APIs not yet released, unlike the guides and the badge.
- **Committing the generated references to xcl-website.** Rejected: large generated diffs on every release, and drift between the pin and the committed output. The user accepted generating at deploy time. Only the two highlighter stylesheets, which change only with the pinned generator version, are committed.
- **Generating the Python reference from xcl-python's source at the tag.** Rejected: xcl-python has no tags (its releases are PyPI uploads at the xcl tag's version). Importing from source also needs the generated protobuf modules, which are committed today but are a build output. Installing `xcl-lang==<version>` from PyPI documents exactly what readers install.
- **Two pins (an xcl tag and a separate `xcl-lang` version).** Rejected: xcl-python's `release.yml` publishes `xcl-lang` at the xcl tag's version (`version=${tag#v}`), so one pin cannot disagree with itself.
- **A pin written in `site.ts` only.** Rejected: the generator is a shell script, and reading a TypeScript file from bash is fragile. A JSON file is read by both `node -p` and `site.ts`.
- **The API references inside the "Language" or "Guides" dropdowns.** Rejected: "Language" is the XCL language reference, not the host-language APIs. A "Guides" label opens its first child, the Go guide, so the references would be buried among topic pages.
- **An "API" nav item visible on phones.** Rejected: the bar already measures roughly the width of a 375px screen (logo, "Language", "Examples", "Guides", GitHub; "Features" is hidden on phones for this reason). On phones the footer, the guides and the home page reach the references.


## Chosen approach — evidence

- `xcl-website:src/components/Nav.astro:22-36`: an existing `Guides` dropdown; the label links to `item.children[0].href` (`:61`).
- `xcl-website:src/pages/index.mdx:220-226`: the closing `CtaBanner` with a primary and a secondary `Button`. This is where the home page links to the guides; they are also listed in the "Documentation" section the language-reference spec adds after "Two examples", after its Language reference entry.
- `xcl-website:src/pages/state-masking.mdx`: the page pattern: front matter with `layout`, `title "<Title> - xcl"` and `description`; `Hero` with a heading and sub; `Prose` body; closing `CtaBanner`.
- `xcl-website:src/config/site.ts`: shared links, including `goReferenceURL = https://pkg.go.dev/github.com/jumppad-labs/xcl` (used only by `Footer.astro:24`) and `pypiURL = https://pypi.org/project/xcl-lang/`. `goReferenceURL` moves to `/api/go/`, `pythonReferenceURL` is added, pkg.go.dev becomes `goPkgDevURL`, and the release pin is re-exported.
- `xcl-website:.github/workflows/deploy.yml`: push to `main` only; one build job (`npm ci`, configure-pages, `npm run build`, upload-pages-artifact) and a deploy job, with Pages permissions already in place. It is where Go, Python and the generation step are added, with `workflow_dispatch`.
- `xcl-website:src/pages/index.mdx:21`: the hero badge, a literal `v0.4.0 · Go and Python`, which the python-out-of-process-engine plan moves to v0.5.0. `:316-322` is the "Documentation" section and `:324-330` the closing `CtaBanner`.
- `xcl-website:src/components/Nav.astro:8-45,61,76`: the items array ("Features", "Language", "Examples", "Guides"); the first item is hidden on phones by index; a dropdown label links to its first child.
- `xcl-python:pyproject.toml` and `xcl-python:README.md:10,18,246`: the distribution is `xcl-lang`, imported as `xcl`, because `xcl` on PyPI belongs to another project. `xcl-python:.github/workflows/release.yml` publishes `xcl-lang` at the xcl tag's version (`version=${tag#v}`). The repository has no git tags. `https://pypi.org/pypi/xcl-lang/json` returned 404 on 2026-10-12, so no release is published yet; the python-out-of-process-engine plan names v0.5.0 the first.
- Public Go packages (`go list ./...` in xcl, without `internal/`): the root, `coreservice`, `coreservice/proto`, `diff`, `e2e/...`, `entity`, `errors`, `events`, `highlight`, `logger`, `mask`, `plugins`, `plugins/example/...`, `plugins/mocks`, `plugins/proto`, `plugins/testing`, `registry`, `state`, `state/mocks` and `types`. The documented set drops the fixtures, mocks, test helpers, examples, generated protobuf and `coreservice` (the message API language bindings use, per its package comment).
- **Trial: doc2go v0.12.2 on xcl v0.4.0** (`git archive v0.4.0` into a scratch directory; `doc2go -embed -home github.com/jumppad-labs/xcl -rel-link-style directory -highlight classes:github -pagefind=false` with the twelve packages):
  - it wrote twelve fragments, `index.html` (package `xcl`) and `<pkg>/index.html`, 656 KB in all;
  - the fragments have no `<html>`/`<head>`, and use `h2`/`h3`, `ul` indexes and `pre.chroma`;
  - the root page ends with a "Directories" table linking `diff/` and the other packages;
  - cross-package links are relative and directory-style (`../events/#Emit`);
  - the standard library and every left-out package (`plugins/proto`, `internal/...`) link to `https://pkg.go.dev/...`;
  - `-help=frontmatter` documents the front matter template fields, and `-highlight-list-themes` includes `github-dark`.
- **Trial: pdoc 16.0.0 on xcl-python's current source** (`PYTHONPATH` at the checkout, only `protobuf` and `cffi` installed, no engine built; a template directory whose `frame.html.jinja2` is `{% block content %}{% endblock %}`):
  - it wrote `xcl.html`, `xcl/aio.html` and `xcl/mask.html` (plus pdoc's own `index.html` and `search.js`) as fragments starting at `<main class="pdoc">`;
  - the pages documented `Config`, `LocalRegistry`, `GitHubRegistry`, `entity`, `Resource`, `Error`, `NotFoundError`, `NotUniqueError` and the other exceptions with their docstrings;
  - links between modules are file-style (`./../xcl.html`), hence the rewrite to directory URLs;
  - `pdoc.render.configure` accepts `show_source=False` and `search=False`;
  - Pygments 2.21 in the same venv has the `github-dark` style.

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
- xcl-website:.github/workflows/deploy.yml — the deploy job the generation joins.
- xcl-website:package.json, Makefile, tsconfig.json, .gitignore, astro.config.mjs, ec.config.mjs (`github-dark`) — build scripts, strict TS config, ignored paths, the dark code theme the references match.
- xcl-website:src/components/Footer.astro — the only user of `goReferenceURL`.
- xcl-website:src/layouts/Shell.astro, src/components/sections/Prose.astro, src/styles/global.css — the layout, prose typography and `@theme` tokens the references render inside.
- xcl-website:.spektacular/knowledge/gotchas/guides-dropdown-label-opens-first-child.md — applies to the new "API" dropdown.
- xcl-python:pyproject.toml, README.md, Makefile, .gitignore, .github/workflows/release.yml, xcl/__init__.py — distribution name, README links, release versioning, the package's public modules.
- xcl:go.mod, `go list ./...`, `go doc` on each public package — the documented package set.

## External references

- doc2go (https://abhinav.github.io/doc2go/, module `go.abhg.dev/doc2go`) — static Go API HTML. Its `-embed` mode writes fragments for other site generators, with front matter templates, `-home` to root the output at a module, `-rel-link-style directory`, Chroma highlighting by class, and pkg.go.dev links for packages outside the set. v0.12.2 (2026-05-17) needs Go 1.25.
- pdoc (https://pdoc.dev) — generates HTML API docs from Python docstrings and type hints with no configuration. Its templates can be overridden from a directory, and replacing `frame.html.jinja2` with the content block alone is its documented way of embedding output in another site. 16.0.0 (2025-10-27) is the current release. It suits a package whose docstrings are complete (python-support enforces this with a test).
- PyPI JSON API (`https://pypi.org/pypi/<name>/json`) and the Go module proxy (`https://proxy.golang.org/<module>/@v/<version>.info`) — used to confirm release dates and that `xcl-lang` is not yet published.
- pkg.go.dev for `github.com/jumppad-labs/xcl` — still linked, now as `goPkgDevURL`.

## Prior plans / specs consulted

- `20261011095738-fad05e41-language-surface` (final plan) — `registry.Local.RegisterFunction(name, fn)`, `XCL_VAR_`, named-block maps (`map[string]T` with `xcl:"listener,block"`), released as v0.3.0. Its Out of Scope says this guide is written elsewhere (here).
- `20261011130709-aa3eb933-python-support` (final plan and context) — the Python API, the context-taking Go operations, `FindEntitiesByType`/`FindOneEntity`, and the release v0.4.0 with matching `pip install xcl==0.4.0`. Python docs on xcl.dev are left to the epic's docs specs (this one and language-reference). The package README uses the Docker plugin through `GitHubRegistry`.
- `20261010160411-19b965db-language-reference` (implemented) — owns the reference pages under `/language/`, the Go/Python type mapping, the site-wide tab component, and adding Python to existing pages. Its "No Go-only examples" criterion must exempt these per-language guides, which our spec forbids from having tabs. It added the "Language" dropdown and the home page "Documentation" section this plan extends; nothing it owns uses `/api/`.
- `20261011211634-a8895873-python-out-of-process-engine` (plan) — v0.5.0 is the first published Python release (`xcl-lang` 0.5.0), and it moves the hero badge to v0.5.0. That release is the one this plan's pin and guides target.
- The previous Go-only plan for this spec (deleted; known only from the orchestrator's brief) — slug, the function-form lookups, dropping Lifecycle Callbacks, the Guides dropdown and the CtaBanner, all reused.

## Open assumptions

- The release the guides and references describe is the one the hero badge advertises when this plan is implemented: expected v0.5.0, with `xcl-lang==0.5.0` on PyPI (the python-out-of-process-engine plan). If the tag differs, use the actual tag. If either feature set is missing from the tag, or `xcl-lang` at that version is not on PyPI, STOP and ask.
- A Docker plugin release built against xcl ≥ v0.3.0 exists on `jumppad-labs/xcl-plugin-docker` when the guide is written; the implementer pins the latest such tag. If none loads under v0.4.0, STOP and ask.
- The jumppad-labs signing public key is published somewhere a reader can fetch it (for `trusted_keys`). If not, the guide shows the registry without keys and links the GitHub registry page's signing section.
- pdoc renders the released `xcl-lang` without starting the engine, since `import xcl` loads nothing and the engine starts only when a config is created (python-support acceptance criterion; confirmed on the current source in the planning trial, not yet on a published wheel).
- doc2go v0.12.2 keeps the flags and fragment shape seen in the trial; the version is pinned, so this changes only when the pin is bumped.
- Astro 5 accepts `import.meta.glob` with `query: "?raw"` over a project-root path (`/generated/...`) in a page's `getStaticPaths`, and renders the strings with `set:html`. If Vite refuses a path outside `src/`, the generator writes into `src/generated/` instead (still git-ignored).

- The sibling language-reference plan publishes its reference under a slug the guides can link to; until then the guides link the type-mapping sentence to wherever that plan lands. The implementer resolves the actual URL at implementation time.

## Drafting assumptions

### Slugs and files (discovery)
- **Decision**: `src/pages/using-xcl-with-go.mdx` at `/using-xcl-with-go/` and `src/pages/using-xcl-with-python.mdx` at `/using-xcl-with-python/`.
- **Rationale**: the slugs match the page titles the spec names and follow the site's flat slugs. The site has no redirects, so a slug is chosen once.
- **Rejected**: `/guides/go/` (a new path convention for two pages).

### Python generated API docs via pdoc on GitHub Pages (discovery) — SUPERSEDED 2026-10-12
- **Superseded by**: "Both API references are generated into xcl.dev" below, and the decision log entry of 2026-10-12. Kept as history.
- **Decision**: add pdoc-generated Python API docs to xcl-python, published to GitHub Pages on each release, and link them from the Python guide as its "generated API docs".
- **Rationale**: the spec requires each guide to link its language's generated API docs, and none exist for Python (python-support ships docstrings only). pdoc needs no configuration, and the docstrings are already enforced.
- **Rejected**: linking PyPI (README, not API docs); generating into xcl.dev (couples the site build to a Python toolchain).

### The Python guide uses the Docker plugin from the GitHub registry (discovery)
- **Decision**: the Python guide's Go plugin is `jumppad-labs/xcl-plugin-docker` through `xcl.GitHubRegistry`, with a Docker engine as a stated prerequisite.
- **Rationale**: it is the published example remote plugin, and the python-support README uses the same path.
- **Rejected**: the plugin template (a template repository); a local plugin path (needs a Go build, which Python readers may not have).

### Chosen direction: two standalone per-language pages (architecture)
- **Decision**: two MDX pages on the existing page pattern; both first in the `Guides` dropdown; the home page's closing CtaBanner points at both; the xcl README and the website README list both; both API references generated into xcl.dev at `/api/go/` and `/api/python/` by the site's deploy workflow (revised 2026-10-12; previously pdoc-generated Python docs on xcl-python's GitHub Pages).
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
- **Decision**: apply no-repository-file tests, the entity glossary, code style for snippets, the root error re-exports, public-packages-only (now also bounding the Go API reference) and dependencies (the two generators are pinned with reasons). Drop the testing, database, never-modify-dependencies and architecture conventions.
- **Rationale**: the work is documentation, a site build change and two pinned build tools; no library code changes.
- **Rejected**: listing every convention.

### The Python package README links to the guide and the API reference (components)
- **Decision**: the xcl-python README gains links to `https://xcl.dev/api/python/` and the xcl.dev Python guide. It is the only xcl-python change (revised 2026-10-12: xcl-python no longer hosts a docs workflow).
- **Rationale**: readers often arrive from PyPI, where the README is the long description.
- **Rejected**: leaving the README untouched (the reference would be discoverable only from xcl.dev).

### Example names and values (data_structures)
- **Decision**: both guides use `database "postgres" "main"`, `app "web"`, a `slugify` application function, a variable `environment` and an output `database_location`. The Python guide adds `docker "network" "guide"` (subnet `10.74.0.0/24`) and `docker "container" "guide"`.
- **Rationale**: these are the site's running examples, and the spec's `slugify` precedent in language-surface. A distinct subnet and name avoid clashing with the Docker plugin's own samples (`10.73.0.0/24`).
- **Rejected**: a new domain example (readers would meet two vocabularies across the site).

### Python API docs URL (data_structures) — SUPERSEDED 2026-10-12
- **Superseded by**: "Site constants for the references" below (`pythonReferenceURL = "/api/python/"`). Kept as history.
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

### Both API references are generated into xcl.dev (architecture, 2026-10-12)
- **Decision**: the xcl-website deploy workflow sets up Go and Python and generates the Go reference (`/api/go/`) and the Python reference (`/api/python/`) into the site on every deploy.
- **Rationale**: the user's direction ("I genuinely don't mind having a python dep in my xcl.dev builds github action ... having Go docs there even though they are also on the docs.go.dev is beneficial"). Both references sit beside the guides in one site, and no repository setting needs a human.
- **Rejected**: pdoc on xcl-python's Pages (superseded); pkg.go.dev only for Go; committing generated output.

### The references follow one release pin (data_structures, user-confirmed default)
- **Decision**: `src/config/release.json` holds the advertised xcl tag. The Go reference is generated from that tag, the Python reference from `xcl-lang` at the tag's version, and the hero badge reads the same value. Bumping the pin and pushing to main regenerates both. `workflow_dispatch` allows a manual rebuild.
- **Rationale**: the references must describe what a reader installs, and the site already advertises one release. One value cannot disagree with itself, because xcl-python publishes at the xcl tag's version.
- **Rejected**: main branches (unreleased API); two pins; a pin in `site.ts` only (unreadable from bash).
- **Revisit only with evidence**: for example, if `xcl-lang` ever ships a Python-only patch release under a different version.

### Generators: doc2go for Go, pdoc for Python, both embedded (architecture, default chosen in this revision)
- **Decision**: doc2go v0.12.2 in `-embed` mode and pdoc 16.0.0 with a content-only frame template. The site renders both fragments inside its own layout with one stylesheet. Fallback, if embedding proves disproportionately fragile: each generator's standalone output, lightly themed, under the same paths.
- **Rationale**: both are maintained, need no configuration beyond flags, and have a supported embedding path. The trial runs produced clean fragments first time. This gives the integrated look the user prefers for one route and one stylesheet.
- **Rejected**: gomarkdoc and pydoc-markdown (unmaintained since 2023; Markdown/MDX fragility); Sphinx (configuration and its own site); griffe JSON with a custom renderer (most native look, but a renderer to write and maintain); pkgsite (a server).

### The Go reference covers twelve public packages (data_structures)
- **Decision**: the root package, `diff`, `entity`, `errors`, `events`, `highlight`, `logger`, `mask`, `plugins`, `registry`, `state` and `types`.
- **Rationale**: these are what a Go application or plugin author imports. The rest are internal, test fixtures (`e2e`), mocks, test helpers (`plugins/testing`), examples, generated protobuf, or `coreservice`, the message API for language bindings.
- **Rejected**: `./...` (documents fixtures, mocks and generated code); including `coreservice` (not for Go applications; still reachable on pkg.go.dev through doc2go's links).

### Site constants for the references (data_structures)
- **Decision**: `goReferenceURL = "/api/go/"`, `pythonReferenceURL = "/api/python/"`, `goPkgDevURL` for pkg.go.dev, plus `xclRelease` and `xclLangVersion` from the pin.
- **Rationale**: the guides and footer link the site's own references by the names they already use. pkg.go.dev stays addressable for the Go reference's heading.
- **Rejected**: keeping `goReferenceURL` as pkg.go.dev and adding `goApiURL` (the guides would link off-site by default).

### Navigation for the references (components)
- **Decision**: an "API" dropdown right after "Language" (Go, then Python), hidden below `sm` like "Features". The footer's "Go Reference" becomes "Go API", "Python API" is added, and the home page's "Documentation" section ends with both references.
- **Rationale**: it stays next to the language reference and consistent with the existing dropdowns. On phones the footer and the home page reach the references without overflowing the bar.
- **Rejected**: inside "Language" or "Guides"; visible on phones.

### Local builds do not need Go or Python (implementation_detail)
- **Decision**: `npm run build` stays plain Astro. Generation is a separate `npm run api-docs` (and `make api-docs`). Without fragments, the routes warn and emit nothing locally, and fail the build when `CI` is set.
- **Rationale**: content edits and other plans' verification steps (`npm run build`) keep working on machines without the toolchains, while a deploy can never ship without the references.
- **Rejected**: a `prebuild` hook (forces Go and Python on every build); a separate check step that reads `dist/` (a check that reads pages).

### No tests for the generator scripts (testing_approach)
- **Decision**: `generate.sh` and `python.py` get no unit tests. The deploy runs them end to end, and the manual review covers links and content.
- **Rationale**: xcl-website has no test tooling, the scripts are small build glue, and a test asserting on generated pages would read pages (convention).
- **Rejected**: a pytest suite for the link rewriting (new tooling in the repo for a few lines of build code).

### The Python distribution is `xcl-lang` (implementation_detail, correction)
- **Decision**: the guide installs `xcl-lang==<version>` and imports `xcl`; every `xcl==<version>` in the plan is corrected.
- **Rationale**: `xcl-python:pyproject.toml` names the distribution `xcl-lang`, and `xcl` on PyPI belongs to another project (`xcl-python:README.md:246`).
- **Rejected**: none.

## Decision log

- **2026-10-12: user chose to host both API references on xcl.dev.** The user said: "I genuinely don't mind having a python dep in my xcl.dev builds github action to generate documentation. I also think that having Go docs there even though they are also on the docs.go.dev is beneficial." This supersedes the 2026-10-11 decision "Python generated API docs via pdoc on GitHub Pages", which had rejected generating into xcl.dev because it couples the site build to a Python toolchain. The user has accepted that coupling.
  - The plan now generates the Go reference (doc2go) and the Python reference (pdoc) in xcl-website's deploy workflow, at `/api/go/` and `/api/python/`, from one release pin.
  - xcl-python keeps only a README link.
  - The human task "Turn on GitHub Pages for xcl-python and publish the API docs" (`1a652690-…`) and the agent task "Generate and publish the Python API docs" (`c349db84-…`) are removed. They are replaced by `efe7da14-…` (generation, xcl-website), `da899ef6-…` (navigation, footer and home page, xcl-website) and `8386a518-…` (xcl-python README).
  - Defaults chosen for this revision, to be revisited only with evidence: generate from the advertised release, not main; integrated look via embedded fragments; Python generation never starts the engine; twelve public Go packages; "API" dropdown hidden on phones with footer links.
- **2026-10-11 (superseded): pdoc-generated Python docs on xcl-python's GitHub Pages; Go linked to pkg.go.dev.** See the superseded drafting assumption above.

## Rehydration cues


- `spektacular spec file read 20261011084948-c995ed25-using-xcl-with-go`
- `spektacular plan file read 20261011130709-aa3eb933-python-support plan` (sections "Python public API", Milestone 2-3 acceptance criteria) and `... context` ("Python classes describe entity types", "The synchronous API…", "The asyncio API…").
- `spektacular plan file read 20261011095738-fad05e41-language-surface plan` ("Data Structures & Interfaces").
- `git -C /home/nicj/code/github.com/jumppad-labs/xcl show bdba483^:README.md | sed -n 600,842p`
- Read `xcl-website:src/components/Nav.astro`, `src/pages/index.mdx`, `src/pages/state-masking.mdx`, `src/config/site.ts`.
- `spektacular knowledge read --data '{"tier":"repo","name":"xcl","path":"architecture/ux-flow.md"}'`
- `cat /home/nicj/code/github.com/jumppad-labs/xcl-website/.github/workflows/deploy.yml /home/nicj/code/github.com/jumppad-labs/xcl-website/src/config/site.ts`
- `go run go.abhg.dev/doc2go@v0.12.2 -help` and `-help=frontmatter`; `python -c "import inspect, pdoc.render as r; print(inspect.signature(r.configure))"` in a venv with `pdoc==16.0.0`.
- `spektacular plan file read 20261011211634-a8895873-python-out-of-process-engine plan` (release v0.5.0, hero badge).
