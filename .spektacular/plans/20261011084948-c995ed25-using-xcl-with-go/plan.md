---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Plan: 20261011084948-c995ed25-using-xcl-with-go

<!-- Metadata -->
<!-- Created: 2026-10-11T15:01:51Z -->
<!-- Commit: 021b4ec4aaabdce2b0b6af19356647fdd7317d37 -->
<!-- Branch: f-libxcl -->
<!-- Repository: git@github.com:jumppad-labs/xcl.git -->

## Overview

Developers adopting XCL have no single walkthrough of its API in their language, only topic pages and generated API docs. This plan adds two guides to xcl.dev, "Using XCL with Go" and "Using XCL with Python". Each takes a developer from an empty program to a working one in their own language: creating a config, registering entity types and a function, applying, finding entities, keeping state and destroying. The Python guide also uses a Go plugin and shows the asyncio API. Both guides are reachable from the main navigation, the home page and the xcl README, and they link out to the topic pages and generated API docs rather than repeating them. xcl.dev also gains generated API references for both languages, at `/api/go/` and `/api/python/`. The site's own deploy workflow builds them from the release the site advertises, inside the site's header, navigation and styling, so each guide links a reference that matches the site and the release it describes. The work serves Go and Python developers getting started, and lets common getting-started questions be answered with one link.

## Conventions

- **Never write tests that inspect repository files** — the guides, the home page, the nav, the READMEs, the CHANGELOG entry, the deploy workflow and the generated API pages are checked by human review. No test reads them, and the guide examples are tried by hand (also a spec non-goal). The API references are guarded only by the site build, which generates and renders them.
- **Glossary: entity** — the guides call every block an entity, and use "resource" only inside Go identifiers such as `types.ResourceBase`, `xcl.Resource` and `FindResource`. This matches the spec's terminology requirement.
- **Code style (gofmt, explicit error handling, descriptive names, `any` over `interface{}`)** — every Go snippet in the guide is gofmt-formatted and handles each error, because readers copy it.
- **Shared error types and the root re-exports** — the guide matches failures with the root-package re-exports (`xcl.ErrNotFound`, `*xcl.NotUniqueError`), the form the convention says consumers use, rather than importing the `errors` package under an alias.
- **Project structure (public packages at the module top level)** — the guide imports only public packages (`github.com/jumppad-labs/xcl`, `/registry`, `/types`), never `internal/`. The Go API reference documents only the public packages, never `internal/`, test helpers, mocks, examples or generated protobuf.
- **Dependencies (document the reason, pin the version)** — the two documentation generators are build-time tools of xcl-website, each pinned to an exact version, with the reason recorded beside the pin (doc2go `v0.12.2` run with `go run`, pdoc `16.0.0` in a requirements file). Neither is a dependency of xcl or of the `xcl-lang` package.

Not applied:
- Testing and mocking, shared test helpers, test state from a real apply, assert ordering on graph parents, testing provider `Changed`/`Update`: no tests are written.
- Database and external services, never modify dependencies, patterns and architecture, development standards: no library code changes.

## Architecture & Design Decisions

**Options weighed.**

1. **Two standalone MDX pages on xcl.dev, one per language, built on the site's existing page pattern, with both API references generated into xcl.dev by the site build** (chosen).
   - `using-xcl-with-go.mdx` and `using-xcl-with-python.mdx` sit beside the other topic pages (`xcl-website:src/pages/state-masking.mdx` is the template: `Hero`, `Prose`, closing `CtaBanner`).
   - Each is a single scroll from an empty directory to a finished program, with a complete program listing at the end.
   - Both go first in the existing `Guides` dropdown (`xcl-website:src/components/Nav.astro:22-36`). The dropdown's label already links to its first child (`Nav.astro:61`), so the label opens the Go guide.
   - The home page's closing `CtaBanner` (`index.mdx:220-226`) points at both guides, and the home page's "Documentation" section (added after "Two examples" by the language-reference spec) lists both after its Language reference entry. The xcl README's Documentation list gains both right after its Language reference entry, and the website README's page table gains both.
   - xcl.dev's deploy workflow (`xcl-website:.github/workflows/deploy.yml`) sets up Go and Python, generates the Go API reference with doc2go and the Python API reference with pdoc from the release pinned in the site, and builds both into the site at `/api/go/` and `/api/python/`, inside the site's own layout. The guides link these pages; pkg.go.dev stays linked from the Go reference.
   - *Pros:* no new components or layout for the guides. It meets the "own language only, no tabs" constraint by construction. Slugs follow the site's flat convention. Both references live on xcl.dev in the site's look, and match the release the site advertises.
   - *Cons:* two pages repeat the same narrative shape, and drift between them is possible. The site build gains Go and Python toolchains and two pinned generators, and two page routes render generated HTML.
   - *Effort:* Medium.
2. **One "Using XCL" page with a language switch.**
   - *Pros:* one narrative to maintain.
   - *Cons:* it breaks the spec constraint (no tabs, each guide its own language). It also depends on the sibling spec's tab component.
   - *Effort:* Medium.
3. **Guides in the language repositories (README walkthroughs), linked from xcl.dev.**
   - *Pros:* close to the code.
   - *Cons:* it breaks the constraint that the pages live on xcl.dev and that the README stays an overview. The python-support README is already a short getting started, not the full walkthrough.
   - *Effort:* Low.

**Shape of the chosen solution.**

*Go guide (`/using-xcl-with-go/`).* The walkthrough follows a small program, `deployer`. It declares two entity types as Go structs embedding `types.ResourceBase` (`database "postgres"` and `app`, the site's running examples), registers them with `registry.NewLocal().RegisterType`, and adds one application function with `RegisterFunction`. It builds a `*xcl.Config` with `WithRegistry`, `WithStatePath` and `WithVariables`, then checks the configuration with `Validate` and applies it with `ApplyContext` under a `signal.NotifyContext` so Ctrl-C stops cleanly. It finds entities:
- by address with `xcl.Find[T]`;
- by type with `xcl.FindByType[T]`, `xcl.FindOne[T]` and `xcl.All[T]`;
- untyped with `Entities` and `FindEntitiesByType`;
- outputs through `xcl.Find[types.Output]` and `Outputs()`.

Failures are matched with `errors.Is(err, xcl.ErrNotFound)` and `errors.As` on `*xcl.NotUniqueError`. A second run uses `Load` to read state back without applying, and `Destroy` removes everything.

There is no plugin. Types are registered directly, so the program runs with nothing else installed. A short "Using plugins" section links to the registries, GitHub registry and plugin example pages. Every lookup uses the function form, since the method form needs Go 1.27 and xcl supports 1.25 (xcl knowledge `architecture/ux-flow.md`). The README's old "Lifecycle Callbacks" are not carried over: `Processable`, `ParseCallback` and `Config.Process` no longer exist.

*Python guide (`/using-xcl-with-python/`).* It follows the same program in Python.
- **Install.** A virtual environment, then the `xcl-lang` distribution from PyPI at the release version, imported as `xcl` (the name `xcl` on PyPI belongs to another project; `xcl-website:src/config/site.ts`, `pypiURL`).
- **Types and functions.** `@xcl.entity` dataclasses subclassing `xcl.Resource`. `xcl.LocalRegistry` with `register_type` and `register_function(slugify)`.
- **Go plugin.** `xcl.GitHubRegistry` registers `jumppad-labs/xcl-plugin-docker`, so the configuration gains a `docker "network"` and a `docker "container"` handled by the Go plugin. A Docker engine is a stated prerequisite. Plugin entities come back as `xcl.Entity`, since Python has no class for them.
- **Synchronous API.** `xcl.Config(...)` as a context manager: `validate`, `apply`, `find`, `find_by_type`, `find_one`, `all` and `outputs`. Exceptions `xcl.NotFoundError`, `xcl.NotUniqueError` and `xcl.ConfigError` with per-problem causes. A second run uses `load` and then `destroy`.
- **Asyncio.** `xcl.aio.Config` with `async with` and `await`, an `async def` function, and cancellation. It explains when to use it: a program already running an event loop, such as a web service, which keeps answering requests during a long apply. Otherwise the synchronous API is simpler. The example uses only the standard library (`asyncio`), so it runs with nothing extra installed.
- **Fork safety.** One paragraph, from python-support's `ForkedProcessError`.

*Shared shape.* Both guides use the same `main.xcl` structure: a variable, a `database "postgres" "main"`, an `app "web"` referring to it and calling the application function, and an output. The Python guide adds the Docker entities. Each guide hands type mapping, struct tags and annotations to the language reference rather than repeating them. Each ends with links to the registries, plugin, state and events pages and to its language's generated API reference on xcl.dev: `/api/go/` for Go and `/api/python/` for Python. Wording follows the spec's positioning and terminology rules and the glossary entry *entity*.

*API references (`/api/go/`, `/api/python/`).*
- **One release pin.** `xcl-website:src/config/release.json` holds the xcl tag the site advertises (`{ "xcl": "v0.5.0" }` at the time of writing; see Dependencies). The hero badge reads it through `site.ts`, the Go reference is generated from that tag, and the Python reference from `xcl-lang` at the same version without the `v` (xcl-python's release workflow publishes `xcl-lang` at the xcl tag's version). Bumping the pin and pushing to main is how the docs move to a new release; the existing push-to-main deploy regenerates them. A `workflow_dispatch` trigger lets anyone rebuild the site by hand.
- **Generation.** `npm run api-docs` runs `scripts/api-docs/generate.sh`, which writes HTML fragments into the git-ignored `generated/api/`:
  - Go: a shallow clone of `github.com/jumppad-labs/xcl` at the pinned tag, documented with `go run go.abhg.dev/doc2go@v0.12.2` in embed mode (`-embed`, no page frame), with `-home github.com/jumppad-labs/xcl`, directory-style relative links and `github-dark` highlight classes. It names the public packages explicitly: the root package, `diff`, `entity`, `errors`, `events`, `highlight`, `logger`, `mask`, `plugins`, `registry`, `state` and `types`. Everything else (`internal/...`, `e2e/...`, `*/mocks`, `plugins/testing`, `plugins/example/...`, `plugins/proto`, `coreservice` and `coreservice/proto`) is left out; doc2go links any mention of them, and of the standard library, to pkg.go.dev.
  - Python: a throwaway virtualenv with `pdoc==16.0.0` and `xcl-lang==<version>` from PyPI. `scripts/api-docs/python.py` drives pdoc's library API with a template directory whose `frame.html.jinja2` emits only the content block (pdoc's documented way of embedding its output in another site), source display off, for `xcl`, `xcl.aio` and `xcl.mask`. It places them at `index.html`, `aio/index.html` and `mask/index.html` and rewrites pdoc's file links (`xcl.html`, `xcl/aio.html`) to the site's directory URLs. Only `import xcl` happens; no config is created, so the engine never starts.
