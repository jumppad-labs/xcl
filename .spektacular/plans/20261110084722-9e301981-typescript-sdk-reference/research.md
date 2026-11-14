---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Research: 20261110084722-9e301981-typescript-sdk-reference

## Alternatives considered and rejected

- **Type-check tabs by extracting fenced code from MDX at build time.** Rejected. It reads website pages and judges their text, which `conventions/testing-and-mocking.md` forbids in spirit ("No test reads ... website pages"), and it needs a parser for MDX, slots and fragments plus a way of writing hidden preambles. Checked source files need none of that: the compiler reads real `.ts` files. Trial in a scratch copy of the site: a `TsExample` component reading the file through `import.meta.glob` and rendering it with `Code` from `astro-expressive-code/components` builds inside a `CodeTabs` slot and yields the same `.expressive-code` frame as a fence.
- **Compile the examples inside the site's own `npm run build` or `deploy.yml`.** Rejected. The spec requires that a release which breaks an example still updates the reference, and the bump gates on the build. Compile checks therefore run in a separate workflow.
- **Add `xcl-lang` to the site's `package.json`.** Rejected. Every release would have to edit the site's lock file, and `npm ci` in the deployment would download eight engine packages. The example project installs the pinned version itself with `--omit=optional`.
- **`typedoc-plugin-markdown` feeding MDX** (spec's named alternative). Rejected. It would generate page source rather than data, so it cannot share the neutral JSON shape and page templates the Python reference introduces, and text would pass through MDX's parser.
- **TypeDoc's built-in `validation.notDocumented` as the doc comment check.** Rejected as the only check. Probe output names object-literal members as `Sensitive.__type.redacted`, reports an inherited undocumented member twice (`XclError.noDocs`, `ConfigError.noDocs`), and is a second definition of "public" beside the walk that writes the data. One walk does both, as in the Python tool.
- **`Type.toString()` for signature text.** Rejected. Probe: `new () => T` prints as `() => T`, and the `Sensitive` const's construct signature loses `new`. `checker.signatureToString` in `Converter.EVENT_CREATE_SIGNATURE` prints them exactly.
- **Hard build failure for a reference link to a missing name.** Rejected, and `PyRef` in the Python plan now matches. An API rename in a release would fail the bump's build and stop the reference updating, against "A release that breaks an example still updates the reference". `TsRef` renders plain code and warns unless `XCL_REFERENCE_STRICT=1`, which only the site's check workflow sets.
- **One source file per page with named regions, for tabs.** Rejected for tabs: names in one scope clash across regions. One file per tab, with named regions used only for the guide's single program.
- **A generated TypeDoc HTML site.** Rejected by the spec's own constraints (inside the site's layout).

## Chosen approach — evidence

- `xcl-website:src/components/CodeTabs.astro:37-40` — the `languages` list and two hard-coded panels; a third is a local change, and the panel rules at `:75-82` are per-language.
- `xcl-website:src/components/Head.astro` — pre-paint accepts only `python` today.
- `xcl-website:src/layouts/Shell.astro` — `current()` returns only `python` or `go`, so it must learn `typescript` or the selected tab desynchronises.
- `xcl-website:.github/workflows/deploy.yml` — runs `npm run build`, so a `prebuild` fetch needs no change here.
- Scratch trial: `Code` inside a `CodeTabs` slot builds; `astro check` fails on a `.ts` example importing an absent `xcl-lang` until `tsconfig.json` has `"exclude": ["typescript-examples", "dist"]`, after which it passes.
- TypeDoc 0.28.20 with TypeScript 5.9.3 probe on a small package: `excludeInternal`, `excludeProtected`, `excludePrivate`, `excludeExternals` give the spec's public rule; inherited members carry `inheritedFrom`; `Error`'s own members are gone while the inherited constructor stays flagged `isExternal`; overloads are separate signatures with comments; a first overload's comment is copied to later undocumented overloads; `export * as events` is a Namespace reflection; `{@link X}` keeps a `target`; `@throws {X}` loses X; a value and a type of one name are two reflections.
- `xcl-typescript` is empty today; every source fact about it comes from the core and printing plans.
- Tab inventory (Python panels): 107 boxes on 19 pages. Files have `index.mdx` 3, `configuration-text` 18, `diff` 8, `events` 7, `examples/configuration-only` 8, `examples/plugins` 12, `github-registry` 6, `language/attributes-and-blocks` 11, `language/entities` 4, `language/functions` 2, `language/modules` 1, `language/references` 1, `language/variables-and-outputs` 2, `plugin-logging` 2, `plugin-template` 3, `problems` 5, `registries` 9, `sensitive-values` 3, `state-masking` 2.
- Known boxes with no TypeScript equivalent: `configuration-text.mdx` box 17 (`saved_entity_tokens`; printing plan Out of Scope excludes public `savedEntityTokens`), `examples/plugins.mdx` box 1 (in-process plugin), `registries.mdx` boxes 7 and 8 (custom registry), `state-masking.mdx` box 1 (custom masker), and `configuration-only.mdx` box 2's `Decode` (the program fills its own class instead, as Python's does).

