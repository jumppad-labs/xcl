---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Context: 20261110084722-9e301981-typescript-sdk-reference

## Current State Analysis

`xcl-typescript` holds only a LICENSE, a short README and its `.spektacular` footprint; its source, CI and release workflows arrive with the core SDK and printing specs, which must land first, so every path in it below comes from those plans. `xcl-website` is an Astro site that deploys on every push to `main`. Its `CodeTabs` component offers Go and Python in 107 boxes on 19 pages, and its preference is stored in `xcl-lang`. The Python reference plan adds `/reference/python/`, the Reference menu, the entry component, the fetch step and `reference-bump.yml`; this plan assumes those are delivered, and they are not on `main` yet, so no line numbers are given for them. The `xcl` README links neither the guide nor the reference.

Requirement to repo: reference, doc comments, CI and release in `xcl-typescript`; page, pin, bump, tabs, preference, guide, navigation and home page in `xcl-website`; the README in `xcl`.

## Per-Task Technical Notes

### Task: Add the API reference tool

**File changes**:
- `xcl-typescript:.github/scripts/api-reference.mjs` (new) — `Application.bootstrap({entryPoints: ["src/index.ts"], tsconfig, excludeInternal: true, excludeProtected: true, excludePrivate: true, excludeExternals: true, skipErrorChecking: true, logLevel: "Error"})`, with `--source` as its base. `app.converter.on(Converter.EVENT_CREATE_SIGNATURE, ...)` records `context.checker.signatureToString(signature, declaration, ts.TypeFormatFlags.NoTruncation, kind)` per signature, and a property's or constant's type comes from `checker.typeToString`; never `Type.toString()`, which loses `new` (`xcl-typescript` gotcha `typedoc-throws-and-signature-text`). Public means `project.children` and namespace children. Reflections of one name merge into one entry, the type's kind winning, with each form in `overloads`. Members are `children`, plus `signatures` and `children` of a variable whose type is an object type. Rules for the check: an entry needs a comment on the declaration, a signature, or a get or set signature; a constructor is documented by its own comment or its class's; enum members are not required; a member with `inheritedFrom` is reported only on its defining class; a member flagged `isExternal`, other than a constructor, is omitted; a `@throws` whose content has no `{@link}` is an error. Sections: one per namespace, the rest by defining source file: `config.ts` Configuration, `registry.ts` Registries, `decorators.ts` and `sensitive.ts` Entity types, `hydrate.ts` Entities, `diff.ts` Diffs, `events.ts` and `interrupt.ts` Events and cancellation, `print.ts` and `highlight/` Printing and formats, `mask.ts` State masks, `errors.ts` Errors, otherwise Other. Prose: `{@link}` becomes a Markdown link to the target entry's id when it resolves and code when it does not, `@example` and `@remarks` are appended, then rendered with markdown-it (`html: false`). Parameters, `default`, `returns` and `raises` per signature. `id` is `xcl-lang.` plus the dotted path. `version` is `package.json`'s `version`.
- `xcl-typescript:.github/scripts/api-reference.test.mjs` (new) — `node:test` against small packages written to a temporary directory: one undocumented method fails and names it; a documented package passes; an inherited undocumented member is reported once, on its defining class; a non-exported name is ignored; `@internal` and protected members are ignored; a `@throws` with no link fails; a value and a type of one name are one entry; two overloads are both listed; a constructor type keeps `new`; the data has the version, entries and members. One behaviour per test, positive and negative apart.
- `xcl-typescript:package.json` — dev dependencies `typedoc` `0.28.20` and `markdown-it`, exact versions. Record the reason in the README's development section, per the dependencies convention.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Complete the package's doc comments

**File changes**:
- `xcl-typescript:src/index.ts` — doc comments on `export * as events` and each other namespace export.
- `xcl-typescript:src/config.ts`, `xcl-typescript:src/registry.ts`, `xcl-typescript:src/decorators.ts`, `xcl-typescript:src/sensitive.ts`, `xcl-typescript:src/hydrate.ts`, `xcl-typescript:src/diff.ts`, `xcl-typescript:src/events.ts`, `xcl-typescript:src/errors.ts`, `xcl-typescript:src/mask.ts`, `xcl-typescript:src/interrupt.ts`, `xcl-typescript:src/print.ts`, `xcl-typescript:src/prettylog.ts`, `xcl-typescript:src/highlight/` — `@param`, `@returns` and `@throws {@link X}` where they apply. Run the tool until it reports nothing. Both the value and the type of `Sensitive`, `Meta` and `Entity` need a comment.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Run the doc comment check in CI

