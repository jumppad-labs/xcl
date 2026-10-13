---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261010160411-19b965db-language-reference

## Current State Analysis

- `xcl-website` (Astro 5, MDX, Expressive Code 0.43, Tailwind 4) has 13 pages: the home page, ten topic pages and two example pages. It has no tab component, and every code example is Go only. About 98 Go fences exist, and roughly half are application code (`src/pages/*.mdx`; inventory in research.md).
- Navigation is a static items array with dropdowns: Features, Examples and Guides (`xcl-website:src/components/Nav.astro:8-33`). The layout has inline scripts at the end of the body (`xcl-website:src/layouts/Shell.astro:28-70`), and the head is small (`xcl-website:src/components/Head.astro:1-14`).
- The site has no language reference. The language sections were removed from the xcl README in `bdba483` (`git show bdba483^:README.md`, lines 1317-2060). Parts of them are stale: `HCL_VAR_`, `local`, GitHub module sources, and custom functions through parser options.
- The xcl README's Documentation list (`xcl:README.md:166-177`) links the topic pages. The CHANGELOG has one `## <spec name>` entry per spec (`xcl:CHANGELOG.md`).
- The language as this plan documents it is delivered by the language-surface plan (v0.3.0) and the python-support plan (v0.4.0, `xcl` on PyPI). Neither has landed at planning time.

## Per-Task Technical Notes

### Task: Tabbed Go and Python code boxes

- `xcl-website:src/components/CodeTabs.astro` (new) — markup: wrapper `div.code-tabs`, `div[role=tablist]` with two `button[role=tab][data-lang-choice=go|python]` (labels "Go", "Python"), then `div[role=tabpanel][data-lang-panel=go]><slot name="go"/>` and the same for `python`. Give the tab and panel ids unique suffixes (`Astro.locals`-free counter or `crypto.randomUUID()` at build) for `aria-controls`/`aria-labelledby`. Scoped `<style is:global>` rules: `html:not([data-lang=python]) [data-lang-panel=python]{display:none}` and `html[data-lang=python] [data-lang-panel=go]{display:none}`; selected tab styling from `aria-selected`. Use the site palette tokens from `xcl-website:src/styles/global.css`, and match the Expressive Code frame (`xcl-website:ec.config.mjs` `borderColor #30363d`, `editorBackground #1c2128`, radius `0.5rem`) so the tab strip sits flush on the frame.
- `xcl-website:src/components/Head.astro:1-14` — add `<script is:inline>`: `try { var l = localStorage.getItem('xcl-lang'); } catch (e) {}` then set `document.documentElement.dataset.lang = (l === 'python' ? 'python' : 'go')`. It must be inline and in `<head>` so it runs before paint.
- `xcl-website:src/layouts/Shell.astro:28-70` — beside the copy-button script, add a handler. On a click of `[data-lang-choice]`, it sets `document.documentElement.dataset.lang`, writes `localStorage.setItem('xcl-lang', …)` in try/catch, and updates `aria-selected`/`tabindex` on every tab. It also adds ArrowLeft/ArrowRight/Home/End handling inside a tablist, following the WAI-ARIA tabs pattern. Sync `aria-selected` once on load from the attribute.
- Smoke usage: add no page in this task. Confirm with a temporary local MDX page, deleted before finishing, that fenced blocks inside `<div slot="go">` render through Expressive Code with their `title`. MDX needs a blank line after `<div slot="go">` and before `</div>` for the fence to parse. If slotted fences do not render, switch the component to take `go`/`python` code strings and render with `astro-expressive-code`'s `<Code>` component. Record which form was used in the component's header comment.
- Gates: `npx astro check` and `npm run build` pass.
- **Complexity**: Medium
- **Token estimate**: ~30k
- **Agent strategy**: Single agent, sequential execution.

### Task: Python tabs on the registry, GitHub registry and events pages

