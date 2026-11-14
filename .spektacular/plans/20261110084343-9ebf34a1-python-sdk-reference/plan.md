---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Plan: 20261110084343-9ebf34a1-python-sdk-reference

<!-- Metadata -->
<!-- Created: 2026-11-10T09:00:15Z -->
<!-- Commit: 788959a -->
<!-- Branch: f-python-plugin -->
<!-- Repository: jumppad-labs/xcl -->

## Overview

Python developers using xcl have a long README but no API reference and no walkthrough on xcl.dev. This plan publishes a complete reference for the Python package at `/reference/python`, generated from the package itself at each release so it stays accurate, and a guide that takes a Python developer from an empty program to a working one. It serves anyone adopting xcl from Python, who can look up any name or follow one path from start to finish.

## Conventions

- **No test inspects repository files or parses the repository's own source (`testing-and-mocking`)** — the doc comment check is a CI lint rather than a pytest, and the generator's tests run against small fixture packages made in a temporary directory.
- **Tests never mix positive and negative cases, and are not table-driven (`testing-and-mocking`)** — applies to the generator's new tests.
- **Document the reason for a third-party dependency and pin it (`dependencies`)** — Griffe and docutils are pinned in `requirements-docs.txt` with a comment saying why, used only by CI.
- **Write a release plan once released work is merged (`release-plan-after-implementation`)** — this spec crosses a package release and a site deploy in a required order, so a release plan follows implementation.
- **Configuration and metadata files are written in xcl (`config-files-are-xcl`)** — considered and not applied: the reference data is a documentation payload a build reads, like a wire payload, not a configuration or metadata file a person or a tool of xcl's own reads; the pin follows the existing `xcl-ref.txt`.

## Architecture & Design Decisions

One small tool in `xcl-python` reads the package statically and is the only place that knows what "public" means. It is `.github/scripts/api_reference.py`, built on Griffe and docutils, and it has two jobs: it fails naming every public name or member with no doc comment (a CI check, in a job beside `check-package`), and it writes the API as structured data (the release job attaches that data to the GitHub release as `xcl-lang-<version>-reference.json`). The site renders that data through its own Astro page at `/reference/python/`, inside `Shell.astro`, so the reference inherits the nav and footer without any styling work. The site never parses Python and its deployment gains no Python toolchain: an npm `prebuild`/`predev` script downloads the asset named by a one-line pin, `xcl-python-ref.txt`, into a gitignored `src/generated/` file. Griffe and docutils are pinned in `.github/scripts/requirements-docs.txt` and installed only by the two CI jobs, so `xcl-lang`'s dependencies do not change.

Public means `xcl.__all__`, recursing into each exported module's own `__all__`, plus the module `xcl.aio` and its `__all__`, which is not an export but which the spec requires as the asyncio client. Members are constructors (a class's doc comment documents its constructor, since a dataclass has none of its own), methods and properties, including those inherited from the package's own classes. Class data attributes, such as dataclass fields and enum members, are listed but not required to carry doc comments, which matches the spec's definition of a member. Constants are checked, so seven names gain attribute doc comments. Doc comments stay in the package's reStructuredText style, and `:param:`, `:returns:` and `:raises:` fields are added on the clients, registries and module functions. The tool's data is language-neutral (`schema`, `language`, `package`, `version`, and a list of sections holding entries), so the later TypeScript reference can reuse the same page shape.

Each entry's anchor is its full dotted name, `#xcl.Config.apply`, which is stable across releases while a name exists. The guide links to entries through a `PyRef` Astro component that looks the name up in the data. An unknown name renders as plain code with a build warning, and fails only under `XCL_REFERENCE_STRICT=1`, which the site's reference check workflow sets, so a renamed method is reported without stopping the release bump. Doc comment cross-references (`:class:`, `:meth:` and friends) become links when they resolve to an entry and plain code when they do not. Because the guide is prose checked by a person, this check is the only automated link guard: per the xcl testing convention no test reads website text.

The release chain extends the one between `xcl` and `xcl-python`. After the release job in `xcl-python` publishes to PyPI and creates the GitHub release with the asset, it dispatches `xcl-python-release` with the tag to `xcl-website`. A new `reference-bump.yml` there writes the tag to the pin, runs `npm run build` as a gate, and pushes to `main` only if the build passes, using the release bot App token so the push starts `deploy.yml`. This is a push rather than a pull request because the website has no labels, no protection and no required checks; the build gate is what answers the spec's risk of a release publishing straight to xcl.dev. The gate fails on a bad asset or a guide link to a removed name, in which case the site stays on the previous version and the failed run is visible. The first website merge can only name a version that already has an asset, so `xcl-python` ships first and the website's pin starts at that release.

