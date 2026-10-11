---
created_date: "2026-10-11"
---

# Planning summary: 20261010160411-19b965db-language-reference

## Decisions
None.

## Order added for shared files
None added.

## 20261011084948-c995ed25-using-xcl-with-go
### Approach

One new MDX page in xcl-website, `src/pages/using-xcl-with-go.mdx`, served at `/using-xcl-with-go/` and titled "Using XCL with Go". It builds one runnable program step by step using registered types only (no plugin): Go 1.25 and the module; `PostgreSQL` registered as `database "postgres"` and `App` registered as `app`, both embedding `types.ResourceBase`; a `main.xcl` where `app "web"` reads from `database "postgres" "main"`; `registry.NewLocal`/`RegisterType`, `xcl.NewConfig(WithRegistry, WithStatePath)`, `Validate` with `errors.ConfigError`, `Apply`, `Find`/`FindByType`/`FindOne`/`All` with `ErrNotFound`/`ErrNotUnique`, `Load` in a later run, and `Destroy`. It ends with the complete program and "Where next" links. Lookups use the function form (Go 1.25), with a note that the method form needs Go 1.27. The stale README "Lifecycle Callbacks" content is dropped. Rejected: a multi-page guide, a top-level nav item, a plugin-based walkthrough, new state or plugins pages.

### Milestones and tasks

- M1: a Go developer can follow one page from an empty program to a working one.
  - Write the Using XCL with Go guide page (xcl-website, agent). Verified by building the complete program in a throwaway module against the latest tag and running apply, status and destroy, then building the site and scanning the wording.
- M2: the guide can be found from the site and the README.
  - Link the guide from the site navigation and home page (xcl-website, agent, depends on M1): first item in the Guides dropdown; a link at the end of "What is XCL?" and a primary CtaBanner button ("Configuration only" becomes secondary); a row in the website README's page table.
  - Link the guide from the xcl README and record it in the changelog (xcl, agent, depends on M1): first bullet in the README's "Documentation" list; a CHANGELOG.md entry.

### Tasks a person must do

None.

### Out of scope

Automated tests for examples; xcl-vscode or highlighter changes; language or library changes; rewriting existing topic or home pages beyond adding links; new state or plugins pages; everything the language reference covers (struct tags, optional fields, nested blocks, variables, modules, functions, expressions); plugins, events, sensitive values, diffs and configuration text (linked only); examples in other languages; API renames; cleaning up the stale website README page table beyond the new row.

### Drafting assumptions

- Slug `/using-xcl-with-go/` rather than `/go/`, because the site has no redirects.
- xcl CHANGELOG.md gets an entry in the existing format, ending with "**Breaking:** None."
- "Plugins" links go to `/examples/plugins/`, `/plugin-template/` and `/github-registry/`; "State" links go to `/state-masking/` and the pkg.go.dev `state` package; the guide is the home for state, apply and destroy.
- The guide links to the language reference only if that page exists when this spec is implemented.
- Example types use plain `xcl:"name" json:"name"` tags.
- On the home page, the CtaBanner gets the guide as its primary button; the hero is unchanged.

### Project-wide rules

- Changelog: each spec gets a `## <spec name>` section at the top of xcl CHANGELOG.md, ending with a `**Breaking:**` list.
- Website nav: new guides go in the Guides dropdown in `Nav.astro`; the first item is also where the "Guides" label links.
- Home page: adds a sentence at the end of "What is XCL?" and a primary CtaBanner button in `index.mdx`.
- xcl README: the guide is the first item in "## Documentation".
- Website README: each new page gets a row in its Pages table.
- Docs are checked by hand, never by tests that read pages.
- Wording: blocks are "entities"; "resource" only inside Go identifiers; no "superset", "interpolation" or "Terraform"; code blocks use the `hcl` fence with a `main.xcl` title.
- The site has no redirects, so a slug is chosen once.
- Examples use the function form for lookups (`xcl.Find[T](c, ...)`).

### Manual checks

- A person confirms the guide covers every Go API topic removed from the README: finding entities (by address, by type, exactly one, all of a type, lookup errors), apply, keeping state, and destroy.
- A person judges that a getting-started question can be answered with a link to the guide.
- Follow the guide top to bottom against the latest tagged release using only the code it shows.
- On the built site, the Guides menu item, the home page links and the xcl README link all open the guide.
- Every link out of the guide opens an existing page.
- Wording review: no "superset", "interpolation" or "Terraform"; "resource" only inside Go identifiers; XCL described as its own language.
- Anything the guide marks "not yet supported" is really unsupported in the latest release.
