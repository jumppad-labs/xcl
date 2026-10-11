---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261011084948-c995ed25-using-xcl-with-go

## Current State Analysis

- The xcl README (`xcl:README.md`) was cut back to an overview in commit bdba483 "Clean up docs"; its removed sections on querying (old lines 600-792) and on state and destroy (old lines 793-841) have no home on xcl.dev yet. Its "Lifecycle Callbacks" section (old lines 2061-2268) is stale: `Processable`, `ParseCallback` and `Config.Process` no longer exist.
- The latest tag is v0.2.1; between v0.2.1 and HEAD only `internal/parser/lifecycle.go` and example code changed among Go files, so the public API at HEAD equals the release.
- xcl-website (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) is Astro 5 + MDX; guides are single pages under `src/pages/` using `Shell.astro`, `Hero`, `Prose`, `CtaBanner` (`src/pages/registries.mdx:1-14`). The nav (`src/components/Nav.astro:8-31`) has Features, Examples and Guides; the Guides dropdown head links to its first child (`Nav.astro:72`). There are no `/state/` or `/plugins/` pages; the nearest are `/state-masking/`, `/examples/plugins/`, `/plugin-template/`, `/github-registry/`. No redirects exist (knowledge xcl-website `gotchas/no-redirects-on-page-removal.md`).
- Public API the guide uses: `xcl.NewConfig` (`config.go:227`), `WithRegistry`/`WithStatePath` (`options.go:22,50`), `registry.NewLocal`/`RegisterType` (`registry/local.go:60,90`), `Validate`/`Apply`/`Destroy`/`Load` (`config.go:337,371,438,489`), `xcl.Find`/`FindByType`/`FindOne`/`All` (`query.go:29,180,190,351`), `xcl.ErrNotFound` (`config.go:42`), `errors.ConfigError.Errors` (`errors/config_error.go:6-11`), `types.ResourceBase` (`types/resource.go:82-92`).

### Requirement → repo and files

- Guide is reachable from the site → xcl-website `src/components/Nav.astro`, `src/pages/index.mdx`, `README.md`; xcl `README.md`.
- Using XCL with Go guide; Guide links rather than repeats; Current behaviour only; Positioning and terminology → xcl-website `src/pages/using-xcl-with-go.mdx` (new).
- Changelog record → xcl `CHANGELOG.md`.

## Per-Task Technical Notes

### Task: Write the Using XCL with Go guide page

**Requirement → repo/files:** "Using XCL with Go guide", "Guide links rather than repeats", "Current behaviour only", "Positioning and terminology" → xcl-website, new `src/pages/using-xcl-with-go.mdx`.