The guide is a new page, `/using-xcl-with-python/`, the URL the package's README already uses, with Python code only and no tabs. It builds one program section by section and ends with the complete listing, synchronous first and then a short asyncio section saying when to use it. Alternatives, including generating on the website and pdoc, are compared in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **API reference tool (new, `xcl-python`)** — owns the definition of a public name and member. Reads the package without importing it and either reports every undocumented name or member, or writes the reference data, including the package version and each entry's rendered description. Used by the doc comment check and by the release job.
- **Doc comment check (new CI job, `xcl-python`)** — runs the tool in check mode beside the package check, and is one of the jobs the `result` check requires. Replaces the runtime docstring test, which could not see constants.
- **Package doc comments (changed, `xcl-python`)** — complete across every public name and member, in the existing reStructuredText style, with parameter, return and error fields on the clients, the registries and module functions. Text only.
- **Release workflow (changed, `xcl-python`)** — also generates the reference for the tag, stops the release before PyPI if generation fails, attaches the data to the GitHub release, and dispatches the website once that release exists.
- **Reference data loader (new, `xcl-website`)** — before a build, fetches the release asset named by the pinned tag, or copies a local file for development; refuses data whose version differs from the pin. Everything the site knows about the Python API comes through this.
- **Reference page (new, `xcl-website`)** — renders the data at `/reference/python/` in the site's layout: the version, an index, then one anchored entry per name with its members, and the two clients labelled synchronous and asyncio.
- **`PyRef` link component (new, `xcl-website`)** — used by the guide and home page to link to an entry by full name; an unknown name is a warning, and a failure in the site's reference check workflow.
- **Pin and bump workflow (new, `xcl-website`)** — one-line pin of the documented release, and a dispatched workflow that moves it, gates on a site build, and pushes as the release bot so the existing deployment runs.
- **Python guide (new, `xcl-website`)** — one page taking a reader from `pip install` to a working program, linking to topic pages and to reference entries through `PyRef`.
- **Site navigation and home page (changed, `xcl-website`)** — a Reference menu shared later by TypeScript, the guide under Guides, and links to both from the home page.
- **READMEs (changed, `xcl-python` and `xcl`)** — the package README's links move to the real URLs and the xcl README links to both.

## Data Structures & Interfaces

**Reference data** is the contract between `xcl-python` and the site: a JSON document, generated by the release and read only by the site's build. It holds the package's version and its entries, grouped into sections, with descriptions already rendered to HTML. No field names Python, so the TypeScript reference can produce the same shape.

```
Reference {
  schema: 1
  language: "python"
  package: "xcl-lang"
  version: "0.15.0"
  sections: Section[]
}
Section { id, title, entries: Entry[] }
Entry {
  id: "xcl.Config.apply"         // the full dotted name; the page anchor
  name: "apply"
  kind: "module" | "class" | "function" | "method" | "property" | "constant" | "constructor" | "attribute"
  label?: "synchronous" | "asyncio"
  signature?: string
  description: html
  parameters?: { name, type?, default?, description?: html }[]
  returns?: { type?, description?: html }
  raises?: { type, description?: html }[]
  inheritedFrom?: string         // the base class, for an inherited member
  members?: Entry[]
}
```

**API reference tool's command line** is `api_reference.py [--check] [--out PATH] [--source DIR]`: `--check` prints each undocumented name and exits non-zero, `--out` writes the data, and `--source` names the directory holding the package, the checkout by default.

**Pin file** `xcl-python-ref.txt` holds one tag, such as `v0.15.0`, the same form as `xcl-ref.txt`.

**Release dispatch** is `repository_dispatch` event `xcl-python-release` to `xcl-website` with `client_payload.tag`.

**`PyRef` component** takes `name`, the entry's full dotted name, and an optional `label`. Its link is `/reference/python/#<name>`.

`XCL_PYTHON_REFERENCE` is a development override: the path of a reference data file the site uses instead of downloading the pinned release's.

## Implementation Detail

Three patterns are new; the rest follows what each repo already does.

The first new pattern is a static reading of the package. The tool never imports `xcl`, so it needs no engine, and it sees string literals following assignments, which runtime introspection cannot. It uses Griffe with its Sphinx docstring parser for the structure of a doc comment and docutils for the prose, with local roles for `:class:` and friends that resolve against the set of entries. A docstring that docutils cannot parse is a failure, so bad markup is caught in the same check as a missing doc comment.

The second is generated site content. Nothing in the site's source holds API text: the page is a template over the data, and the build fails if the data is missing, carries a different version from the pin, or lacks a name the guide links to. The file is gitignored, like `dist/`. A contributor without network access can point `XCL_PYTHON_REFERENCE` at a file from a local run of the tool.

The third is a release that reaches across to the site with a build as the gate. It follows `engine-bump.yml`: a dispatch, an App token, the bot's commit identity, an idempotent no-op when the pin is already there and a concurrency group. It differs in pushing directly, and in refusing to move the pin to an older version than it holds.

Elsewhere the changes follow existing shapes. The release workflow gains jobs in its existing style, with the version written into the source from the tag as the sdist job does. The guide is an MDX page using `Shell`, `Hero`, `Prose` and `CtaBanner` like the other guides, with the Reference menu a dropdown of the existing shape. The new page is added second under Guides, so the Guides label still opens the same page, per the website's dropdown gotcha.

