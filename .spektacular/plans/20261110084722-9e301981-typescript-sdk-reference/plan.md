---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Plan: 20261110084722-9e301981-typescript-sdk-reference

<!-- Metadata -->
<!-- Created: 2026-11-10T09:29:55Z -->
<!-- Commit: 788959a6ecad23b6d101f55f74fb793d6ca9cbad -->
<!-- Branch: f-python-plugin -->
<!-- Repository: jumppad-labs/xcl -->

## Overview

xcl's TypeScript developers get what Python and Go developers have on xcl.dev: a complete reference for the `xcl-lang` package at `/reference/typescript/`, generated from the package at each release so it stays accurate, a guide from an empty program to a working one, and TypeScript beside Go and Python in every multi-language example on the site. Today the package has no reference or guide, and the site's 107 code boxes offer only Go and Python. It serves anyone adopting xcl from TypeScript, who can look up any name, follow one path from start to finish, and read the site's examples in their own language, knowing those examples compile against the released package.

## Conventions

- **Never write tests that inspect repository files (`testing-and-mocking`)** — decides the example check: `tsc` compiles real source files rather than a test reading pages. The doc comment check is a CI lint, and the tool's tests run against fixture packages in a temporary directory. Guide wording and links are checked by people.
- **Never table-driven; positive and negative cases in separate tests; verbose over abstract (`testing-and-mocking`)** — applies to the reference tool's tests.
- **Prefer the standard library, document and pin third-party dependencies (`dependencies`)** — TypeDoc and markdown-it are exact-pinned dev dependencies with their reason recorded; the example project's TypeScript and `@types/node` are pinned too.
- **Write a release plan once released work is merged (`release-plan-after-implementation`)** — this spec crosses an `xcl-typescript` release and a site deploy in a required order.
- **Configuration and metadata files are written in xcl (`config-files-are-xcl`)** — considered and not applied, as in the Python reference: reference data is a documentation payload, the pin follows the existing `xcl-ref.txt`, and `package.json` and `tsconfig.json` are tool-owned.
- **Breaking changes are allowed until xcl is stable (`no-compatibility-before-stable`)** — no redirects or aliases are kept for renamed reference entries, and anchors follow the current names.
- **Spec worktrees belong to the agent (`spec-worktrees-belong-to-the-agent`)** — work is delivered to the branches of `xcl-typescript`, `xcl-website` and `xcl`.

## Architecture & Design Decisions

One tool in `xcl-typescript`, `.github/scripts/api-reference.mjs`, is the only place that knows what "public" means for the TypeScript package. It drives TypeDoc over `src/index.ts`, with internal, protected, private and external members excluded, and walks the result once to do two jobs: fail naming every public name or member with no doc comment, and write the API as data. The data has the shape the Python reference introduced (`schema`, `language`, `package`, `version`, `sections` of entries), delivered as the release asset `xcl-lang-<version>-reference.json`, so the site renders it through the page and entry components that already exist. TypeDoc and markdown-it are exact-pinned dev dependencies, so nothing reaches `xcl-lang`'s runtime dependencies. Doc comments follow TSDoc, and the check also refuses a `@throws` with no `{@link}`, because TypeDoc silently drops a `{Type}`. Signature text comes from the compiler rather than TypeDoc's type printer, which loses `new`. Names that are both a value and a type (`Sensitive`, `Meta`, `Entity`) become one entry and one anchor, and overloads become an `overloads` list on their entry.

The site follows the Python reference's pattern with TypeScript as a sibling: a one-line pin `xcl-typescript-ref.txt`, the existing fetch step extended to fetch both assets before every build, `/reference/typescript/` beside `/reference/python/`, a `TypeScript` child in the Reference menu, and anchors using the full dotted name, here `xcl-lang.Config.apply`. The xcl-typescript release generates the data, stops before npm if generation fails, attaches the asset, and dispatches `xcl-typescript-release`. `reference-bump.yml` learns that event, moves the pin, builds as a gate and pushes to `main` as the release bot, so the existing deployment runs unchanged.

Examples are checked by compiling real files rather than by reading pages, which the project's testing rule forbids. Each TypeScript tab's code lives in a file under `typescript-examples/` in the website repository, a strict TypeScript project of its own, and a page shows it through a `TsExample` component that renders the file, or a named region of it, through Expressive Code. A fragment is simply a region of a complete program, so the surrounding code that makes it compile is real code that the page does not show. A separate `typescript-examples.yml` workflow installs the pinned `xcl-lang` and runs `tsc` on pull requests and on every push to `main`. It is deliberately not part of the deployment or the bump's build gate: a release that breaks an example still updates the reference, and the failing run is the report. For the same reason `TsRef`, the guide's link to a reference entry, warns and renders plain code for an unknown name unless `XCL_REFERENCE_STRICT=1`, which only that workflow sets. This is the one place the plan differs from the Python reference's `PyRef`, whose spec has no such requirement.