- `xcl-website:src/pages/registries.mdx:32,58,96,118,150,198,216` — wrap each in `CodeTabs`. Python uses `xcl.LocalRegistry(plugin_pattern=…)`, `register_type(Deployment)`, `register_external_plugin`, `register_plugin_directory`, and `xcl.Config(registries=[local, shared])`. Errors are `xcl.TypeNameClashError` / `xcl.PluginLoadError`, the python-support exception per code; use the exact class names from the released package. The type examples at `:58` use `@xcl.entity("database", "postgres")` dataclasses.
- `xcl-website:src/pages/registries.mdx:266,319` — "Writing your own registry": the Go `Registry` interface and `Toolkit`. The Python panel is a "not yet available from Python" note. It says Python programs use `LocalRegistry` and `GitHubRegistry`, and links the xcl-python parity list.
- `xcl-website:src/pages/github-registry.mdx:28,56,91,114,139,167` — Python uses `xcl.GitHubRegistry(trusted_keys=[key], token=os.environ[...], cache_dir=…)` and `.register_plugin("jumppad-labs/xcl-plugin-docker", "v1.2.0")`. The panel at `:56` shows the same invalid versions raising, using the package's actual error type. At `:167`, the install error is caught as the Python exception.
- `xcl-website:src/pages/events.mdx:30,76,105,206` — Python `on_event=` handler, `event_mask=xcl.mask.hmac_sha256(key)`, `event_buffer_size=4096`, and a `logging`-based handler in place of `events.SlogHandler`. Mention that `xcl.aio.Config` accepts `async def` handlers, with a link to the Python guide (sibling spec, slug as that plan sets it; until it exists, link the xcl-python README).
- `xcl-website:src/pages/events.mdx:238,266` — the `example/prettylog` box is Go example-program code. The Python panel shows an equivalent handler writing coloured lines with the standard library.
- `xcl-website:src/config/site.ts` — add `pythonRepoURL = "https://github.com/jumppad-labs/xcl-python"`, `pythonParityURL = \`${pythonRepoURL}/blob/main/PARITY.md\`` and `pypiURL = "https://pypi.org/project/xcl/"`. Shared with the other tab tasks; whichever lands first adds them.
- **Complexity**: Medium
- **Token estimate**: ~40k
- **Agent strategy**: 2-3 parallel agents, one per page.

### Task: Python tabs on the sensitive values, state masking, configuration text and diff pages

- `xcl-website:src/pages/sensitive-values.mdx:29,67,112` — Python dataclass with `password: xcl.Sensitive[str]`, `.reveal()`. At `:112`, the plain-field error is shown as the Python exception the release raises when a sensitive value meets a plain `str` field.
- `xcl-website:src/pages/state-masking.mdx:42` — `xcl.Config(state_path=…, state_mask=xcl.mask.aes256gcm(key))` with `base64.b64decode(os.environ["XCL_STATE_KEY"])`.
- `xcl-website:src/pages/state-masking.mdx:114` — custom masker: a "not yet available from Python" note naming the built-in `mask.aes256gcm`, `hmac_sha256`, `redact` and `omit`.
- `xcl-website:src/pages/configuration-text.mdx:28,95,163` — `xcl.EncodeEntity`, `IncludeComputed` and `ShowReferences` have no Python form (python-support out of scope). Each gets a "not yet available" note pointing to `encode_saved_entity`.
- `xcl-website:src/pages/configuration-text.mdx:65,80` — `c.encode_saved_entity(event.data)` and the entity-from-event form, using the Python `Event.entity()`.
- `xcl-website:src/pages/configuration-text.mdx:212,227,249,278` — highlighting. Python offers `encode_saved_entity(data, highlight=True)` only. `:212`/`:227` show that; `:249` (theme file) and `:278` (custom renderer) get "not yet available" notes.
- `xcl-website:src/pages/diff.mdx:30,52,129` — `d = config.diff("./config")`, iterating `d.resources` / `changes` with `before`/`after`/`unknown`, and `reveal_sensitive=True`.
- `xcl-website:src/pages/diff.mdx:218` — highlight renderer: the same treatment as the configuration-text page.
- **Complexity**: Medium
- **Token estimate**: ~40k
- **Agent strategy**: 2-3 parallel agents, one per page pair.

### Task: Python tabs on the home page and the configuration-only example