- **Rendering.** Two catch-all routes, `src/pages/api/go/[...path].astro` and `src/pages/api/python/[...path].astro`, read the fragments at build time and render each one inside `Shell` and `Prose` through one shared `ApiReference` component. That component adds a heading naming the language and release, and an external link (pkg.go.dev for Go, PyPI for Python). A small stylesheet, `src/styles/api-reference.css`, styles the generators' markup (doc2go's index lists and signatures, pdoc's `.docstring`, `.signature` and member blocks) with the site's tokens, and carries the `github-dark` Chroma and Pygments classes. The references therefore sit inside the site's header, nav, footer and dark palette.
- **Navigation.** A new "API" dropdown after "Language", with "Go" (`/api/go/`) and "Python" (`/api/python/`); its label opens the Go reference (xcl-website knowledge `gotchas/guides-dropdown-label-opens-first-child.md`). It is hidden below the `sm` breakpoint like "Features", because the bar already fills a 375px phone. On phones the footer's "Go API" and "Python API" links reach the references, and so do the guides and the home page's "Documentation" section, which lists both as its last two items.

**Why this direction.** Option 1 is the only one that meets all three: two guides, each in one language, on xcl.dev and reachable from the nav, the home page and the README. It reuses the page, nav and banner patterns the site already has. Generating both references into xcl.dev closes the one gap the research found: the spec's "generated API docs" criterion has nothing to link to for Python. It also puts the Go reference in the site's look, next to the guides, while pkg.go.dev stays one link away. The user accepted a Python and Go toolchain in the site's deploy workflow (research decision log, 2026-10-12). The cost is that workflow change, two pinned generators, and a small rendering route and stylesheet. Fragment output was chosen over each generator's standalone site so the references share the site's header and navigation. Trial runs during planning (doc2go on the xcl v0.4.0 tag, pdoc on xcl-python's current source) produced the expected fragments without the engine (research, "Chosen approach — evidence"). The evidence for each rejected option is in `research.md#alternatives-considered-and-rejected`.

**Cross-spec boundaries.** The sibling language-reference spec owns the reference pages, the Go/Python type mapping, the tab component and adding Python to existing pages. These guides link to it and do not use tabs. The guides' placement is:
- the first two entries of `Guides`;
- the home page's closing banner;
- the home page's "Documentation" section, Go guide then Python guide, right after the Language reference entry;
- the README's Documentation list, Go guide then Python guide, right after the Language reference entry (or first, if that entry has not landed yet).

The API references' placement is:
- `/api/go/` and `/api/python/`, a path no other plan uses (the language-reference plan owns `/language/`);
- an "API" dropdown right after "Language" in the nav;
- the footer, where "Go API" and "Python API" replace the old external "Go Reference" link;
- the last two items of the home page's "Documentation" section.

The hero badge reads the release pin. The python-out-of-process-engine plan moves the badge text to v0.5.0 before this plan runs; this plan replaces the literal with the pin, so the badge and the generated references always name the same release.

This is named as a project-wide rule so the plans' edits to `Nav.astro`, `index.mdx`, `Footer.astro`, `site.ts` and `README.md` can be reconciled.

## Component Breakdown

**xcl-website**

- **"Using XCL with Go" page** (new)
  - Owns the Go walkthrough from an empty module to a working program: install, entity types, registering types and a function, creating the config, validating and applying with cancellation, finding entities, outputs, handling lookup errors, keeping state with a second run, destroying, and where to go next.
  - Ends with the full program and configuration.
  - Links out to the language reference for type mapping, to the topic pages for registries, plugins, state and events, and to the Go API reference on xcl.dev.
- **"Using XCL with Python" page** (new)
  - Owns the Python walkthrough over the same program shape: install from PyPI, entity dataclasses, registering types and a function, using the Docker plugin through the GitHub registry, the synchronous API with operations, lookups, errors, state and destroy, then the asyncio API and when to choose it, and fork safety.
  - Ends with the full program, plus the asyncio variant.
  - Links out to the same pages, and to the Python API reference on xcl.dev.
- **Release pin** (new): `src/config/release.json`, the one place the advertised xcl tag is written. The hero badge, both references and their headings read it.
- **API reference generation** (new): `scripts/api-docs/` holds `generate.sh` (the entry point behind `npm run api-docs` and `make api-docs`), `python.py` (pdoc driver and link rewriting), `pdoc-templates/frame.html.jinja2` (content-only frame), `go-frontmatter.tmpl` (title and synopsis for each Go package page) and `requirements.txt` (pinned pdoc). It writes fragments into the git-ignored `generated/api/go/` and `generated/api/python/`, and needs Go, Python, git and network access, never a built engine.
- **Go and Python API reference pages** (new): `src/pages/api/go/[...path].astro`, `src/pages/api/python/[...path].astro` and the shared `src/components/ApiReference.astro` render each fragment inside the site layout, with `src/styles/api-reference.css`. In CI (`process.env.CI` set) a route with no fragments fails the build. Locally it logs a warning and emits no pages, so `npm run build` still works for content edits without Go or Python.
- **Deploy workflow** (changed): `.github/workflows/deploy.yml` gains a `workflow_dispatch` trigger, `actions/setup-go` and `actions/setup-python` steps, and an `npm run api-docs` step before the Astro build. The job keeps its permissions, concurrency group and Pages deploy.
- **Site navigation** (changed): the existing `Guides` dropdown gains the two guides as its first entries, so its label opens the Go guide. A new "API" dropdown, hidden on phones, follows "Language". No other nav entry moves.
- **Footer** (changed): "Go Reference" (external, pkg.go.dev) becomes "Go API" (`/api/go/`), and "Python API" (`/api/python/`) is added beside it.
- **Home page** (changed): the closing banner invites the reader to start building with a guide, offering both. The "Documentation" section lists both guides after the Language reference, and the Go and Python API references as its last two items. The hero badge reads the release pin. The examples stay linked from the hero and the "Two examples" section.
- **Shared site links** (changed): `goReferenceURL` becomes `/api/go/`, `pythonReferenceURL` (`/api/python/`) is added, the pkg.go.dev address moves to `goPkgDevURL`, and the release pin is re-exported as `xclRelease` and `xclLangVersion`. Pages and components link from one place.
- **Website README** (changed): the page table lists both guide pages and both API references. A short "API references" note says how the references are generated, which toolchains `npm run api-docs` needs, and that bumping `src/config/release.json` moves them to a new release.

**xcl**

- **README Documentation list** (changed): gains both guides right after the Language reference entry, so the README points to them and stays an overview.
- **Changelog** (changed): one entry for this spec. It says where the guides and the API references are and that the README links the guides, with **Breaking:** None.

**xcl-python**

- **Package README** (changed): links `https://xcl.dev/api/python/` and the xcl.dev Python guide, so a reader arriving from PyPI reaches the reference and the walkthrough. Nothing else changes in xcl-python: no docs build, no workflow, no Pages site, no new dependency.

**Interactions**

- Both guides depend on the published release containing language-surface and python-support (Go module tag and PyPI package at the same version), and on a published Docker plugin release. They describe that release only, and the release pin names the same release.
- The guides link the references through `goReferenceURL` and `pythonReferenceURL`. Guides and references ship in the same site deploy, so neither link can point at a page that is not there yet.
- The references are generated from the pinned tag and the matching `xcl-lang` on PyPI. The deploy fails, and the live site stays as it was, if either is missing.
- The sibling language-reference work owns the reference pages and the tabbed code component. The guides link into the reference, and the reference's "no Go-only code boxes" rule must exempt these two pages.

## Data Structures & Interfaces

No library types or interfaces are introduced or changed. The plan adds documentation, a release pin, a few site constants, the API reference generation and its two routes. The contracts that matter are the example program both guides teach, which must run unchanged against the release, and the small site data shapes the pages plug into.

