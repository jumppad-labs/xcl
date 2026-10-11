---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Context: 20261011084948-c995ed25-using-xcl-with-go

## Current State Analysis

- **xcl.dev has topic pages only.** `xcl-website:src/pages/` holds `index.mdx`, eleven topic pages and two example pages. None walks a developer from an empty program to a working one, and none shows Python.
- **The nav already has a Guides dropdown.** `xcl-website:src/components/Nav.astro:18-30` lists ten topic pages. The dropdown label links to its first child (`:63`).
- **The home page ends with an examples banner.** `xcl-website:src/pages/index.mdx:249-255` is a `CtaBanner` with two example buttons. The hero (`:22`) and the "Two examples" section also link the examples.
- **The README's Go API sections were removed.** Commit `bdba483` cut the README to an overview. "Querying a configuration" and "State and Destroy" (`bdba483^:README.md:600-842`) now have no home on the site. "Lifecycle Callbacks" (`:2061-2268`) describes API that no longer exists. `xcl:README.md:166-177` is the Documentation list.
- **The Go API in the target release:**
  - `Find`/`FindByType`/`FindOne`/`All`/`As` (`xcl:query.go`), function form only below Go 1.27 (`xcl:go.mod:3` is 1.25.0);
  - `NewConfig`, `Validate`, `Apply`, `Destroy`, `Load`, `Entities` and `Outputs` (`xcl:config.go`);
  - from language-surface (v0.3.0): `RegisterFunction` and `XCL_VAR_`;
  - from python-support (expected v0.4.0): `ApplyContext` and the other context forms, and `FindEntitiesByType`/`FindOneEntity`;
  - `ConfigError` is not re-exported from the root package.
- **The Python package does not exist yet.** `xcl-python` holds only its Spektacular footprint. Its API is defined by the python-support plan and ships at the same version as the xcl tag. It has docstrings but no generated API docs site.
- **Go generated docs exist** at pkg.go.dev (`xcl-website:src/config/site.ts:9`, `goReferenceURL`).

## Per-Task Technical Notes

### Requirement → repo and files

| Requirement | Repo | Main files |
|---|---|---|
| Guides are reachable from the site | xcl-website, xcl | `xcl-website:src/components/Nav.astro`, `xcl-website:src/pages/index.mdx`, `xcl:README.md` |
| Using XCL with Go guide | xcl-website | `xcl-website:src/pages/using-xcl-with-go.mdx` |
| Using XCL with Python guide | xcl-website | `xcl-website:src/pages/using-xcl-with-python.mdx`, `xcl-website:src/config/site.ts` |
| Guides link rather than repeat | xcl-website, xcl-python (Python API docs) | both guide pages; `xcl-python:.github/workflows/docs.yml`, `xcl-python:Makefile`, `xcl-python:pyproject.toml` |
| Current behaviour only | xcl-website | both guide pages, checked against the release tag |
| Positioning and terminology | xcl-website | both guide pages |
| Changelog (project rule) | xcl | `xcl:CHANGELOG.md` |

**Release gate (cross-plan).** The guide tasks start only after python-support's human task `75d985e2-10fa-4cdc-896f-afe9d9d2df79` ("Cut the first xcl release with Python support") has published the tag (expected `v0.4.0`) and `xcl==0.4.0` on PyPI. That task belongs to another plan, so it cannot be listed under **Depends on**. Check with `git -C /home/nicj/code/github.com/jumppad-labs/xcl tag` and `pip index versions xcl`. If the tag is missing, or lacks `RegisterFunction`, `ApplyContext` or `FindEntitiesByType`, STOP and ask.

### Task: Write the Using XCL with Go guide

