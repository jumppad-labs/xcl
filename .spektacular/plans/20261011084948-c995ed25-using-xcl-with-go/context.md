---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261011084948-c995ed25-using-xcl-with-go

## Current State Analysis

- **xcl.dev has topic pages only.** `xcl-website:src/pages/` holds `index.mdx`, eleven topic pages and two example pages. None walks a developer from an empty program to a working one, and none shows Python.
- **The nav already has a Guides dropdown.** `xcl-website:src/components/Nav.astro:30-44` lists ten topic pages. The dropdown label links to its first child (`:76`).
- **The home page ends with an examples banner.** `xcl-website:src/pages/index.mdx:324-330` is a `CtaBanner` with two example buttons. The hero (`:23`) and the "Two examples" section also link the examples.
- **The README's Go API sections were removed.** Commit `bdba483` cut the README to an overview. "Querying a configuration" and "State and Destroy" (`bdba483^:README.md:600-842`) now have no home on the site. "Lifecycle Callbacks" (`:2061-2268`) describes API that no longer exists. `xcl:README.md:166-177` is the Documentation list.
- **The Go API in the target release:**
  - `Find`/`FindByType`/`FindOne`/`All`/`As` (`xcl:query.go`), function form only below Go 1.27 (`xcl:go.mod:3` is 1.25.0);
  - `NewConfig`, `Validate`, `Apply`, `Destroy`, `Load`, `Entities` and `Outputs` (`xcl:config.go`);
  - from language-surface (v0.3.0): `RegisterFunction` and `XCL_VAR_`;
  - from python-support (expected v0.4.0): `ApplyContext` and the other context forms, and `FindEntitiesByType`/`FindOneEntity`;
  - `ConfigError` is not re-exported from the root package.
- **The Python package exists in xcl-python but is not yet on PyPI.** `xcl-python:xcl/` holds the package (`__init__.py` with `__all__`, `aio.py`, `mask.py`, docstrings in prose style). It is distributed as `xcl-lang` and imported as `xcl` (`xcl-python:pyproject.toml`), and released at each xcl tag's version by `xcl-python:.github/workflows/release.yml`. The repository has no tags, and `https://pypi.org/pypi/xcl-lang/json` returned 404 on 2026-10-12. The python-out-of-process-engine plan makes v0.5.0 the first published release. There is no generated Python API reference anywhere.
- **Go generated docs exist only on pkg.go.dev.** `xcl-website:src/config/site.ts:9` holds `goReferenceURL = "https://pkg.go.dev/github.com/jumppad-labs/xcl"`. Its only user is the footer's external "Go Reference" link (`xcl-website:src/components/Footer.astro:2,24-29`).
- **xcl.dev deploys from one workflow.** `xcl-website:.github/workflows/deploy.yml` runs on push to `main` only. One `ubuntu-latest` build job runs `npm ci`, `actions/configure-pages@v5`, `npm run build` and `actions/upload-pages-artifact@v3` (`./dist`), then a deploy job runs `actions/deploy-pages@v4`, with permissions `contents: read`, `pages: write` and `id-token: write`. Pages is already enabled. No Go or Python is set up, and there is no manual trigger.
- **The advertised release is a literal.** `xcl-website:src/pages/index.mdx:21` is `<Badge slot="badge" text="v0.4.0 · Go and Python" ... />`; the python-out-of-process-engine plan moves it to v0.5.0. Local xcl tags run up to `v0.5.0`.
- **The nav and home page after language-reference.** `Nav.astro:8-45` has "Features", a "Language" dropdown (`:10-22`), "Examples" (`:23-29`) and "Guides" (`:30-44`). The first item is hidden on phones by index (`i === 0`, `:61`), and a dropdown's label links to its first child (`:76`). `index.mdx:316-322` is the "Documentation" section with one "Language reference" item, and `:324-330` the closing `CtaBanner`. `/api/` is unused.
- **The site has no test tooling.** It has `npm run build`, `make check` (`npx astro check`) and `.gitignore` entries for `node_modules/`, `dist/` and `.astro/`.


## Per-Task Technical Notes

### Requirement → repo and files