`CodeTabs` gains a third language, with `Head.astro`'s pre-paint and the handler in `Shell.astro` taught to accept it, so one stored choice still drives every box with no flash of another language. Once every page has its tab, `CodeTabs` requires the `typescript` slot and fails the build naming the page, which is how "every box has a TypeScript tab" stays true. The guide, `/using-xcl-with-typescript/`, follows the Python guide's form: TypeScript only, one program built section by section from named regions of `typescript-examples/guide/program.ts`, and the complete listing at the end, so the guide's code is compiled like every tab. Alternatives, including extracting tabs from MDX and `typedoc-plugin-markdown`, are compared in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **API reference tool (new, `xcl-typescript`)** — owns the definition of a public name and member for TypeScript. Reads the package's entry point through TypeDoc and either reports every undocumented name or member, or writes the reference data, including the package version and each entry's rendered description. Used by the doc comment check and by the release job.
- **Doc comment check (new CI job, `xcl-typescript`)** — runs the tool in check mode, and is one of the jobs the `result` check requires.
- **Package doc comments (changed, `xcl-typescript`)** — complete across every public name and member, with parameter, return and error tags. Text only.
- **Release workflow (changed, `xcl-typescript`)** — also generates the reference for the tag, stops the release before npm if generation fails, attaches the data to the GitHub release, and dispatches the website once that release exists.
- **Reference data loader (changed, `xcl-website`)** — the Python reference's fetch step also fetches the TypeScript asset named by its pin, or copies a local file for development, and refuses data whose version differs from the pin.
- **Reference page (changed, `xcl-website`)** — the page template and entry component the Python reference added are shared, so one template serves `/reference/python/` and `/reference/typescript/`. The entry component gains overloads and the TypeScript kinds.
- **`TsRef` link component (new, `xcl-website`)** — used by the guide and home page to link to an entry by full name. An unknown name is a warning, and a failure in the site's checks.
- **Pin and bump workflow (changed, `xcl-website`)** — a second one-line pin, and the existing bump workflow learns the TypeScript release event and moves the pin that event names.
- **Example project (new, `xcl-website`)** — a strict TypeScript project holding every TypeScript example as real source: one file per tab, and the guide's single program. Compiled against the pinned release.
- **`TsExample` component (new, `xcl-website`)** — shows a file, or a named region of one, through the same code frame as every other block. A missing file or region fails the build, since both are in the site's own source.
- **Example check workflow (new, `xcl-website`)** — installs the pinned `xcl-lang`, compiles the example project and builds the site strictly, on pull requests and on every push to `main`. Separate from the deployment and the bump's gate.
- **Language tabs (changed, `xcl-website`)** — the tab box, the pre-paint script and the tab handler accept a third language, and the box requires a TypeScript panel once every page has one.
- **TypeScript tabs (changed, `xcl-website`)** — all 107 boxes on the 19 pages gain a panel: code from the example project, or a statement that the behaviour has no TypeScript equivalent.
- **TypeScript guide (new, `xcl-website`)** — one page taking a reader from `npm install` to a working program, linking to topic pages and to reference entries through `TsRef`.
- **Site navigation, home page and language lists (changed, `xcl-website`)** — a TypeScript child in the Reference menu, the guide under Guides, the install command and links on the home page, and TypeScript named wherever the site lists its languages.
- **READMEs (changed, `xcl-typescript` and `xcl`)** — both link to the reference and guide.

## Data Structures & Interfaces

**Reference data** is the Python reference's contract between a package's release and the site: a JSON document generated by the release and read only by the site's build. TypeScript uses the same shape, with `language: "typescript"`, `package: "xcl-lang"` and additive fields. `schema` stays 1.

```
Entry {
  id: "xcl-lang.Config.apply"    // the full dotted name; the page anchor
  name: "apply"
  kind: "namespace" | "class" | "interface" | "type" | "enum" | "function" | "method"
      | "property" | "constant" | "constructor" | "attribute"
  signature?: string             // printed by the compiler
  description: html
  parameters?: { name, type?, default?, description?: html }[]
  returns?: { type?, description?: html }
  raises?: { type, description?: html }[]   // type is the name in {@link}
  inheritedFrom?: string
  members?: Entry[]
  overloads?: Overload[]         // new: present when there are two or more signatures
}
Overload { signature, description?: html, parameters?, returns?, raises? }
```

Each namespace (`events`, `prettylog`) is a section of its own, and the rest are sectioned by the source file that defines them. A value and a type exported under one name are one entry; its `overloads` carry each form's signature.

**API reference tool's command line** is `api-reference.mjs [--check] [--out PATH] [--source DIR]`: `--check` prints each undocumented name and exits non-zero, `--out` writes the data, and `--source` names the directory holding the package, the checkout by default.

**Pin file** `xcl-typescript-ref.txt` holds one tag, the same form as `xcl-ref.txt` and `xcl-python-ref.txt`. The package has its own version line, separate from the xcl tag it bundles, as xcl-python's does.

**Release dispatch** is `repository_dispatch` event `xcl-typescript-release` to `xcl-website` with `client_payload.tag`. `reference-bump.yml`'s manual trigger gains a `package` input of `python` or `typescript`.

**`TsExample` component** takes `file`, the path below `typescript-examples/`, with optional `region` and `title`. A region is the lines between `// #region <name>` and `// #endregion`. With no `region` the whole file is shown, less any region markers.

**`TsRef` component** takes `name`, the entry's full dotted name, and an optional `label`. Its link is `/reference/typescript/#<name>`.

**Environment overrides**, for development:

- `XCL_TYPESCRIPT_REFERENCE`: path of a reference data file to use instead of downloading the pinned release's.
- `XCL_TYPESCRIPT_PACKAGE`: a tarball of `xcl-lang` to compile the examples against instead of the pinned release.
- `XCL_REFERENCE_STRICT=1`: an unknown `TsRef` name fails the build.

**Language preference** stays in `localStorage` key `xcl-lang`, with `typescript` a new accepted value; `data-lang` on `<html>` takes the same values.

## Implementation Detail

