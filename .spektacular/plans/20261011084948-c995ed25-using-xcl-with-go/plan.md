---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Plan: 20261011084948-c995ed25-using-xcl-with-go

<!-- Metadata -->
<!-- Created: 2026-10-11T09:08:50Z -->
<!-- Commit: xcl bdba483, xcl-website 55b91d0 -->
<!-- Branch: main -->
<!-- Repository: github.com/jumppad-labs/xcl, github.com/jumppad-labs/xcl-website -->

## Overview

A Go developer adopting XCL has no single walkthrough of the core API now that the xcl README is an overview. This plan adds a "Using XCL with Go" guide to xcl.dev that builds one program from empty to working (define entity types, create a config, check, apply, find entities, keep state, destroy), checked by hand against the latest tagged release, and makes it reachable from the site's navigation, the home page and the xcl README. It is for Go developers building XCL into their applications, and it points to the existing topic pages and the generated API docs for detail.

## Conventions

- **Never write tests that inspect repository files (testing-and-mocking)** — the guide, nav, home page and README links are checked by human review and a hand-run scratch program; no test reads the new page or README.
- **Never modify dependency packages (never-modify-dependencies)** — the throwaway verification module pulls xcl at the latest tag from the module proxy; nothing in the module cache or the xcl source is edited to make an example work, and any gap found is reported, not fixed (spec non-goal).
- **Go code style (code-style)** — the guide's Go snippets are gofmt-formatted, handle every error explicitly, use descriptive names and `any` rather than `interface{}`, since readers will copy them.

## Architecture & Design Decisions

The guide is one new MDX page in **xcl-website**, `src/pages/using-xcl-with-go.mdx`, served at `/using-xcl-with-go/` and titled "Using XCL with Go". It follows the layout every existing guide uses (`Shell.astro` layout, `Hero`, `Prose`, a closing `CtaBanner`; see `src/pages/registries.mdx`) and is written as one continuous walkthrough that builds a single program, `main.go`, against one configuration file, `main.xcl`. In order it covers: adding the module to a Go 1.25 project; defining two entity types as Go structs that embed `types.ResourceBase` (`PostgreSQL` for `database "postgres" "main"` and `App` for `app "web"`, with `app.web` reading values from `database.postgres.main`); creating a config with `registry.NewLocal`, `RegisterType` and `xcl.NewConfig(xcl.WithRegistry(...), xcl.WithStatePath(...))`; parsing and checking the configuration with `Validate` and reading `*errors.ConfigError`; applying it with `Apply` and seeing the references filled in; finding entities by address (`xcl.Find`) and by type (`xcl.FindByType`, `xcl.FindOne`, `xcl.All`) with the lookup errors (`xcl.ErrNotFound`, `xcl.ErrNotUnique`); keeping state between runs (`WithStatePath`, then `Load` in a later run so lookups answer from saved state, and the next `Apply` destroying blocks removed from configuration); and destroying everything with `Destroy`. It ends with a "Where next" section linking the registries, plugins, state and events pages and pkg.go.dev.

The walkthrough uses **registered types only, no plugin**. Registered types are applied, saved to state, loaded back, found and destroyed without any provider (`xcl:config.go` Destroy and Load doc comments), so the reader can run every step with only the code the page shows, which is what the "Guide is complete" acceptance criterion needs. Plugins, providers and computed values are named once and linked out (`/examples/plugins/`, `/plugin-template/`, `/github-registry/`) instead of being taught here. The guide's scope stops at the core Go API workflow: struct tags beyond the plain `xcl:"name"` form, optional fields, nested blocks, variables, modules and functions belong to the sibling language reference and are not explained here; the guide links to that reference only if its page exists when this spec is implemented. Lookups are shown in the package-level function form (`xcl.Find[T](c, ...)`), which compiles on Go 1.25, with one sentence noting the method form (`c.Find[T](...)`) is available from Go 1.27, matching what the code actually ships (`query_methods_go127.go` build constraint). The removed README sections on querying and on state and destroy are the raw material; the stale "Lifecycle Callbacks" content (`Processable`, `ParseCallback`, `Config.Process`) is not carried over because none of it exists in the code.