| Requirement | Repo | Main files |
|---|---|---|
| Guides are reachable from the site | xcl-website, xcl | `xcl-website:src/components/Nav.astro`, `xcl-website:src/pages/index.mdx`, `xcl:README.md` |
| Using XCL with Go guide | xcl-website | `xcl-website:src/pages/using-xcl-with-go.mdx` |
| Using XCL with Python guide | xcl-website | `xcl-website:src/pages/using-xcl-with-python.mdx`, `xcl-website:src/config/site.ts` |
| Guides link rather than repeat (generated API docs) | xcl-website, xcl-python (README links) | both guide pages; `xcl-website:src/config/release.json`, `xcl-website:src/config/site.ts`, `xcl-website:scripts/api-docs/*`, `xcl-website:src/pages/api/go/[...path].astro`, `xcl-website:src/pages/api/python/[...path].astro`, `xcl-website:src/components/ApiReference.astro`, `xcl-website:src/styles/api-reference.css`, `xcl-website:.github/workflows/deploy.yml`, `xcl-website:package.json`, `xcl-website:Makefile`, `xcl-website:.gitignore`, `xcl-website:src/components/Nav.astro`, `xcl-website:src/components/Footer.astro`, `xcl-website:src/pages/index.mdx`, `xcl-python:README.md` |
| Current behaviour only | xcl-website | both guide pages, checked against the release tag |
| Positioning and terminology | xcl-website | both guide pages |
| Changelog (project rule) | xcl | `xcl:CHANGELOG.md` |

**Release gate (cross-plan).** The guide tasks and the API reference task start only after the release the hero badge advertises is published: the xcl tag and `xcl-lang` at the same version on PyPI. Python-support's human task `75d985e2-10fa-4cdc-896f-afe9d9d2df79` ("Cut the first xcl release with Python support") and the python-out-of-process-engine plan lead there; that plan makes v0.5.0 the first published Python release (`xcl-lang==0.5.0`). Those tasks belong to other plans, so they cannot be listed under **Depends on**. Check with `git -C /home/nicj/code/github.com/jumppad-labs/xcl tag`, `curl -s https://pypi.org/pypi/xcl-lang/json` and the badge in `xcl-website:src/pages/index.mdx`. If the tag is missing, or lacks `RegisterFunction`, `ApplyContext` or `FindEntitiesByType`, or `xcl-lang` at that version is not on PyPI, STOP and ask. The guides, `src/config/release.json` and the badge all use that one release.

### Task: Write the Using XCL with Go guide

- `xcl-website:src/pages/using-xcl-with-go.mdx` (new). Copy the frame of `xcl-website:src/pages/state-masking.mdx:1-14`: front matter (`layout: ../layouts/Shell.astro`, `title: "Using XCL with Go - xcl"`, `description`), imports of `Hero`, `Prose`, `CtaBanner` and `Button`, and a `Hero` with heading "Using XCL with Go" and a one-sentence sub. Also import `goReferenceURL` from `../config/site` (`xcl-website:src/config/site.ts:9`).
- Body inside `<Prose>`, in this order. Each code block is titled (`go title="main.go"`, `hcl title="main.xcl"`, `bash`).
  1. **Before you start**: Go 1.25 or later (`xcl:go.mod:3`), `mkdir deployer && cd deployer && go mod init example.com/deployer`, `go get github.com/jumppad-labs/xcl@<tag>`, where `<tag>` is `xclRelease` from `src/config/site.ts`.
  2. **Write some configuration**: `main.xcl` exactly as in the plan's Data Structures (without the Docker entities). One sentence on entities, labels and addresses (`database.postgres.main`, `app.web`), linking the language reference for the full rules.
  3. **Describe your entities**: `types.go` with `PostgreSQL` and `App` embedding `types.ResourceBase` `xcl:",remain"`. Link the reference's type-mapping pages for tags (optional, block, computed, named-block maps). Do not restate the tag rules.
  4. **Register them**:
     - `local := registry.NewLocal()`, then `RegisterType(&PostgreSQL{}, "database", "postgres")` and `RegisterType(&App{}, "app")` (`xcl:registry/local.go:75-106`).
     - `local.RegisterFunction("slugify", slugify)`, where `slugify` lower-cases and joins words with `-` (language-surface: `registry.Local.RegisterFunction(name string, fn any)`).
     - Note that a malformed declaration panics at registration and that clashes are returned by `NewConfig`, linking `/registries/`.
  5. **Create a config**:
     - `xcl.NewConfig(xcl.WithRegistry(local), xcl.WithVariables(map[string]any{"environment": "prod"}), xcl.WithStatePath("./.xcl"))` (`xcl:options.go:22,50,126`).
     - Mention `XCL_VAR_environment` as the environment override (language-surface).
     - Mention `WithEventHandler(events.SlogHandler(slog.Default()))` in one line, linking `/events/`.
  6. **Check the configuration**:
     - `c.Validate("./main.xcl")` (`xcl:config.go:337`). A failure is a `*xclerrors.ConfigError`, imported as `xclerrors "github.com/jumppad-labs/xcl/errors"`, listing every problem.
     - Show the loop over `ce.Errors` from xcl knowledge `architecture/ux-flow.md` §4.
     - Say `Apply` validates first anyway.
  7. **Apply it**:
     - `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)`, then `c.ApplyContext(ctx, "./main.xcl")` (python-support).
     - Say that Ctrl-C stops new work, saves progress, and returns an error matching `context.Canceled`.
     - Say plain `c.Apply` exists for callers with no context.
  8. **Find entities**:
     - `xcl.Find[App](c, "app.web")`, `xcl.FindByType[PostgreSQL](c, "database", "postgres")`, `xcl.FindOne[App](c, "app")` and `xcl.All[PostgreSQL](c)` (`xcl:query.go:29,180,190,351`).
     - Untyped: `c.Entities()` and `c.FindEntitiesByType("database", "postgres")` (python-support).
     - A note that the method form `c.Find[T]` needs Go 1.27, so the guide uses the function form.
     - Print `app.Name` to show the function's result (`web-store-prod`).
  9. **Read outputs**: `xcl.Find[types.Output](c, "output.database_location")` and `.Value`, and `c.Outputs()` (`xcl:config.go:309`). Do not reference an output from inside configuration (xcl knowledge `gotchas/root-outputs-are-not-referenceable.md`).
  10. **When a lookup cannot be answered**: `errors.Is(err, xcl.ErrNotFound)` for `xcl.Find[App](c, "app.missing")`, and `errors.As(err, &many)` with `*xcl.NotUniqueError` and `.Count` for `xcl.FindOne[PostgreSQL]` after adding a second database (`xcl:config.go:42-64,138-144`). Point to the API docs for the other sentinels.
  11. **Keep state between runs**:
      - The program takes a command: `apply`, `status` or `destroy`.
      - `status` builds the same config and calls `c.Load()` (`xcl:config.go:489`), then the same lookups, without applying.
      - Say that removing an entity from `main.xcl` destroys it on the next apply.
      - Link `/state-masking/` for encrypting sensitive values in state.
  12. **Destroy it**: `c.Destroy()` (`xcl:config.go:438`), which needs only the saved state. Show `go run . destroy`.
  13. **Using plugins**: one paragraph, no code. Block types can also come from plugins, in-process or as separate programs, or installed from GitHub releases. Link `/registries/`, `/github-registry/`, `/examples/plugins/` and `/plugin-template/`.
  14. **The whole program**: final `main.go`, `types.go` and `main.xcl`.
  15. **Next steps**: links to the language reference, `/registries/`, `/examples/plugins/`, `/state-masking/`, `/events/` and `goReferenceURL` (the Go API reference, `/api/go/`, once the API reference task has landed; until then the constant still points at pkg.go.dev, so the link works either way). Point "every exported name" sentences at it too.