- `xcl-website:src/pages/using-xcl-with-go.mdx` (new). Copy the frame of `xcl-website:src/pages/state-masking.mdx:1-14`: front matter (`layout: ../layouts/Shell.astro`, `title: "Using XCL with Go - xcl"`, `description`), imports of `Hero`, `Prose`, `CtaBanner` and `Button`, and a `Hero` with heading "Using XCL with Go" and a one-sentence sub. Also import `goReferenceURL` from `../config/site` (`xcl-website:src/config/site.ts:9`).
- Body inside `<Prose>`, in this order. Each code block is titled (`go title="main.go"`, `hcl title="main.xcl"`, `bash`).
  1. **Before you start**: Go 1.25 or later (`xcl:go.mod:3`), `mkdir deployer && cd deployer && go mod init example.com/deployer`, `go get github.com/jumppad-labs/xcl@<tag>`.
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
  15. **Next steps**: links to the language reference, `/registries/`, `/examples/plugins/`, `/state-masking/`, `/events/` and `goReferenceURL`.
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

- `xcl-website:src/components/Nav.astro:19-30`: prepend `{ label: "Using XCL with Go", href: "/using-xcl-with-go/" }` as the first child of `Guides`. The label's link (`Nav.astro:63`, `item.children[0].href`) then opens the Go guide. No other item moves.
- `xcl-website:src/pages/index.mdx:249-255`:
  - Replace the closing `CtaBanner`'s heading and body with "Start building with XCL" and "Follow a guide from an empty program to a working one."
  - The primary button becomes `<Button href="/using-xcl-with-go/" label="Using XCL with Go →" variant="primary" large />`.
  - The secondary button becomes `/examples/configuration-only/` "See an example →", until the Python task replaces it.
  - The examples remain linked from the hero (`index.mdx:22`) and the "Two examples" section.
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

### Task: Generate and publish the Python API docs