Most of this work follows shapes the Python reference and the core SDK plan set up. The release gains jobs in its existing style, the site gains a sibling pin, and the reference page is the same template over different data. Two patterns are new.

The first is examples as compiled source. A reader of a page sees an ordinary code frame, but an author writes a small program in the example project and names the part to show. Everything that makes a fragment compile, such as imports, a `config` and the class it uses, sits outside the region as real code, so there is no hidden preamble mechanism and no parser. An editor gives normal TypeScript errors in these files, and the page stays short. The Go and Python tabs on the same page remain inline fences: only the language with a checked package moves to files.

The second is a check that reports without blocking. Everything that depends on the released package's API lives in the example check workflow, and nothing in the deployment or the bump does, apart from reading the asset. A developer who needs to know why a release went out with a broken example finds a failed run of that workflow on the bot's commit, naming the file and line.

The doc comment tool differs from the Python tool in its reading, not its role. TypeDoc gives it a model of the package, and the compiler's own printer gives it signatures. Two TypeDoc behaviours shape the tool: a value and a type under one name arrive as two records to be merged, and an error's type is only kept when written as a link. A doc comment's Markdown is rendered to HTML here, so the site never interprets it. Links to other entries are resolved against the entry list, and one that does not resolve becomes plain code.

A reader sees TypeScript as a third tab in the same place on every box, with one choice across the site. Boxes with no equivalent say so in prose. The order of work on the site protects the build at every step: the example project and the panels come first, since Astro ignores a slot the component does not name, and the tab, the preference and the requirement that every box has a panel arrive together last.

## Dependencies

- Design documents this plan was built on: none. The spec carries no design references. The API names in the guide and tabs come from the released package.
- **Spec `20261031080136-264e0eab-typescript-core-sdk` (must land first, spec constraint)** — the package, its CI, and the release workflow this plan extends.
- **Spec `20261110083940-53be9068-typescript-printing` (must land first, spec constraint)** — printing and the pretty logger, which the guide and many tabs use.
- **Spec `20261110084343-9ebf34a1-python-sdk-reference` (must land first on `xcl-website`)** — the Reference menu, page and entry components, fetch step, `reference-bump.yml`, and the release bot App installed on `xcl-website` with its secrets. If it is not delivered when the website tasks start, STOP and ask.
- **`xcl-typescript` release before the website's first merge** — the website's first pin must name a release carrying the reference asset. A release of `xcl-lang` happens when a pull request labelled `patch`, `minor` or `major` merges, so the `xcl-typescript` work must be merged with a release label before the site's first pin. The release plan orders this.
- **TypeDoc 0.28.20 (new, dev only)** — reads the package's signatures and doc comments. Exact-pinned in `xcl-typescript`'s dev dependencies; never a runtime dependency of `xcl-lang`. Supports TypeScript 5.0 to 6.0, which covers the core plan's pin.
- **markdown-it (new, dev only)** — renders doc comment Markdown to HTML. Exact-pinned beside TypeDoc.
- **TypeScript and `@types/node` (example project, `xcl-website`)** — exact-pinned in the example project only; the site's own dependencies do not change.
- **The release bot GitHub App** — already installed on `xcl-typescript` by the core plan and on `xcl-website` by the Python reference plan. No new setup.
- **GitHub releases** — hosts the reference asset, fetched by the site over HTTPS. No new service.
- **npm's `xcl-lang` at the pinned version** — the example check compiles against it. Its optional engine packages are not installed.
- **Astro 5, Expressive Code 0.43, Node 22 (existing)** — no version changes; `TsExample` uses Expressive Code's `Code` component.
- **The signed `xcl-plugin-docker` release** — the guide installs it. The implementing task checks it against the latest package, and the walkthrough needs a local Docker daemon.

## Testing Approach

The API reference tool gets unit tests, since it holds the only logic that decides what is public and what is missing. They run with Node's test runner against small packages created in a temporary directory, never the repository's own source, as the xcl testing convention requires, and they run from the same check job. The tests guarantee that a package with one undocumented public method fails and names that method, a fully documented one passes, an inherited member is listed and reported once on its defining class, a non-exported name and `@internal` and protected members are ignored, a `@throws` with no link fails, a value and a type of one name become one entry, overloads are all listed, a constructor type keeps its `new` in the signature, and the data carries the package version. Positive and negative cases are separate test functions, and none are table-driven.

The tool's other half is exercised for real: it runs over the package in the check job and from the release. The example check is the compiler over real source, and it is what proves each tab compiles against the pinned release. The site's guards are exercised by building: missing data, a wrong version, a missing example file or region, a missing TypeScript panel, and in strict mode an unknown `TsRef` name, all fail. There are no new site tests: the site has no test harness and the convention forbids tests over page text.

Workflows and documentation are checked by running them and by review, not by tests over their files. The package's own test suites are unaffected, since its code does not change.

Deliberate gaps: no automated check runs the guide's program, because it needs npm, a Docker daemon and a network; a person runs it. Examples are compiled, not run, so a tab can compile and still misdescribe behaviour; review covers that.

Success metrics:

- "A TypeScript developer can answer what a method takes and returns from the site without opening the source": **Manual — captured in the implementation test plan** (a person looks up several methods on the built page).
- "A new TypeScript developer reaches a working apply against a Go plugin by following the guide alone": **Manual — captured in the implementation test plan** (the guide is followed in an empty directory).
- "The published reference shows the latest package version within one hour of each release": **Manual — captured in the implementation test plan** (observed across a real release).
- "No open issue reports a TypeScript example on the site that does not work": **Manual — captured in the implementation test plan** (the issue tracker is checked after release). The example check is the automated guard behind it.