**File changes:**
- `xcl-website:src/pages/using-xcl-with-go.mdx` (new) — model the frontmatter and imports on `xcl-website:src/pages/registries.mdx:1-14` (`layout: ../layouts/Shell.astro`, `title: "Using XCL with Go - xcl"`, a one-sentence `description`, import `Hero`, `Prose`, `CtaBanner`, `Button`). `Hero` heading "Using XCL with Go", sub one sentence. `Prose` sections, in this order:
  1. *Before you start* — Go 1.25 or newer (`xcl:go.mod:3`), `go get github.com/jumppad-labs/xcl`.
  2. *Describe your entities* — `PostgreSQL` (registered `database "postgres"`) and `App` (registered `app`), each embedding `types.ResourceBase` with `xcl:",remain"` (`xcl:types/resource.go:82-92`); plain `xcl:"name" json:"name"` tags; one sentence that tag options are covered by the language reference (link only if that page exists in `xcl-website:src/pages/` at implementation time; otherwise link pkg.go.dev `types`).
  3. *Write the configuration* — `main.xcl` (fence ```hcl title="main.xcl"```, matching `xcl-website:src/pages/index.mdx:83`), with `database "postgres" "main"` and `app "web"` whose `database_location`/`database_port` read `database.postgres.main.location`/`.port`. Explain type, subtype, name and address (raw material: `xcl:README.md` "Block types" section, reworded with "entity").
  4. *Create a config* — `registry.NewLocal()`, `RegisterType(&PostgreSQL{}, "database", "postgres")`, `RegisterType(&App{}, "app")` (`xcl:registry/local.go:60,90`), `xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath("./.xcl"))` (`xcl:config.go:227`, `xcl:options.go:22,50`). Note registration returns no errors and link `/registries/#how-problems-are-reported`.
  5. *Check the configuration* — `c.Validate("./config")`; on error `errors.As(err, &ce)` with `*xclerrors.ConfigError` from `github.com/jumppad-labs/xcl/errors` (`xcl:errors/config_error.go:6`) and range `ce.Errors`; note `Apply` validates first (`xcl:config.go:330-337`, knowledge `architecture/ux-flow.md` §4). Verify field name `Errors` in `errors/config_error.go`.
  6. *Apply it* — `c.Apply("./config")`; show `app.web`'s `DatabaseLocation` filled in after apply; mention `xcl.ErrEmptyConfiguration` (`xcl:config.go:26,357-370`).
  7. *Find entities* — `xcl.Find[PostgreSQL](c, "database.postgres.main")`, `xcl.FindByType[PostgreSQL](c, "database", "postgres")`, `xcl.FindOne[App](c, "app")`, `xcl.All[App](c)` (`xcl:query.go:29,180,190,351`); `errors.Is(err, xcl.ErrNotFound)`, `xcl.ErrNotUnique` — confirm re-export on package `xcl` (`xcl:errors.go`, `xcl:errors_reexport_test.go`); one sentence on the method form from Go 1.27 (`xcl:query_methods_go127.go:1`). Raw material: old README `git show bdba483^:README.md` lines 600-792 (use `database.postgres.main`, not `resource.…`). Do not show `state.State` Find methods (knowledge `architecture/config-is-the-public-query-surface.md`) nor `Config.FindResource`.
  8. *Keep state between runs* — `WithStatePath` writes `state.json` in the directory (`xcl:options.go:44-60`); a later process calls `c.Load()` and lookups answer from saved state (`xcl:config.go:478-515`); removing a block and applying again destroys it first (`xcl:config.go:357-370`); no state option = memory only. Raw material: old README lines 793-841.
  9. *Destroy it* — `c.Destroy()` needs no configuration; registered types never reach a provider and are removed from state; interrupted destroy resumes (`xcl:config.go:425-470`).
  10. *The whole program* — one ```go title="main.go"``` block: `package main`, imports, the two types, `newConfig()`, and `main` switching on `os.Args[1]`: `apply` (Validate + Apply + print `app.web`), `status` (Load + Find + FindByType + print), `destroy` (Destroy). gofmt-formatted, every error handled.
  11. *Where next* — `/registries/`, plugins (`/examples/plugins/`, `/plugin-template/`, `/github-registry/`), state (`/state-masking/` and pkg.go.dev `state`), `/events/`, pkg.go.dev (`goReferenceURL` in `xcl-website:src/config/site.ts`), the examples. Closing `CtaBanner` with buttons to the configuration-only example and pkg.go.dev, modelled on `xcl-website:src/pages/registries.mdx` ending.
- Do not carry over the stale "Lifecycle Callbacks" content (old README lines 2061-2268): `Processable`, `ParseCallback` and `Config.Process` no longer exist.

**Verification (not committed):** in the session scratchpad (outside both repos) create a module with the "whole program" block as `main.go` and the `main.xcl` block as `config/main.xcl`, `go mod init guidecheck && go get github.com/jumppad-labs/xcl@<latest tag>` (`git -C /home/nicj/code/github.com/jumppad-labs/xcl tag --sort=-creatordate | head -1`; v0.2.1 at planning). Run `go run . apply`, `go run . status`, `go run . destroy`, then `go run . status` again (expect nothing found / ErrNotFound path as the guide says). Fix the page until output matches. Never edit the module cache or xcl source (convention never-modify-dependencies); report library bugs. Delete the module. Then `npm run build` in xcl-website and check `dist/using-xcl-with-go/index.html` exists and every internal href the page adds exists under `dist/`. Grep the page for `superset|interpolation|Terraform` (none) and `resource` (only in Go identifiers).

**Complexity:** Medium
**Token estimate:** ~45k tokens
**Agent strategy:** Single agent, sequential: draft the page, build and run the scratch program, correct the page, build the site.

### Task: Link the guide from the site navigation and home page

