---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Research: 20261011084948-c995ed25-using-xcl-with-go

## Alternatives considered and rejected

- **Split the guide into several pages (one per step: config, types, apply, state, queries).** Rejected: the spec asks for a *short* guide that goes top to bottom from an empty program to a working one; every existing guide on the site is one MDX page (`xcl-website:src/pages/*.mdx`, 149-583 lines each). One page keeps the walkthrough copyable in one read.
- **A new top-level nav item ("Go") instead of a Guides child.** Rejected: the nav has three top-level items and hides the first on small screens (`xcl-website:src/components/Nav.astro:56`); adding a fourth crowds mobile. Putting the guide first in the Guides dropdown also makes the "Guides" label itself open it, since a dropdown head links to `children[0].href` (`Nav.astro:72`).
- **Build the guide around a plugin (Docker or template) so apply/destroy do visible work.** Rejected: a plugin needs Docker or an external binary, so a reader could not run the guide "with only the code it shows". Registered (configuration-only) types are applied, saved to state, loaded back, found and destroyed without any provider (`xcl:config.go:425-438` Destroy doc, `xcl:config.go:478-489` Load doc), which covers every acceptance step. Plugins are linked out to instead.
- **Create new "state" and "plugins" topic pages so the spec's named link targets exist literally.** Rejected: the spec's non-goals rule out rewriting topic pages and the requirement is to link to *existing* pages. No `/state/` or `/plugins/` page exists today; the nearest existing pages are `/state-masking/`, `/examples/plugins/`, `/plugin-template/` and `/github-registry/`. The guide itself becomes the home of "keeping state, apply and destroy".
- **Copy the old README "State and Destroy", "Querying a configuration" and "Lifecycle Callbacks" sections verbatim.** Rejected: "Lifecycle Callbacks" (`Processable`/`Process() error`, `ParseCallback`, `Config.Process`) no longer exists in the code (grep of non-test Go for `Processable|ParseCallback|Process() error` finds nothing); the old querying section uses addresses like `resource.postgres.main` in places that are stale. The sections are raw material only.
- **Automated doc tests for the guide's code.** Rejected by spec non-goal and by knowledge `xcl:conventions/testing-and-mocking.md` (no tests that read website pages). A scratch program built during implementation and discarded is allowed as a hand check.

## Chosen approach — evidence

- One new MDX page in xcl-website following the existing guide layout: frontmatter `layout: ../layouts/Shell.astro`, `title: "<Name> - xcl"`, `description`; imports `Hero`, `Prose`, `CtaBanner`, `Button` (`xcl-website:src/pages/registries.mdx:1-14`, `state-masking.mdx:1-12`). Code blocks use `title="main.go"` / `title="main.xcl"` with Expressive Code (`astro.config.mjs:9`).
- Nav entry pattern: `xcl-website:src/components/Nav.astro:8-31` (array of `{label, href}` children under "Guides").
- Home page link points: "What is XCL?" section and the `CtaBanner` at the bottom (`xcl-website:src/pages/index.mdx:30-125, 245-255`).
- README link point: the "Documentation" list (`xcl:README.md`, section "## Documentation").
- Public API at the latest tag v0.2.1 (no exported Go API changes between v0.2.1 and HEAD: `git diff --stat v0.2.1 HEAD -- '*.go'` touches only `internal/parser/lifecycle.go` and example code):
  - `xcl.NewConfig(opts ...ConfigOption) (*Config, error)` (`config.go:227`)
  - `xcl.WithRegistry`, `xcl.WithStatePath`, `xcl.WithStateStore`, `xcl.WithVariables`, `xcl.WithEventHandler` (`options.go:22,37,50,106,126`)
  - `registry.NewLocal()`, `(*Local).RegisterType(prototype any, name ...string)` (`registry/local.go:60,90`)
  - `(*Config).Validate(paths...)`, `Apply(paths...)`, `Destroy()`, `Load()` (`config.go:337,371,438,489`); `Entities()`, `EntityCount()`, `Outputs()` (`config.go:262,282,309`); `Diff` (`config_diff.go:34`, link out only)
  - `xcl.Find[T]`, `xcl.As[T]`, `xcl.FindByType[T]`, `xcl.FindOne[T]`, `xcl.All[T]` (`query.go:29,62,180,190,351`); method forms only under `//go:build go1.27` (`query_methods_go127.go:1`); module is `go 1.25.0` (`go.mod:3`)
  - `xcl.Decode` / `(*Config).Decode` (`decode.go:35,41`) — mention only; the Go-type mapping belongs to the language reference
  - `types.ResourceBase` holds `DependsOn`, `Disabled`, `Meta` (`types/resource.go:82-92`), embedded with `xcl:",remain"` (`xcl-website:index.mdx` resources.go sample)
  - `errors.ConfigError` in package `github.com/jumppad-labs/xcl/errors` (`errors/config_error.go:6`); query sentinels such as `ErrNotFound` (`errors/query_errors.go:28`), re-exported on `xcl` per the README and `errors_reexport_test.go`
  - `xcl.ErrEmptyConfiguration` (`config.go:26`)