**The guides' shared configuration (`main.xcl`)**

Both guides apply the same configuration. The Python guide adds two Docker entities to it.

```hcl
variable "environment" {
  default = "dev"
}

database "postgres" "main" {
  location = "localhost"
  port     = 5432
  username = "admin"
}

app "web" {
  name              = slugify("Web Store ${variable.environment}") # the application function
  database_location = database.postgres.main.location
}

output "database_location" {
  value = database.postgres.main.location
}

# Python guide only: entities handled by the Go Docker plugin
docker "network" "guide" { subnet = "10.74.0.0/24" }
docker "container" "guide" {
  image = "nginx:1.27-alpine"
  network { name = docker.network.guide.meta.name }
}
```

**Go types the Go guide declares**

```go
type PostgreSQL struct {
    types.ResourceBase `xcl:",remain"`
    Location string `xcl:"location" json:"location"`
    Port     int    `xcl:"port" json:"port"`
    Username string `xcl:"username" json:"username"`
}

type App struct {
    types.ResourceBase `xcl:",remain"`
    Name             string `xcl:"name" json:"name"`
    DatabaseLocation string `xcl:"database_location" json:"database_location"`
}

local := registry.NewLocal()
local.RegisterType(&PostgreSQL{}, "database", "postgres")
local.RegisterType(&App{}, "app")
local.RegisterFunction("slugify", func(s string) (string, error) { ... })
```

These are the public API calls the guide teaches, all in the release:
- `xcl.NewConfig(WithRegistry, WithStatePath, WithVariables)`;
- `Validate`, `ApplyContext`, `Load`, `Destroy`;
- `xcl.Find[T]`, `xcl.FindByType[T]`, `xcl.FindOne[T]`, `xcl.All[T]`;
- `Entities`, `FindEntitiesByType`, `Outputs`;
- `xcl.ErrNotFound` and `*xcl.NotUniqueError`.

**Python classes the Python guide declares**

```python
@xcl.entity("database", "postgres")
@dataclass
class PostgreSQL(xcl.Resource):
    location: str
    port: int
    username: str

@xcl.entity("app")
@dataclass
class App(xcl.Resource):
    name: str
    database_location: str

local = xcl.LocalRegistry()
local.register_type(PostgreSQL)
local.register_type(App)
local.register_function(slugify)          # def slugify(text: str) -> str

github = xcl.GitHubRegistry()
github.register_plugin("jumppad-labs/xcl-plugin-docker", "<release tag>")
```

These are the API calls the guide teaches:
- `xcl.Config(registries=[local, github], state_path=..., variables=...)` as a context manager;
- `validate`, `apply`, `find`, `find_by_type`, `find_one`, `all`, `outputs`, `load`, `destroy`;
- `xcl.NotFoundError`, `xcl.NotUniqueError` and `xcl.ConfigError` (with `.causes`);
- `xcl.aio.Config` with the same names, awaited.

**Site data shapes (xcl-website)**

- `Nav.astro` items keep their existing shape, `{ label, href }`, or `{ label, children: { label, href }[] }`, plus one optional field, `hideOnSmall?: boolean`. It replaces the hard-coded "first item is hidden on phones" check and is set on "Features" and "API". Two children are prepended to `Guides`: `{ label: "Using XCL with Go", href: "/using-xcl-with-go/" }` and `{ label: "Using XCL with Python", href: "/using-xcl-with-python/" }`. A new item follows "Language": `{ label: "API", hideOnSmall: true, children: [{ label: "Go", href: "/api/go/" }, { label: "Python", href: "/api/python/" }] }`.
- `src/config/release.json` (new), the release pin:

  ```json
  { "xcl": "v0.5.0" }
  ```

  The value is the xcl tag the site advertises when the task runs. It is `v0.5.0` once the python-out-of-process-engine plan has moved the badge there; `xcl-lang` 0.4.0 was never published.
- `src/config/site.ts` (changed):

  ```ts
  import release from "./release.json";
  // The xcl release the site advertises, and the xcl-lang version published
  // from it; the hero badge and both API references follow this pin
  export const xclRelease = release.xcl;                     // "v0.5.0"
  export const xclLangVersion = xclRelease.replace(/^v/, ""); // "0.5.0"
  // Generated API references on xcl.dev, built from the pinned release
  export const goReferenceURL = "/api/go/";
  export const pythonReferenceURL = "/api/python/";
  // The same Go module on pkg.go.dev, linked from the Go reference and for
  // packages the site does not document
  export const goPkgDevURL = "https://pkg.go.dev/github.com/jumppad-labs/xcl";
  ```

  `goReferenceURL` keeps its name but now points inside the site, so its one existing user, `Footer.astro`, drops `target="_blank"`.
- Each guide page's front matter follows the existing pages: `layout`, `title: "Using XCL with Go - xcl"` (and the Python equivalent) and `description`.

**API reference build contract (xcl-website)**

- `npm run api-docs` (also `make api-docs`) reads `src/config/release.json`, empties `generated/api/`, and writes:
  - `generated/api/go/index.html` (package `xcl`) and `generated/api/go/<pkg>/index.html` for `diff`, `entity`, `errors`, `events`, `highlight`, `logger`, `mask`, `plugins`, `registry`, `state` and `types`;
  - `generated/api/python/index.html` (`xcl`), `aio/index.html` (`xcl.aio`) and `mask/index.html` (`xcl.mask`).
- Each fragment starts with a front matter block (`title`, `description`: the package or module name and the first sentence of its documentation), then HTML with no `<html>`, `<head>` or `<body>`. Links between fragments are relative or absolute under `/api/go/` or `/api/python/`. Links to anything else go to pkg.go.dev or docs.python.org.
- The script exits non-zero if the tag cannot be cloned, `xcl-lang==<version>` cannot be installed, or either generator fails.
- `src/pages/api/go/[...path].astro` and `src/pages/api/python/[...path].astro` load the fragments at build time (`import.meta.glob(..., { query: "?raw", import: "default", eager: true })`). Each turns a fragment's directory into its `path` (the root is `undefined`), and renders `ApiReference` with the title `"<name> - Go API - xcl"` or `"<name> - Python API - xcl"`. If no fragments are found and `process.env.CI` is set, `getStaticPaths` throws "API reference fragments missing: run npm run api-docs".
- `ApiReference.astro` props: `{ language: "go" | "python", title: string, description?: string, html: string }`. It renders `Shell` (title, description), a heading line ("Go API reference · github.com/jumppad-labs/xcl `xclRelease`" or "Python API reference · xcl-lang `xclLangVersion`"), an external link (`goPkgDevURL` or `pypiURL`), then `Prose` wrapping `<div class="api-reference" set:html={html} />`.
- `generated/` is added to `.gitignore`.

## Implementation Detail

**A new kind of page: the build-up walkthrough.**
- Every existing xcl.dev page is a topic page: one feature, explained fully.
- The guides are a new kind, a single program grown section by section.
  - Each section adds a few lines to the same files, shows only what is new, and says what running it now prints.
  - The page ends with the whole `main.go` (or `main.py`) and `main.xcl`.
  - A reader who copies the snippets in order, or only the final listing, ends with the same program.
- The guides are built from the existing parts (`Hero`, `Prose`, fenced code with titles, `CtaBanner`), so the site gains no component and no layout.
- Section headings follow the reader's task, such as "Describe your entities", "Create a config" and "Find entities", not the API's names.

**Linking out, not repeating.** Each guide states a feature in a sentence or two, shows the call, and links to the page that owns it:
- type mapping and tags/annotations go to the language reference;
- plugins and registries go to the registries and GitHub registry pages;
- state encryption goes to state masking;
- events go to the events page;
- every exported name goes to the language's API reference on xcl.dev.

The topic pages stay the single home of detail, and the guides stay short enough to follow in one sitting. Text recovered from the old README ("Querying a configuration", "State and Destroy") is rewritten against the release, not pasted.

**Verified against a release, by hand, in a scratch directory.**
- Each guide's program is assembled from the page's own snippets in an empty directory outside the repos, and run against the release, never a local copy of xcl:
  - the Go guide: a new module that fetches the tagged xcl release;
  - the Python guide: a fresh virtualenv with the released `xcl-lang` from PyPI, running the synchronous program, then the asyncio variant.
- What the guide says each run prints is copied from that run.
- This is the spec's "checked by hand" rule. Nothing is added to any repo's test suite, following the convention against tests that read repository files.

**Consistent across the two guides.**
- Both guides use the same configuration, the same entity names, the same section order up to the point where Python adds the plugin and asyncio, and the same closing links.
- A reader moving from one language to the other recognises every step, and a reviewer can compare them side by side.
- Language idiom wins where the two APIs differ:
  - Go uses package-level generic functions and `errors.Is`;
  - Python uses methods on `Config`, exceptions and `with` blocks.

**Wording rules applied in review, not by tooling.**
- XCL is presented as its own language. Blocks are entities, and "resource" appears only inside Go identifiers and Python's `xcl.Resource`.
- The words "superset", "interpolation" and "Terraform" never appear. A reference inside a string is described as "a reference in a string".
- Anything the release does not support is said to be not yet supported:
  - in Go, the method-form lookups below Go 1.27;
  - in Python, writing plugins in Python.

