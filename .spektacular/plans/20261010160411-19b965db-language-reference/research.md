---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Research: 20261010160411-19b965db-language-reference

## Alternatives considered and rejected

- **Expressive Code / Starlight `<Tabs syncKey>`.** The site uses `astro-expressive-code` 0.43 (`xcl-website:astro.config.mjs:10`, `xcl-website:ec.config.mjs`) without Starlight; Starlight's `Tabs` component is not available outside Starlight, and Expressive Code has no tab feature. Adopting Starlight would restructure every page and the layout (`xcl-website:src/layouts/Shell.astro`), which the spec's "fit the existing structure" constraint rules out. Rejected.
- **A third-party Astro tabs package.** Adds a dependency for ~60 lines of Astro + inline script, and none of the common ones store a site-wide choice and switch every box on the page together. The repo prefers few dependencies (`xcl:conventions/dependencies.md`). Rejected.
- **Tabs only inside new reference pages, Python as separate boxes elsewhere.** Contradicts the spec's "one reusable tabbed code component across the site". Rejected.
- **One single long reference page.** Every acceptance area (files, mapping, meta, references, variables, modules, ~57 functions, expressions) on one page would exceed the longest existing page (`examples/plugins.mdx`, 1853 lines) and makes per-section links weak. Rejected in favour of a `/language/` section of pages.
- **Collect the config-to-type mapping in its own "Go types" page.** The spec's Technical Approach asks for the mapping alongside each feature. Rejected.
- **Automated browser test of the tabs (Playwright).** The website has no test tooling (`xcl-website:package.json` scripts: dev/build/preview only), the spec's non-goal excludes automated tests of examples, and the convention routes website checks to human review (`xcl:conventions/testing-and-mocking.md`). Rejected; `astro check` + `astro build` are the automated gates, tab behaviour is a manual check.
- **Describing the README's old `local` block, `HCL_VAR_` and "around sixty" functions.** Stale: no `local` in `DefaultResources` (`xcl:internal/resources/default.go:6-13`), the env prefix becomes `XCL_VAR_` (language-surface plan, Milestone 1), there are 57 built-ins (`xcl:internal/functions/functions.go:430-486`). Rejected as text; README history used only as raw material.

## Chosen approach — evidence