Reachability is three small edits. In **xcl-website**, `src/components/Nav.astro` gets "Using XCL with Go" as the first child of the "Guides" dropdown (the dropdown head links to its first child, so "Guides" opens the guide), and `src/pages/index.mdx` links the guide from the end of the "What is XCL?" section and from a button in the closing `CtaBanner`; `README.md`'s Pages table gains the new row. In **xcl**, `README.md`'s "Documentation" list gets the guide as its first entry, and `CHANGELOG.md` gets an entry for this spec. The README otherwise stays the overview it is.

Correctness against the release is checked by hand, not by tests (spec non-goal; convention: no tests that read website pages). During implementation the agent builds the guide's program as a throwaway module outside both repositories against the latest tag and runs it twice (apply, then load and destroy), then deletes it; a person repeats that check before publishing. This beats a multi-page guide (harder to follow top to bottom), a plugin-based walkthrough (needs Docker or a binary) and new state/plugins topic pages (out of scope); see `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **"Using XCL with Go" guide page (new, xcl-website)** — owns the walkthrough of the core Go API: installing the module, defining two entity types, creating a config with a local registry, validating, applying, finding entities by address and by type, keeping state between runs and destroying. It owns one worked program and one configuration that every section builds on, and a "Where next" section. It reuses the site's existing page layout and section components rather than introducing any. It links out to the registries, plugins, state and events pages, the examples and the generated API docs, and is linked to by the components below.
- **Site navigation (changed, xcl-website)** — the Guides dropdown gains the guide as its first entry, so the guide is reachable from every page and the "Guides" label opens it.
- **Home page (changed, xcl-website)** — gains a link to the guide at the end of its "What is XCL?" introduction and a button in its closing call to action, beside the existing example buttons.
- **Website README pages table (changed, xcl-website)** — gains a row for the new page so maintainers know where its source is.
- **xcl README (changed, xcl)** — its Documentation list gains the guide as its first entry; the README otherwise stays an overview.
- **xcl changelog (changed, xcl)** — gains an entry for this spec recording the new guide and the README link, with no breaking changes.
- **Throwaway verification program (temporary, outside both repos)** — a scratch Go module assembled from the guide's code exactly as shown, built against the latest tagged xcl release and run twice to prove the guide works end to end; deleted afterwards and never committed.

## Data Structures & Interfaces

No new types, interfaces or serialization formats are added to the xcl library or the website; this is a documentation change, and the library is not modified (spec non-goal).

The guide does introduce two **example** entity types that live only on the page and in the throwaway verification program. They are the contract every section of the guide builds on, so their shape is fixed here:

```go
// PostgreSQL is registered as database "postgres": written database "postgres" "main",
// referred to as database.postgres.main
type PostgreSQL struct {
	types.ResourceBase `xcl:",remain"`

	Location string `xcl:"location" json:"location"`
	Port     int    `xcl:"port" json:"port"`
	Username string `xcl:"username" json:"username"`
}

// App is registered as app: written app "web", referred to as app.web
type App struct {
	types.ResourceBase `xcl:",remain"`

	DatabaseLocation string `xcl:"database_location" json:"database_location"`
	DatabasePort     int    `xcl:"database_port" json:"database_port"`
}
```

with the configuration they read:

```
database "postgres" "main" { location = "localhost", port = 5432, username = "admin" }
app "web" { database_location = database.postgres.main.location, database_port = database.postgres.main.port }
```

Only library surface that exists in the latest tagged release is used: `xcl.NewConfig`, `xcl.WithRegistry`, `xcl.WithStatePath`, `registry.NewLocal`, `(*registry.Local).RegisterType`, `(*xcl.Config).Validate/Apply/Load/Destroy/Entities`, `xcl.Find/FindByType/FindOne/All`, `xcl.ErrNotFound`, `xcl.ErrNotUnique`, `*errors.ConfigError` and `types.ResourceBase`.

## Implementation Detail

**Page shape follows the existing guides.** The new page uses the same frontmatter, `Hero` introduction, `Prose` body and closing `CtaBanner` as the registries and state masking guides, so a maintainer editing it meets nothing new. Code blocks carry a file title (`main.go`, `main.xcl`) as the other pages do. No new components, styles or site configuration are introduced.