- Precondition: python-support's xcl-python package exists (`xcl/__init__.py` with `__all__`, `xcl/aio.py`, `xcl/mask.py`, docstrings enforced by `tests/unit/test_docstrings.py`). Read its layout first. All paths below are relative to that layout.
- `xcl-python:pyproject.toml`: add `pdoc` to the development/docs optional dependency group, pinned to an exact version with a comment giving the reason (docs only, no runtime use).
- `xcl-python:Makefile` (or the repo's existing task runner): a `docs` target running `pdoc xcl xcl.aio xcl.mask -o site/ --docformat google`. Match whichever docstring style python-support used. It must not build or load libxcl: `import xcl` is lazy (python-support acceptance criterion).
- `xcl-python:.gitignore`: ignore `site/`.
- `xcl-python:.github/workflows/docs.yml` (new):
  - triggers on `release: [published]` and `workflow_dispatch`;
  - one `ubuntu-latest` job: checkout, `actions/setup-python` (3.12), install with the docs extra, `make docs`, then `actions/configure-pages`, `actions/upload-pages-artifact` (path `site`) and `actions/deploy-pages`;
  - permissions `pages: write`, `id-token: write`, `contents: read`;
  - environment `github-pages`.

  If the package install needs the generated `_pb2` modules or the engine, generate the messages as the CI workflow does, but never build libxcl.
- `xcl-python:README.md`: add a "Documentation" line with links to `https://jumppad-labs.github.io/xcl-python/` (API reference) and `https://xcl.dev/using-xcl-with-python/` (guide).
- Verify: `make docs` locally produces `site/xcl.html` listing `Config`, `LocalRegistry`, `GitHubRegistry`, `entity`, the exceptions, `site/xcl/aio.html` and `site/xcl/mask.html`. Check the workflow with `actionlint` if it is available.
**Complexity**: Low
**Token estimate**: ~15k
**Agent strategy**: single agent.

### Task: Turn on GitHub Pages for xcl-python and publish the API docs

- A person with admin rights on `jumppad-labs/xcl-python`:
  1. opens Settings → Pages and sets Source to "GitHub Actions";
  2. confirms the `github-pages` environment allows deployments from the default branch and from releases;
  3. runs the "docs" workflow by hand (Actions → docs → Run workflow) on the released commit.
- Check: `https://jumppad-labs.github.io/xcl-python/` opens, and its `xcl` page shows the docstring of `xcl.Config` for the released version.

### Task: Write the Using XCL with Python guide

- `xcl-website:src/config/site.ts:9`: add `export const pythonReferenceURL = "https://jumppad-labs.github.io/xcl-python/";` with a one-line comment, after `goReferenceURL`.
- `xcl-website:src/pages/using-xcl-with-python.mdx` (new). Same frame as the Go guide (`title: "Using XCL with Python - xcl"`), importing `pythonReferenceURL`. Body inside `<Prose>`:
  1. **Before you start**:
     - CPython 3.10 or later, and a Docker engine for the plugin section.
     - `python -m venv .venv && . .venv/bin/activate && pip install xcl==<version>`.
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
  - fresh virtualenv in a scratch directory, `pip install xcl==<version>`, a Docker engine running;
  - run `python main.py apply`, `python main.py status`, `python main.py destroy` and `python main_async.py`;
  - copy printed output into the page;
  - run `npm run build`.
**Complexity**: Medium
**Token estimate**: ~55k
**Agent strategy**: single agent, sequential (run the sync program, then the asyncio one, then write the page).

### Task: Put the Python guide in the navigation and on the home page

- `xcl-website:src/components/Nav.astro:19-31`: insert `{ label: "Using XCL with Python", href: "/using-xcl-with-python/" }` as the second child of `Guides`, after the Go guide.
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
  - that the Python API reference is published from xcl-python;
  - that the README's Documentation list links both.

  Then `**Breaking:**` and `None.`, following `xcl:CHANGELOG.md:3-11`. If other specs' entries landed meanwhile, put this one at the top.
- No tests (convention).
**Complexity**: Low
**Token estimate**: ~6k
**Agent strategy**: single agent.

## Testing Strategy

No automated tests are added in any repository (spec non-goal; xcl knowledge `conventions/testing-and-mocking.md` forbids tests that read pages, READMEs or changelogs).

- **Write the Using XCL with Go guide**: the implementer runs the assembled program against the release tag in a scratch directory (`apply`, `status`, `destroy`) and copies its output. `npm run build` passes. Manual checks: the Go guide run-through, links, terminology and no tabs.
- **Put the Go guide in the navigation and on the home page** and **Put the Python guide in the navigation and on the home page**: `npm run build` passes. Manual check: the nav, home page and README links open the right guide, and the home page "Documentation" section and the README's Documentation list read Language reference, Go guide, Python guide.
- **Link the Go guide from the xcl README** and **Link the Python guide from the xcl README and record the guides in the changelog**: manual review of the README links and of the CHANGELOG entry format (prose, **Breaking:** None).
- **Generate and publish the Python API docs**: `make docs` builds locally without the engine. A manual run of the workflow publishes (checked after the human Pages task).
- **Turn on GitHub Pages for xcl-python and publish the API docs**: the person confirms the Pages URL serves the docs.
- **Write the Using XCL with Python guide**: the implementer runs the assembled sync and asyncio programs against `xcl==<version>` in a fresh virtualenv with Docker. `npm run build` passes. Manual checks: the Python guide run-through, including asyncio; links, including the Python API docs; terminology; no tabs; the not-yet-supported claims.

The success metrics map to the manual checks: the README Go topics have a home, and getting-started questions are answered by a guide link.

## Project References

- Spec: `20261011084948-c995ed25-using-xcl-with-go` (no design references).
- Dependency plans: `20261011095738-fad05e41-language-surface` (plan), and `20261011130709-aa3eb933-python-support` (plan and context: "Python public API", Milestones 2-3, release task `75d985e2-10fa-4cdc-896f-afe9d9d2df79`).
- Sibling spec: `20261010160411-19b965db-language-reference` (reference pages, tabs, type mapping).
- Knowledge:
  - xcl `architecture/ux-flow.md` (function-form lookups, error vocabulary);
  - xcl `glossary/entity.md`;
  - xcl `conventions/testing-and-mocking.md` (no tests reading pages);
  - xcl `conventions/shared-errors-package.md` (root re-exports);
  - xcl `gotchas/root-outputs-are-not-referenceable.md`;
  - xcl-website `gotchas/no-redirects-on-page-removal.md`.
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

The two guide tasks are Medium (~45k and ~55k), because each runs a real program before writing. The others are Low.

## Migration Notes

None. Two pages are added and none removed or renamed, so no redirects are needed. The Guides dropdown label now opens the Go guide instead of the events page.

## Performance Considerations

None. The pages are static. The docs workflow runs only on release or by hand.