**File changes**:
- `xcl-typescript:.github/workflows/test.yml` — new `check-docs` job: setup-node, `npm ci`, `node .github/scripts/api-reference.mjs --check`, `node --test .github/scripts/api-reference.test.mjs`; add to the `result` job's `needs`.
- `xcl-typescript:Makefile` — `check-docs` target; add to `.PHONY`.

**Complexity**: Low
**Token estimate**: ~4k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Publish the reference with each release

**File changes**:
- `xcl-typescript:.github/workflows/release.yml` — new `reference` job after `settings`: check out the tag; setup-node; `npm ci`; `npm pkg set version="$VERSION"`; `node .github/scripts/api-reference.mjs --check --out "xcl-lang-$VERSION-reference.json"`; upload artifact `reference`.
- `xcl-typescript:.github/workflows/release.yml` — add `reference` to `publish.needs`.
- `xcl-typescript:.github/workflows/release.yml` — `github-release` needs `reference`, downloads the artifact, and runs `gh release upload "$TAG" xcl-lang-*-reference.json`.
- `xcl-typescript:.github/workflows/release.yml` — new `dispatch-website` job after `github-release`, using `actions/create-github-app-token@v1` with `owner` and `repositories: xcl-website` and secrets `RELEASE_APP_ID`, `RELEASE_APP_PRIVATE_KEY`, running `gh api repos/jumppad-labs/xcl-website/dispatches -f event_type=xcl-typescript-release -f 'client_payload[tag]='"$TAG"`.

**Complexity**: Medium
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Load the TypeScript reference data into the site

**File changes**:
- `xcl-website:xcl-typescript-ref.txt` (new) — the pinned tag. It holds the current xcl tag until the first release with an asset is known; the human "Set the site's first TypeScript pin" task changes it.
- `xcl-website:scripts/fetch-python-reference.mjs` — rename to `xcl-website:scripts/fetch-reference.mjs` and run once per language: for `typescript`, pin `xcl-typescript-ref.txt`, repository `xcl-typescript`, override `XCL_TYPESCRIPT_REFERENCE`, output `src/generated/typescript-reference.json`. The asset name `xcl-lang-<version>-reference.json` is the same in both repositories.
- `xcl-website:package.json` — the `reference` script runs the renamed file.
- `xcl-website:README.md` — document the pin and override.

Build locally with `XCL_TYPESCRIPT_REFERENCE` pointing at data from the tool.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Render the TypeScript reference page

**File changes**:
- `xcl-website:src/components/reference/ReferencePage.astro` (new) — the layout and index extracted from `xcl-website:src/pages/reference/python.astro`, taking the data and a title.
- `xcl-website:src/pages/reference/python.astro` — renders `ReferencePage` with its data.
- `xcl-website:src/pages/reference/typescript.astro` (new) — renders `ReferencePage` with the TypeScript data.
- `xcl-website:src/lib/typescriptReference.ts` (new) — imports the JSON; exports `hasEntry(id)`.
- `xcl-website:src/components/reference/Entry.astro` — shows `overloads` when present, and the new kinds as badges.

**Complexity**: Medium
**Token estimate**: ~12k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Add TypeScript to the Reference menu and TsRef links

**File changes**:
- `xcl-website:src/components/Nav.astro` — `{ label: "TypeScript", href: "/reference/typescript/" }` after the Python child in Reference.
- `xcl-website:src/components/TsRef.astro` (new) — `hasEntry` false: `console.warn` and plain `<code>`, or throw when `process.env.XCL_REFERENCE_STRICT === "1"`; otherwise `<a href="/reference/typescript/#<name>"><code>…</code></a>`.

**Complexity**: Low
**Token estimate**: ~3k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Extend the pin bump workflow to TypeScript

**File changes**:
- `xcl-website:.github/workflows/reference-bump.yml` — `repository_dispatch` types gain `xcl-typescript-release`; `workflow_dispatch` gains a `package` input of `python` or `typescript`; a step chooses `xcl-python-ref.txt` or `xcl-typescript-ref.txt` from the event, and the later steps and commit message use it. The build gate, concurrency group and App token are unchanged.

**Complexity**: Low
**Token estimate**: ~4k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Add the checked example project