**Generated API references inside the site, from the advertised release.**
- **Released code only.** The references are generated from the pinned tag and the `xcl-lang` release built from it, never from a main branch, so they describe exactly what a reader installs. Moving to a new release is a one-line change to `src/config/release.json`; the push to main redeploys the site and regenerates both references. The hero badge reads the same pin, so the site cannot advertise one release and document another.
- **Generated at deploy time, never committed.** `generated/` is git-ignored. The deploy workflow runs `npm run api-docs` after `npm ci` and before `npm run build`, on the one `ubuntu-latest` job it already has, with Go (`actions/setup-go`, `1.27.x`, which satisfies doc2go's and xcl's `go 1.25.0`) and Python (`actions/setup-python`, `3.12`). A `workflow_dispatch` trigger allows a rebuild by hand, for example after a release whose PyPI upload finished after the last push.
- **Fragments, not standalone sites.** Both generators run in their embed form: doc2go `-embed`, and pdoc with a content-only frame template. The site supplies the page, so the references get the site's header, nav, footer and palette, and Prose's typography styles their headings, lists, tables and code. If one generator's embedded output cannot be made readable with the stylesheet at reasonable effort, the accepted fallback is that generator's standalone output, lightly themed, served under the same path; the implementer reports the switch.
 The stylesheet only adds what Prose does not cover: signature blocks, pdoc's member layout, deprecated tags, and the two highlighters' `github-dark` classes. The highlighter CSS is generated once with the pinned tools (`doc2go -highlight classes:github-dark -highlight-print-css` and `pygmentize -S github-dark -f html -a .pdoc-code`) and committed, with the commands in a comment.
- **Search and source display are off.** doc2go's pagefind and pdoc's search index and "view source" toggles need their own scripts and CSS. They are turned off (`-pagefind=false`, `show_source=False`), and the browser's find-in-page serves the single-page packages.
- **The engine is never needed.** pdoc only imports `xcl`, `xcl.aio` and `xcl.mask`, and `import xcl` loads nothing; the engine starts only when a config is created (python-support acceptance criterion). doc2go reads Go source and needs no build of libxcl. The planning trial rendered the three Python modules from source with only `protobuf` and `cffi` installed.
- **pkg.go.dev stays reachable.** The Go reference's heading links the same module on pkg.go.dev (`goPkgDevURL`), and doc2go links every package the site leaves out, and the standard library, to pkg.go.dev.

## Dependencies

- **Design documents this plan was built on: none.** The spec carries no design references. The libxcl design reaches this plan only through the python-support plan it shaped.
- **Spec `20261011095738-fad05e41-language-surface` and its final plan — must land and be released first.** It provides:
  - `registry.Local.RegisterFunction`, which the Go guide uses;
  - `XCL_VAR_`, which the guides mention for variables;
  - named-block maps, which the guides link to the reference for.
  
  It is released as v0.3.0. No change is needed.
- **Spec `20261011130709-aa3eb933-python-support` and its final plan — must land and be released first.** It provides:
  - the whole Python API (the `xcl` PyPI package, `xcl.Config`, `xcl.aio.Config`, `@xcl.entity`, `LocalRegistry`, `GitHubRegistry`, the exceptions);
  - the Go additions `ApplyContext` (and the other context forms) and `FindEntitiesByType`/`FindOneEntity`;
  - the docstrings the Python API reference is generated from.

  Writing the guides waits on its human task "Cut the first xcl release with Python support" (`75d985e2-10fa-4cdc-896f-afe9d9d2df79`). No change is needed.
- **Spec `20261011211634-a8895873-python-out-of-process-engine` (an epic dependency) — the release the guides and references describe.** Its plan makes v0.5.0 the first published Python release (`xcl-lang==0.5.0`; `xcl-lang` 0.4.0 was never published, PyPI returned 404 on 2026-10-12) and moves the hero badge to v0.5.0. The guides, `src/config/release.json` and the badge all name that release, or whichever later release the badge advertises when this plan is implemented. No change is needed.
- **Sibling spec `20261010160411-19b965db-language-reference` (planned in parallel; no ordering dependency).**
  - The guides link to its reference pages for type mapping, and to the URL that plan settles.
  - Its "no Go-only code boxes" rule must exempt these two per-language guides.
  - Both plans edit `Nav.astro`, `index.mdx` and the xcl README's Documentation list, so the edits are reconciled at merge. The guides join the home page "Documentation" section that plan adds, after its Language reference entry, and follow the Language reference in the README's Documentation list.
  - If the reference is not yet published when a guide is written, the guide links to the reference's planned URL. The final link check then confirms that URL resolves before publishing.
- **`jumppad-labs/xcl-plugin-docker` — a published, signed release that loads under the release above.** The Python guide installs it through the GitHub registry. language-surface's task rebuilds it against v0.3.0, and protocol 1 is unchanged in v0.4.0. No change is needed; the guide pins the latest such tag.
- **A Docker engine** on the machine trying the Python guide. It is a stated prerequisite of the guide, and is needed for the manual check.
- **Go 1.25 or later and CPython 3.10 or later** — the minimum toolchains the guides state, taken from `xcl`'s `go.mod` and python-support's supported versions.
- **doc2go `v0.12.2`** (`go.abhg.dev/doc2go`, released 2026-05-17) — new build-time tool of xcl-website, run with `go run go.abhg.dev/doc2go@v0.12.2`, so the exact version is pinned in `generate.sh` and nothing is installed globally. Reason: it generates static Go API HTML with an embed mode made for other site generators, directory-style relative links and pkg.go.dev links for everything outside the documented set. Chosen over gomarkdoc (last release 2023) and over linking pkg.go.dev alone (research).
- **pdoc `16.0.0`** (released 2025-10-27, the current release) — new build-time tool of xcl-website, pinned in `scripts/api-docs/requirements.txt` and installed only into the generator's throwaway virtualenv. Its own dependencies (Jinja2, MarkupSafe, Pygments) are pinned at exact versions in the same file, taken from `pip freeze` after installing pdoc. Reason: it renders a package's docstrings and type hints with no configuration, and its template override supports content-only output for embedding. Chosen over Sphinx, griffe with a custom renderer, and pydoc-markdown (research). It is not a dependency of xcl-python or of `xcl-lang`.
- **`xcl-lang==<pinned version>` on PyPI** — installed into the same throwaway virtualenv so pdoc documents exactly the released package. Linux x86-64 wheels are part of every release (xcl-python `release.yml`).
- **GitHub Actions `actions/setup-go@v5` and `actions/setup-python@v5`** — added to xcl.dev's existing deploy job. xcl.dev's GitHub Pages is already enabled with GitHub Actions as its source, so no repository setting changes and no human task is needed.
- **xcl-website (Astro 5, MDX, expressive-code/Shiki)** — renders `go`, `python`, `hcl` and `bash` code blocks as they are, and renders the API fragments through `set:html`. Two guide pages and the `/api/go/...` and `/api/python/...` pages are added. None are removed or renamed, so no redirects are needed (`gotchas/no-redirects-on-page-removal`).
- **pkg.go.dev** — still linked as `goPkgDevURL` from the Go reference's heading, and by doc2go for packages the site does not document. No change is needed.
- **Prior plans consulted as history only:** none beyond the two dependency plans above. The deleted earlier Go-only plan for this spec is known only from the orchestrator's brief, and its choices are reused here.

## Testing Approach

**Kinds of test**

- **No automated tests are added.** The spec makes this a non-goal: "no automated tests for the examples on the new pages; they are checked by hand". The project convention forbids tests that read README, CHANGELOG or website text. Neither guide is guarded by a test, and nor are the nav, home page, READMEs or changelog.
- **Build checks that already exist.**
  - The site's build and type check must pass on xcl-website with the two new pages and the edited nav and home page. These are build checks of the site, not tests of the text.
  - `npm run api-docs` followed by `npm run build` must succeed and produce the API pages (`dist/api/go/index.html`, one page per documented Go package, and `dist/api/python/index.html`, `aio/` and `mask/`). In CI the build fails outright if the fragments are missing, so a deploy can never publish the site without its references.
  - After merge, the deploy workflow (on push to main, or run by hand) must complete. That proves the generators run on the runner from the pinned release, and that pdoc renders the released `xcl-lang` without starting the engine.
- **Run-through by the implementer (not committed).** While writing each guide, the implementer assembles its program from the page's own snippets in a scratch directory and runs it against the release:
  - Go: a new module fetching the tagged release;
  - Python: the released package in a fresh virtualenv, with a Docker engine.

  The outputs the guide quotes are copied from those runs. This is how the guides are kept to current behaviour. It leaves nothing behind in any repository.

**What the checks guarantee**
- Every snippet in each guide, taken in order, builds a program that runs against the release and prints what the guide says.
- Every link from the nav, the home page, the footer, the xcl and xcl-python READMEs and each guide opens an existing page.
- The site still builds, with both API references generated from the pinned release.

**Deliberate gaps**
- No link checker or snippet-extraction test is added. Both would be tests that read pages, which the convention forbids.
- No test reads the generated API pages or the deploy workflow, and no check asserts which names a reference lists (convention). The build proves the references were generated and rendered; a reviewer checks their content.
- The generator scripts (`generate.sh`, `python.py`) have no unit tests. They are small build tooling in a repository with no test tooling, and every deploy exercises them end to end; a broken link rewrite is caught by the manual link review.
- The Docker-backed part of the Python guide is checked on one platform with a Docker engine. It is not checked across platforms; the python-support parity suite already covers cross-platform behaviour.