- Fenced code blocks inside MDX are rendered by Expressive Code before MDX (`xcl-website:astro.config.mjs:8-10`), so fenced blocks nested inside an Astro component's named slots (`<div slot="go">`) still get highlighting, frames and titles.
- Every page uses `layout: ../layouts/Shell.astro` and `Shell.astro` already carries an inline script block (`xcl-website:src/layouts/Shell.astro:28-70`) and imports `Head.astro`, which is where an early, pre-paint script can set `<html data-lang>` to avoid a flash.
- Navigation is a static array of items with dropdown children (`xcl-website:src/components/Nav.astro:8-33`); adding a dropdown is a data change.
- Go code boxes per page (from a scan of ```` ```go ```` fences): configuration-text 9, diff 4, events 6, github-registry 6, index 2, plugin-logging 6, plugin-template 4, registries 9, replacement 9, sensitive-values 3, state-masking 2, examples/configuration-only 7, examples/plugins 31.
- Python API to show: python-support plan "Python public API" — `@xcl.entity(type, subtype)` on dataclasses subclassing `xcl.Resource`, `T | None`/defaults optional, `xcl.Sensitive[T]`, `xcl.Computed[T]`, nested dataclass = block, `list[Block]` = repeated block, `dict[str, Block]` = named-block map with `xcl.field(label=…)`, `dict[str, str]` = map attribute, `xcl.field(name=, block=, label=)`; `LocalRegistry.register_type/register_function/register_external_plugin/register_plugin_directory`, `GitHubRegistry(cache_dir, trusted_keys, token, api_url).register_plugin`, `xcl.Config(registries=, state_path=, state_store=, state_mask=, variables=, on_event=, event_data=, event_buffer_size=, event_mask=)`, `find/find_by_type/find_one/all/outputs/diff(reveal_sensitive=)/encode_saved_entity(highlight=)`, `xcl.aio.Config`, `mask.aes256gcm/hmac_sha256/redact/omit`. No custom registries, custom maskers, `EncodeEntity` or custom highlight renderers in Python (python-support "Out of Scope": no Python `decode`/`EncodeEntity`).
- Go type mapping: tags in language-surface plan "Go types authors write" — `xcl:"name"`, `,optional`, `,block` (pointer = optional, slice = repeated, `map[string]T` = named blocks), `,label`, `,remain` on `types.ResourceBase`; computed fields (`xcl:"…,optional,computed"` per existing site text, `xcl-website:index.mdx` FeatureCard "computed").
- Built-in fields: `types.ResourceBase{DependsOn, Disabled, Meta}` and `types.Meta{ID, Name, Type, Subtype, Module, File, Line, Column, …}` (`xcl:types/resource.go:1-95`); configuration refers to them as `<address>.meta.name` / `.meta.id` (`xcl:example/configonly/config/deployment.xcl:62,99`).
- Files: `.xcl` files discovered per directory (`xcl:internal/parser/util.go:59-66`), `.vars` files auto-discovered in root directories (`xcl:internal/parser/parser.go:659`), precedence defaults < .vars < env < direct (`xcl:internal/parser/context.go:202-213`).
- Modules: local directory sources are what the parser resolves (`xcl:docs/modules.md:17-30,120-126`: `internal/getter` has no importers, so remote sources do not work today). The removed README claimed GitHub sources; that claim is stale and is checked during verification.
- Module boundary and full-address lookup: removed README "The module boundary" (git `bdba483^:README.md:1760-1805`), `xcl:config_module_boundary_test.go`.
- Functions after language-surface: built-ins + `registry.Local.RegisterFunction(name, fn)` + plugin `Functions()`; clashes are `FunctionNameClashError` (NewConfig for app functions, first operation for plugin clashes); directives `%{ }` rejected (language-surface plan Milestones 1 and 3).

## Files examined

- xcl-website:astro.config.mjs:8-10 — Expressive Code runs before MDX.
- xcl-website:ec.config.mjs — theme, line numbers, frame style overrides to match when styling tab headers.
- xcl-website:src/components/Nav.astro:8-33 — nav items array, dropdown children.
- xcl-website:src/layouts/Shell.astro:28-70 — inline scripts at body end (copy buttons, anchor scroll).
- xcl-website:src/components/Head.astro — head meta; place for the pre-paint language script.
- xcl-website:src/config/site.ts — shared URLs (githubURL, goReferenceURL); add xcl-python URL / PyPI here.
- xcl-website:src/pages/index.mdx:45-120,130-255 — Go type and main.go boxes; FeatureCards for variables, modules, functions; "Two examples" section.
- xcl-website:src/pages/*.mdx — Go box inventory above; plugin-code boxes in replacement, plugin-logging, plugin-template, examples/plugins.
- xcl-website:.spektacular knowledge gotchas/no-redirects-on-page-removal.md — slugs are chosen once.
- xcl:README.md:166-177 — Documentation list to link from.
- xcl:CHANGELOG.md — `## <spec name>` entries, prose then **Breaking:**.
- xcl:internal/functions/functions.go:430-486 — 57 built-in function names; 292-295 template_file helpers `quote`, `trim`.
- xcl:types/resource.go — Meta and ResourceBase fields.
- xcl:internal/resources/default.go — built-in entity types: variable, output, module, root.
- xcl:internal/parser/util.go:59-66, parser.go:659-696, context.go:202-213 — files and variable precedence.
- xcl:internal/getter/getter.go — go-getter wrapper, unused.
- xcl:docs/modules.md — modules resolve local relative paths only.
- xcl:git bdba483^:README.md:1317-2060 — removed Struct Tags, References, Variables, Local, Modules, Functions sections (raw material).

## External references

- Astro docs, named slots in MDX (`<div slot="name">`) — how the tab component receives one fenced block per language.
- WAI-ARIA Authoring Practices "Tabs" pattern — roles, `aria-selected`, arrow-key navigation for the tab component.
- go-cty `function/stdlib` — signatures of the stdlib built-ins listed in the reference.

## Prior plans / specs consulted

- `20261011095738-fad05e41-language-surface` plan (final) — the language this reference describes: `XCL_VAR_`, named-block maps, optional labels, `for` references, optional object attrs, directive rejection, app/plugin functions, clash timing; ships as v0.3.0. Its website task fixes `HCL_VAR_` and the function count only.
- `20261011130709-aa3eb933-python-support` plan (final) — the Python API shown in every Python tab; ships as v0.4.0; explicitly leaves Python docs on xcl.dev to this epic's docs specs.
- `20261011084948-c995ed25-using-xcl-with-go` spec (sibling, planned in parallel) — two per-language guides, no tabs, also linked from nav, home page and README.

## Open assumptions

- The python-support Python API lands as its plan states (names above). If names differ in the released package, Python tabs follow the release; a missing capability is marked "not yet available in Python".
- Remote module sources do not work in the release; verification confirms which sources work before the page names them.
- Pages are published only after the release containing both dependency specs (v0.4.0) is tagged and on PyPI.
- Fenced blocks inside named slots render through Expressive Code; if not, the component takes `code`/`lang` props rendered with Expressive Code's `<Code>` component instead.

## Drafting assumptions

### Plugin authoring pages are four pages (discovery)
- **Decision**: The plugin authoring pages are plugin-template, examples/plugins, replacement and plugin-logging. On them, plugin code stays Go-only; boxes that show an application loading or using a plugin get Python tabs; each states plugins are written in Go for now and usable from Go and Python applications.
- **Rationale**: replacement documents provider Changed/Update and plugin-logging documents logging from inside a plugin; both are plugin authoring. Provider code is not application code, so the spec's requirement is met.
- **Rejected**: Only plugin-template and examples/plugins (would force Python tabs on provider code that has no Python form).

### Go-only capabilities get a Python tab that says so (discovery)
- **Decision**: Where application Go code has no Python equivalent (custom Registry implementation, custom masker, EncodeEntity, custom highlight renderer), the Python tab holds a short "not yet available from Python" note naming what Python does offer and linking the xcl-python parity list.
- **Rationale**: Meets "no Go code box without a Python tab" honestly, and the "current behaviour only" rule. That rule exempts the four plugin authoring pages and the sibling spec's two per-language guides (`/using-xcl-with-go/`, `/using-xcl-with-python/`), which are single-language by their spec.
- **Rejected**: Leaving those boxes Go-only (fails the acceptance criterion); inventing Python API (contradicts the python-support API constraint).

### Chosen direction: CodeTabs component and /language/ section (architecture)
- **Decision**: Option 1. A hand-written `CodeTabs.astro` with named `go`/`python` slots, a pre-paint `<html data-lang>` script, `localStorage` key `xcl-lang` (default `go`), and eight reference pages under `/language/`.
- **Rationale**: Meets "one reusable component, choice stored in browser", adds no dependency, keeps the existing structure and URLs. Topic pages make sections linkable.
- **Rejected**: Starlight migration (restructures the site); third-party tabs plus a single long page (no synced or persisted choice, and the page would be too long).

### Nav and home placement (architecture)
- **Decision**: A new "Language" dropdown in the nav between Features and Examples. On the home page, a "Learn the language" link in "What is XCL?" and a new "Documentation" section after "Two examples" that the sibling guides also use. In the README, the reference is the first item of the Documentation list.
- **Rationale**: The reference is the language's home, distinct from task guides, so it gets its own nav entry. The sibling's guides sit at the top of the existing Guides dropdown.
- **Rejected**: Putting the reference inside the Guides dropdown (already ten items, and the reference is not a guide).

### Conventions selected (architecture)
- **Decision**: Applied: no tests inspecting repo files, entity glossary, dependencies, project structure (imports), and the no-redirects gotcha. Dropped: all Go test/code conventions.
- **Rationale**: This work adds no Go library code.
- **Rejected**: Listing every convention.

### CHANGELOG entry for a docs-only spec (architecture)
- **Decision**: The xcl CHANGELOG gets a `## 20261010160411-19b965db-language-reference` entry in prose (the reference is on xcl.dev and the README links to it), then `**Breaking:** None`.
- **Rationale**: Follows the project rule that each spec touching xcl has one entry, and this spec edits the xcl README.
- **Rejected**: No entry (breaks the one-entry-per-spec pattern).

### Tab behaviour is checked by hand (architecture)
- **Decision**: No automated browser test for CodeTabs. `astro check` and `astro build` are the automated gates, and tab switching and persistence are manual checks in the test plan.
- **Rationale**: The site has no test tooling, website checks are human review by convention, and the spec excludes automated tests for the examples.
- **Rejected**: Adding Playwright (new tooling for one behaviour).

### Site links config gains Python URLs (components)
- **Decision**: Add the xcl-python repository and PyPI URLs to the shared site links config.
- **Rationale**: This follows the existing pattern of one place for outward links (githubURL, templateURL, goReferenceURL).
- **Rejected**: Hard-coding the URLs in each page.

### Verification is an agent task needing the released versions (tasks)
- **Decision**: An agent task tries every example against the tagged release, using throwaway programs. It STOPs if v0.4.0 and the PyPI package are not published. Releases are human tasks in the dependency plans, not in this plan.
- **Rationale**: Trying examples is work that makes the pages correct, so it is a task, not a review. Cutting releases is already owned upstream.
- **Rejected**: A human task for verification (it would stay open as a review); a duplicate release task here.

### Python examples for Go example programs are illustrative (tasks)
- **Decision**: The Python tabs on example pages (configuration-only, the plugins example program) show equivalent Python. They are not added to the xcl repository as runnable examples, and a sentence says so.
- **Rationale**: The spec scopes changes to the website plus links. Adding Python example programs to the repo is not asked for.
- **Rejected**: Adding `example/configonly-python` to xcl (scope creep).

### Home page Python tab loads the plugin externally (tasks)
- **Decision**: Where Go registers a plugin in process, the Python tab loads the same plugin as an external binary and says why.
- **Rationale**: Python cannot host Go plugins in process (python-support plan), and this keeps a working example.
- **Rejected**: A "not available" note on the home page's main example (a poor first impression, and an equivalent exists).

### Hero badge and tagline updated with the home page links (tasks)
- **Decision**: The home page link task updates the hero's version badge ("v0.2.2") to the release the reference is verified against (the release containing language-surface and python-support, expected v0.4.0), and changes the "Available for Go today, with Python … to come" line to say XCL is available for Go and Python. Decided by the user at epic review.
- **Rationale**: Once the reference and Python tabs are published against that release, a hero that names v0.2.2 and calls Python "to come" contradicts the rest of the site. The change is adding Python to an existing page, in line with the spec's non-goal.
- **Rejected**: Leaving it to whoever cuts v0.4.0 (the original choice): no plan owned it, so the hero would stay stale after publishing.

## Rehydration cues

- `spektacular plan file read 20261011095738-fad05e41-language-surface plan` and `... 20261011130709-aa3eb933-python-support plan` (and `context`).
- `spektacular spec file read 20261010160411-19b965db-language-reference`; sibling `spektacular spec file read 20261011084948-c995ed25-using-xcl-with-go`.
- `git -C /home/nicj/code/github.com/jumppad-labs/xcl show bdba483^:README.md | sed -n 1317,2060p` for the removed language sections.
- Read `xcl-website/src/components/Nav.astro`, `src/layouts/Shell.astro`, `astro.config.mjs`, `ec.config.mjs`.