Manual reviews and checks:

- Every reference page shows the site's nav and footer, shows the release's version, and its internal links open: **Manual — captured in the implementation test plan**.
- Every name the entry point exports has an entry, and each public class has its constructor, methods and properties, inherited ones included: **Manual — captured in the implementation test plan**.
- A doc comment changed in a local package appears in a site built against the regenerated data, with no edit to the site: **Manual — captured in the implementation test plan**.
- A real release moves the pin and updates the site with no hand commit: **Manual — captured in the implementation test plan**.
- A release that breaks an example still updates the reference, and the example check fails naming the example: **Manual — captured in the implementation test plan**.
- Removing a public method's doc comment fails the package's checks naming it, and restoring it passes: **Manual — captured in the implementation test plan**.
- Every example in the guide, tried against the latest release, behaves as the guide says, and anything marked not yet supported is unsupported: **Manual — captured in the implementation test plan**.
- The guide links to the registries, plugins, state and events pages, and each API name links to an entry that exists: **Manual — captured in the implementation test plan**.
- The guide meets the wording rules: no "superset", "interpolation" or "Terraform", XCL described as its own language, and "resource" only in code identifiers: **Manual — captured in the implementation test plan**.
- The navigation, home page and both READMEs link to the reference and guide, and each link opens the right page: **Manual — captured in the implementation test plan**.
- Every code box on every page has a TypeScript tab, and each statement of no equivalent is true of the released package: **Manual — captured in the implementation test plan**.
- Choosing TypeScript switches every box and persists across pages and reloads; with no stored choice Go shows first, with Python stored Python shows first, with no flash of another language: **Manual — captured in the implementation test plan**.
- The home page shows `npm install xcl-lang`, and every page listing the languages names TypeScript: **Manual — captured in the implementation test plan**.

## Milestones & Tasks

### Milestone 1: Every public name in the TypeScript package is documented, and new gaps fail the checks

**What changes**: Every name the package exports and every public method and property has a doc comment, its parameters, results and errors are documented, and a pull request that adds an undocumented public name fails its checks by naming it. Each release also produces the package's API as structured data and tells the website about it. Readers see no site change yet; maintainers get a check that keeps the documentation whole.

**Validation point**: the doc comment check passes over the package and fails, naming the method, when one doc comment is removed; running the tool writes reference data carrying the package's version.

#### - [x] Task: Add the API reference tool
**Id:** 0dbad344-2aff-483d-a1ef-7bdadfa40f8d
**Repo:** xcl-typescript
**Depends on:** none
**Execution:** agent

Adds a script that reads the package through TypeDoc and decides what is public. It either reports every undocumented name or member, or writes the API as structured data in the shape the Python reference uses. It is the one definition of a public name for both the check and the site.