**File changes**:
- `xcl-website:typescript-examples/package.json` (new) — private, `type: module`, exact `typescript` and `@types/node`. No `xcl-lang`.
- `xcl-website:typescript-examples/tsconfig.json` (new) — `strict`, `noEmit`, `target: ES2022`, `module: NodeNext`, `lib` with `esnext.disposable`, standard decorators.
- `xcl-website:scripts/check-typescript-examples.mjs` (new) — reads the pin, runs `npm install --no-save --no-package-lock --omit=optional xcl-lang@<version>` in the project (or the tarball in `XCL_TYPESCRIPT_PACKAGE`), then `tsc -p`.
- `xcl-website:src/components/TsExample.astro` (new) — `import.meta.glob("/typescript-examples/**/*.ts", { query: "?raw", import: "default", eager: true })`, region selection, removes marker lines, dedents, renders `Code` from `astro-expressive-code/components`. Throws, naming the page, for a missing file or region. Trialled in a copy of the site.
- `xcl-website:tsconfig.json` — add `"exclude": ["typescript-examples", "dist"]`, or `astro check` reads the examples and fails.
- `xcl-website:.gitignore` — `typescript-examples/node_modules/`.
- `xcl-website:package.json` — script `check:typescript`.
- `xcl-website:Makefile` — `check` also runs `npm run check:typescript`.
- `xcl-website:README.md` — how examples work and the override.

Test locally with a tarball from `npm pack` in an `xcl-typescript` checkout. Keep examples as real programs: everything a fragment needs goes outside its region.

**Complexity**: Medium
**Token estimate**: ~8k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Add the example check workflow

**File changes**:
- `xcl-website:.github/workflows/typescript-examples.yml` (new) — on `pull_request` to `main`, `push` to `main`, `workflow_dispatch`: checkout, setup-node 22 with npm cache, `npm ci`, `npm run check:typescript`, `XCL_REFERENCE_STRICT=1 npm run build`. Read-only permissions.

**Complexity**: Low
**Token estimate**: ~3k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Add TypeScript tabs to the language pages

**File changes**:
Add `<div slot="typescript">` after each `slot="python"`, with the code as a `typescript-examples/<page>/<name>.ts` file and a `TsExample` (import it at the top of the page). Write the prose beside the code for TypeScript, using the released API names, as the Python panel does for Python.

- `xcl-website:src/pages/language/attributes-and-blocks.mdx` — 11 boxes.
- `xcl-website:src/pages/language/entities.mdx` — 4 boxes.
- `xcl-website:src/pages/language/functions.mdx` — 2 boxes.
- `xcl-website:src/pages/language/modules.mdx` — 1 box.
- `xcl-website:src/pages/language/references.mdx` — 1 box.
- `xcl-website:src/pages/language/variables-and-outputs.mdx` — 2 boxes.
- `xcl-website:typescript-examples/language/` (new) — one file per box.

**Complexity**: High
**Token estimate**: ~35k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Add TypeScript tabs to the operations guides

**File changes**:
Add `<div slot="typescript">` after each `slot="python"`, with the code as a `typescript-examples/<page>/<name>.ts` file and a `TsExample` (import it at the top of the page). Write the prose beside the code for TypeScript, using the released API names, as the Python panel does for Python.

- `xcl-website:src/pages/events.mdx` — 7 boxes. Box 0: `events.toLogger` where Python uses its logging handler.
- `xcl-website:src/pages/diff.mdx` — 8 boxes.
- `xcl-website:src/pages/problems.mdx` — 5 boxes.
- `xcl-website:src/pages/sensitive-values.mdx` — 3 boxes.
- `xcl-website:src/pages/state-masking.mdx` — 2 boxes. Box 1: no custom masker in TypeScript; state so.
- `xcl-website:src/pages/plugin-logging.mdx` — 2 boxes.
- `xcl-website:src/pages/plugin-template.mdx` — 3 boxes.
- `xcl-website:typescript-examples/` (new per-page directories) — one file per code box.

**Complexity**: High
**Token estimate**: ~40k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Add TypeScript tabs to the registry pages

**File changes**:
Add `<div slot="typescript">` after each `slot="python"`, with the code as a `typescript-examples/<page>/<name>.ts` file and a `TsExample` (import it at the top of the page). Write the prose beside the code for TypeScript, using the released API names, as the Python panel does for Python.

- `xcl-website:src/pages/registries.mdx` — 9 boxes. Boxes 7 and 8: no custom registry in TypeScript; state so.
- `xcl-website:src/pages/github-registry.mdx` — 6 boxes.
- `xcl-website:typescript-examples/registries/`, `xcl-website:typescript-examples/github-registry/` (new) — one file per code box.