- Behaviour to describe: without a state option nothing is persisted (`options.go:44-49`); `Apply` destroys removed blocks first and saves state (`config.go:357-370`); `Destroy` needs no configuration and registered types never reach a provider (`config.go:425-438`); `Load` reads saved state so lookups answer from it, and succeeds with nothing when no state exists (`config.go:478-489`).

## Files examined

- xcl:README.md — current overview; Quick start, registration problems and a Documentation list linking xcl.dev pages; target for the guide link.
- xcl:README.md@bdba483^ (lines 446-842, 2061-2290) — removed sections: configuration-only types, querying, outputs, lookup errors, two spellings, State and Destroy, Lifecycle Callbacks (stale).
- xcl:config.go:215-520 — NewConfig, Entities, FindResource, Validate, Apply, Destroy, Load and their doc comments.
- xcl:options.go:22-180 — config options.
- xcl:query.go — Find/As/FindByType/FindOne/All signatures.
- xcl:query_methods_go127.go:1 — method form gated by go1.27.
- xcl:decode.go:35-41 — Decode function and method.
- xcl:registry/local.go:45-155 — local registry API.
- xcl:types/resource.go:82-92 — ResourceBase fields.
- xcl:errors/config_error.go, errors/query_errors.go — ConfigError and query sentinels.
- xcl:example/configonly/main.go — complete config-only program shape to model the guide's program on.
- xcl:CHANGELOG.md — per-spec sections headed by spec name, ending with a `**Breaking:**` list; docs-only specs (plugin template) also get an entry.
- xcl-website:src/components/Nav.astro — nav items; Guides dropdown; dropdown head links to first child.
- xcl-website:src/pages/index.mdx — home page; hero, "What is XCL?", features grid, CtaBanner.
- xcl-website:src/pages/registries.mdx, state-masking.mdx — guide page template.
- xcl-website:src/config/site.ts — shared URLs incl. `goReferenceURL` (pkg.go.dev) and `examplesURL`.
- xcl-website:README.md — Pages table listing page sources (kept up to date per page added).
- xcl-website:astro.config.mjs — no redirects; Expressive Code before MDX.

## External references

- pkg.go.dev/github.com/jumppad-labs/xcl — the generated API docs the guide links to (`goReferenceURL` in `site.ts`).
- Astro MDX pages (`src/pages/*.mdx` file-based routing) — slug `using-xcl-with-go.mdx` serves `/using-xcl-with-go/`.

## Prior plans / specs consulted

- Epic 20261010160411-19b965db-language-reference — this spec and the language reference spec; no dependency between them.
- Spec 20261010160411-19b965db-language-reference (sibling, planned in parallel) — owns files and blocks, struct tags / config-to-Go-type mapping, meta, references, variables/locals/outputs, modules, functions, expressions. The guide must not duplicate those; it shows one entity type with plain tags and points onward.
- Knowledge xcl:architecture/ux-flow.md — intended user flow (NewConfig → Apply → query → Validate → Destroy); some addresses there use the old `resource.` form, checked against code instead.
- Knowledge xcl:architecture/config-is-the-public-query-surface.md — `Config` is the only public query surface; the guide must not show `state.State` find methods.
- Knowledge xcl:learnings/state-save-and-load-points.md — state saved after Apply and during Destroy.
- Knowledge xcl-website:gotchas/no-redirects-on-page-removal.md — no redirects; choose the slug once.
- Knowledge xcl:glossary/entity.md — an entity is anything a configuration declares.

## Open assumptions

- The latest tagged release at publication is v0.2.1 (or a later tag with the same public API). If a new tag changes any API the guide uses, STOP and re-check every example.
- (Verified during planning) `xcl.ErrNotFound` is re-exported on package `xcl` (`config.go:42`), and `errors.ConfigError` holds its problems in `Errors []error` (`errors/config_error.go:6-11`).
- A registered type with no provider is saved to state by `Apply` with `WithStatePath` and read back by `Load` into the same Go type. Verify with the scratch program.
- The language reference page's URL is decided by the sibling plan; the guide links to it only if it exists in xcl-website when this spec is implemented.

## Drafting assumptions