*Technical detail:* [context.md#task-add-the-api-reference-tool](./context.md#task-add-the-api-reference-tool)

**Acceptance criteria**:
- [x] The tool lists every public name and member lacking a doc comment and exits with a failure
- [x] With every name documented it exits successfully
- [x] The tool writes reference data holding every public name, its signature, description, parameters, return value, errors and members, and the package version
- [x] Inherited members appear on a class, and names outside the entry point, protected members and `@internal` members do not
- [x] A name that is both a value and a type is one entry, and overloads are each listed
- [x] TypeDoc is not a runtime dependency of `xcl-lang`

#### - [ ] Task: Complete the package's doc comments
**Id:** 43cab42d-61c3-4aba-84f1-1a7ecaa56d60
**Repo:** xcl-typescript
**Depends on:**
- 0dbad344-2aff-483d-a1ef-7bdadfa40f8d — Add the API reference tool
**Execution:** agent

Fills every remaining gap in the package's doc comments and adds parameter, return and error tags, including the namespaces and the decorators' forms. Text only, so behaviour stays the same.

*Technical detail:* [context.md#task-complete-the-packages-doc-comments](./context.md#task-complete-the-packages-doc-comments)

**Acceptance criteria**:
- [ ] The tool reports no undocumented public name or member, with no exclusions
- [ ] Methods and functions document their parameters, return values and errors where they have them
- [ ] Every documented error is written so the reference names its type
- [ ] No code other than doc comments changes

#### - [ ] Task: Run the doc comment check in CI
**Id:** c1d42031-5daf-4359-a904-7a06e38af506
**Repo:** xcl-typescript
**Depends on:**
- 0dbad344-2aff-483d-a1ef-7bdadfa40f8d — Add the API reference tool
- 43cab42d-61c3-4aba-84f1-1a7ecaa56d60 — Complete the package's doc comments
**Execution:** agent

Adds the check as a job in the package's test workflow, so a pull request adding an undocumented public name fails and names it.

*Technical detail:* [context.md#task-run-the-doc-comment-check-in-ci](./context.md#task-run-the-doc-comment-check-in-ci)

**Acceptance criteria**:
- [ ] A pull request with an undocumented public method fails its required checks, naming the method
- [ ] The package's own check run passes
- [ ] A local Makefile target runs the same check

#### - [ ] Task: Publish the reference with each release
**Id:** 8798e4d2-8337-4782-8b82-397a83e7dad1
**Repo:** xcl-typescript
**Depends on:**
- 0dbad344-2aff-483d-a1ef-7bdadfa40f8d — Add the API reference tool
**Execution:** agent

Makes each release generate the reference for its tag, stop before npm if generation fails, attach the data to the GitHub release, and tell the website about the new tag.

*Technical detail:* [context.md#task-publish-the-reference-with-each-release](./context.md#task-publish-the-reference-with-each-release)

**Acceptance criteria**:
- [ ] A release attaches reference data whose version is the released version to the GitHub release
- [ ] A failure generating the reference stops the release before anything reaches npm
- [ ] Once the GitHub release exists, the website is dispatched with the tag
### Milestone 2: The TypeScript reference is published on xcl.dev and follows each release

**What changes**: A TypeScript developer can open `/reference/typescript/` on the site and find every public name, with its signature, description and members, along with the package version it describes. The Reference menu lists TypeScript beside Python. Each new package release reaches the page by itself, after a site build has proved it works.

**Validation point**: the built site serves the page with the site's navigation and footer, showing the pinned version; a dispatched bump moves the TypeScript pin only after a successful build.

#### - [ ] Task: Load the TypeScript reference data into the site
**Id:** 0bca4a4a-6968-4ada-9d0f-32a9b3beff8b
**Repo:** xcl-website
**Depends on:**
- 0dbad344-2aff-483d-a1ef-7bdadfa40f8d — Add the API reference tool
**Execution:** agent

Adds the TypeScript pin and extends the fetch step the Python reference added, so every build and dev server fetches both releases' data, or copies a local file during development. The deployment still runs the same build.

*Technical detail:* [context.md#task-load-the-typescript-reference-data-into-the-site](./context.md#task-load-the-typescript-reference-data-into-the-site)

**Acceptance criteria**:
- [ ] A build fetches the pinned release's data and keeps it out of git
- [ ] A build fails with a clear message when the data is missing or its version differs from the pin
- [ ] Setting the development override uses a local file instead of downloading
- [ ] The site's deployment workflow is unchanged

#### - [ ] Task: Render the TypeScript reference page
**Id:** 2465d985-4f45-4cce-8e53-79c0d11c4a91
**Repo:** xcl-website
**Depends on:**
- 0bca4a4a-6968-4ada-9d0f-32a9b3beff8b — Load the TypeScript reference data into the site
**Execution:** agent

Builds the page at `/reference/typescript/` from the data, inside the site's layout, using one template for both languages. It shows the version, an index, and one anchored entry per name with its members and overloads.

*Technical detail:* [context.md#task-render-the-typescript-reference-page](./context.md#task-render-the-typescript-reference-page)

**Acceptance criteria**:
- [ ] The built page shows the site's navigation and footer
- [ ] Every entry in the data has an anchored entry on the page showing its signature and description, and its parameters, return value and errors where documented
- [ ] Each overload of a name is shown
- [ ] The version shown equals the data's version
- [ ] The Python reference is unchanged

#### - [ ] Task: Add TypeScript to the Reference menu and TsRef links
**Id:** afcf33ce-1c61-42ec-bb44-2f4daa9791a0
**Repo:** xcl-website
**Depends on:**
- 2465d985-4f45-4cce-8e53-79c0d11c4a91 — Render the TypeScript reference page
**Execution:** agent

Adds TypeScript beside Python in the Reference menu, and a `TsRef` link component that warns on a name not in the reference and fails the site's checks.

*Technical detail:* [context.md#task-add-typescript-to-the-reference-menu-and-tsref-links](./context.md#task-add-typescript-to-the-reference-menu-and-tsref-links)

**Acceptance criteria**:
- [ ] The Reference menu opens the TypeScript reference as well as the Python one
- [ ] A `TsRef` link goes to the named entry
- [ ] A name not in the reference fails a strict build and only warns otherwise

#### - [ ] Task: Extend the pin bump workflow to TypeScript
**Id:** 21ef54ec-b8a2-4b04-a031-80e95096b556
**Repo:** xcl-website
**Depends on:**
- 0bca4a4a-6968-4ada-9d0f-32a9b3beff8b — Load the TypeScript reference data into the site
**Execution:** agent

Teaches the workflow that moves a pin to handle a TypeScript release, proving the site builds and pushing as the release bot. A failing build leaves the site on the version it has.

*Technical detail:* [context.md#task-extend-the-pin-bump-workflow-to-typescript](./context.md#task-extend-the-pin-bump-workflow-to-typescript)

**Acceptance criteria**:
- [ ] A dispatched TypeScript release moves the TypeScript pin and pushes to main only after the site builds
- [ ] A build failure leaves the pin and site unchanged
- [ ] A dispatch for a version already pinned or older does nothing
- [ ] A Python release still moves only the Python pin
### Milestone 3: Every example on the site is available in TypeScript

**What changes**: Every code box offering Go or Python also offers TypeScript, with code that compiles against the released package or a statement that the behaviour has no TypeScript equivalent. Choosing TypeScript switches every box on the page and is remembered on later pages, while readers who chose nothing still see Go and readers who chose Python still see Python. A release that breaks an example is reported by the site's checks without holding the reference back. Places that say xcl is available from Go and Python now name TypeScript too.

**Validation point**: the example check passes against the pinned release and fails when a tab's code is broken; the build fails if a box has no TypeScript panel; choosing TypeScript carries across pages with no flash of another language.

#### - [ ] Task: Add the checked example project
**Id:** b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

Adds a strict TypeScript project for the site's examples, a script that compiles it against the pinned package, and a component that shows a file or a region of one in a page. Nothing on the site changes yet.

*Technical detail:* [context.md#task-add-the-checked-example-project](./context.md#task-add-the-checked-example-project)

**Acceptance criteria**:
- [ ] The project compiles under strict settings against the pinned `xcl-lang`, with no engine package installed
- [ ] A type error fails the check, naming the file and line
- [ ] A page can show a file, or a named region of it, in the site's normal code frame
- [ ] A missing file or region fails the build
- [ ] The site's own type check does not read the example project

#### - [ ] Task: Add the example check workflow
**Id:** 5d0e51a0-c7c2-4b6c-b0f3-b95fc310d2b4
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds a workflow that compiles the examples against the pinned package and builds the site strictly, on pull requests and on every push to main. It is separate from the deployment and from the pin bump's build, so a release that breaks an example still updates the reference.

*Technical detail:* [context.md#task-add-the-example-check-workflow](./context.md#task-add-the-example-check-workflow)

**Acceptance criteria**:
- [ ] A tab that stops compiling fails the workflow, naming the file and line
- [ ] An unknown `TsRef` name fails the workflow
- [ ] The workflow runs on every push to main, including the release bot's
- [ ] The deployment and the pin bump do not run the example compile

#### - [ ] Task: Add TypeScript tabs to the language pages
**Id:** 5d282308-4de2-4e6b-b666-63d6649a06e0
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds the TypeScript panel to the 21 code boxes on the language pages, showing entity types as decorated classes. Each is a real program in the example project.

*Technical detail:* [context.md#task-add-typescript-tabs-to-the-language-pages](./context.md#task-add-typescript-tabs-to-the-language-pages)

**Acceptance criteria**:
- [ ] Each of the 21 boxes on these pages has a TypeScript panel
- [ ] Every panel's code compiles in the example check

#### - [ ] Task: Add TypeScript tabs to the operations guides
**Id:** c6dd19e2-1887-46fc-af7b-635fc509e68d
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds the TypeScript panel to the 30 boxes on the events, diff, problems, sensitive values, state masking, plugin logging and plugin template pages. A box whose behaviour has no TypeScript equivalent says so.

*Technical detail:* [context.md#task-add-typescript-tabs-to-the-operations-guides](./context.md#task-add-typescript-tabs-to-the-operations-guides)

**Acceptance criteria**:
- [ ] Each of the 30 boxes on these pages has a TypeScript panel holding code or a statement of no equivalent
- [ ] Every panel's code compiles in the example check
- [ ] Each statement of no equivalent is true of the released package

#### - [ ] Task: Add TypeScript tabs to the registry pages
**Id:** e398e58b-1c67-4d68-913e-3cce54d21f91
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds the TypeScript panel to the 15 boxes on the registries and GitHub registry pages. Writing a custom registry has no TypeScript equivalent, and those boxes say so.

*Technical detail:* [context.md#task-add-typescript-tabs-to-the-registry-pages](./context.md#task-add-typescript-tabs-to-the-registry-pages)

**Acceptance criteria**:
- [ ] Each of the 15 boxes on these pages has a TypeScript panel holding code or a statement of no equivalent
- [ ] Every panel's code compiles in the example check

#### - [ ] Task: Add TypeScript tabs to the configuration text page
**Id:** f358430c-0fad-4bf7-aa03-91280497c245
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds the TypeScript panel to the 18 boxes on the configuration text page, covering encoding, formats, themes and labelled text. Labelling a saved record has no TypeScript equivalent, and that box says so.

*Technical detail:* [context.md#task-add-typescript-tabs-to-the-configuration-text-page](./context.md#task-add-typescript-tabs-to-the-configuration-text-page)

**Acceptance criteria**:
- [ ] Each of the 18 boxes on this page has a TypeScript panel holding code or a statement of no equivalent
- [ ] Every panel's code compiles in the example check

#### - [ ] Task: Add TypeScript tabs to the example programs and home page
**Id:** 9ff5e5b2-e406-4f8b-8a2c-c42ff073ada4
**Repo:** xcl-website
**Depends on:**
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Adds the TypeScript panel to the 23 boxes on the two example program pages and the home page, including the npm install command. These are the longest programs on the site.

*Technical detail:* [context.md#task-add-typescript-tabs-to-the-example-programs-and-home-page](./context.md#task-add-typescript-tabs-to-the-example-programs-and-home-page)

**Acceptance criteria**:
- [ ] Each of the 23 boxes on these pages has a TypeScript panel holding code or a statement of no equivalent
- [ ] Every panel's code compiles in the example check
- [ ] The home page's install box shows `npm install xcl-lang`

#### - [ ] Task: Offer TypeScript in every code box
**Id:** 10dcd52e-b895-4647-bd0d-624b3b6ec7d8
**Repo:** xcl-website
**Depends on:**
- 5d282308-4de2-4e6b-b666-63d6649a06e0 — Add TypeScript tabs to the language pages
- c6dd19e2-1887-46fc-af7b-635fc509e68d — Add TypeScript tabs to the operations guides
- e398e58b-1c67-4d68-913e-3cce54d21f91 — Add TypeScript tabs to the registry pages
- f358430c-0fad-4bf7-aa03-91280497c245 — Add TypeScript tabs to the configuration text page
- 9ff5e5b2-e406-4f8b-8a2c-c42ff073ada4 — Add TypeScript tabs to the example programs and home page
**Execution:** agent

Adds TypeScript as a third tab with its panel, and teaches the pre-paint script and the tab handler the new choice, so one choice carries across pages with no flash of another language. A box with no TypeScript panel now fails the build.

*Technical detail:* [context.md#task-offer-typescript-in-every-code-box](./context.md#task-offer-typescript-in-every-code-box)

**Acceptance criteria**:
- [ ] Every box shows a TypeScript tab beside Go and Python
- [ ] Choosing TypeScript switches every box on the page, and the choice survives moving page and reloading
- [ ] With no stored choice the page first shows Go, and with Python stored it first shows Python, with no flash of another language
- [ ] A box missing a TypeScript panel fails the build, naming the page

#### - [ ] Task: Name TypeScript across the site
**Id:** 3bf4942c-2f7c-4eed-ae0b-985fc4631aaf
**Repo:** xcl-website
**Depends on:**
- 10dcd52e-b895-4647-bd0d-624b3b6ec7d8 — Offer TypeScript in every code box
**Execution:** agent

Makes each place that says xcl is available from Go and Python also name TypeScript, including the home page and the site's description.

*Technical detail:* [context.md#task-name-typescript-across-the-site](./context.md#task-name-typescript-across-the-site)

**Acceptance criteria**:
- [ ] Every page that lists the languages xcl is available from also names TypeScript
- [ ] The home page badge and description name TypeScript
### Milestone 4: A guide takes a TypeScript developer from nothing to a working program, and readers can find it

**What changes**: A new guide page walks through installing from npm to a working program using a Go plugin, linking each name it introduces to the reference. The guide and reference are reachable from the navigation, the home page and both READMEs, and the site pins the first release that carries the reference.

**Validation point**: following the guide in an empty directory produces a working program, its code compiles in the example check, the wording rules are met, and xcl.dev shows both pages at the pinned version.

#### - [ ] Task: Write the TypeScript guide
**Id:** aa87b1df-ae12-40c3-9dba-fa62c2524f4f
**Repo:** xcl-website
**Depends on:**
- afcf33ce-1c61-42ec-bb44-2f4daa9791a0 — Add TypeScript to the Reference menu and TsRef links
- b4e67d3f-5e8b-4ea3-a0e9-712c3c1b1d22 — Add the checked example project
**Execution:** agent

Writes the guide that takes a TypeScript developer from `npm install` to a working program using a Go plugin, with every API name linked to the reference. The guide's code is one program in the example project, shown in sections, so it compiles like every tab. Everything in it is checked against the latest released package.

*Technical detail:* [context.md#task-write-the-typescript-guide](./context.md#task-write-the-typescript-guide)

**Acceptance criteria**:
- [ ] Following the guide from the top with only its code gives a program that creates a configuration, defines and registers an entity type and a function, uses a Go plugin, applies, keeps state between runs, finds an entity by address and by type, handles a configuration error, cancels an operation, turns on logging, prints an entity and destroys what it applied
- [ ] The guide links to the registries, plugins, state and events pages, and each API name links to its reference entry
- [ ] The guide has TypeScript code only, with XCL configuration allowed, and meets the wording rules
- [ ] Anything planned but not built is marked as not yet supported

#### - [ ] Task: Link the guide from the site
**Id:** 93c63e3f-fb01-43b4-a5b6-b15823d5c45f
**Repo:** xcl-website
**Depends on:**
- aa87b1df-ae12-40c3-9dba-fa62c2524f4f — Write the TypeScript guide
- 3bf4942c-2f7c-4eed-ae0b-985fc4631aaf — Name TypeScript across the site
**Execution:** agent

Puts the guide in the Guides menu and links the guide and reference from the home page, where every reader sees them. Adds the new pages to the site's README.

*Technical detail:* [context.md#task-link-the-guide-from-the-site](./context.md#task-link-the-guide-from-the-site)

**Acceptance criteria**:
- [ ] The guide appears under Guides beside the Python guide, and the Guides label still opens the same page
- [ ] The home page links to the guide and reference outside any language tab
- [ ] Every new link opens the right page

#### - [ ] Task: Update the package README's links
**Id:** 62b2a1ad-5812-4a6d-9920-735ed112094e
**Repo:** xcl-typescript
**Depends on:** none
**Execution:** agent

Adds links to the guide and reference on the site. Done before the release so the published package carries them.

*Technical detail:* [context.md#task-update-the-package-readmes-links](./context.md#task-update-the-package-readmes-links)

**Acceptance criteria**:
- [ ] The README links to `/reference/typescript/` and `/using-xcl-with-typescript/`
- [ ] The README says the reference follows releases

#### - [ ] Task: Link the guide and reference from the xcl README
**Id:** a2e58aad-2d7a-47e8-9f28-cc2f9b84e956
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds the TypeScript guide and reference to the xcl README's documentation list and its other-languages section.

*Technical detail:* [context.md#task-link-the-guide-and-reference-from-the-xcl-readme](./context.md#task-link-the-guide-and-reference-from-the-xcl-readme)

**Acceptance criteria**:
- [ ] The xcl README links to the TypeScript reference and guide
- [ ] Each link opens the right page

#### - [ ] Task: Release the package with the reference asset
**Id:** f2b1a6d6-3447-42a3-a150-efd00f757a89
**Repo:** xcl-typescript
**Depends on:**
- 43cab42d-61c3-4aba-84f1-1a7ecaa56d60 — Complete the package's doc comments
- c1d42031-5daf-4359-a904-7a06e38af506 — Run the doc comment check in CI
- 8798e4d2-8337-4782-8b82-397a83e7dad1 — Publish the reference with each release
- 62b2a1ad-5812-4a6d-9920-735ed112094e — Update the package README's links
**Execution:** human — releasing is action outside the repo

Merges the `xcl-typescript` work and sees a release through, so the reference asset exists for the site to pin. A release of this package happens when its pull request merges with a `patch`, `minor` or `major` label. The website cannot merge before this exists.

*Technical detail:* [context.md#task-release-the-package-with-the-reference-asset](./context.md#task-release-the-package-with-the-reference-asset)

**Acceptance criteria**:
- [ ] A released version on npm has a GitHub release carrying its reference data

#### - [ ] Task: Set the site's first TypeScript pin
**Id:** 37eee2bc-8691-48a2-88e5-a6c308811b1a
**Repo:** xcl-website
**Depends on:**
- f2b1a6d6-3447-42a3-a150-efd00f757a89 — Release the package with the reference asset
- 93c63e3f-fb01-43b4-a5b6-b15823d5c45f — Link the guide from the site
- 21ef54ec-b8a2-4b04-a031-80e95096b556 — Extend the pin bump workflow to TypeScript
- 5d0e51a0-c7c2-4b6c-b0f3-b95fc310d2b4 — Add the example check workflow
- 10dcd52e-b895-4647-bd0d-624b3b6ec7d8 — Offer TypeScript in every code box
**Execution:** human — requires a just-released version, and merging ahead of the release would deploy a broken site

Writes the released tag into the pin on the website's branch, then merges so the site deploys. Later releases move the pin automatically.

*Technical detail:* [context.md#task-set-the-sites-first-typescript-pin](./context.md#task-set-the-sites-first-typescript-pin)

**Acceptance criteria**:
- [ ] The pin names a release with the reference asset
- [ ] xcl.dev shows the reference and guide at that version

## Open Questions

- **Does every name the tabs use exist in the released package?** This depends on the core and printing specs as delivered. If a tab needs a behaviour the plans listed but the package lacks, STOP and ask the user rather than writing around it; a behaviour the plans deliberately excluded gets the "no TypeScript equivalent" statement.

Whether the Docker plugin release named in the guide still installs against the latest package is a check the guide task makes when it runs, not an open question; if it no longer does, the guide names the release that does.

## Out of Scope

- A reference or guide for Python: the earlier spec `20261110084343-9ebf34a1-python-sdk-reference` in this epic.
- A Go guide; Go's reference stays on pkg.go.dev.
- Reference pages for older package versions: only the pinned version is published.
- Language tabs in examples that show only XCL configuration.
- Languages other than Go, Python and TypeScript in the tabs.
- Any change to the package's behaviour: only doc comments, checks, their development dependencies and configuration, the README and release workflows change.
- Running the TypeScript tabs: they are compiled, not run.
- Rewriting the package's README: only its links and one sentence change.
- A site-wide link checker; unknown guide references fail the strict check, and the rest is checked by hand.
- Automatically opening an issue for a broken example: the failing check is the report.
- Standard library members a class inherits, such as `Error`'s.
- Providers, plugins or a plugin SDK in TypeScript: the guide marks them as not yet supported.

## Changelog

### 2026-11-13 — Task: Add the API reference tool

**What was done**: Added `.github/scripts/api-reference.mjs`, which reads the package through TypeDoc and either reports every public name or member with no doc comment (`--check`) or writes the reference data (`--out`) in xcl-python's shape, with `language: "typescript"`, the package version and rendered HTML descriptions. Added its `node:test` suite against fixture packages in a temporary directory, exact-pinned `typedoc` 0.28.20 and `markdown-it` 14.3.3 as dev dependencies, and recorded why in a new Development section of the README.

**Deviations**:
- Plan/code drift noted at the start (trusted the code): configurations are `new Config({...})` with an optional `setup()`, there is a public `engineVersion()`, and the platform packages are `@jumppad-labs/xcl-lang-<platform>`. None changed this task's work.
- `markdown-it` is pinned at 14.3.3, the version TypeDoc 0.28.20 depends on, rather than the newer 15.x, so one copy is installed.
- Types, parameter types and return types are printed from the source's type annotation where one is written (the compiler's `signatureToString` for whole signatures), because the checker's `typeToString` adds `Uint8Array<ArrayBufferLike>`, expands aliases and prints `unique symbol` as `typeof X`.
- Additions the plan did not name: a type alias of an interface the package does not export (`Entity`, `Meta`) lists that interface's members, read from the compiler, so the reference shows them; a static member's id is `<Class>.static.<name>` because `XclError.code` exists as both a static and an instance property; a re-export (`Mask`, and the `events` namespace's re-exports) is an entry saying "The same as" its target; a class constructor with no doc comment gets Python's generated "Creates a `X`, as the class describes."; the `mask` namespace is a section of its own beside `events` and `prettylog`; `engine-version.ts` and `internal/poll.ts` (`StateStore`) are in the Configuration section and `entity.ts` in Entity types.
- The README's new Development section is outside the goal's file list; the plan's context asked for the dependency reason to be recorded there.

**Files changed**:
- `xcl-typescript: .github/scripts/api-reference.mjs`
- `xcl-typescript: .github/scripts/api-reference.test.mjs`
- `xcl-typescript: package.json`
- `xcl-typescript: package-lock.json`
- `xcl-typescript: README.md`

**Discoveries**: The package already had a doc comment on every public name and member, so `--check` passes before the doc comment task; that task's work is the `@param`, `@returns` and `@throws {@link}` tags, which the source has none of. One TypeScript symbol carries both the value and the type declaration of a merged name, so a declaration must be picked by the reflection's kind, not `declarations[0]`. Application.bootstrap's default readers look for a TypeDoc options file in the working directory; the tool passes only a `TSConfigReader`.