**Complexity**: Medium
**Token estimate**: ~20k tokens
**Agent strategy**: 2-3 parallel agents for independent changes

### Task: Add TypeScript tabs to the configuration text page

**File changes**:
Add `<div slot="typescript">` after each `slot="python"`, with the code as a `typescript-examples/<page>/<name>.ts` file and a `TsExample` (import it at the top of the page). Write the prose beside the code for TypeScript, using the released API names, as the Python panel does for Python.

- `xcl-website:src/pages/configuration-text.mdx` — 18 boxes. Box 15 (Jinja) becomes a server template. Box 16: a format is a function of the `Formatter` type. Box 17: no public `savedEntityTokens`; state so.
- `xcl-website:typescript-examples/configuration-text/` (new) — one file per code box.

**Complexity**: High
**Token estimate**: ~30k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Add TypeScript tabs to the example programs and home page

**File changes**:
Add `<div slot="typescript">` after each `slot="python"`, with the code as a `typescript-examples/<page>/<name>.ts` file and a `TsExample` (import it at the top of the page). Write the prose beside the code for TypeScript, using the released API names, as the Python panel does for Python.

- `xcl-website:src/pages/examples/configuration-only.mdx` — 8 boxes. Box 2: `Decode` has no TypeScript form; fill a class by lookups, as the Python panel does.
- `xcl-website:src/pages/examples/plugins.mdx` — 12 boxes. Box 1: no in-process plugin in TypeScript; state so.
- `xcl-website:src/pages/index.mdx` — 3 boxes. Box 0 is an `sh` fence with `npm install xcl-lang`, kept inline.
- `xcl-website:typescript-examples/examples/`, `xcl-website:typescript-examples/index/` (new) — one file per code box.

**Complexity**: High
**Token estimate**: ~40k tokens
**Agent strategy**: Parallel analysis, sequential integration

### Task: Offer TypeScript in every code box

**File changes**:
- `xcl-website:src/components/CodeTabs.astro:37-40` — add `{ key: "typescript", label: "TypeScript" }`; update the header comment.
- `xcl-website:src/components/CodeTabs.astro:66-73` — add the third panel after Python's.
- `xcl-website:src/components/CodeTabs.astro:78-83` — hide the `typescript` panel unless `html[data-lang="typescript"]`, and hide `go` when it is.
- `xcl-website:src/components/CodeTabs.astro:135-136` — the lit tab rule covers `typescript`, and Go's excludes both other values.
- `xcl-website:src/components/CodeTabs.astro` — throw naming `Astro.url.pathname` when `Astro.slots.has("typescript")` is false.
- `xcl-website:src/components/Head.astro` — accept `python` or `typescript` as the stored value.
- `xcl-website:src/layouts/Shell.astro` — `current()` returns `python` or `typescript` when set, otherwise `go`.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Name TypeScript across the site

**File changes**:
- `xcl-website:src/pages/index.mdx:4,20,22,219,357` — description, hero, badge and two sentences.
- `xcl-website:src/components/Head.astro:9` — the fallback description.
- `xcl-website:src/pages/plugin-logging.mdx:27`, `xcl-website:src/pages/replacement.mdx:24`, `xcl-website:src/pages/plugin-template.mdx:27`, `xcl-website:src/pages/examples/plugins.mdx:66-67`, `xcl-website:src/pages/configuration-text.mdx:24`, `xcl-website:src/pages/language/index.mdx:59`, `xcl-website:src/pages/language/entities.mdx:276`, `xcl-website:src/pages/language/expressions.mdx:57` — add TypeScript, naming its form where the sentence gives one (for example `Record<string, string>`). Re-run a search for "Go and Python" and "Go or Python" before finishing.

**Complexity**: Low
**Token estimate**: ~4k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Write the TypeScript guide