**Requirement → repo/files:** "Guide is reachable from the site" (nav and home page) → xcl-website.

**File changes:**
- `xcl-website:src/components/Nav.astro:19-20` — insert `{ label: "Using XCL with Go", href: "/using-xcl-with-go/" },` as the first entry of the Guides `children` array (before "Events and logging"); the dropdown head links to `children[0].href` (`Nav.astro:72`), so "Guides" opens the guide.
- `xcl-website:src/pages/index.mdx:124-126` — before the closing `</Section>` of "What is XCL?", add one sentence linking `[Using XCL with Go](/using-xcl-with-go/)` as the walkthrough from an empty program to a working one. Use "entity" wording; do not touch other home page text.
- `xcl-website:src/pages/index.mdx:249-255` — in the `CtaBanner`, add `<Button href="/using-xcl-with-go/" label="Using XCL with Go →" variant="primary" large />` as the first button and make the existing "Configuration only →" button `variant="secondary"` so only one primary button shows; adjust the banner heading/body only if it no longer reads correctly.
- `xcl-website:README.md:35-46` — add a row `| \`/using-xcl-with-go/\` | \`src/pages/using-xcl-with-go.mdx\`: the core Go API from an empty program to a working one |` after the `/` row.

Check: `npm run build`, then confirm `dist/index.html` and a guide page's nav contain `href="/using-xcl-with-go/"`.

**Complexity:** Low
**Token estimate:** ~10k tokens
**Agent strategy:** Single agent, sequential execution.

### Task: Link the guide from the xcl README and record it in the changelog

**Requirement → repo/files:** "Guide is reachable from the site" (README part) and success metric "every Go API topic removed from the README has a home" → xcl.

**File changes:**
- `xcl:README.md:168-170` — insert `- [Using XCL with Go](https://xcl.dev/using-xcl-with-go/): the core Go API, from an empty program to a working one` as the first bullet of the "## Documentation" list (line 166). No other README changes.
- `xcl:CHANGELOG.md:1-3` — add `## 20261011084948-c995ed25-using-xcl-with-go` directly under `# Changelog`, above the plugin-template entry, following that entry's format: one paragraph describing the new xcl.dev guide (what it walks through, that it links to the topic pages and pkg.go.dev, that it is linked from the site nav, home page and README), then `**Breaking:**` and `None.`

**Complexity:** Low
**Token estimate:** ~6k tokens
**Agent strategy:** Single agent, sequential execution.

## Testing Strategy

- **Write the Using XCL with Go guide page** — throwaway scratch module built from the page's complete program and configuration against the latest tag, run as `apply`, `status`, `destroy`, `status`; output must match the page. `npm run build` passes and `dist/using-xcl-with-go/index.html` exists; every internal href on the page exists in `dist/`. Terminology grep. Nothing committed.
- **Link the guide from the site navigation and home page** — `npm run build`; built home page and any guide page contain `href="/using-xcl-with-go/"` in the expected places.
- **Link the guide from the xcl README and record it in the changelog** — read-through only; no tests read README or CHANGELOG (convention testing-and-mocking).
- Manual items for the test plan: following the guide end to end against the release; reachability on the built site; outbound links; terminology review; "not yet supported" items really unsupported; the two success metrics (topics removed from the README have a home; getting-started questions answerable with a link).

## Project References

- Spec: `20261011084948-c995ed25-using-xcl-with-go` (epic `20261010160411-19b965db-language-reference`).
- Sibling spec: `20261010160411-19b965db-language-reference` (language reference; owns struct tags and the rest of the language).
- Design documents: none referenced.
- Knowledge: xcl `architecture/ux-flow.md`, `architecture/config-is-the-public-query-surface.md`, `learnings/state-save-and-load-points.md`, `glossary/entity.md`, `conventions/testing-and-mocking.md`, `conventions/never-modify-dependencies.md`, `conventions/code-style.md`; xcl-website `gotchas/no-redirects-on-page-removal.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The guide page task is Medium (~45k) but sequential by nature (draft, verify, correct); the two linking tasks are Low and can run in parallel once the page exists.

## Migration Notes

None. No API or URL changes; one new URL (`/using-xcl-with-go/`) is added, chosen once because the site has no redirects.

## Performance Considerations

None. One static page is added to the site.