### Chosen direction: one guide page built on registered types (architecture)
- **Decision**: A single MDX page `xcl-website:src/pages/using-xcl-with-go.mdx` at `/using-xcl-with-go/`, building one program on two registered types with no plugin; linked first under the Guides dropdown, from the home page ("What is XCL?" and the CtaBanner) and from the xcl README Documentation list. Lookups shown in function form; method form mentioned for Go 1.27.
- **Rationale**: Matches every existing guide's single-page shape, runs with only the code shown, and keeps mobile nav uncluttered.
- **Rejected**: Multi-page guide; top-level nav item; plugin-based walkthrough; new state/plugins pages.

### Conventions selected (architecture)
- **Decision**: Apply testing-and-mocking (no file-inspecting tests), never-modify-dependencies, code-style. Drop database, logging, patterns-and-architecture, project-structure, shared errors/test helpers, state/graph test conventions.
- **Rationale**: Only these bear on a documentation change whose code lives on a web page.
- **Rejected**: Listing all conventions.

### Slug and title (architecture)
- **Decision**: Slug `/using-xcl-with-go/`, nav label and page heading "Using XCL with Go".
- **Rationale**: Matches the spec's name for the guide; the site has no redirects, so the slug is chosen once to be descriptive.
- **Rejected**: `/go/` (too terse, could clash with a future per-language section).

### xcl CHANGELOG entry (architecture)
- **Decision**: Add a `## 20261011084948-c995ed25-using-xcl-with-go` section to xcl `CHANGELOG.md`, ending with `**Breaking:**` None.
- **Rationale**: The plugin-template spec, also mostly documentation, got an entry in the same format.
- **Rejected**: No entry (breaks the per-spec record).

### Link targets for "plugins" and "state" (discovery)
- **Decision**: No `/plugins/` or `/state/` page exists on xcl.dev. The guide links "plugins" to `/examples/plugins/`, `/plugin-template/` and `/github-registry/`, and "state" to `/state-masking/` plus the `state` package on pkg.go.dev; the guide itself is the home of keeping state, apply and destroy.
- **Rationale**: The spec requires links to existing pages that open, and forbids rewriting topic pages; these are the nearest existing pages.
- **Rejected**: Creating new state/plugins pages (out of scope); linking to `docs/state.md` in the xcl repo (contributor internals, not user docs).

### Guide is built on registered types, no plugin (discovery)
- **Decision**: The guide's program registers two plain Go types (`database "postgres"` and `app`) with `RegisterType`; no plugin.
- **Rationale**: Runs with only the code shown, and still covers apply, state, load, find and destroy.
- **Rejected**: A plugin-based walkthrough (needs Docker or a binary).

### Language reference link is conditional (discovery)
- **Decision**: Link to the sibling language reference only if its page exists in xcl-website when this spec is implemented.
- **Rationale**: The two specs have no dependency; the reference URL is not decided yet.
- **Rejected**: Hard-coding a guessed URL.

### Example types: plain fields, json tags, no sensitive or computed fields (data_structures)
- **Decision**: The guide's `PostgreSQL` and `App` types use plain `xcl:"name"` tags with matching `json` tags, and no `types.Sensitive`, `optional`, `computed` or nested blocks.
- **Rationale**: Keeps the guide on the core workflow; tag options are the language reference's subject. `json` tags match the site's existing samples and how entities are saved to state.
- **Rejected**: Reusing the home page sample with `Sensitive` and `computed` fields (computed values need a provider, which the guide does not have).

### No human tasks (tasks)
- **Decision**: All three tasks are agent tasks; publishing happens through the site's existing deploy on merge to main, and the hand checks are manual items in the Testing Approach.
- **Rationale**: No secrets, releases or out-of-repo actions are needed.
- **Rejected**: A human "publish the site" task (deploy is automatic) or a human "check the guide" task (reviews are never tasks).

### Home page CtaBanner gets the guide as its primary button (tasks)
- **Decision**: Add a "Using XCL with Go →" primary button first in the closing CtaBanner and demote "Configuration only →" to secondary; add one linking sentence to "What is XCL?". Hero left unchanged.
- **Rationale**: Two visible home-page links without redesigning the hero.
- **Rejected**: Changing the hero's primary "See an example" button.

## Rehydration cues

- `spektacular spec file read 20261011084948-c995ed25-using-xcl-with-go`
- `git -C /home/nicj/code/github.com/jumppad-labs/xcl show bdba483^:README.md | sed -n '446,842p'`
- Read `/home/nicj/code/github.com/jumppad-labs/xcl/config.go` (NewConfig..Load) and `query.go`.
- Read `/home/nicj/code/github.com/jumppad-labs/xcl-website/src/components/Nav.astro`, `src/pages/index.mdx`, `src/pages/registries.mdx`.
- `spektacular knowledge read --data '{"tier":"repo","name":"xcl","path":"architecture/ux-flow.md"}'`