**File changes**:
- `xcl-website:src/pages/using-xcl-with-typescript.mdx` (new) — `Shell`, `Hero`, `Prose`, `CtaBanner`, no `CodeTabs`. One program by section: `npm install xcl-lang` and the TypeScript setup for decorators and `await using`; `@Entity` class and `@Func`; the signed Docker plugin on `GitHubRegistry` (check the latest `xcl-plugin-docker` release installs against the latest package, key from `/keys/jumppad-labs-releases.asc`, note the Docker requirement); `new Config` with `statePath`, and a short note on the optional `setup()` and why a program might call it; `apply`, and a second run with `load`; `find` and `findByType`; `ConfigError` with `printError`; `AbortController` and `interruptSignal`; `prettylog.handler` and `events.toLogger`; `printEntity`; `destroy`; the complete listing. Links: `/registries/`, `/github-registry/`, `/plugin-template/`, `/state-masking/`, `/events/`. Every API name is a `TsRef`. TypeScript providers are not built, so mark them not yet supported if mentioned. Follow `decisions/xcl-is-its-own-language.md`.
- `xcl-website:typescript-examples/guide/program.ts` (new) — the whole program, with a named region per section. Run it against npm's latest in an empty directory before finishing, with Docker running.

**Complexity**: Medium
**Token estimate**: ~22k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Link the guide from the site

**File changes**:
- `xcl-website:src/components/Nav.astro` — the guide after the Python guide under Guides, so the first child stays "Events and logging".
- `xcl-website:src/pages/index.mdx:30-55` — beside the Python line after the Install `</CodeTabs>`, a line linking the guide and reference.
- `xcl-website:README.md` — add both pages to the Pages table.

**Complexity**: Low
**Token estimate**: ~3k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Update the package README's links

**File changes**:
- `xcl-typescript:README.md` — links near the top, and one sentence in the release or development section that a release also updates the reference.

**Complexity**: Low
**Token estimate**: ~2k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Link the guide and reference from the xcl README

**File changes**:
- `xcl:README.md:215-225` — add the guide and reference to the documentation list; "with Go and Python types" names TypeScript.
- `xcl:README.md:241-247` — link them in "Other languages".

**Complexity**: Low
**Token estimate**: ~2k tokens
**Agent strategy**: Single agent, sequential execution

### Task: Release the package with the reference asset

A person merges the `xcl-typescript` work. The package releases when a pull request labelled `patch`, `minor` or `major` merges (xcl's shared auto-tag), as xcl-python's does, so merging the work with a release label releases it with no xcl release. Checked by the release's jobs all passing, including the install checks, and the GitHub release carrying `xcl-lang-<version>-reference.json`.

### Task: Set the site's first TypeScript pin

A person writes the released tag into `xcl-website:xcl-typescript-ref.txt` on the website's branch, checks that a build and the example check pass, and merges so the deployment runs. Merge only after the release is on npm, per the release chain.

## Testing Strategy

- The API reference tool has unit tests beside it, against temporary fixture packages, one behaviour per test function, positive and negative separate. They and the tool over the real package run in the `check-docs` job.
- The example check compiles every tab against the pinned release, on pull requests and pushes to `main`.
- The site's guards are exercised by building: missing data, wrong version, missing example file or region, missing TypeScript panel, and an unknown `TsRef` name under strict mode.
- The guide, the workflows, the release path and the tab behaviour are checked by hand from the implementation test plan, per the manual items in the plan's Testing Approach.

## Project References

- Spec: `20261110084722-9e301981-typescript-sdk-reference`, epic `20261031080131-8c2a58ec-typescript-sdk`; builds on `20261031080136-264e0eab-typescript-core-sdk`, `20261110083940-53be9068-typescript-printing` and `20261110084343-9ebf34a1-python-sdk-reference`.
- Design documents: none referenced by the spec; `typescript-sdk.md` (source `design`) read for API names.
- Knowledge: `architecture/release-chain.md`, `decisions/xcl-is-its-own-language.md`, `conventions/testing-and-mocking.md`, `conventions/dependencies.md`, `conventions/release-plan-after-implementation.md`, `conventions/config-files-are-xcl.md` (xcl); `gotchas/guides-dropdown-label-opens-first-child.md` (xcl-website); `gotchas/typedoc-throws-and-signature-text.md` (xcl-typescript).
- Repo roots: `xcl-typescript` /home/nicj/code/github.com/jumppad-labs/xcl-typescript; `xcl-website` /home/nicj/code/github.com/jumppad-labs/xcl-website; `xcl` /home/nicj/code/github.com/jumppad-labs/xcl.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

No code migrations. `scripts/fetch-python-reference.mjs` is renamed when it gains TypeScript. Merge order: `xcl-typescript` first (released), then the first pin and the website.

## Performance Considerations

The reference is one page, built at site build time, and the fetch adds one download. The example check's install and compile run outside the deployment, so it does not lengthen a deploy.