- `xcl-website:src/pages/index.mdx:45` — the `resources.go` `PostgreSQL` struct gets a Python tab: `@xcl.entity("database", "postgres") @dataclass class PostgreSQL(xcl.Resource)`, with `location: str`, `port: int`, `username: str`, `password: xcl.Sensitive[str]`, `timeouts: Timeouts | None = None`, and the computed field as `xcl.Computed[str]`.
- `xcl-website:src/pages/index.mdx:100` — `main.go` registers `postgres.Plugin` in process. The Python tab registers the same plugin built as an external binary (`local.register_external_plugin("./bin/xcl-plugin-postgres")`). One sentence says Python loads Go plugins as separate programs. The rest of the program (`apply`, `find`) mirrors Go.
- `xcl-website:src/pages/examples/configuration-only.mdx:49,115` — `example/configonly/resources/resources.go` types become dataclasses (`resources.py`). The `ConfigMap` map attribute is `data: dict[str, str]`.
- `xcl-website:src/pages/examples/configuration-only.mdx:314,367,405` — `main.go` and `routes.go` become `main.py` and `routes.py` using `xcl.Config`, `find`, `find_by_type` and `all`.
- `xcl-website:src/pages/examples/configuration-only.mdx:518,554` — `routes_test.go` becomes pytest functions in `test_routes.py`, one behaviour per function.
- Add one sentence near the top of the example page. It says the Python version is not in the repository and is shown for comparison.
- **Complexity**: Medium
- **Token estimate**: ~35k
- **Agent strategy**: Single agent, sequential execution.

### Task: Plugin authoring pages say plugins are Go and tab the application code