**One program, built up section by section.** The guide's sections, in order: *Before you start* (Go 1.25 and adding the module); *Describe your entities* (the two Go types); *Write the configuration* (`main.xcl` with `database "postgres" "main"` and `app "web"`, explaining type, subtype, name and address); *Create a config* (local registry, `RegisterType`, `NewConfig` with `WithStatePath`); *Check the configuration* (`Validate` and every problem in a `ConfigError`); *Apply it* (`Apply`, references filled in, `ErrEmptyConfiguration` for an empty configuration); *Find entities* (by address with `Find`, by type with `FindByType`, exactly one with `FindOne`, every one of a Go type with `All`, and what `ErrNotFound` and `ErrNotUnique` mean; one note that the same lookups exist as methods from Go 1.27); *Keep state between runs* (what `WithStatePath` writes, `Load` in a later run, a removed block destroyed by the next `Apply`, no state option meaning memory only); *Destroy it* (`Destroy` needs no configuration, registered types never reach a provider); *The whole program* (the complete `main.go` assembled from the sections, so a reader can copy one block); *Where next* (links out). Each section shows only the lines it adds and says in one or two sentences what happens; detail lives in the linked pages.

**The complete program is the source of truth for verification.** The "whole program" block is what the throwaway verification module copies verbatim, alongside the `main.xcl` block. It takes a command argument (`apply`, `status`, `destroy`) so the two-run state story can be exercised with one binary, mirroring how the examples are run.

**Wording rules applied throughout.** XCL is described as its own language. Blocks are "entities" (a block is how an entity is written), never "resources", except inside Go identifiers such as `types.ResourceBase`. The guide does not use "superset", "interpolation" or "Terraform"; references are "resolved" or "filled in". Anything the code does not do is either left out or marked "not yet supported" — the only planned-but-unbuilt item the guide is expected to touch is non-Go implementations, which the guide does not mention, since Go is the only implementation shown.

**Links rather than repeats.** Where a topic has its own page, the guide states the one fact the walkthrough needs and links: registries for other registry kinds and load problems; the plugins example, plugin template and GitHub registry for providers and plugins; state masking for encrypting state; events for logging; pkg.go.dev for the full API; the language reference (if present) for struct tags and the rest of the language.

## Dependencies

- **xcl library, latest tagged release (v0.2.1 at planning time)** — the public Go API the guide documents and the throwaway program builds against. No changes; any gap or bug found is reported, not fixed. If a newer tag is cut before publishing, the examples are re-checked against it.
- **xcl-website (Astro 5, MDX, Expressive Code, Tailwind)** — hosts the page; existing layout and section components are reused unchanged. The site build must pass.
- **Existing xcl.dev topic pages** — registries, events, state masking, plugin template, GitHub registry and the plugins example are link targets; they are not rewritten.
- **pkg.go.dev for github.com/jumppad-labs/xcl** — the generated API docs the guide links to; no changes.
- **Sibling spec 20261010160411-19b965db-language-reference (same epic, no dependency)** — owns struct tags and the rest of the language. This plan does not wait for it; the guide links to the reference only if its page exists when this spec is implemented. Both specs edit `Nav.astro`, `index.mdx` and the xcl README, so whichever lands second rebases onto the other's link changes.
- **Removed README content (xcl commit bdba483 "Clean up docs", parent revision)** — raw material for the querying and state/destroy sections; checked against the code, not copied.
- Design documents this plan was built on: none.

## Testing Approach

**No automated tests are added.** The spec rules out automated tests for the page's examples, and the project convention forbids tests that read README, changelog or website text. Nothing in the xcl library changes, so its existing suite is unaffected and is not extended.

**Implementation-time checks (done by the implementing agent, not committed):**

- **Throwaway program run.** The complete `main.go` and `main.xcl` from the guide are copied verbatim into a scratch Go module outside both repositories, which requires `github.com/jumppad-labs/xcl` at the latest tag. It must build with Go 1.25 with no edits, and running it as `apply`, then `status`, then `destroy` must print what the guide says each step prints: the references filled in after apply, the entity found by address and by type after a fresh-process load, and nothing left in state after destroy. Any mismatch is fixed in the guide (or, if it is a library bug, reported and the guide written around current behaviour). The module is deleted afterwards.
- **Site build.** The xcl-website site build succeeds and the built output contains `/using-xcl-with-go/index.html`; every internal link the guide adds points to a page present in the built output.
- **Terminology scan.** The guide's source contains no "superset", "interpolation" or "Terraform", and every "resource" in it is inside a Go identifier.