- Closing `<CtaBanner heading="Next: the language" ...>`, with buttons to the language reference (primary) and `/examples/configuration-only/` (secondary).
- Raw material: `git show bdba483^:README.md`, lines 600-842 ("Querying a configuration", "State and Destroy"), rewritten rather than pasted. Do not carry over "Lifecycle Callbacks" (lines 2061-2268): `Processable`, `ParseCallback` and `Config.Process` do not exist.
- Language reference URL: use the slug the language-reference plan settles (read its plan with `spektacular plan file read 20261010160411-19b965db-language-reference plan`). If it is not yet built, link the planned URL and record it for the manual link check.
- Verify:
  - in a scratch directory outside all repos, assemble the program from the page and run `go run . apply`, then `go run . status`, then `go run . destroy` against the tag (no `replace`);
  - copy printed lines into the page;
  - run `gofmt -l` on the scratch files;
  - run `npm run build` in xcl-website.
- Wording: "entity" for every block; no "superset", "interpolation" or "Terraform" (write "a reference in a string").
**Complexity**: Medium
**Token estimate**: ~45k
**Agent strategy**: single agent, sequential (write the program and run it first, then the page around it).

### Task: Put the Go guide in the navigation and on the home page

- `xcl-website:src/components/Nav.astro:30-44`: prepend `{ label: "Using XCL with Go", href: "/using-xcl-with-go/" }` as the first child of `Guides`. The label's link (`Nav.astro:76`, `item.children[0].href`) then opens the Go guide. No other item moves.
- `xcl-website:src/pages/index.mdx:324-330`:
  - Replace the closing `CtaBanner`'s heading and body with "Start building with XCL" and "Follow a guide from an empty program to a working one."
  - The primary button becomes `<Button href="/using-xcl-with-go/" label="Using XCL with Go →" variant="primary" large />`.
  - The secondary button becomes `/examples/configuration-only/` "See an example →", until the Python task replaces it.
  - The examples remain linked from the hero (`index.mdx:23`) and the "Two examples" section.
- `xcl-website:src/pages/index.mdx` `<Section heading="Documentation">` (added after "Two examples" by the language-reference spec): add a "Using XCL with Go" item (`/using-xcl-with-go/`) immediately after the "Language reference" item. If the section is not there yet, add it after "Two examples" with the Go guide as its first item, using the markup the language-reference plan describes; the reference's item then goes before it.
- `xcl-website:README.md:33-43`: add the row `| /using-xcl-with-go/ | src/pages/using-xcl-with-go.mdx: the Go walkthrough from an empty program to a working one |` after `/`.
- Verify with `npm run build`, then open `dist/index.html` and a topic page to check the dropdown order.
**Complexity**: Low
**Token estimate**: ~10k
**Agent strategy**: single agent.