## Files examined

- `xcl-website:src/components/CodeTabs.astro:1-40` — slot form, language list, per-language CSS.
- `xcl-website:src/components/Head.astro` — stored-preference pre-paint, fallback description says "Go implementation available now".
- `xcl-website:src/layouts/Shell.astro` — tab handler and `current()`.
- `xcl-website:src/components/Nav.astro:9-48` — dropdown items, Guides first-child rule.
- `xcl-website:src/pages/index.mdx:4,20,22,30-55,219,357` — language lists, install box.
- `xcl-website:src/pages/plugin-logging.mdx:27`, `replacement.mdx:24`, `plugin-template.mdx:27`, `configuration-text.mdx:24`, `examples/plugins.mdx:66-67`, `language/index.mdx:59`, `language/entities.mdx:276`, `language/expressions.mdx:57` — sentences naming the languages.
- `xcl-website:Makefile`, `package.json`, `tsconfig.json`, `.gitignore`, `README.md`, `astro.config.mjs` — build and check surface.
- `xcl-website:.github/workflows/deploy.yml` — deployment, unchanged.
- `xcl-website:node_modules/astro-expressive-code/package.json` — 0.43.1 exports `./components`.
- `xcl:README.md:215-225,241-247` — documentation list and "Other languages".

## External references

- TypeDoc 0.28.20 on npm; peer range TypeScript 5.0 to 6.0 — the version pinned in the plan.
- TypeDoc options `excludeInternal`, `excludeProtected`, `excludePrivate`, `excludeExternals`, `skipErrorChecking`, and event `Converter.EVENT_CREATE_SIGNATURE` — verified by the probe.
- markdown-it — renders doc comment Markdown to HTML; already a TypeDoc dependency, pinned directly.

## Prior plans / specs consulted

- `20261110084343-9ebf34a1-python-sdk-reference` plan and context — reference data shape, pin, fetch, page, `PyRef`, bump workflow, nav, guide order, human tasks. Reused rather than reinvented.
- `20261031080136-264e0eab-typescript-core-sdk` plan and context — public surface, `.github/workflows/test.yml`, `release.yml` jobs, tag equals pin, `result` job, Node 22 test runner, release bot secrets on `xcl-typescript`.
- `20261110083940-53be9068-typescript-printing` plan — printing and `prettylog` exports, namespaces, `Formatter` is a function type, no public `savedEntityTokens`.
- Design `typescript-sdk.md` (source `design`) read for API names used by the guide and tabs; the spec itself references no design.

## Open assumptions

- The Python reference is delivered on `xcl-website` first, so `Nav.astro`'s Reference menu, `Entry.astro`, the fetch script and `reference-bump.yml` exist. If not, STOP and ask.
- The core and printing plans deliver the names they list; the doc comment and tab tasks read the real exports. A name the plans list but the package lacks is a STOP.
- `npm install --no-save --omit=optional` installs a type-complete `xcl-lang` with no engine packages.
- `xcl-plugin-docker` is still the right plugin for the guide at implementation; the task checks its latest release.

## Drafting assumptions

### Chosen direction (architecture)
- **Decision**: One TypeDoc-based tool in `xcl-typescript` writes neutral reference data and is the doc comment check; the site reuses the Python reference's pin, fetch, page and bump pattern; TypeScript tabs are real files in `typescript-examples/` compiled by a separate workflow and shown through `TsExample`.
- **Rationale**: Keeps one definition of "public", reuses what Python adds, keeps the deployment unchanged, respects the rule against tests reading pages, and lets a release update the reference even when it breaks an example.
- **Rejected**: extracting code from MDX; compiling examples in the build; `typedoc-plugin-markdown`; `xcl-lang` as a site dependency.