- Statement (same wording on each page, placed after the page's opening paragraph or `Prose` intro): "Plugins are written in Go for now. A plugin built from this template works from both Go and Python applications." Adapt "this template" per page.
  - `xcl-website:src/pages/plugin-template.mdx`
  - `xcl-website:src/pages/examples/plugins.mdx` (before `## The configuration`, line 63)
  - `xcl-website:src/pages/replacement.mdx`
  - `xcl-website:src/pages/plugin-logging.mdx`
- `xcl-website:src/pages/plugin-template.mdx:128,160,190` — application loading boxes become CodeTabs: `register_external_plugin`, `GitHubRegistry(trusted_keys=[public_key]).register_plugin(...)`. At `:128`, the in-process `RegisterPlugin(&notes.Plugin{})` line has no Python form, so the Python tab shows only the external binary line, with a comment.
- `xcl-website:src/pages/plugin-template.mdx:77` — provider code. It stays Go-only.
- `xcl-website:src/pages/plugin-logging.mdx:185,226` — application boxes become CodeTabs. Python uses `LocalRegistry.register_external_plugin` / `register_plugin_directory`, and catches `xcl.PluginLoadError`.
- `xcl-website:src/pages/plugin-logging.mdx:31,100,142,147` — plugin code stays Go-only.
- `xcl-website:src/pages/replacement.mdx:28-342` — all boxes are provider code. They stay Go-only; the page gets the statement only.
- `xcl-website:src/pages/examples/plugins.mdx:985,1029,1058,1076,1101,1122,1154,1177,1188,1208,1247,1266` — "The program" (application code) boxes become CodeTabs.
  - Where Go registers the in-process template plugin, the Python tab loads the Docker plugin externally. A "not yet available from Python" note covers in-process registration.
  - Boxes at `:155-:902` and `:1282`, `:1313` are plugin code and tests, and stay Go-only.
- **Complexity**: Medium
- **Token estimate**: ~45k
- **Agent strategy**: 2-3 parallel agents (the plugins example page alone; the other three together).

### Task: Reference overview, entities, and attributes and blocks pages

- `xcl-website:src/pages/language/index.mdx` (new, `layout: ../../layouts/Shell.astro`, `title: "Language reference"`). It follows the structure of `xcl-website:src/pages/sensitive-values.mdx` (Hero/Prose pattern) and covers:
  - XCL as its own language;
  - files: `.xcl` files in a directory, discovered per directory (`xcl:internal/parser/util.go:59-66`), and `.vars` files in the root directories (`xcl:internal/parser/parser.go:659`);
  - the list of the seven topic pages.
- `xcl-website:src/pages/language/entities.mdx` (new). Headings:
  - `## Blocks and labels`;
  - `## Addresses` (`database.postgres.main`, `app.web`; addresses inside modules use the `module.<name>.` prefix);
  - `## Built-in fields`: `depends_on` and `disabled` (written), and `meta.id`, `meta.name`, `meta.type`, `meta.subtype`, `meta.module`, `meta.file`, `meta.line`, `meta.column` (read-only), from `xcl:types/resource.go:5-95`. Example: `config_map = config_map.api.meta.id` (`xcl:example/configonly/config/deployment.xcl:99`). The Python `xcl.Resource` exposes the same as `.meta`, `.depends_on`, `.disabled`.
  - `## Entity types`: registered by the application or a plugin, plus the language's own `variable`, `output`, `module` (`xcl:internal/resources/default.go:6-13`).
- `xcl-website:src/pages/language/attributes-and-blocks.mdx` (new). One heading per shape, each with XCL then `CodeTabs`:
  - `## Required attributes` — `xcl:"location"` / `location: str`;
  - `## Optional attributes` — `,optional` / `T | None = None` or a default;
  - `## Nested blocks` — `*Timeouts` `,block` (pointer is optional) / a dataclass field;
  - `## Lists of blocks` — `[]Volume` `,block` / `list[Volume] = xcl.field(default_factory=list, block=True)`; order is kept;
  - `## Named blocks collected by name` — `map[string]Listener` `xcl:"listener,block"` with `Name string xcl:"name,label"` / `dict[str, Listener]` with `name: str = xcl.field(label=True)`, using the exact `xcl.field` label form from the release. Referenced as `app.web.listener.https.port`. Also covers a missing name and a duplicate name as errors, and the mix of named and unnamed blocks in a list;
  - `## Map attributes` — `map[string]string` / `dict[str, str]`. It says this shape is supported, but named blocks are the encouraged style;
  - `## Computed fields` — `xcl:"…,optional,computed"` (confirm the tag form in `xcl:internal/xcl/tags` at the release) / `xcl.Computed[str]`;
  - `## Sensitive fields` — `types.Sensitive[string]` / `xcl.Sensitive[str]`, linking `/sensitive-values/`;
  - `## Optional fields inside object values` — the `optional` tag honoured in an object/map attribute (language-surface Milestone 1);
  - `## The entity base` — `types.ResourceBase \`xcl:",remain"\`` / `xcl.Resource`, and shared fields through an embedded struct (from the removed README section "Defining shared fields") / dataclass inheritance.
- Raw material: `git -C xcl show bdba483^:README.md` lines 1317-1575. Do not copy its `javascript` fences: XCL blocks use the site's existing XCL fence language (check what existing pages use, e.g. `xcl-website:src/pages/examples/plugins.mdx:69`).
- **Complexity**: High
- **Token estimate**: ~60k
- **Agent strategy**: Parallel analysis (Go tags vs Python annotations), sequential integration.

### Task: References and expressions pages

- `xcl-website:src/pages/language/references.mdx` (new). Headings:
  - `## Referring to another entity` (an attribute, a whole entity, a list of entities, from the removed README lines 1464-1537);
  - `## Referring to a named block` (`app.web.listener.https.port`);
  - `## Processing order` (references and `depends_on` build a graph; a referenced entity is processed first; independent entities run concurrently; destroy runs in reverse; cycles are errors);
  - `## depends_on`.
- `xcl-website:src/pages/language/expressions.mdx` (new). Headings, each with an example:
  - `## Strings with references` (`"http://${app.web.host}"`; never call it interpolation);
  - `## Lists` and `## Maps`;
  - `## Conditionals`;
  - `## for expressions` (list and map forms, `for name, l in app.web.listener : name => l.port`);
  - `## Splat` (`app.web.volume[*].source`);
  - `## Indexing and attribute access`;
  - `## Operators`;
  - `## Templating`: `%{ if }`/`%{ for }` are rejected with an error that points to functions; use `format`, `join`, `template_file`; `%%{` writes a literal `%{`.
- **Complexity**: Medium
- **Token estimate**: ~35k
- **Agent strategy**: Single agent, sequential execution.

### Task: Variables and outputs, and modules pages

- `xcl-website:src/pages/language/variables-and-outputs.mdx` (new). Headings:
  - `## Declaring variables`;
  - `## Defaults built from other values` (`variable "url" { default = "http://${app.web.host}" }`, from the language-surface acceptance criteria). This section is the home for what locals used to do, without naming locals;
  - `## Setting variables from the application` (CodeTabs: `xcl.WithVariables(map[string]any{…})` / `xcl.Config(variables={…})`);
  - `## .vars files`;
  - `## Environment variables` (`XCL_VAR_<name>`);
  - `## Precedence` (defaults < .vars < environment < application, from `xcl:internal/parser/context.go:202-213`);
  - `## Outputs` (declare, lists and maps, `output.<name>`);
  - `## Reading outputs from the application` (CodeTabs: `xcl.Find[types.Output]` or the outputs lookup the release has / `config.outputs()`).
- `xcl-website:src/pages/language/modules.mdx` (new). Headings:
  - `## Using a module` (`source`, `variables`);
  - `## Reading a module's outputs` (`module.<name>.output.<name>`);
  - `## The module boundary` (from the removed README lines 1760-1805, error text as the release prints it);
  - `## Finding entities inside a module from the application` (CodeTabs: `xcl.Find[*Postgres](c, "module.db.database.postgres.main")` / `config.find(Postgres, "module.db.database.postgres.main")`);
  - `## Module sources`: local relative directories work. Remote sources (git, GitHub, HTTP) are not supported today (`xcl:docs/modules.md:120-126`, `xcl:internal/getter/getter.go` has no importers). Confirmed in the verification task.
- **Complexity**: Medium
- **Token estimate**: ~40k
- **Agent strategy**: Single agent, sequential execution.

### Task: Functions page

- `xcl-website:src/pages/language/functions.mdx` (new). Headings:
  - `## When functions run`;
  - `## Built-in functions`, grouped as strings, numbers, collections, encoding, dates, files and environment. It lists all 57 from `xcl:internal/functions/functions.go:430-486`, each with a one-line description and an example: abs, ceil, chomp, chunklist, coalescelist, compact, concat, contains, csvdecode, dir, distinct, element, env, file, flatten, floor, format, formatdate, formatlist, home, indent, join, jsondecode, jsonencode, keys, len, log, lower, max, merge, min, parseint, pow, range, regex, regexall, reverse, setintersection, setproduct, setsubtract, setunion, signum, slice, sort, split, strrev, substr, template_file, timeadd, title, trim, trimprefix, trimspace, trimsuffix, upper, values, zipmap. `template_file` documents its `{{ }}` (raymond) helpers `quote` and `trim` (`xcl:internal/functions/functions.go:292-295`);
  - `## Functions from your application` (CodeTabs: `local.RegisterFunction("slugify", func(s string) (string, error) {…})` / `local.register_function(slugify)`; the supported parameter and return types);
  - `## Functions from plugins` (a provider's `Functions()`, linking `/plugin-template/`);
  - `## Name clashes` (`FunctionNameClashError`: application clashes fail when the config is created; clashes involving a plugin fail when plugins load on the first operation; the error names both sources).
- **Complexity**: Medium
- **Token estimate**: ~45k
- **Agent strategy**: 2 parallel agents (built-in catalogue; registration sections), sequential integration.

### Task: Link the reference from the navigation and the home page

- `xcl-website:src/components/Nav.astro:8-33` — insert `{ label: "Language", children: [ {label:"Overview",href:"/language/"}, {label:"Entities",href:"/language/entities/"}, {label:"Attributes and blocks",href:"/language/attributes-and-blocks/"}, {label:"References",href:"/language/references/"}, {label:"Expressions",href:"/language/expressions/"}, {label:"Variables and outputs",href:"/language/variables-and-outputs/"}, {label:"Modules",href:"/language/modules/"}, {label:"Functions",href:"/language/functions/"} ] }` between `Features` and `Examples`. Check the dropdown fits at narrow widths. The existing `max-sm:hidden` rule hides only the first item.
- `xcl-website:src/pages/index.mdx:26-126` — add a "Learn the language →" `Button` or link to `/language/` at the end of "What is XCL?".
- `xcl-website:src/pages/index.mdx:232-247` — after "Two examples", add `<Section heading="Documentation">` with a list, starting with "Language reference" (`/language/`). The sibling guides spec adds "Using XCL with Go" and "Using XCL with Python" items to this same section, after the reference. If that spec landed first and the section already exists, insert the "Language reference" item as its first item instead of adding a second section.
- `xcl-website:src/pages/index.mdx:15-20` (hero) — change the `Badge` text "v0.2.2 · Initial Release" to the release the reference is verified against (the release containing language-surface and python-support, expected v0.4.0; use the tag the verification task used). In the `sub`, replace "Available for Go today, with Python, JavaScript and more to come." with a sentence saying XCL is available for Go and Python.
- `xcl-website:src/pages/index.mdx` FeatureCards "Variables and outputs", "Modules", "Functions" (around lines 150-175) — append a `[… →](/language/…/)` link, as other cards do.
- **Complexity**: Low
- **Token estimate**: ~15k
- **Agent strategy**: Single agent, sequential execution.

### Task: Point the xcl README and changelog at the reference

- `xcl:README.md:166-177` — insert `- [Language reference](https://xcl.dev/language/): files, entities, attributes and blocks, references, expressions, variables, modules and functions, with Go and Python types` as the first item of the Documentation list.
- `xcl:CHANGELOG.md:1-3` — add `## 20261010160411-19b965db-language-reference` above the newest entry. Prose: xcl.dev has a language reference, application-code examples across the site offer Go and Python tabs, and plugin pages say plugins are written in Go. Then `**Breaking:** None.` Follow the shape of the existing entries.
- **Complexity**: Low
- **Token estimate**: ~8k
- **Agent strategy**: Single agent, sequential execution.

### Task: Try every example against the release and correct the pages

- Precondition: the tagged xcl release containing both language-surface (v0.3.0) and python-support (v0.4.0) exists on the Go module proxy, and `xcl` at that version is on PyPI. If it is not, STOP and report. Do not verify against unreleased code.
- Work in a scratch directory outside every repository:
  - a Go module requiring `github.com/jumppad-labs/xcl@<tag>`;
  - a Python venv with `pip install xcl==<version>`.
  
  For each example on `xcl-website:src/pages/language/*.mdx`, write the XCL to files, build the matching Go and Python program, and run validate/apply/find as the page describes.
- For Python tabs on `xcl-website:src/pages/{registries,github-registry,events,sensitive-values,state-masking,configuration-text,diff,index,plugin-template,plugin-logging}.mdx` and `xcl-website:src/pages/examples/{configuration-only,plugins}.mdx`, run each snippet in a minimal program. Plugin-dependent snippets use the Docker example plugin release.
- Check the built-in list on the functions page against the release's `internal/functions` map, and check `.vars`/env precedence, module sources (local path; a `github.com/...` source; a `git::` source) and every "not yet supported/available" claim.
- Correct pages in place. Record each example tried, its result, and any gap to report, in the implement workflow's task notes (not in a repo file).
- Gate: `npm run build` passes after corrections.
- **Complexity**: High
- **Token estimate**: ~80k
- **Agent strategy**: Parallel analysis (one agent per reference page; one for topic-page Python tabs), sequential integration of corrections.

## Testing Strategy

- Site tasks (all `xcl-website` tasks): `npx astro check` and `npm run build` must pass at the end of each task. There is no test tooling, and none is added.
- Tabbed Go and Python code boxes: check by hand in `npm run dev`. Test switching, persistence across pages, keyboard use, no flash on load, and blocked storage (private window or storage disabled). These are captured as manual items in the test plan.
- Python-tab tasks and reference-page tasks: the examples are written from the dependency plans' APIs. Correctness against the release is established only by the verification task.
- Verification task: throwaway Go and Python programs in a scratch directory outside the repositories try every example against the release. Nothing is committed. Results go in the task notes.
- README and changelog task: reviewed by a person, and no test reads these files (`xcl:conventions/testing-and-mocking.md`).
- Success metrics and manual reviews: these are listed in plan.md § Testing Approach, each flagged "Manual — captured in the implementation test plan".

## Project References

- Spec: `20261010160411-19b965db-language-reference`.
- Dependency plans: `20261011095738-fad05e41-language-surface`, `20261011130709-aa3eb933-python-support`. Sibling spec: `20261011084948-c995ed25-using-xcl-with-go`.
- Design documents: none referenced by the spec.
- Knowledge: `xcl:conventions/testing-and-mocking.md` (no tests inspecting repo files), `xcl:glossary/entity.md`, `xcl:conventions/dependencies.md`, `xcl:conventions/project-structure.md`, `xcl-website:gotchas/no-redirects-on-page-removal.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`, xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The Python-tab tasks can run in parallel once the tab component exists. The verification task fans out one agent per reference page.

## Migration Notes

There is no data migration. No page is moved or renamed. The new `/language/` slugs are permanent, because the site has no redirects. Readers with no stored choice see Go, as today.

## Performance Considerations

The language script is a few lines of inline JavaScript in the head plus a click handler, and adds no request. CSS-driven switching avoids reflow work for each box.