## Dependencies

- Design documents this plan was built on: none. The spec carries no design references.
- Upstream specs: none must land first. This spec has no dependencies in the epic; the later TypeScript reference reuses the Reference menu and the data shape this introduces.
- **`xcl-python` release before the website's first merge** — the website's first pin must name a release carrying the reference asset, so the package's work is released before the website merges. The release plan orders this.
- **Griffe 2.4.1 (new, CI only)** — statically parses the package, including `__all__`, inherited members and dataclass constructors. Pinned in `requirements-docs.txt`; never a dependency of `xcl-lang`.
- **docutils 0.23 (new, CI only)** — renders reStructuredText doc comments to HTML. Pinned alike.
- **The release bot GitHub App** — already in use by `xcl-python`. A person installs it on `xcl-website` and adds `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` to that repository; none exist there today.
- **GitHub releases** — hosts the reference asset, fetched by the site over HTTPS. No new service.
- **Astro 5, MDX, Node 22 (existing)** — no changes; the fetch script uses Node's built-in `fetch`.
- **PyPI's `xcl-lang` and the signed `xcl-plugin-docker` release** — the guide installs both; the implementing task checks them against the latest release, and the Docker plugin needs a local Docker daemon for the walkthrough.

## Testing Approach

The API reference tool gets unit tests, since it holds the only logic that decides what is public and what is missing. They run against small packages created in a temporary directory, never the repository's own source, as the xcl testing convention requires, and they run from the same `check-docs` job. The tests guarantee that a package with one undocumented public method fails and names that method, a fully documented one passes, an inherited member and a documented constant are handled, a name outside the export list is ignored, the data contains the entries, and a package version is carried through. Positive and negative cases are separate test functions, and none are table-driven.

The tool's other half is exercised for real: it runs over `xcl` itself in the check job, and the reference is built from the actual package by the release job. The site's generated page and the `PyRef` guard are verified by building the site, which fails if the data is missing, and by the reference check workflow, which also fails if a guide link's name has gone. There are no new site tests: the site has no test harness and the convention forbids tests over page text.

Workflows and documentation are checked by running them and by review, not by tests over their files. The Python test suite is unaffected, since the code does not change.

Deliberate gaps: no automated check that the guide's code runs, because it needs PyPI, a Docker daemon and a network, and a test that extracted the guide's code would read documentation text; a person runs it instead.

Success metrics:

- "A Python developer can answer what a method takes and returns from the site without opening the source": **Manual — captured in the implementation test plan** (a person looks up several methods on the built page).
- "A new Python developer reaches a working apply against a Go plugin by following the guide alone": **Manual — captured in the implementation test plan** (the guide is followed in a clean virtual environment).
- "The published reference shows the latest package version within one hour of each release": **Manual — captured in the implementation test plan** (observed across a real release).

Manual reviews and checks:

- The reference page shows the nav and footer, labels both clients and shows the release's version, and every internal link opens: **Manual — captured in the implementation test plan**.
- Every name in the export list has an entry, with members: **Manual — captured in the implementation test plan**.
- A doc comment changed in a local package appears in a site built against the regenerated data: **Manual — captured in the implementation test plan**.
- A real release moves the pin and updates the site with no hand commit: **Manual — captured in the implementation test plan**.
- Every example in the guide, tried against the latest release, behaves as the guide says, and anything marked not yet supported is unsupported: **Manual — captured in the implementation test plan**.
- The guide meets the wording rules: no "superset", "interpolation" or "Terraform", XCL described as its own language, and "resource" only in code identifiers: **Manual — captured in the implementation test plan**.
- The navigation, home page and both READMEs link to the reference and guide, and each link opens the right page: **Manual — captured in the implementation test plan**.

## Milestones & Tasks

### Milestone 1: Every public name in the Python package is documented, and new gaps fail the checks

**What changes**: Every name the package exports and every public method and property has a doc comment, the clients and registries say what their parameters and results are, and a pull request that adds an undocumented public name fails its checks by naming it. Each release also produces the package's API as structured data. Readers see no site change yet; maintainers get a tool that keeps the documentation whole.

**Validation point**: the doc comment check passes over the package and fails, naming the method, when one doc comment is removed; running the tool writes reference data carrying the package's version.

#### - [x] Task: Add the API reference tool
**Id:** ebd3dbc7-930e-45d1-a358-8d28c144de30
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

A script that reads the package without importing it and decides what is public. It either reports every undocumented name or member, or writes the API as structured data. It is the one definition of a public name for both the check and the site.