**Success metrics**

- *Every Go API topic removed from the README (finding entities, apply and destroy) has a home on xcl.dev.* **Manual — captured in the implementation test plan.** A reviewer checks that the Go guide covers finding entities by address and by type, outputs, lookup errors, apply, state with `Load`, and destroy.
- *Questions about getting started with XCL in Go or Python can be answered with a link to that language's guide.* **Manual — captured in the implementation test plan.** A reviewer takes the common getting-started questions (install, declare a type, apply, find, keep state, destroy, use a plugin, asyncio) and confirms each is answered by a section of the matching guide.

**Manual reviews**

- **Manual — captured in the implementation test plan:** follow the Go guide top to bottom in an empty directory against the release tag, using only the code it shows. The program creates a config, registers an entity type, parses, applies, keeps state between runs, finds an entity by address and by type, and destroys it. Each printed output matches the guide.
- **Manual — captured in the implementation test plan:** follow the Python guide top to bottom in a fresh virtualenv with a Docker engine, using only the code it shows. The program installs `xcl-lang` from PyPI, creates a config, registers an entity type and a function, uses the Docker plugin, applies, keeps state between runs, finds by address and by type, and destroys. The asyncio version of apply also runs.
- **Manual — captured in the implementation test plan:** on the built site, the Go and Python guides appear in the main navigation, the home page links to both, and the xcl README links to both; every one of those links opens the right guide.
- **Manual — captured in the implementation test plan:** each guide links to the registries, plugins, state and events pages and to its language's API reference on xcl.dev (`/api/go/`, `/api/python/`), and every link opens a page that exists.
- **Manual — captured in the implementation test plan:** on the built site, `/api/go/` lists `Config`, `NewConfig`, `Find`, `FindByType`, `FindOne`, `All`, `ErrNotFound` and `NotUniqueError` with their doc comments. Its package list reaches `registry` (`Local`, `NewLocal`, `RegisterType`, `RegisterFunction`) and `types` (`ResourceBase`, `Output`). No `internal`, `mocks`, `e2e` or `plugins/testing` package appears. The heading names the pinned tag and links pkg.go.dev.
- **Manual — captured in the implementation test plan:** on the built site, `/api/python/` lists `xcl.Config`, `xcl.LocalRegistry`, `xcl.GitHubRegistry`, `xcl.entity`, `xcl.Resource`, `xcl.Error`, `xcl.NotFoundError`, `xcl.NotUniqueError` and `xcl.ConfigError` with their docstrings. `/api/python/aio/` shows `xcl.aio.Config` and `/api/python/mask/` the masks. Links between the three pages work, and the heading names the pinned `xcl-lang` version and links PyPI.
- **Manual — captured in the implementation test plan:** both references sit inside the site's header, nav, footer and dark palette, with readable signatures and highlighted code. At 375px wide the nav does not overflow, the "API" item is hidden, and the footer's "Go API" and "Python API" links open the references.
- **Manual — captured in the implementation test plan:** the "API" nav dropdown lists Go then Python, its label opens `/api/go/`, and the home page's "Documentation" section ends with the Go and Python API references.
- **Manual — captured in the implementation test plan:** the hero badge shows the same release as the API reference headings, and changing `src/config/release.json` then running `npm run api-docs && npm run build` changes all three.
- **Manual — captured in the implementation test plan:** each feature either guide marks as not yet supported is indeed unsupported in the release.
- **Manual — captured in the implementation test plan:** terminology review. Neither guide contains "superset", "interpolation" or "Terraform", nor uses "resource" for a block outside Go identifiers and `xcl.Resource`. Each describes XCL as its own language.
- **Manual — captured in the implementation test plan:** each guide shows only its own language, with no tabs.
- **Manual — captured in the implementation test plan:** review the xcl CHANGELOG entry for this spec: prose, then **Breaking:** None.
- **Manual — captured in the implementation test plan:** the xcl-python README links `https://xcl.dev/api/python/` and `https://xcl.dev/using-xcl-with-python/`, and both open.

## Milestones & Tasks

### Milestone 1: Go developers can go from an empty program to a working one with one guide

**What changes**
- xcl.dev gains "Using XCL with Go". It walks a Go developer through:
  - installing the library;
  - describing entities as Go structs and registering them, with an application function;
  - creating a config, then validating and applying configuration, stoppable with Ctrl-C;
  - finding entities by address, by type and as exactly one, and reading outputs;
  - handling lookups that cannot be answered;
  - keeping state so a later run can read it back, and destroying everything.
- It ends with the complete program, and links to the language reference, the registries, plugin, state and events pages, and the Go API reference on xcl.dev.
- The guide is the first entry under Guides in the main navigation. The home page's closing banner and its "Documentation" section offer it, and the xcl README's documentation list links it.
- The Go API topics removed from the README now have a home on the site again.

**Validation point**: The site builds with the new page. Following the guide by hand in an empty directory against the release tag produces a program that applies, finds, reloads and destroys exactly as the guide shows. The nav entry, the home page button and the README link each open the guide.

#### - [x] Task: Write the Using XCL with Go guide
**Id:** f4b3152b-7ef8-4b8d-bad3-6ab83c097d74
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

This adds the "Using XCL with Go" page. It takes a Go developer from an empty module to a program that:
- declares two entity types and an application function;
- validates and applies configuration, and can be stopped with Ctrl-C;
- finds entities by address, by type and as exactly one, and reads outputs;
- handles lookups that cannot be answered;
- reads its state back in a second run, and destroys everything.