### Task: Link the Go guide from the xcl README

- `xcl:README.md:168-170`: insert directly after the "Language reference" bullet under "The documentation lives at xcl.dev" (or as the first bullet, if that entry is not there yet): `- [Using XCL with Go](https://xcl.dev/using-xcl-with-go/): from an empty program to one that applies, finds and destroys configuration`.
- No other README change. It stays an overview.
**Complexity**: Low
**Token estimate**: ~5k
**Agent strategy**: single agent.

### Task: xcl.dev generates the Go and Python API references from the advertised release

- Precondition: the release gate above. The tag the hero badge shows exists, and `xcl-lang` at the same version (tag without `v`) is on PyPI. Read `xcl-website:src/pages/index.mdx:21` for the badge.
- **Release pin.**
  - `xcl-website:src/config/release.json` (new): `{ "xcl": "<badge tag>" }`, for example `{ "xcl": "v0.5.0" }`.
  - `xcl-website:src/config/site.ts`:
    - `import release from "./release.json";`, then export `xclRelease` and `xclLangVersion` (`xclRelease.replace(/^v/, "")`) with a comment saying the badge and both references follow the pin.
    - Change `goReferenceURL` to `"/api/go/"`, add `pythonReferenceURL = "/api/python/"` and `goPkgDevURL = "https://pkg.go.dev/github.com/jumppad-labs/xcl"`, each with a one-line comment (see the plan's Data Structures).
    - `tsconfig.json` extends Astro's strict config, which already allows JSON imports. If `astro check` complains, add `"resolveJsonModule": true`.
  - `xcl-website:src/pages/index.mdx:21`: import `xclRelease` from `../config/site` and make the badge text `` {`${xclRelease} · Go and Python`} ``. Keep the variant and dot.
  - `xcl-website:src/components/Footer.astro:24-29`: `goReferenceURL` is now internal, so drop `target="_blank"` and `rel="noopener"` from that link. The label changes in the nav/footer task; leave it for now.
- **Generator** (`xcl-website:scripts/api-docs/`, new):
  - `generate.sh` (bash, `set -euo pipefail`):
    - reads the tag with `node -p "require('./src/config/release.json').xcl"` and derives the Python version;
    - `rm -rf generated/api && mkdir -p generated/api/go generated/api/python`;
    - Go:
      - `git clone --depth 1 --branch "$tag" https://github.com/jumppad-labs/xcl "$work/xcl"`;
      - `go run go.abhg.dev/doc2go@v0.12.2 -C "$work/xcl" -embed -home github.com/jumppad-labs/xcl -rel-link-style directory -highlight classes:github-dark -pagefind=false -frontmatter "$PWD/scripts/api-docs/go-frontmatter.tmpl" -out "$PWD/generated/api/go" . ./diff ./entity ./errors ./events ./highlight ./logger ./mask ./plugins ./registry ./state ./types`;
      - a comment above the package list says why the others are left out (internal, e2e fixtures, mocks, the `plugins/testing` helpers, examples, generated protobuf, and `coreservice`, the message API for language bindings);
    - Python:
      - `python3 -m venv "$work/venv"`;
      - `"$work/venv/bin/pip" install -r scripts/api-docs/requirements.txt "xcl-lang==$version"`;
      - `"$work/venv/bin/python" scripts/api-docs/python.py --out generated/api/python`;
    - `$work` is a `mktemp -d` removed on exit (`trap`).
  - `go-frontmatter.tmpl`: a `---` block with `title:` (the package import path, or `xcl` for the root) and `description:` (the package synopsis), quoted for YAML. Run `doc2go -help=frontmatter` for the template fields (`Path`, `Basename`, `Package.Name`, `Package.Synopsis`).
  - `requirements.txt`: `pdoc==16.0.0` and its dependencies at exact versions (from `pip freeze` in a clean venv), with a header comment: build-time only, renders the Python API reference for xcl.dev.
  - `pdoc-templates/frame.html.jinja2`: `{% block content %}{% endblock %}` only, with a Jinja comment explaining that the site supplies the page frame (pdoc's documented embedding pattern).
  - `python.py`:
    - `pdoc.render.configure(template_directory=<pdoc-templates>, show_source=False, search=False)`, using the docformat that fits python-support's prose docstrings (pdoc's default);
    - render `xcl`, `xcl.aio` and `xcl.mask` with `pdoc.pdoc(..., output_directory=<tmp>)`, or module by module with `pdoc.render.html_module`;
    - write `xcl` to `index.html`, `xcl.aio` to `aio/index.html` and `xcl.mask` to `mask/index.html`, each with a front matter block (`title`, and `description` from the first line of the module docstring);
    - rewrite pdoc's relative file links (`xcl.html`, `./../xcl.html`, `xcl/aio.html`, `aio.html`, keeping any `#anchor`) to `/api/python/`, `/api/python/aio/` and `/api/python/mask/`;
    - drop pdoc's own `index.html` and `search.js`;
    - exit non-zero on any failure.
  - The planning trial (pdoc 16.0.0 on xcl-python's current source) produced `xcl.html`, `xcl/aio.html` and `xcl/mask.html`, with links such as `./../xcl.html`. Passing `xcl.mask` as well as `xcl` printed a "module already added" warning, since `xcl` already documents its public submodules, so pass only the specs that are needed.
- **Styles**: `xcl-website:src/styles/api-reference.css` (new), imported by `ApiReference.astro`. Scope every rule under `.api-reference`.
  - doc2go: `pre.chroma` blocks, `h3`/`h4` declaration headings, the index `ul`, the "Directories" table, and `.deprecated-tag`.
  - pdoc: `.docstring`, `.signature`, `.name`, `.param`, `.annotation`, `.classattr`, `.attr`, member separators, `.headerlink` and `.pdoc-code`.
  - Use only the `@theme` tokens in `src/styles/global.css` (bg-code, border-subtle, text-secondary, accent-light, link-blue).
  - Append the Chroma `github-dark` classes, from `go run go.abhg.dev/doc2go@v0.12.2 -highlight classes:github-dark -highlight-print-css`, and the Pygments `github-dark` classes for `.pdoc-code`, from `pygmentize -S github-dark -f html -a ".api-reference .pdoc-code"` in the generator venv. Put the generating command in a comment above each.
- **Rendering**:
  - `xcl-website:src/components/ApiReference.astro` (new): props as in the plan's Data Structures.
    - It renders `<Shell title={...} description={...}>`, a short header strip in the frame (`mx-auto max-w-[1100px] px-md`): "Go API reference" or "Python API reference", the module and version (`xclRelease`, `xclLangVersion`) and an external link to `goPkgDevURL` or `pypiURL`.
    - Then `<Prose><div class="api-reference" set:html={html} /></Prose>`.
  - `xcl-website:src/pages/api/go/[...path].astro` and `.../api/python/[...path].astro` (new):
    - `getStaticPaths` globs `/generated/api/<lang>/**/index.html` with `import.meta.glob(..., { query: "?raw", import: "default", eager: true })`;
    - it splits the front matter (a small regex on the leading `---` block; no YAML package is added) and maps the directory under `<lang>/` to `params.path` (`undefined` for the root);
    - with zero matches it throws if `process.env.CI` is set, and otherwise `console.warn`s "API reference fragments missing: run npm run api-docs" and returns `[]`;
    - page titles: `"<title> - Go API - xcl"` and `"<title> - Python API - xcl"`.
- **Build wiring**:
  - `xcl-website:package.json`: add `"api-docs": "bash scripts/api-docs/generate.sh"`. Leave `build` as it is, so content edits never need Go or Python.
  - `xcl-website:Makefile`: an `api-docs` target running `npm run api-docs`, added to `.PHONY`.
  - `xcl-website:.gitignore`: add `generated/`.
  - `xcl-website:.github/workflows/deploy.yml`:
    - add `workflow_dispatch:` under `on:`, beside `push: branches: [main]`;
    - in the build job, after "Install npm dependencies", add `actions/setup-go@v5` (`go-version: '1.27.x'`, `cache: false`), `actions/setup-python@v5` (`python-version: '3.12'`) and a "Generate API references" step running `npm run api-docs`, all before "Build with Astro";
    - keep everything else (concurrency, permissions, configure-pages, upload, deploy) unchanged;
    - check the file with `actionlint` if it is available.
- **README**: `xcl-website:README.md` gains a short "API references" section after the build instructions. It says `npm run api-docs` needs Go 1.25+, Python 3.10+, git and network access; that it writes `generated/` (git-ignored); and that changing `src/config/release.json` moves the badge and both references to a new release once pushed to main (or by running the deploy workflow by hand).
- Verify:
  - `npm run api-docs`, then `npm run build` and `make check`;
  - open `dist/api/go/index.html`, `dist/api/go/registry/index.html`, `dist/api/python/index.html` and `dist/api/python/aio/index.html` with `npm run preview`, and check the names, docstrings, links between pages and the styling;
  - `CI=1 npm run build` with `generated/` removed fails with the "fragments missing" message;
  - `npm run build` with `generated/` removed succeeds with the warning.

  These are checks run by hand, not committed tests.
- If pdoc's import of the released `xcl-lang` starts or requires the engine, STOP and ask (plan, Open Questions).
**Complexity**: Medium
**Token estimate**: ~40k
**Agent strategy**: single agent, sequential (generator first, run it, then routes and styles against the real output, then the workflow).

### Task: Put the API references in the navigation, footer and home page

- `xcl-website:src/components/Nav.astro`:
  - add `hideOnSmall?: boolean` to both `NavItem` variants;
  - set it on `{ label: "Features", ... }`;
  - replace the `i === 0` check at `:61` with `item.hideOnSmall`;
  - insert after the "Language" item (`:10-22`): `{ label: "API", hideOnSmall: true, children: [{ label: "Go", href: "/api/go/" }, { label: "Python", href: "/api/python/" }] }`.

  The label links to `/api/go/` through the existing first-child rule (`:76`), and `matches` marks it active on every `/api/...` page.
- `xcl-website:src/components/Footer.astro`:
  - import `pythonReferenceURL`;
  - rename the "Go Reference" link to "Go API" (`goReferenceURL`, same tab);
  - add "Python API" (`pythonReferenceURL`, same tab) right after it;
  - keep GitHub and Releases.
- `xcl-website:src/pages/index.mdx` "Documentation" section (`:316-322`): append, as the last two items:
  - `- [Go API reference](/api/go/): every public Go package, generated from the release above.`
  - `- [Python API reference](/api/python/): the xcl, xcl.aio and xcl.mask modules, generated from the same release.`

  The guide tasks insert their items after "Language reference", so the API items stay last whichever lands first.
- `xcl-website:README.md` page table: add `| /api/go/ | generated at deploy by scripts/api-docs from src/config/release.json: the Go API reference |` and the matching `/api/python/` row.
- Verify:
  - `npm run api-docs && npm run build`, then `npm run preview`;
  - check the dropdown and the footer;
  - at 375px wide, check that the bar does not wrap or scroll and that "Features" and "API" are hidden.
**Complexity**: Low
**Token estimate**: ~10k
**Agent strategy**: single agent.

### Task: Write the Using XCL with Python guide

- `pythonReferenceURL` (`/api/python/`) is already in `xcl-website:src/config/site.ts`, added by the API reference task this task depends on.
- `xcl-website:src/pages/using-xcl-with-python.mdx` (new). Same frame as the Go guide (`title: "Using XCL with Python - xcl"`), importing `pythonReferenceURL` and `xclLangVersion`. Body inside `<Prose>`:
  1. **Before you start**:
     - CPython 3.10 or later, and a Docker engine for the plugin section.
     - `python -m venv .venv && . .venv/bin/activate && pip install xcl-lang==<version>`, where `<version>` is `xclLangVersion`. Say the distribution is `xcl-lang` and the import is `xcl`.
     - Wheels need no Go or C compiler; list the supported platforms from python-support.
  2. **Write some configuration**: the shared `main.xcl` including the Docker entities. Say the `docker` entities come from a plugin added in step 5.
  3. **Describe your entities**: `@xcl.entity("database", "postgres")` / `@xcl.entity("app")` `@dataclass` classes subclassing `xcl.Resource`. Link the reference for annotations (`T | None`, defaults, `xcl.Sensitive[T]`, `xcl.Computed[T]`, nested dataclasses, `dict[str, Block]`) rather than repeating them.
  4. **Register types and a function**: `local = xcl.LocalRegistry()`, `register_type(...)` and `register_function(slugify)` with `def slugify(text: str) -> str`. Say the signature comes from the annotations, and that a name clashing with a built-in raises `xcl.FunctionNameClashError` when the config is created.
  5. **Use a Go plugin**:
     - `github = xcl.GitHubRegistry()` and `github.register_plugin("jumppad-labs/xcl-plugin-docker", "<tag>")`.
     - Say plugins are written in Go for now and used from Python unchanged; they are downloaded, verified and cached.
     - Show `trusted_keys=` in one sentence, linking `/github-registry/` for signing.
     - Plugin entities have no Python class and come back as `xcl.Entity`, read by name (`container["ip_address"]`, `container.meta`).
  6. **Create a config**: `with xcl.Config(registries=[local, github], state_path=".xcl", variables={"environment": "prod"}) as config:`. Mention `XCL_VAR_environment`, and `on_event=` in one line linking `/events/`.
  7. **Check and apply**: `config.validate("main.xcl")` and `config.apply("main.xcl")`. `xcl.ConfigError` lists each problem with its file and line via `.causes`. Ctrl-C cancels cleanly.
  8. **Find entities**: `config.find(App, "app.web")`, `config.find_by_type(PostgreSQL, "database", "postgres")`, `config.find_one(App, "app")`, `config.all(PostgreSQL)`, and `config.find(None, "docker.container.guide")` for the plugin entity.
  9. **Read outputs**: `config.outputs()["output.database_location"]`. Check the exact key shape against the release.
  10. **Handle errors**: `except xcl.NotFoundError` and `except xcl.NotUniqueError`, all subclasses of `xcl.Error` with `.code`, `.details` and `.causes`.
  11. **Keep state between runs**: a `status` command that creates the same config and calls `config.load()` before the lookups, in a new process. Link `/state-masking/` (`state_mask=xcl.mask.aes256gcm(key)`).
  12. **Destroy it**: `config.destroy()`.
  13. **Use asyncio**:
      - When: the program already runs an event loop, such as a web service or a bot, and must keep answering while an apply runs; otherwise prefer the synchronous API.
      - Show `async with xcl.aio.Config(...) as config: await config.apply("main.xcl")`, and the same lookups awaited.
      - Show an `async def` function registered the same way.
      - A small standard-library demo: `asyncio.gather` of the apply and a ticker that keeps printing, or an `asyncio.start_server` handler answering during the apply.
      - Cancelling the awaiting task stops the engine's work and saves progress.
      - Name web frameworks in prose only.
  14. **Processes and fork**: `ForkedProcessError`; use the `spawn` start method, or create the config after the fork.
  15. **The whole program**: final `main.py` (sync, with `apply`/`status`/`destroy` commands), `main_async.py`, and `main.xcl`.
  16. **Next steps**: the language reference, `/registries/`, `/github-registry/`, `/examples/plugins/`, `/state-masking/`, `/events/` and `pythonReferenceURL`.
- Closing `CtaBanner` mirroring the Go guide.
- API shapes come from the python-support plan ("Python public API"). Confirm every name against the installed package (`python -c "import xcl, inspect; ..."`). If a name differs, use the package's, and report the difference to the user instead of changing xcl-python (spec non-goal).
- Verify:
  - fresh virtualenv in a scratch directory, `pip install xcl-lang==<version>`, a Docker engine running;
  - run `python main.py apply`, `python main.py status`, `python main.py destroy` and `python main_async.py`;
  - copy printed output into the page;
  - run `npm run build`.
**Complexity**: Medium
**Token estimate**: ~55k
**Agent strategy**: single agent, sequential (run the sync program, then the asyncio one, then write the page).

### Task: Put the Python guide in the navigation and on the home page

- `xcl-website:src/components/Nav.astro:30-45`: insert `{ label: "Using XCL with Python", href: "/using-xcl-with-python/" }` as the second child of `Guides`, after the Go guide.
- `xcl-website:src/pages/index.mdx` closing `CtaBanner` (the one the Go task rewrote): the secondary button becomes `<Button href="/using-xcl-with-python/" label="Using XCL with Python →" variant="secondary" large />`. The banner then offers both guides.
- `xcl-website:src/pages/index.mdx` "Documentation" section: add a "Using XCL with Python" item (`/using-xcl-with-python/`) immediately after the Go guide's item.
- `xcl-website:README.md` page table: add the `/using-xcl-with-python/` row after the Go guide's.
- Verify with `npm run build`.
**Complexity**: Low
**Token estimate**: ~8k
**Agent strategy**: single agent.

### Task: Link the Python guide from the xcl README and record the guides in the changelog

- `xcl:README.md` Documentation list: add a bullet directly after the Go guide's: `- [Using XCL with Python](https://xcl.dev/using-xcl-with-python/): the same walkthrough with the xcl Python package, synchronous and asyncio`.
- `xcl:CHANGELOG.md:1-2`: insert `## 20261011084948-c995ed25-using-xcl-with-go` directly under `# Changelog`, above the newest entry. It is one or two prose paragraphs saying:
  - xcl.dev has "Using XCL with Go" and "Using XCL with Python";
  - what each walks through;
  - that xcl.dev now hosts generated Go and Python API references at `/api/go/` and `/api/python/`, built by the site's deploy from the release it advertises;
  - that the README's Documentation list links both guides.

  Then `**Breaking:**` and `None.`, following `xcl:CHANGELOG.md:3-11`. If other specs' entries landed meanwhile, put this one at the top.
- No tests (convention).
**Complexity**: Low
**Token estimate**: ~6k
**Agent strategy**: single agent.

### Task: Link the API reference and the Python guide from the xcl-python README

- `xcl-python:README.md:9-10`: after the "Language and documentation: [xcl.dev](https://xcl.dev)" bullet, add:
  - `- Python API reference: [xcl.dev/api/python](https://xcl.dev/api/python/)`
  - `- Getting started guide: [Using XCL with Python](https://xcl.dev/using-xcl-with-python/)`
- No other xcl-python change: no docs build, workflow, Pages site or dependency. The README is also the PyPI long description, so the links appear there from the next release.
- Verify both URLs open once the site has deployed. No tests (convention).
**Complexity**: Low
**Token estimate**: ~3k
**Agent strategy**: single agent.

## Testing Strategy

No automated tests are added in any repository (spec non-goal; xcl knowledge `conventions/testing-and-mocking.md` forbids tests that read pages, READMEs or changelogs).

- **Write the Using XCL with Go guide**: the implementer runs the assembled program against the release tag in a scratch directory (`apply`, `status`, `destroy`) and copies its output. `npm run build` passes. Manual checks: the Go guide run-through, links, terminology and no tabs.
- **Put the Go guide in the navigation and on the home page** and **Put the Python guide in the navigation and on the home page**: `npm run build` passes. Manual check: the nav, home page and README links open the right guide, and the home page "Documentation" section and the README's Documentation list read Language reference, Go guide, Python guide.
- **Link the Go guide from the xcl README** and **Link the Python guide from the xcl README and record the guides in the changelog**: manual review of the README links and of the CHANGELOG entry format (prose, **Breaking:** None).
- **xcl.dev generates the Go and Python API references from the advertised release**: `npm run api-docs`, then `npm run build` and `make check` pass, and the build output has `/api/go/` (one page per documented package) and `/api/python/` (`aio/`, `mask/`). `CI=1 npm run build` without fragments fails. After merge, the deploy workflow completes on push to main, and a manual dispatch run also completes. Manual checks: the API pages' names, docstrings, links and styling, pkg.go.dev and PyPI links, and the badge matching the pin.
- **Put the API references in the navigation, footer and home page**: `npm run build` passes. Manual checks: the "API" dropdown and its label, the footer links, the home page "Documentation" entries, and the 375px nav.
- **Write the Using XCL with Python guide**: the implementer runs the assembled sync and asyncio programs against `xcl-lang==<version>` in a fresh virtualenv with Docker. `npm run build` passes. Manual checks: the Python guide run-through, including asyncio; links, including the Python API reference; terminology; no tabs; the not-yet-supported claims.
- **Link the API reference and the Python guide from the xcl-python README**: manual review that both links open.

The success metrics map to the manual checks: the README Go topics have a home, and getting-started questions are answered by a guide link.

## Project References

- Spec: `20261011084948-c995ed25-using-xcl-with-go` (no design references).
- Dependency plans: `20261011095738-fad05e41-language-surface` (plan), `20261011130709-aa3eb933-python-support` (plan and context: "Python public API", Milestones 2-3, release task `75d985e2-10fa-4cdc-896f-afe9d9d2df79`), and `20261011211634-a8895873-python-out-of-process-engine` (plan: first published Python release v0.5.0, hero badge move).
- Sibling spec: `20261010160411-19b965db-language-reference` (reference pages, tabs, type mapping).
- Knowledge:
  - xcl `architecture/ux-flow.md` (function-form lookups, error vocabulary);
  - xcl `glossary/entity.md`;
  - xcl `conventions/testing-and-mocking.md` (no tests reading pages);
  - xcl `conventions/shared-errors-package.md` (root re-exports);
  - xcl `gotchas/root-outputs-are-not-referenceable.md`;
  - xcl-website `gotchas/no-redirects-on-page-removal.md`;
  - xcl-website `gotchas/guides-dropdown-label-opens-first-child.md` (applies to the new "API" dropdown too);
  - xcl `conventions/dependencies.md` (document and pin the two generators).
- External: doc2go (`go.abhg.dev/doc2go`, `doc2go -help`, `-help=frontmatter`, `-help=highlight`); pdoc 16 (https://pdoc.dev/docs/pdoc.html, "Edit pdoc's HTML template", and `pdoc.render.configure`).
- Repo roots:
  - xcl `/home/nicj/code/github.com/jumppad-labs/xcl`;
  - xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`;
  - xcl-python `/home/nicj/code/github.com/jumppad-labs/xcl-python`;
  - xcl-plugin-docker `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker` (read only).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

The two guide tasks are Medium (~45k and ~55k), because each runs a real program before writing. The API reference generation task is Medium (~40k), because it runs both generators and builds the routes and styles against their real output. The others are Low.

## Migration Notes

No redirects are needed. Pages are added (two guides, `/api/go/...` and `/api/python/...`) and none are removed or renamed. Behaviour that changes:
- The Guides dropdown label now opens the Go guide instead of the events page.
- `goReferenceURL` now means `/api/go/`, not pkg.go.dev. pkg.go.dev moves to `goPkgDevURL`, and the footer link becomes an internal "Go API".
- The hero badge's release comes from `src/config/release.json` instead of a literal in `index.mdx`.
- Contributors: `npm run build` works as before without Go or Python, but the API pages appear locally only after `npm run api-docs`.

## Performance Considerations

The pages are static. Each deploy now also clones xcl at a tag, compiles and runs doc2go (`go run`), and installs pdoc and `xcl-lang` into a virtualenv. Expect a minute or two more per deploy, with no runtime cost to readers. Deploys run on push to main or by hand, as before.