*Technical detail:* [context.md#task-add-the-api-reference-tool](./context.md#task-add-the-api-reference-tool)

**Acceptance criteria**:
- [x] The tool lists every public name and every public member (a constructor, method or property, including inherited ones; class data attributes are listed in the reference but not checked, and a class's doc comment documents its constructor) lacking a doc comment, and exits with a failure
- [x] With every name documented it exits successfully
- [x] The tool writes reference data holding every public name, its signature, description, parameters, return value, errors and members, and the package version
- [x] Inherited members appear on a class, and names outside the export lists do not
- [x] The tool's dependencies are not dependencies of `xcl-lang`

#### - [x] Task: Complete the package's doc comments
**Id:** 908eec92-a30a-4376-b82d-b5eec4be0d01
**Repo:** xcl-python
**Depends on:**
- ebd3dbc7-930e-45d1-a358-8d28c144de30 — Add the API reference tool
**Execution:** agent

Fills every remaining gap in the package's doc comments, including the seven constants the tool reports, and adds parameter, return and error fields to the two clients, the registries and module functions. Text only, so behaviour stays the same.

*Technical detail:* [context.md#task-complete-the-packages-doc-comments](./context.md#task-complete-the-packages-doc-comments)

**Acceptance criteria**:
- [x] The tool reports no undocumented public name or member with no exclusions
- [x] The clients' and registries' methods and the module functions document their parameters, return values and errors where they have them
- [x] No code other than doc comments changes

#### - [x] Task: Run the doc comment check in CI
**Id:** 10e2a4ea-c97d-4b0e-b6ec-365eaf2995c9
**Repo:** xcl-python
**Depends on:**
- ebd3dbc7-930e-45d1-a358-8d28c144de30 — Add the API reference tool
- 908eec92-a30a-4376-b82d-b5eec4be0d01 — Complete the package's doc comments
**Execution:** agent

Adds the check as a job beside the package check, so a pull request adding an undocumented public name fails and names it. The runtime docstring test it replaces is removed, since it cannot see constants.

*Technical detail:* [context.md#task-run-the-doc-comment-check-in-ci](./context.md#task-run-the-doc-comment-check-in-ci)

**Acceptance criteria**:
- [x] A pull request with an undocumented public method fails its required checks, naming the method
- [x] The package's own check run passes
- [x] A local Makefile target runs the same check

#### - [x] Task: Publish the reference with each release
**Id:** eea8e109-1cc0-4faf-88d2-19715b7cc4e1
**Repo:** xcl-python
**Depends on:**
- ebd3dbc7-930e-45d1-a358-8d28c144de30 — Add the API reference tool
**Execution:** agent

Makes each release generate the reference for its tag, stop before PyPI if generation fails, attach the data to the GitHub release, and tell the website about the new tag.

*Technical detail:* [context.md#task-publish-the-reference-with-each-release](./context.md#task-publish-the-reference-with-each-release)

**Acceptance criteria**:
- [x] A release attaches reference data whose version is the released version to the GitHub release
- [x] A failure generating the reference stops the release before anything reaches PyPI
- [x] Once the GitHub release exists, the website is dispatched with the tag

### Milestone 2: The Python reference is published on xcl.dev and follows each release

**What changes**: A Python developer can open `/reference/python/` on the site and find every public name, with its signature, description, members and the two clients labelled, along with the package version it describes. Each new package release reaches the page by itself, after a site build has proved it works. The site's navigation gains a Reference menu.

**Validation point**: the built site serves the page with the site's navigation and footer, showing the pinned version; a dispatched bump moves the pin only after a successful build.

#### - [x] Task: Load the reference data into the site
**Id:** 67201ccb-09af-47cf-9776-b337e787d427
**Repo:** xcl-website
**Depends on:**
- ebd3dbc7-930e-45d1-a358-8d28c144de30 — Add the API reference tool
**Execution:** agent

Adds the pin, and a script run before every build and dev server that fetches the pinned release's reference data, or copies a local file during development. Nothing else in the site changes yet, and the deployment still runs the same build.

*Technical detail:* [context.md#task-load-the-reference-data-into-the-site](./context.md#task-load-the-reference-data-into-the-site)

**Acceptance criteria**:
- [x] A build fetches the pinned release's data and keeps it out of git
- [x] A build fails with a clear message when the data is missing or its version differs from the pin
- [x] Setting the development override uses a local file instead of downloading
- [x] The site's deployment workflow is unchanged

#### - [x] Task: Render the reference page
**Id:** ede26098-bb56-4f03-aff7-8a0144c1a347
**Repo:** xcl-website
**Depends on:**
- 67201ccb-09af-47cf-9776-b337e787d427 — Load the reference data into the site
**Execution:** agent

Builds the page at `/reference/python/` from the data, inside the site's layout. It shows the version, an index, and one anchored entry per name with its members, with the two clients labelled.

*Technical detail:* [context.md#task-render-the-reference-page](./context.md#task-render-the-reference-page)

**Acceptance criteria**:
- [x] The built page shows the site's navigation and footer
- [x] Every entry in the data has an anchored entry on the page showing its signature and description, and its parameters, return value and errors where documented
- [x] `xcl.Config` is labelled synchronous and `xcl.aio.Config` asyncio
- [x] The version shown equals the data's version

#### - [x] Task: Add the Reference menu and PyRef links
**Id:** 41a1194e-de88-42b4-91b7-054459bba6c9
**Repo:** xcl-website
**Depends on:**
- ede26098-bb56-4f03-aff7-8a0144c1a347 — Render the reference page
**Execution:** agent

Adds a Reference menu to the site's navigation, ready for TypeScript to join later, a `PyRef` link component that warns when given a name not in the reference, and a check workflow that fails on one.

*Technical detail:* [context.md#task-add-the-reference-menu-and-pyref-links](./context.md#task-add-the-reference-menu-and-pyref-links)

**Acceptance criteria**:
- [x] The Reference menu opens the Python reference
- [x] A `PyRef` link goes to the named entry
- [x] A name not in the reference warns in a normal build and the site still builds
- [x] A name not in the reference fails the reference check workflow

#### - [x] Task: Add the pin bump workflow
**Id:** 57f994cb-1b54-4aaf-9400-7749301b93ef
**Repo:** xcl-website
**Depends on:**
- 67201ccb-09af-47cf-9776-b337e787d427 — Load the reference data into the site
**Execution:** agent

Adds the workflow that moves the pin when `xcl-python` releases, proves the site builds, and pushes as the release bot. A failing build leaves the site on the version it has.

*Technical detail:* [context.md#task-add-the-pin-bump-workflow](./context.md#task-add-the-pin-bump-workflow)

**Acceptance criteria**:
- [x] A dispatched release moves the pin and pushes to main only after the site builds
- [x] A build failure leaves the pin and site unchanged
- [x] A dispatch for a version already pinned or older does nothing

#### - [x] Task: Give the website the release bot's access
**Id:** 922a027d-f5e9-4e10-ac0e-feeaa0a7bdbb
**Repo:** xcl-website
**Depends on:**
- 57f994cb-1b54-4aaf-9400-7749301b93ef — Add the pin bump workflow
**Execution:** human — needs admin access to the GitHub organisation and repository settings

Installs the release bot GitHub App on `xcl-website` and adds its secrets, so the release can dispatch the website and the bump can push.

*Technical detail:* [context.md#task-give-the-website-the-release-bots-access](./context.md#task-give-the-website-the-release-bots-access)

**Acceptance criteria**:
- [ ] The release bot App is installed on `xcl-website` with contents write
- [x] `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` are set on `xcl-website`

### Milestone 3: A guide takes a Python developer from nothing to a working program

**What changes**: A new guide page walks through installing from PyPI to a working program using a Go plugin, synchronous first and then asyncio, linking each name it introduces to the reference. The guide and reference are reachable from the navigation, the home page and both READMEs.

**Validation point**: following the guide in a clean environment produces a working program, the build fails if a guide link names something absent from the reference, and the wording rules are met.

#### - [x] Task: Write the Python guide
**Id:** 8de9b791-d6c1-47e5-9c23-bb5d218f66b0
**Repo:** xcl-website
**Depends on:**
- 41a1194e-de88-42b4-91b7-054459bba6c9 — Add the Reference menu and PyRef links
**Execution:** agent

Writes the guide that takes a Python developer from `pip install` to a working program using a Go plugin, synchronous first and then asyncio, with every API name linked to the reference. Everything in it is checked against the latest released package.

*Technical detail:* [context.md#task-write-the-python-guide](./context.md#task-write-the-python-guide)

**Acceptance criteria**:
- [x] Following the guide from the top with only its code gives a program that creates a configuration, registers an entity type and a function, uses a Go plugin, applies, keeps state between runs, finds an entity by address and by type, handles a configuration error, turns on logging, prints an entity and destroys it, and that runs the asyncio apply
- [x] The synchronous API comes before asyncio, and the asyncio section says when to use it
- [x] The guide links to the registries, plugins, state and events pages, and each API name links to its reference entry
- [x] The guide has Python code only and meets the wording rules
- [x] Anything planned but not built is marked as not yet supported

#### - [x] Task: Link the guide from the site
**Id:** 355d703e-1ddf-4d7a-8317-a26a03d2f376
**Repo:** xcl-website
**Depends on:**
- 8de9b791-d6c1-47e5-9c23-bb5d218f66b0 — Write the Python guide
**Execution:** agent

Puts the guide in the Guides menu and links the guide and reference from the home page, where every reader sees them. Adds the new pages to the site's README.

*Technical detail:* [context.md#task-link-the-guide-from-the-site](./context.md#task-link-the-guide-from-the-site)

**Acceptance criteria**:
- [x] The guide appears under Guides, and the Guides label still opens the same page
- [x] The home page links to the guide and reference outside any language tab
- [x] Every new link opens the right page

#### - [x] Task: Update the package README's links
**Id:** 8d5dafab-a44c-42ee-bd99-2ae8d8dd0f91
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Replaces the README's dead `/api/python/` link with the reference's URL and keeps the guide's. Done before the release so the published package carries it.

*Technical detail:* [context.md#task-update-the-package-readmes-links](./context.md#task-update-the-package-readmes-links)

**Acceptance criteria**:
- [x] The README links to `/reference/python/` and `/using-xcl-with-python/`
- [x] The README says the reference follows releases

#### - [x] Task: Link the guide and reference from the xcl README
**Id:** 7c6275a4-b334-4cab-af1a-ec9ab49248d1
**Repo:** xcl
**Depends on:** none
**Execution:** agent

Adds the guide and reference to the xcl README's documentation list and its Python section.

*Technical detail:* [context.md#task-link-the-guide-and-reference-from-the-xcl-readme](./context.md#task-link-the-guide-and-reference-from-the-xcl-readme)

**Acceptance criteria**:
- [x] The xcl README links to the reference and the guide
- [x] Each link opens the right page

#### - [ ] Task: Release the package with the reference asset
**Id:** 11c7bfbb-0d6c-44d0-b617-2826581d9c56
**Repo:** xcl-python
**Depends on:**
- 908eec92-a30a-4376-b82d-b5eec4be0d01 — Complete the package's doc comments
- 10e2a4ea-c97d-4b0e-b6ec-365eaf2995c9 — Run the doc comment check in CI
- eea8e109-1cc0-4faf-88d2-19715b7cc4e1 — Publish the reference with each release
- 8d5dafab-a44c-42ee-bd99-2ae8d8dd0f91 — Update the package README's links
**Execution:** human — releasing is action outside the repo

Merges the `xcl-python` work with a `patch` label so a release publishes the completed doc comments and the first reference asset. The website cannot merge before this exists.

*Technical detail:* [context.md#task-release-the-package-with-the-reference-asset](./context.md#task-release-the-package-with-the-reference-asset)

**Acceptance criteria**:
- [ ] A released version on PyPI has a GitHub release carrying its reference data

#### - [ ] Task: Set the site's first pin
**Id:** 952c0be3-d1b9-48d0-82a8-8b99b7bb0339
**Repo:** xcl-website
**Depends on:**
- 11c7bfbb-0d6c-44d0-b617-2826581d9c56 — Release the package with the reference asset
- 355d703e-1ddf-4d7a-8317-a26a03d2f376 — Link the guide from the site
- 41a1194e-de88-42b4-91b7-054459bba6c9 — Add the Reference menu and PyRef links
**Execution:** human — requires a just-released version, and merging ahead of the release would deploy a broken site

Writes the released tag into the pin on the website's branch, then merges so the site deploys. Later releases move the pin automatically.

*Technical detail:* [context.md#task-set-the-sites-first-pin](./context.md#task-set-the-sites-first-pin)

**Acceptance criteria**:
- [ ] The pin names the release with the reference asset
- [ ] xcl.dev shows the reference and guide at that version

## Open Questions

No implementation-time-only uncertainties remain. Whether the Docker plugin release named in the guide still installs against the latest package is a check the guide task makes when it runs, not an open question; if it no longer does, the guide names the release that does.

## Out of Scope

- A reference or guide for TypeScript: the later spec `20261110084722-9e301981-typescript-sdk-reference` in this epic.
- A Go guide; Go's reference stays on pkg.go.dev.
- Reference pages for older package versions: only the pinned version is published.
- Rewriting the package's README: only its links and one sentence change.
- Any change to the package's behaviour: only doc comments, checks, the README and release workflows change.
- Documenting names outside the export lists, such as `xcl.diff.Dependency` or `xcl.registry.Registration`, or standard library members a class inherits.
- Field lists on every member: they are added on the clients, registries and module functions.
- A site-wide link checker; broken guide references fail the reference check workflow, and the rest is checked by hand.

## Changelog

### 2026-11-10 — Task: Add the API reference tool

**What was done**: Added `.github/scripts/api_reference.py`, which reads the package statically with Griffe and either reports every public name or member with no doc comment or writes the API as the JSON the site renders, with docutils turning doc comments into HTML and cross-references into links. Griffe and docutils are pinned in `requirements-docs.txt`, and the tool has 32 tests over small fixture packages.

**Deviations**: Goal check attempt 1 judged criterion 1 not met because class data attributes are not checked, which the plan and spec exclude on purpose. The user chose to correct the criterion's wording to the spec's definition of a member (constructor, methods and properties, including inherited ones, with class data attributes listed but not checked); the code did not change. The plan's line numbers for `release.yml` and `engine-bump.yml` are stale; the named jobs and steps were matched by name.

**Amendments**: Plan `plan.md`, this task's acceptance criterion 1, reworded as above at the user's direction.

**Files changed**:
- `xcl-python: .github/scripts/api_reference.py`
- `xcl-python: .github/scripts/requirements-docs.txt`
- `xcl-python: .github/scripts/test_api_reference.py`

**Discoveries**: `griffe.load` tries the name as a relative path first, so an `xcl` directory in the current directory silently beats `search_paths`; the tool passes `try_relative_path=False`. docutils' `publish_parts` calls `sys.exit` on a halt unless `traceback` is set, so the setting is what lets bad markup be reported as a problem. The html5 writer emits inline literals as `<span class="docutils literal">`, so a small translator makes them `<code>`.

### 2026-11-10 — Task: Complete the package's doc comments

**What was done**: Added doc comments to the seven undocumented constants and `:param:`, `:returns:` and `:raises:` fields to `xcl.Config`, `xcl.aio.Config`, the registries and the module functions. Only doc comments changed, confirmed by comparing each file's AST to HEAD with doc comments removed.

**Deviations**: The goal check took two rounds, on criterion 2: each attempt named errors one level further down the call chain, so the field lists grew from direct raises to lookup errors and then engine start-up errors. After round 1 the user chose `keep_fixing` and placed `EngineNotFoundError` and `EngineVersionError` on `engine_version` and both `Config` classes; they are also on `aio.Config.open`, which is where the asyncio client starts the engine. Attempt 1 of round 2 also judged criterion 1 not met over class attributes, the point the user settled for task 1; no attribute doc comments were added, the wording of this criterion was not changed, and the next attempt judged it met. A small fix to the tool from task 1 went in here: a variadic parameter no longer shows the default Griffe gives it.

**Files changed**:
- `xcl-python: xcl/__init__.py`
- `xcl-python: xcl/_logbridge.py`
- `xcl-python: xcl/_print.py`
- `xcl-python: xcl/_types.py`
- `xcl-python: xcl/_version.py`
- `xcl-python: xcl/aio.py`
- `xcl-python: xcl/config.py`
- `xcl-python: xcl/highlight/_css.py`
- `xcl-python: xcl/highlight/_formatters.py`
- `xcl-python: xcl/highlight/_kinds.py`
- `xcl-python: xcl/highlight/_stream.py`
- `xcl-python: xcl/highlight/_style.py`
- `xcl-python: xcl/highlight/_theme.py`
- `xcl-python: xcl/highlight/_vscode.py`
- `xcl-python: xcl/mask.py`
- `xcl-python: xcl/prettylog.py`
- `xcl-python: xcl/registry.py`
- `xcl-python: .github/scripts/api_reference.py`

**Discoveries**: The asyncio client starts the engine on its first operation, not in its constructor, unlike `xcl.Config`.

### 2026-11-10 — Task: Run the doc comment check in CI

**What was done**: Added a `check-docs` job to `test.yml`, required through the `result` job, that runs the API reference tool's check and its tests, added a `make check-docs` target, and removed the runtime docstring test the check replaces.

**Deviations**: None.

**Files changed**:
- `xcl-python: .github/workflows/test.yml`
- `xcl-python: Makefile`
- `xcl-python: tests/unit/test_docstrings.py`

**Discoveries**: None.

### 2026-11-10 — Task: Publish the reference with each release

**What was done**: The release workflow gained a `reference` job that writes the reference for the tag and that `publish` needs, the `github-release` job attaches the data to the release, and a `dispatch-website` job tells `xcl-website` about the tag.

**Deviations**: The plan's line numbers for `release.yml` are stale; each change was placed by job name.

**Files changed**:
- `xcl-python: .github/workflows/release.yml`

**Discoveries**: None.

### 2026-11-10 — Task: Load the reference data into the site

**What was done**: Added the `xcl-python-ref.txt` pin and a script run before every build and dev server that downloads the pinned release's reference data, or copies a local file named by `XCL_PYTHON_REFERENCE`, into a gitignored `src/generated/`. It fails with a clear message when the data is missing or its version differs from the pin.

**Deviations**: None. The pin holds `v0.14.1` until the first release carrying the asset, as planned, so a build without the override fails on the branch until the first-pin task.

**Files changed**:
- `xcl-website: xcl-python-ref.txt`
- `xcl-website: scripts/fetch-python-reference.mjs`
- `xcl-website: package.json`
- `xcl-website: .gitignore`
- `xcl-website: Makefile`
- `xcl-website: README.md`

**Discoveries**: None.

### 2026-11-10 — Task: Render the reference page

**What was done**: Added the page at `/reference/python/`, inside `Shell`, showing the package's version, an index by section and one anchored entry per name with its signature, description, parameters, return value, errors and members, with the two clients labelled.

**Deviations**: Section containers use `section-<id>` as their anchors, because a section and the module entry at its head share a name (`xcl`, `xcl.aio`).

**Files changed**:
- `xcl-website: src/lib/pythonReference.ts`
- `xcl-website: src/pages/reference/python.astro`
- `xcl-website: src/components/reference/Entry.astro`
- `xcl-website: src/styles/global.css`

**Discoveries**: None.

### 2026-11-10 — Task: Add the Reference menu and PyRef links

**What was done**: Added a Reference dropdown holding Python to the navigation, a `PyRef` component that links a full dotted name to its reference entry and warns on an unknown one, and a `reference-check.yml` workflow whose strict build fails on an unknown name.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/components/PyRef.astro`
- `xcl-website: .github/workflows/reference-check.yml`

**Discoveries**: None.

### 2026-11-10 — Task: Add the pin bump workflow

**What was done**: Added `reference-bump.yml`, which on a dispatch from `xcl-python` writes the tag to the pin, builds the site and pushes to `main` as the release bot only if the build passes, and does nothing for a tag already pinned or older.

**Deviations**: The tag check accepts only `vX.Y.Z`, stricter than the `v*` check in `engine-bump.yml`, because the tag is compared and used in a URL.

**Files changed**:
- `xcl-website: .github/workflows/reference-bump.yml`

**Discoveries**: None.

### 2026-11-13 — Task: Give the website the release bot's access

**What was done**: A person installed the release bot GitHub App on `xcl-website` and added `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` to it; the orchestrator confirmed both secrets with `gh secret list`.

**Deviations**: Accepted shortfall. The goal check judged criterion 1 (the App installed on `xcl-website` with contents write) not met over three attempts, because the App's repository selection cannot be read with the local token and `reference-bump.yml` cannot run until it is on the website's `main`. The user accepted the task on their own confirmation that the App is installed; criterion 1 stays unticked. The first run of `reference-bump.yml` after merge is the check that the App can push.

**Files changed**:
- none (repository settings on GitHub)

**Discoveries**: The release bot App, `xcl-release-bot`, is installed on the organisation for selected repositories, so each repository it acts on has to be added to its selection.

### 2026-11-13 — Task: Write the Python guide

**What was done**: Added `/using-xcl-with-python/`, which builds one program a section at a time, from `pip install xcl-lang` through a registered type and function, the signed Docker plugin, apply, state, lookups, a configuration error, logging, printing and destroy, then shows the complete listing and an asyncio version. Every package name in the prose links to the reference through `PyRef`.

**Deviations**: None. The listings were generated from the files that were run in a clean virtual environment against PyPI's `xcl-lang` 0.14.1 and `xcl-plugin-docker` v0.4.1 (still the latest) on a local Docker engine. Goal check attempt 1 asked for links on a few names in the asyncio and logging prose; they were added.

**Files changed**:
- `xcl-website: src/pages/using-xcl-with-python.mdx`

**Discoveries**: `import xcl` does not load `xcl.aio`, so the asyncio client needs its own import. The asyncio client starts the engine on its first operation, not in its constructor.

### 2026-11-13 — Task: Link the guide from the site

**What was done**: The guide is second under Guides, so the Guides label still opens `/events/`. The home page links the guide and the reference in a line after the Install tabs, and the site's README lists both pages.

**Deviations**: None.

**Files changed**:
- `xcl-website: src/components/Nav.astro`
- `xcl-website: src/pages/index.mdx`
- `xcl-website: README.md`

**Discoveries**: None.

### 2026-11-13 — Task: Update the package README's links

**What was done**: The README's dead `/api/python/` link now points to `/reference/python/`, the guide link is kept, and the releases section says a release attaches the reference and moves the one on xcl.dev.

**Deviations**: None.

**Files changed**:
- `xcl-python: README.md`

**Discoveries**: None.

### 2026-11-13 — Task: Link the guide and reference from the xcl README

**What was done**: The xcl README's documentation list and its "Other languages" section link the Python guide and the Python reference.

**Deviations**: None.

**Files changed**:
- `xcl: README.md`

**Discoveries**: None.

### 2026-11-13 — Task: Release the package with the reference asset

**What was done**: Nothing in the repositories: this is the release a person makes after the `xcl-python` work merges.

**Deviations**: Skipped at the user's decision ("human release step, done after merge per the release plan"). The goal check judged its one criterion not met over three attempts, since no release exists yet; heading and criterion stay unticked. Steps for the release plan: merge the `xcl-python` work with a `patch` label; check every job of the release run passes, including the `reference` job and the install checks; check the GitHub release carries `xcl-lang-<version>-reference.json`; check the `dispatch-website` job ran.

**Files changed**:
- none

**Discoveries**: None.

### 2026-11-13 — Task: Set the site's first pin

**What was done**: Nothing in the repositories: the first pin is written by a person once the first `xcl-python` release carrying the reference asset exists.

**Deviations**: Skipped at the user's decision ("human release step, done after merge per the release plan"). The goal check judged both criteria not met over three attempts, since `xcl-python-ref.txt` still holds the placeholder `v0.14.1`, which has no asset; heading and criteria stay unticked. Steps for the release plan, after the release task: write the released tag into `xcl-website:xcl-python-ref.txt` on the website's branch, check `npm run build` passes and the Reference check passes, then merge and push so `deploy.yml` deploys; check xcl.dev shows the reference at that version and the guide. Do not push the website's work to `main` before this, or `deploy.yml` and `reference-check.yml` fail at the reference download.

**Files changed**:
- none

**Discoveries**: None.