Each section adds to the same program, and the page ends with the full program and configuration. The page links out to the language reference, the registries, plugin, state and events pages and the Go API reference (`goReferenceURL`, `/api/go/`) instead of repeating them. Work starts once the xcl release containing language-surface and python-support is tagged (python-support's "Cut the first xcl release with Python support"). Every snippet is tried against that tag.

*Technical detail:* [context.md#task-write-the-using-xcl-with-go-guide](./context.md#task-write-the-using-xcl-with-go-guide)

**Acceptance criteria**:
- [x] The page is served at `/using-xcl-with-go/` and the site builds.
- [x] Copying only the code the page shows, in order, into an empty directory gives a program that runs against the release tag and prints what the page says at each step.
- [x] The page covers creating a config, registering an entity type, parsing (validating) configuration, applying it, keeping state between runs, finding an entity by address and by type, reading outputs, matching not-found and not-unique errors, and destroying.
- [x] Every lookup uses the package-level function form, and the page says the method form needs Go 1.27.
- [x] The page links to the language reference, the registries, plugin, state and events pages, and the Go API reference through `goReferenceURL`.
- [x] The page shows only Go and XCL code, with no language tabs, and uses "entity" for every block.
- [x] The page never uses the words "superset", "interpolation" or "Terraform".

#### - [x] Task: Put the Go guide in the navigation and on the home page
**Id:** 3c3d939c-fba4-4353-848f-76e05faed545
**Repo:** xcl-website
**Depends on:**
- f4b3152b-7ef8-4b8d-bad3-6ab83c097d74 — Write the Using XCL with Go guide
**Execution:** agent

Readers must be able to find the guide without knowing its address. This task adds the Go guide as the first entry of the Guides dropdown, so the Guides label itself opens it. It turns the home page's closing banner into an invitation to start building, with the Go guide as its main button, and adds the Go guide to the home page's "Documentation" section right after the Language reference entry. It also lists the page in the website README's page table.

*Technical detail:* [context.md#task-put-the-go-guide-in-the-navigation-and-on-the-home-page](./context.md#task-put-the-go-guide-in-the-navigation-and-on-the-home-page)

**Acceptance criteria**:
- [x] On every page, the Guides dropdown lists "Using XCL with Go" first, and clicking the Guides label opens the Go guide.
- [x] The home page's closing banner has a button that opens the Go guide.
- [x] The home page's "Documentation" section lists "Using XCL with Go" immediately after the Language reference entry, and the item opens the Go guide.
- [x] The examples remain reachable from the home page.
- [x] No existing page is removed or renamed.
- [x] The website README's page table lists the Go guide.

#### - [x] Task: Link the Go guide from the xcl README
**Id:** 0f64b79a-1c34-4d66-a66f-2af359635d8c
**Repo:** xcl
**Depends on:**
- f4b3152b-7ef8-4b8d-bad3-6ab83c097d74 — Write the Using XCL with Go guide
**Execution:** agent

The README is an overview that points to xcl.dev. This task adds the Go guide to its Documentation list, immediately after the Language reference entry, so a Go developer landing on the repository finds the walkthrough near the top.

*Technical detail:* [context.md#task-link-the-go-guide-from-the-xcl-readme](./context.md#task-link-the-go-guide-from-the-xcl-readme)

**Acceptance criteria**:
- [x] The README's Documentation list links `https://xcl.dev/using-xcl-with-go/` with a one-line description, immediately after the Language reference entry (or first, if that entry is not there yet).
- [x] The README stays an overview, with no walkthrough content added.

### Milestone 2: The Go and Python APIs are browsable on xcl.dev, generated from the advertised release

**What changes**
- xcl.dev gains generated API references for both languages, in the site's own header, navigation and styling:
  - `/api/go/` documents the public packages of `github.com/jumppad-labs/xcl`;
  - `/api/python/` documents `xcl`, `xcl.aio` and `xcl.mask` from the `xcl-lang` package.
- Both are generated by the site's deploy workflow from the release the site advertises, pinned in one file. The hero badge reads the same pin. Bumping it and pushing to main moves the badge and both references to a new release, and the site can also be rebuilt by hand.
- The references are reachable from a new "API" nav dropdown, the footer and the home page's "Documentation" section. pkg.go.dev stays one link away from the Go reference.
- This gives each guide an API reference to point to. It is a prerequisite for Milestone 3 and is useful on its own to anyone using either language.

**Validation point**: `npm run api-docs && npm run build` succeeds locally and produces `/api/go/` (with a page per documented package) and `/api/python/` (with `aio/` and `mask/`). The pages list `xcl.Config`, `NewConfig`, `Find`, the registry and types packages, and `xcl.Config`, `xcl.aio.Config`, `xcl.LocalRegistry`, `xcl.GitHubRegistry` and the exceptions, with their doc comments and docstrings, inside the site layout. The "API" dropdown, the footer links and the home page entries open them. After merge, the deploy workflow publishes them.

#### - [x] Task: xcl.dev generates the Go and Python API references from the advertised release
**Id:** efe7da14-70e1-4741-a54b-92108b6b947d
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

There is no generated Python API reference anywhere, and the Go one lives only on pkg.go.dev, outside the site's look. This task makes xcl.dev generate both from the release it advertises. It adds a release pin that the hero badge also reads, and a generator script: doc2go for the public Go packages, and pdoc for `xcl`, `xcl.aio` and `xcl.mask` from the released `xcl-lang`. Two routes render the generated HTML inside the site layout, with a small stylesheet. The shared site links point the Go and Python reference constants at the new pages and keep pkg.go.dev as its own constant. The deploy workflow gains Go and Python, the generation step and a manual trigger. Work starts once the release the badge advertises has its tag and its `xcl-lang` on PyPI.

*Technical detail:* [context.md#task-xcldev-generates-the-go-and-python-api-references-from-the-advertised-release](./context.md#task-xcldev-generates-the-go-and-python-api-references-from-the-advertised-release)

**Acceptance criteria**:
- [x] `src/config/release.json` is the only place the advertised xcl tag is written. The hero badge, the Go reference heading and the Python reference heading all show it, and the Python version is the tag without its `v`.
- [x] `npm run api-docs` generates the Go reference from the pinned xcl tag and the Python reference from `xcl-lang` at the matching version on PyPI, never from a main branch, and exits non-zero if either cannot be fetched or rendered.
- [x] `npm run build` after `npm run api-docs` serves `/api/go/` plus one page for each of `diff`, `entity`, `errors`, `events`, `highlight`, `logger`, `mask`, `plugins`, `registry`, `state` and `types`, and `/api/python/`, `/api/python/aio/` and `/api/python/mask/`. No `internal`, `e2e`, `mocks`, `plugins/testing`, example, protobuf or `coreservice` package is documented.
- [x] Every API page shows every public name with its doc comment or docstring, inside the site's header, nav, footer and palette, with highlighted code. Links between API pages stay on xcl.dev, and links to undocumented packages and the standard library go to pkg.go.dev or docs.python.org.
- [x] Generating the Python reference creates no config and starts no engine.
- [x] The Go reference heading links pkg.go.dev (`goPkgDevURL`), and the Python reference heading links PyPI.
- [x] Without generated fragments, `npm run build` still succeeds locally with a warning, and fails when `CI` is set.
- [x] `deploy.yml` sets up Go and Python, runs `npm run api-docs` before the Astro build, deploys on push to main as before, and can be run by hand (`workflow_dispatch`).
- [x] doc2go and pdoc (and pdoc's dependencies) are pinned to exact versions, each with a comment giving the reason. `generated/` is git-ignored, and nothing generated is committed except the two highlighter stylesheets.
- [x] `site.ts` exports `xclRelease`, `xclLangVersion`, `goReferenceURL = "/api/go/"`, `pythonReferenceURL = "/api/python/"` and `goPkgDevURL`, and the site's type check passes.
- [x] The website README explains `npm run api-docs`, its toolchain needs and how to move the references to a new release.

#### - [x] Task: Put the API references in the navigation, footer and home page
**Id:** da899ef6-6fda-44e5-a1e4-0c3970f10b61
**Repo:** xcl-website
**Depends on:**
- efe7da14-70e1-4741-a54b-92108b6b947d — xcl.dev generates the Go and Python API references from the advertised release
**Execution:** agent

Readers must find the references without knowing their address. This task adds an "API" dropdown right after "Language", with Go then Python, so its label opens the Go reference. It is hidden on phones, like "Features", because the bar already fills a 375px screen. The footer's external "Go Reference" link becomes "Go API" and gains "Python API", both pointing inside the site. The home page's "Documentation" section ends with the two references, and the website README's page table lists them.

*Technical detail:* [context.md#task-put-the-api-references-in-the-navigation-footer-and-home-page](./context.md#task-put-the-api-references-in-the-navigation-footer-and-home-page)

**Acceptance criteria**:
- [x] On every page wider than the `sm` breakpoint, the nav shows "API" after "Language". Its dropdown lists "Go" then "Python", and clicking the label opens `/api/go/`.
- [x] At 375px wide the nav bar does not overflow, and "Features" and "API" are hidden.
- [x] The footer shows "Go API" (`/api/go/`) and "Python API" (`/api/python/`), opening in the same tab, and no longer links pkg.go.dev directly.
- [x] The home page's "Documentation" section ends with "Go API reference" and "Python API reference", each opening its page.
- [x] The website README's page table lists `/api/go/` and `/api/python/`.
- [x] No existing page is removed or renamed, and the site builds.

### Milestone 3: Python developers can go from an empty program to a working one, synchronously or with asyncio

**What changes**
- xcl.dev gains "Using XCL with Python". It walks a Python developer through:
  - installing `xcl-lang` from PyPI;
  - describing entities as dataclasses and registering them with a Python function;
  - using the Go Docker plugin from the GitHub registry;
  - creating a config, then validating and applying;
  - finding entities and outputs, and handling errors as exceptions;
  - keeping state between runs, and destroying.
- It then shows the asyncio API and when to choose it, for example a web service that must keep answering while an apply runs, and closes with fork safety.
- It links to the same topic pages and to the Python API reference on xcl.dev.
- It sits next to the Go guide in the navigation, on the home page and in the xcl README.
- The xcl-python README links the Python API reference and the guide, so readers arriving from PyPI reach both.
- The xcl changelog records both guides and the API references.

**Validation point**: The site builds. Following the guide by hand in a fresh virtualenv with a Docker engine, against the released package, produces a program that applies (with the plugin), finds, reloads and destroys, and an asyncio version whose apply runs while other work continues. All links from the nav, home page, footer, the xcl and xcl-python READMEs and the guide open existing pages. The changelog entry is present.

#### - [x] Task: Write the Using XCL with Python guide
**Id:** 561506a0-3890-4708-a844-8b3bd3d746a6
**Repo:** xcl-website
**Depends on:**
- efe7da14-70e1-4741-a54b-92108b6b947d — xcl.dev generates the Go and Python API references from the advertised release
- f4b3152b-7ef8-4b8d-bad3-6ab83c097d74 — Write the Using XCL with Go guide
**Execution:** agent

This adds the "Using XCL with Python" page. It takes a Python developer from an empty virtualenv to a program that:
- installs `xcl-lang` from PyPI;
- declares entity dataclasses and a Python function;
- uses the Go Docker plugin from the GitHub registry;
- validates, applies and finds entities by address and by type;
- reads outputs and handles failures as exceptions;
- reads its state back in a second run, and destroys everything.

A second part shows the same apply with the asyncio API, explains when to choose it (for example a web service that must keep answering while an apply runs), and shows cancellation. A short note covers fork safety. The page follows the Go guide's shape and configuration, so the two read side by side. Every snippet is tried against the released package.

*Technical detail:* [context.md#task-write-the-using-xcl-with-python-guide](./context.md#task-write-the-using-xcl-with-python-guide)

**Acceptance criteria**:
- [x] The page is served at `/using-xcl-with-python/` and the site builds.
- [x] Copying only the code the page shows, in order, into a fresh virtualenv with a Docker engine gives a program that installs `xcl-lang` from PyPI and runs against the released package. It prints what the page says at each step.
- [x] The page covers installing, creating a config, registering an entity type and a function, using a Go plugin, applying, keeping state between runs, finding an entity by address and by type, handling errors, and destroying, in that order.
- [x] The synchronous API comes first. The asyncio section that follows runs the asyncio version of apply while other work keeps running, and says when to use it.
- [x] The page says plugins are written in Go for now and can be used from Python.
- [x] The page links to the language reference, the registries, plugin, state and events pages, and the Python API reference through `pythonReferenceURL`.

- [x] The page shows only Python and XCL code (plus shell commands), with no language tabs.
- [x] The page uses "entity" for every block, and never uses the words "superset", "interpolation" or "Terraform".

#### - [x] Task: Put the Python guide in the navigation and on the home page
**Id:** 1c7d59de-6cbf-4f72-b35b-51d54c8f4ae8
**Repo:** xcl-website
**Depends on:**
- 561506a0-3890-4708-a844-8b3bd3d746a6 — Write the Using XCL with Python guide
- 3c3d939c-fba4-4353-848f-76e05faed545 — Put the Go guide in the navigation and on the home page
**Execution:** agent

This adds the Python guide as the second entry of the Guides dropdown, right after the Go guide. It gives the home page's closing banner a Python button beside the Go one, adds the Python guide to the home page's "Documentation" section after the Go guide, and lists the page in the website README.

*Technical detail:* [context.md#task-put-the-python-guide-in-the-navigation-and-on-the-home-page](./context.md#task-put-the-python-guide-in-the-navigation-and-on-the-home-page)

**Acceptance criteria**:
- [x] On every page, the Guides dropdown lists "Using XCL with Go" then "Using XCL with Python" before the topic pages.
- [x] The home page's closing banner has buttons opening the Go guide and the Python guide.
- [x] The home page's "Documentation" section lists the Language reference, then "Using XCL with Go", then "Using XCL with Python".
- [x] The website README's page table lists the Python guide.

#### - [x] Task: Link the Python guide from the xcl README and record the guides in the changelog
**Id:** 55f704ae-a2d1-4515-9814-592e3e52f0c6
**Repo:** xcl
**Depends on:**
- 0f64b79a-1c34-4d66-a66f-2af359635d8c — Link the Go guide from the xcl README
- 561506a0-3890-4708-a844-8b3bd3d746a6 — Write the Using XCL with Python guide
**Execution:** agent

The README's Documentation list gains the Python guide immediately after the Go guide. The changelog gains this spec's single entry, which says the two guides exist, what they cover, that xcl.dev now hosts generated Go and Python API references built from the advertised release, and that the README links the guides, with **Breaking:** None.

*Technical detail:* [context.md#task-link-the-python-guide-from-the-xcl-readme-and-record-the-guides-in-the-changelog](./context.md#task-link-the-python-guide-from-the-xcl-readme-and-record-the-guides-in-the-changelog)

**Acceptance criteria**:
- [x] The README's Documentation list has the Language reference, then "Using XCL with Go", then "Using XCL with Python" (the two guides first, if the reference entry is not there yet).
- [x] The changelog has one `## 20261011084948-c995ed25-using-xcl-with-go` entry, in prose, followed by **Breaking:** and None.

#### - [x] Task: Link the API reference and the Python guide from the xcl-python README
**Id:** 8386a518-8676-4f3e-ac0a-ca6c63526956
**Repo:** xcl-python
**Depends on:**
- efe7da14-70e1-4741-a54b-92108b6b947d — xcl.dev generates the Go and Python API references from the advertised release
- 561506a0-3890-4708-a844-8b3bd3d746a6 — Write the Using XCL with Python guide
**Execution:** agent

Readers often arrive at the package from PyPI, where the README is the page they see. This task adds two links to the README, to the Python API reference on xcl.dev and to the Python guide, so they reach both. It is the only change to xcl-python: no docs build, workflow or Pages site is added there.

*Technical detail:* [context.md#task-link-the-api-reference-and-the-python-guide-from-the-xcl-python-readme](./context.md#task-link-the-api-reference-and-the-python-guide-from-the-xcl-python-readme)

**Acceptance criteria**:
- [x] The README links `https://xcl.dev/api/python/` (API reference) and `https://xcl.dev/using-xcl-with-python/` (guide), near its existing xcl.dev link.
- [x] No other file in xcl-python changes, and the package gains no dependency.

## Open Questions

- **Does the released Python package match the API names in the python-support plan?** This depends on what python-support actually ships, which can only be checked against the installed `xcl-lang==<version>`. Examples are `find(None, …)` for untyped lookups, the `outputs()` key shape, and the exception class names. If a name differs, the guide uses the shipped name and the difference is reported to the user. If a capability the spec's acceptance criteria require is missing, such as asyncio apply, function registration or plugin use, STOP and ask, because the guide could not describe it as working.
- **Does a Docker plugin release load under the guides' xcl release?** This depends on the plugin releases published after language-surface. If the latest plugin release fails to load under the release (a protocol or schema mismatch), STOP and ask rather than building the plugin locally in the guide.
- **Does pdoc render the released `xcl-lang` without starting the engine?** The planning trial rendered xcl-python's current source with only `protobuf` and `cffi` installed, and `import xcl` loads nothing by design. The released wheel (out-of-process engine) is only checked when the generator first runs. If importing `xcl`, `xcl.aio` or `xcl.mask` starts or needs the engine, STOP and ask: running the engine in the site build is a larger change than this plan intends.
- **Is the matching `xcl-lang` on PyPI for the pinned tag?** The release pin assumes xcl-python publishes `xcl-lang` at every xcl tag's version. If the tag the badge advertises has no matching `xcl-lang` on PyPI when the generation task runs, STOP and ask rather than pinning the Python reference to a different version or generating it from source.

No other questions remain open. Every design choice is settled in the architecture and recorded in the assumption log.

## Out of Scope

- No automated tests for the examples on the new pages, and no link checker. They are checked by hand (spec non-goal; convention against tests that read pages).
- No changes to the xcl-vscode highlighter. Adding formatting to it and publishing it is separate, later work (spec non-goal).
- No library fixes. Gaps or bugs found while writing the guides are reported to the user, not fixed here (spec non-goal). One is known already: `ConfigError` is not re-exported from the root `xcl` package, so the Go guide imports `github.com/jumppad-labs/xcl/errors` under an alias.
- No changes to existing topic pages beyond the nav, the footer, the home page banner and hero badge, the guide and API reference entries in the home page "Documentation" section, and links to and from the guides.
 Adding Python to existing pages, the Go/Python type mapping, the language reference pages and the site-wide tabbed code component belong to spec `20261010160411-19b965db-language-reference`.
- No language tabs on the guides. Each shows only its own language (spec constraint).
- No Go plugin in the Go guide. Plugins are linked to the registries, GitHub registry, plugin template and plugin example pages.
- No guide for writing plugins in Python, Python providers or out-of-process Python plugins. These are not built (python-support Out of Scope); the Python guide says plugins are written in Go for now.
- No Node or other language guide (no implementation exists).
- No changes to the python-support package API, its README's getting-started content, or `PARITY.md`. The only xcl-python change is two README links. xcl-python gets no docs build, no docs workflow and no GitHub Pages site.
- No API references for unreleased code. The references follow the release pin, never a main branch, and there are no per-version archives: only the advertised release is documented.
- No API reference for the packages the site leaves out (`internal/...`, `e2e/...`, mocks, `plugins/testing`, examples, generated protobuf, `coreservice`). They stay on pkg.go.dev, where doc2go's links send readers.
- No search inside the API references, and no "view source" panels. doc2go's pagefind and pdoc's search index are turned off.
- No hand-written API pages and no custom renderer of a generator's data model. The generators' HTML is embedded and styled, not rebuilt.
- No redirects. Pages are added and none are removed or renamed. `goReferenceURL` changes from an external URL to `/api/go/`; its only user is the footer, which this plan updates.

## Changelog

### 2026-10-13 — Task: Write the Using XCL with Go guide

**What was done**: Added `/using-xcl-with-go/`, a build-up walkthrough of one program, `deployer`, from an empty module: entity types as structs, a local registry with `RegisterType` and `RegisterFunction`, `NewConfig`, `Validate` with the `ConfigError` loop, `ApplyContext` under `signal.NotifyContext`, `Find`/`FindByType`/`FindOne`/`All` and the untyped `FindEntitiesByType`/`Entities`, outputs, `ErrNotFound` and `*NotUniqueError`, a `status` command using `Load`, and `Destroy`, ending with the full program and links out. Every snippet was assembled in page order in an empty directory against `github.com/jumppad-labs/xcl@v0.5.3` and run (`apply`, the temporary replica, `status`, `destroy`); the printed lines in the page are copied from that run.

**Deviations**: The release is v0.5.3, not v0.5.0 (v0.5.0 was already taken and v0.5.1/v0.5.2 never reached PyPI; orchestrator brief), so `go get` pins `@v0.5.3`. The `go get` line is a literal for now because the release pin (`src/config/release.json`) arrives with the API reference task. The program is split into one file per concern (`types.go`, `registry.go`, `config.go`, `check.go`, `apply.go`, `find.go`, `outputs.go`, `lookups.go`, `main.go`) so each section adds a whole file with its own imports, and `run` grows by "replace the final return" edits; this keeps "copy the snippets in order" exact under Go's unused-import rule. The not-unique error is shown by adding a replica database temporarily, and removing it again shows that an entity dropped from the configuration is removed on the next apply. No tests were written (plan convention and spec non-goal), so the test step had nothing to delegate.

**Files changed**:
- `xcl-website: src/pages/using-xcl-with-go.mdx`

**Discoveries**: The validation error text from the release still says "resource 'app.web' refers to ..., which is not defined", so the guide describes validation problems rather than quoting them, to keep the "entity, not resource" rule; this is a library wording gap to report, not fix here. `Outputs()` is keyed by the full address (`output.database_location`). After `Destroy`, `Load` succeeds with nothing in state, so a lookup then returns `ErrNotFound`.

### 2026-10-13 — Task: Put the Go guide in the navigation and on the home page

**What was done**: "Using XCL with Go" is the first child of the Guides dropdown, so the Guides label now opens the Go guide. The home page's closing banner is now "Start building with XCL" with the Go guide as its primary button and "See an example →" as the secondary, and the home page's Documentation section lists the Go guide right after the Language reference. The website README's page table lists the guide.

**Deviations**: None. The Documentation section from the language-reference spec was already in place (`index.mdx` around line 346), so the entry was added after its Language reference item.

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: Link the Go guide from the xcl README

**What was done**: The README's Documentation list links `https://xcl.dev/using-xcl-with-go/` with a one-line description, directly after the Language reference entry. Nothing else in the README changed.

**Deviations**: None.

**Files changed**:
- `xcl: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: xcl.dev generates the Go and Python API references from the advertised release

**What was done**: Added the release pin `src/config/release.json` (`{ "xcl": "v0.5.3" }`), re-exported from `site.ts` as `xclRelease` and `xclLangVersion`, with `goReferenceURL = "/api/go/"`, `pythonReferenceURL = "/api/python/"` and `goPkgDevURL`. The hero badge and the Go guide's `go get` line read the pin. `npm run api-docs` (`scripts/api-docs/generate.sh`) clones xcl at the pinned tag and runs doc2go v0.12.2 in embed mode over the twelve public packages, then installs pdoc 16.0.0 (pinned with its dependencies in `requirements.txt`) and `xcl-lang==0.5.3` into a throwaway venv and runs `python.py`, which renders `xcl`, `xcl.aio` and `xcl.mask` through a content-only `frame.html.jinja2` and rewrites pdoc's file links to `/api/python/...` (an undocumented target fails the run). Two catch-all routes render the fragments inside `Shell` through `ApiReference.astro`, with a heading naming the release, a pkg.go.dev or PyPI link and a list of the reference's packages or modules; `api-reference.css` styles both generators' markup with the site tokens and carries the generated Chroma and Pygments `github-dark` classes. `deploy.yml` gains `workflow_dispatch`, Go 1.27.x, Python 3.12 and an `npm run api-docs` step before the build; `generated/` is git-ignored; `make api-docs` and a README section were added. The footer's Go link now opens in the same tab.

**Deviations**: Release v0.5.3 instead of v0.5.0 (orchestrator brief: v0.5.0 was taken, v0.5.1/v0.5.2 never reached PyPI). The fragment parsing is shared in `src/lib/api-fragments.ts` rather than repeated in both routes, and `ApiReference.astro` takes the parsed page plus the list of all pages (to render a package or module list in the header, which also gives the three Python pages links to each other since pdoc's own nav is dropped) instead of separate `title`/`description`/`html` props. `process.env.CI` is read through `globalThis`, because the site has no Node type definitions and `astro check` rejected `process`. The Go guide's install block switched from a fenced block to expressive-code's `<Code>` component so the version comes from the pin.

**Files changed**:
- `xcl-website: src/config/release.json`
- `xcl-website: src/config/site.ts`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: src/pages/using-xcl-with-go.mdx`
- `xcl-website: src/components/Footer.astro`
- `xcl-website: src/components/ApiReference.astro`
- `xcl-website: src/lib/api-fragments.ts`
- `xcl-website: src/pages/api/go/[...path].astro`
- `xcl-website: src/pages/api/python/[...path].astro`
- `xcl-website: src/styles/api-reference.css`
- `xcl-website: scripts/api-docs/generate.sh`
- `xcl-website: scripts/api-docs/python.py`
- `xcl-website: scripts/api-docs/go-frontmatter.tmpl`
- `xcl-website: scripts/api-docs/requirements.txt`
- `xcl-website: scripts/api-docs/pdoc-templates/frame.html.jinja2`
- `xcl-website: .github/workflows/deploy.yml`
- `xcl-website: package.json`
- `xcl-website: Makefile`
- `xcl-website: .gitignore`
- `xcl-website: README.md`

**Discoveries**: doc2go's front matter `.Path` is the full import path (the root is `github.com/jumppad-labs/xcl`), not a path relative to `-home`. doc2go run with Go 1.27 also documents the Go 1.27 method-form lookups (`Config.Find` and friends) beside the functions. The released `xcl-lang` 0.5.3 imports with only `protobuf` beside it, and importing `xcl`, `xcl.aio` and `xcl.mask` spawns no process (checked by spying on `subprocess.Popen`). Its docstrings are reStructuredText (double backticks), pdoc's default format. The whole generation takes about five seconds locally once the Go module cache is warm. The live deploy (push to main or a manual dispatch) has not run: it is outstanding until the user pushes.

### 2026-10-13 — Task: Put the API references in the navigation, footer and home page

**What was done**: The nav gains an "API" dropdown right after "Language", with Go then Python, so its label opens `/api/go/`. Nav items now carry an optional `hideOnSmall`, set on "Features" and "API", replacing the hard-coded "first item hidden on phones" check. The footer's "Go Reference" became "Go API" (`/api/go/`) and "Python API" (`/api/python/`) follows it, both in the same tab. The home page's Documentation section ends with the Go and Python API references, and the website README's page table lists both.

**Deviations**: None. Checked with headless Chrome: at 375px the bar shows xcl, Language, Examples, Guides and GitHub without overflow; at desktop width "API" sits between "Language" and "Examples".

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/components/Footer.astro`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: Write the Using XCL with Python guide

**What was done**: Added `/using-xcl-with-python/`, the same `deployer` walkthrough in Python: install `xcl-lang` (version from the release pin) in a virtualenv, `@xcl.entity` dataclasses, a `LocalRegistry` with `register_type` and `register_function(slugify)`, the Go Docker plugin through `xcl.GitHubRegistry(trusted_keys=...)` at `v0.2.0` with the key from `https://xcl.dev/keys/jumppad-labs-releases.asc`, `xcl.Config` as a context manager, `validate` with `ConfigError.problems`, `apply`, `find`/`find_by_type`/`find_one`/`all` and `find(None, ...)` for the plugin's container, outputs, `NotFoundError` and `NotUniqueError`, a `status` command with `load`, `destroy`, then `main_async.py` with `xcl.aio.Config`, an `async def` function, a ticker that keeps running during the apply and `asyncio.timeout` for cancellation, and a fork-safety section. Every snippet was assembled in page order in an empty directory with a fresh virtualenv holding `xcl-lang==0.5.3` from PyPI and a Docker-compatible engine (podman), and run: apply, the temporary replica, status, destroy and the asyncio program. The printed lines in the page are copied from those runs.

**Deviations**: `xcl.ConfigError` exposes `problems` (file, line, column, message), not `causes` as the plan's context said; the guide uses `problems`. `NotUniqueError` carries the count in `details["count"]`, as a string. A plugin entity is read as `container["ip_address"]` and `container.meta`. The program is one `main.py` that grows by adding functions above `main` and replacing `main`, plus `main_async.py`, which imports the classes and the GitHub registry from `main.py` and defines its own registry for the `async def` slugify. The asyncio demo uses a ticker rather than `asyncio.start_server`. The plugin is pinned at `v0.2.0`, the latest release, which loads under v0.5.3.

**Files changed**:
- `xcl-website: src/pages/using-xcl-with-python.mdx`

**Discoveries**: `jumppad-labs/xcl-plugin-docker` v0.2.0 (built 2026-10-10) loads and works under xcl v0.5.3 from Python, against podman's Docker-compatible socket. The sync API turns Ctrl-C into a cancel that drains the operation so progress is saved, then re-raises `KeyboardInterrupt`. A leftover `guide` network from an earlier run makes the next apply fail with "network already exists", so the run-through must start from a destroyed state. Engine error details reach Python as strings (`details["count"] == "2"`).

### 2026-10-13 — Task: Put the Python guide in the navigation and on the home page

**What was done**: "Using XCL with Python" is the second entry of the Guides dropdown, after the Go guide. The home page's closing banner offers both guides (Go primary, Python secondary), the Documentation section reads Language reference, Go guide, Python guide, then the two API references, and the website README's page table lists the Python guide.

**Deviations**: None. The examples stay linked from the hero and the "Two examples" section.

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: README.md`

**Discoveries**: None.

### 2026-10-13 — Task: Link the Python guide from the xcl README and record the guides in the changelog

**What was done**: The README's Documentation list now reads Language reference, "Using XCL with Go", "Using XCL with Python". `CHANGELOG.md` gains this spec's single entry at the top: one prose paragraph on the two guides, what each covers, the generated API references at `/api/go/` and `/api/python/` and the README links, then **Breaking:** None.

**Deviations**: None.

**Files changed**:
- `xcl: README.md`
- `xcl: CHANGELOG.md`

**Discoveries**: None.

### 2026-10-13 — Task: Link the API reference and the Python guide from the xcl-python README

**What was done**: The xcl-python README links `https://xcl.dev/api/python/` (Python API reference) and `https://xcl.dev/using-xcl-with-python/` (getting started guide), right after its xcl.dev bullet. It is the only change in xcl-python: no docs build, workflow, Pages site or dependency. The links reach PyPI with the next `xcl-lang` release, since the README is the package's long description.

**Deviations**: None.

**Files changed**:
- `xcl-python: README.md`

**Discoveries**: None. Both URLs open only once xcl.dev has been deployed with this work; that is outstanding until the user pushes.