**Spec success metrics:**

- *Every Go API topic removed from the README (finding entities, apply and destroy) has a home on xcl.dev* — **Manual — captured in the implementation test plan.** A person confirms the guide covers finding entities (by address, by type, exactly one, all of a type, lookup errors), apply, keeping state and destroy.
- *Questions about getting started with XCL in Go can be answered with a link to the guide* — **Manual — captured in the implementation test plan.** A person judges, by reading the guide, that a newcomer's getting-started question is answered by it.

**Manual reviews before acceptance:**

- **Manual — captured in the implementation test plan.** Follow the guide top to bottom against the latest tagged xcl release, using only the code it shows, and confirm the program creates a config, registers an entity type, validates, applies, keeps state between runs, finds an entity by address and by type, and destroys (acceptance "Guide is complete" and "Matches the current release").
- **Manual — captured in the implementation test plan.** On the built site, the guide appears in the main navigation and is linked from the home page, the xcl README links to it, and each of those links opens it (acceptance "Reachable").
- **Manual — captured in the implementation test plan.** Every link the guide makes, to the registries, plugins, state and events pages and pkg.go.dev, opens an existing page (acceptance "Guide links out").
- **Manual — captured in the implementation test plan.** Terminology review: no "superset", "interpolation" or "Terraform"; "resource" only inside Go identifiers; XCL described as its own language (acceptance "Terminology").
- **Manual — captured in the implementation test plan.** Anything the guide marks "not yet supported" is indeed unsupported in the latest release.

## Milestones & Tasks

### Milestone 1: A Go developer can follow one page from an empty program to a working one

**What changes**: xcl.dev gains a "Using XCL with Go" guide at `/using-xcl-with-go/`. A Go developer reads one page that has them define two entity types, write a configuration where one entity reads values from another, create a config, check it, apply it, find entities by address and by type, keep state between runs and destroy everything, ending with the complete program to copy. Every example has been run against the latest tagged xcl release, and the page points to the registries, plugins, state and events pages and the generated API docs for detail. At this point the page exists and builds but is reachable only by its URL.

**Validation point**: The site builds; the guide's complete program and configuration, copied into a scratch module, build against the latest tag and behave as the page says across an apply run, a status run and a destroy run; the terminology scan finds nothing.

#### - [ ] Task: Write the Using XCL with Go guide page
**Id:** d13baf52-fa9c-4511-b880-4d25bed91f5e
**Repo:** xcl-website
**Depends on:** none
**Execution:** agent

Add the "Using XCL with Go" page to xcl.dev. It walks a Go developer through one program, section by section: define two entity types, write a configuration where `app "web"` reads values from `database "postgres" "main"`, create a config with a local registry and a state directory, validate, apply, find entities by address and by type, keep state between runs and destroy, then shows the complete program and links out to the registries, plugins, state and events pages and the generated API docs. Every example is checked by building and running the complete program, in a throwaway module, against the latest tagged xcl release.