### TsRef is lenient outside the check workflow (architecture)
- **Decision**: An unknown name in `TsRef` renders plain code with a build warning, and fails only under `XCL_REFERENCE_STRICT=1`, which the examples workflow sets.
- **Rationale**: The acceptance criterion says a release that breaks an example still updates the reference, and an API rename would otherwise fail the bump's build through the guide's links.
- **Rejected**: Python's hard failure; it would hold the reference back on a rename.

### Failing check is the report (architecture)
- **Decision**: A broken example is reported by a failed `typescript-examples.yml` run on `main`; no issue is opened automatically.
- **Rationale**: The spec says it "fails the site's checks" and "is reported separately"; a failed run is both, and the site has no other reporting.
- **Rejected**: opening a GitHub issue from the workflow, which adds permissions and noise; left as a review point because a red run on a bot's commit is easy to miss.

### Python reference delivered first (architecture)
- **Decision**: The website tasks assume the Python reference is on `xcl-website` first.
- **Rationale**: The spec shares its Reference menu and templates "if the Python reference is already there", and the epic lists Python before TypeScript.
- **Rejected**: building those parts here too, which would duplicate or collide with the Python plan. The epic's `depends_on` does not state this order, and the hand-back flags it.

### What counts as a member for the check (architecture)
- **Decision**: Interface and object-type members are checked like class members, since `Config` is exported as a type. Enum members are listed but need no doc comment. A class's doc comment documents its constructor. Standard library members a class inherits are left out.
- **Rationale**: `Config` is the main API; enum members and constructors follow the Python reference's rules.
- **Rejected**: checking classes only, which would leave `Config` unchecked; requiring comments on every enum member, which the spec does not ask.

### Conventions selected (architecture)
- **Decision**: Kept the testing, dependency, release plan, config-files, no-compatibility and worktree conventions; dropped Go-specific ones.
- **Rationale**: The work is TypeScript, CI and documentation.
- **Rejected**: the Go style, parser and state conventions, which do not bear on this work.

### Share the Python page template (components)
- **Decision**: Extract one reference page template used by both languages, and extend the entry component.
- **Rationale**: The Python plan designed its data to be reused; a copy would drift.
- **Rejected**: a separate TypeScript page, which would duplicate layout and index code.

### Additive schema for overloads (data_structures)
- **Decision**: Add an optional `overloads` list and more `kind` values, keeping `schema: 1`.
- **Rationale**: Python's data needs no change and the fields are optional. TypeScript's overloads and merged names need a place for each form's signature.
- **Rejected**: one entry per overload, which would need several anchors for one name.

### Anchors use the import name (data_structures)
- **Decision**: Full dotted names start with `xcl-lang`, for example `xcl-lang.events.toLogger`.
- **Rationale**: That is what a reader writes in `import`, as Python's anchors use the import name `xcl`.
- **Rejected**: starting at the export name, which drops the agreed "full dotted name" form.

### Examples are compiled, not run (testing_approach)
- **Decision**: Tabs are type-checked only; only the guide is run, by hand.
- **Rationale**: The acceptance criterion asks for compilation, and running 107 fragments would need an engine and plugins for each.
- **Rejected**: running every tab.

### Tab tasks grouped by subject (tasks)
- **Decision**: The 107 boxes are split into five tasks by page subject, and the tab itself arrives in a later task.
- **Rationale**: Each group is one reader subject and a sensible agent unit, and the build stays green throughout, since Astro ignores a slot the component does not name.
- **Rejected**: one task per page, 19 tasks with little independent value.

### First pin is a human task (tasks)
- **Decision**: A person sets the first TypeScript pin and merges the site.
- **Rationale**: Same as the Python reference: the site must name a release that carries the asset, and that release happens outside the repository.
- **Rejected**: letting the first release's dispatch set it, because the site needs its other pages merged first.

## Rehydration cues

- `spektacular plan file read 20261110084343-9ebf34a1-python-sdk-reference plan` and `context`.
- `spektacular plan file read 20261031080136-264e0eab-typescript-core-sdk plan` and `context`.
- `spektacular plan file read 20261110083940-53be9068-typescript-printing plan`.
- `spektacular design read --data '{"source":"design","path":"typescript-sdk.md"}'`.
- Knowledge: `xcl-typescript` `gotchas/typedoc-throws-and-signature-text.md`, `xcl-website` `gotchas/guides-dropdown-label-opens-first-child.md`, `xcl` `architecture/release-chain.md`.
- Read `xcl-website:src/components/CodeTabs.astro`, `Head.astro`, `layouts/Shell.astro`.