*Technical detail:* [context.md#task-write-the-using-xcl-with-go-guide-page](./context.md#task-write-the-using-xcl-with-go-guide-page)

**Acceptance criteria**:
- [ ] The site builds and serves the guide at `/using-xcl-with-go/`, laid out like the other guides.
- [ ] The guide covers, in order: creating a config, registering entity types, checking (parsing) the configuration, applying it, finding entities by address and by type, keeping state between runs and destroying.
- [ ] The guide ends with the complete program, and that program with the guide's configuration builds unchanged against the latest tagged xcl release and behaves as the guide describes across an apply run, a later status run and a destroy run.
- [ ] The guide links to the registries, plugins, state and events pages and to the generated API docs, and each link points to a page that exists.
- [ ] The guide describes XCL as its own language, calls blocks entities, uses "resource" only inside Go identifiers, and contains none of the words "superset", "interpolation" or "Terraform".
- [ ] Nothing the guide describes as working is missing from the release; anything not built is either left out or marked as not yet supported.

### Milestone 2: The guide can be found from the site and the README

**What changes**: The guide is the first entry in the site's Guides menu, the home page links to it from its introduction and its closing call to action, and the xcl README lists it first under Documentation. The website README's page table and the xcl changelog record the addition. A developer arriving at xcl.dev or the GitHub repository now finds the guide without knowing its URL.

**Validation point**: On the built site the Guides menu, the home page links and the README link all open the guide; the changelog entry and the website page table are present.

#### - [ ] Task: Link the guide from the site navigation and home page
**Id:** ecca3b6c-dc0d-4aab-911d-e52dd53f4b8f
**Repo:** xcl-website
**Depends on:**
- d13baf52-fa9c-4511-b880-4d25bed91f5e — Write the Using XCL with Go guide page
**Execution:** agent

Make the guide reachable from every page of xcl.dev. It becomes the first entry in the Guides menu, so the "Guides" label itself opens it, and the home page links to it at the end of its introduction and with a button in its closing call to action. The website README's page table gains the new page.

*Technical detail:* [context.md#task-link-the-guide-from-the-site-navigation-and-home-page](./context.md#task-link-the-guide-from-the-site-navigation-and-home-page)

**Acceptance criteria**:
- [ ] "Using XCL with Go" is the first item in the Guides menu on every page, and choosing it, or the Guides label, opens the guide.
- [ ] The home page links to the guide from its "What is XCL?" introduction and from its closing call to action, and both links open the guide.
- [ ] The website README's page table lists the guide and its source.

#### - [ ] Task: Link the guide from the xcl README and record it in the changelog
**Id:** a35d4b31-b4a0-48c4-845d-a69b10b08987
**Repo:** xcl
**Depends on:**
- d13baf52-fa9c-4511-b880-4d25bed91f5e — Write the Using XCL with Go guide page
**Execution:** agent

Point readers of the xcl repository at the guide: it becomes the first entry in the README's Documentation list, and the README otherwise stays an overview. Add this spec's changelog entry describing the new guide and the README link, with no breaking changes.

*Technical detail:* [context.md#task-link-the-guide-from-the-xcl-readme-and-record-it-in-the-changelog](./context.md#task-link-the-guide-from-the-xcl-readme-and-record-it-in-the-changelog)

**Acceptance criteria**:
- [ ] The README's Documentation list starts with a link to the guide on xcl.dev, and the link opens it once the site is published.
- [ ] No other README section is expanded; it remains an overview.
- [ ] The changelog has an entry for this spec describing the guide and the README link, listing no breaking changes.

## Open Questions

- **Does the scratch run of the guide's program expose a library bug that blocks a step the spec requires?** Depends on running the complete program against the latest tag, which only happens once the page is written. If the behaviour merely differs from what was expected, write the guide to the actual behaviour. If a bug makes one of the required steps (apply, keep state and load, find by address or type, destroy) impossible with registered types, STOP and ask the user: the spec forbids fixing the library in this work, and dropping a required step would break the "Guide is complete" criterion.
- **Is a newer xcl tag cut between implementation and publication?** Depends on release timing. If a new tag exists when the page is published, re-run the scratch program against it; if any example then behaves differently, STOP and ask whether to update the guide or hold publication.

## Out of Scope

- **Automated tests for the guide's examples** — the examples are checked by hand and by a throwaway program that is never committed (spec non-goal).
- **Changes to the xcl-vscode highlighter** — formatting and publishing it are separate, later work (spec non-goal). XCL code blocks keep the `hcl` fence the site already uses.
- **Changes to the XCL language or the xcl library** — gaps or bugs found while documenting are reported, not fixed (spec non-goal); this includes the deprecated `GetResources`/`ResourceCount` and the vestigial `state.State` find methods, which the guide simply does not show.
- **Rewriting existing topic pages or the home page** — only links to the new guide are added (spec non-goal). Existing wording elsewhere on the site (for example the home page's comparisons with HCL) is left as it is.
- **New "state" or "plugins" topic pages** — the guide links to the nearest existing pages instead; a dedicated page for either would be its own spec.
- **The language reference: struct tags, optional fields, nested blocks, references in depth, variables, locals, outputs, modules, functions and expressions** — covered by the sibling spec 20261010160411-19b965db-language-reference in the same epic.
- **Plugins, providers, computed values, events, sensitive values, diffs and configuration text** — linked to their existing pages, not taught in the guide.
- **Examples in languages other than Go** — Go is the only implementation (spec constraint).
- **Renaming the library's "resource" identifiers** — no API rename; the guide uses current names such as `types.ResourceBase`.
- **Fixing the website README's page table beyond adding the new row** — it already omits several pages; that cleanup is separate.
