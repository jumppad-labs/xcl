---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Context: 20261110084343-9ebf34a1-python-sdk-reference

## Current State Analysis

The package exports 63 names from `xcl-python:xcl/__init__.py:64`, with `xcl.aio` public but not exported. A static walk finds 94 public names, 75 members and seven undocumented names. `xcl-python:tests/unit/test_docstrings.py` is a runtime check that cannot see constants. The package's `README.md:13-14` links to pages that do not exist. `xcl-website` is an Astro site that deploys on every push to `main` with no Python, branch protection or secrets; it has no reference and no Python guide.

## Per-Task Technical Notes

### Task: Add the API reference tool

**File changes**:
- `xcl-python:.github/scripts/api_reference.py` (new) — the tool: `griffe.load("xcl", search_paths=[source], docstring_parser="sphinx", allow_inspection=False, resolve_aliases=True, resolve_external=False)`. Public names: `xcl.__all__` (the `__init__.py:64` list), recursing into exported modules' `__all__`, plus `xcl.aio`. Members: `all_members` minus names starting with `_` except `__init__`. A class's doc comment documents its constructor. Attributes are listed, not checked. Sections: `xcl` non-errors in `__all__` order, `xcl.aio`, `xcl.mask`, `xcl.highlight`, `xcl.prettylog`, Errors (`xcl.Error` and subclasses). `label` is `synchronous`/`asyncio` from the path. Descriptions: Griffe sections, text through `docutils.core.publish_parts(writer_name="html5")["fragment"]` with `report_level=2`, `halt_level=2` so bad markup fails; local roles for `class`, `func`, `meth`, `attr`, `data`, `mod`, `exc`, resolved against entry ids, by bare name or the enclosing class. `version` is the `__version__` attribute's value with quotes stripped.
- `xcl-python:.github/scripts/requirements-docs.txt` (new) — pins `griffe==2.4.1` and `docutils==0.23` with a comment giving the reason, per the dependencies convention.
- `xcl-python:.github/scripts/test_api_reference.py` (new) — pytest against small packages built in `tmp_path`, loading the tool with `importlib.util.spec_from_file_location`: one undocumented method fails and names it; a documented package passes; an inherited member is reported; an unexported name is ignored; a constant without a doc comment fails and with one passes; the data has the version, entries and members. Positive and negative cases are separate functions, not table-driven. `tests/` is left alone, so the default `pytest` run is unaffected and Griffe never enters `tests/`.

Complexity: Medium. Token estimate: ~20k. Agent strategy: single agent, sequential.

### Task: Complete the package's doc comments

**File changes**:
- `xcl-python:xcl/_version.py:14` — doc comment after `__version__`; the release's regex at `xcl-python:.github/workflows/release.yml:526` and the `sed` at `:652` still match.
- `xcl-python:xcl/_types.py` — doc comment after `UNKNOWN`.
- `xcl-python:xcl/highlight/_formatters.py` — doc comments after `PLAIN`, `TERMINAL`, `HTML`, `HTML_INLINE`, `SVG`.
- `xcl-python:xcl/config.py`, `xcl-python:xcl/aio.py`, `xcl-python:xcl/registry.py` — `:param:`, `:returns:`, `:raises:` on `Config`, `aio.Config`, `LocalRegistry`, `GitHubRegistry` methods.
- `xcl-python:xcl/_print.py`, `xcl-python:xcl/_logbridge.py`, `xcl-python:xcl/__init__.py`, `xcl-python:xcl/mask.py`, `xcl-python:xcl/prettylog.py` and the `highlight` modules' public functions — the same fields on module-level functions. Run the tool until it reports nothing.

Complexity: Medium. Token estimate: ~25k. Agent strategy: single agent; each module is independent.

### Task: Run the doc comment check in CI

**File changes**:
- `xcl-python:.github/workflows/test.yml:206-225` — new `check-docs` job beside `check-package`: setup-python, `pip install -r .github/scripts/requirements-docs.txt pytest`, `python .github/scripts/api_reference.py --check`, `python -m pytest .github/scripts`. Add it to the `result` job's `needs`.
- `xcl-python:tests/unit/test_docstrings.py` — delete, replaced by the check, which also covers constants.
- `xcl-python:Makefile` — `check-docs` target; add to `.PHONY`.

Complexity: Low. Token estimate: ~5k. Agent strategy: single agent.

### Task: Publish the reference with each release

**File changes**:
- `xcl-python:.github/workflows/release.yml:1-20` — extend the header comment.
- `xcl-python:.github/workflows/release.yml` — new `reference` job after `settings`: check out the tag; setup-python; set `__version__` in `xcl/_version.py` from `VERSION` as the sdist job does (`:652`); install requirements; run the tool with `--check --out xcl-lang-$VERSION-reference.json`; upload artifact `reference`.
- `xcl-python:.github/workflows/release.yml:664` — add `reference` to `publish.needs` so a broken reference stops the release before PyPI. The publish job's collection step only takes `.whl` and `.tar.gz` (`:676`).
- `xcl-python:.github/workflows/release.yml:763-786` — `github-release` needs `reference`, downloads the artifact, and runs `gh release upload "$TAG" xcl-lang-*-reference.json`.
- `xcl-python:.github/workflows/release.yml` — new `dispatch-website` job after `github-release`, using `actions/create-github-app-token@v1` with `owner` and `repositories: xcl-website`, running `gh api repos/jumppad-labs/xcl-website/dispatches -f event_type=xcl-python-release -f 'client_payload[tag]='"$TAG"`.

Complexity: Medium. Token estimate: ~8k. Agent strategy: single agent.

### Task: Load the reference data into the site

**File changes**:
- `xcl-website:xcl-python-ref.txt` (new) — the pinned tag, `v0.14.1` until the first release with an asset is known. The human "Set the site's first pin" task changes it.
- `xcl-website:scripts/fetch-python-reference.mjs` (new) — honours `XCL_PYTHON_REFERENCE`, else downloads `https://github.com/jumppad-labs/xcl-python/releases/download/<tag>/xcl-lang-<version>-reference.json` with Node's `fetch` and asserts `version` equals the pin without its `v`. Writes `src/generated/python-reference.json`.
- `xcl-website:package.json` — scripts `reference`, `predev`, `prebuild`.
- `xcl-website:.gitignore` — `src/generated/`.
- `xcl-website:Makefile` — `check` runs `npm run reference` first.
- `xcl-website:README.md` — document the pin and override.

Complexity: Low. Token estimate: ~6k. Agent strategy: single agent. Generate local data with the tool from the `xcl-python` task, and build with `XCL_PYTHON_REFERENCE`.

### Task: Render the reference page

**File changes**:
- `xcl-website:src/lib/pythonReference.ts` (new) — imports the JSON; exports types and `hasEntry(id)`.
- `xcl-website:src/pages/reference/python.astro` (new) — `Shell` layout; heading with the version, an index by section, then entries.
- `xcl-website:src/components/reference/Entry.astro` (new) — recursive entry: kind, label badge, signature block, description via `set:html`, parameters, returns, raises, members; `id` is the dotted name.
- `xcl-website:src/styles/global.css` — reference page styles only if the prose styles don't already suit.

Note: `Shell.astro`'s `querySelector` for hash links returns null for ids with dots, which is guarded and falls through to the browser's own jump, so anchors work.

Complexity: Medium. Token estimate: ~15k. Agent strategy: single agent.

### Task: Add the Reference menu and PyRef links

**File changes**:
- `xcl-website:src/components/Nav.astro:9-48` — new `{ label: "Reference", children: [{ label: "Python", href: "/reference/python/" }] }` after Guides.
- `xcl-website:src/components/PyRef.astro` (new) — when `hasEntry` is false: `console.warn` and plain `<code>`, or throw when `process.env.XCL_REFERENCE_STRICT === "1"`; otherwise renders `<a href="/reference/python/#<name>"><code>…</code></a>`.
- `xcl-website:.github/workflows/reference-check.yml` (new) — on `pull_request` to `main`, `push` to `main` and `workflow_dispatch`: checkout, setup-node 22 with npm cache, `npm ci`, `XCL_REFERENCE_STRICT=1 npm run build`. Read-only permissions. `reference-bump.yml` and `deploy.yml` do not set the variable. Like any build, it fails until the first pin task runs, unless `XCL_PYTHON_REFERENCE` is set.

Complexity: Low. Token estimate: ~4k. Agent strategy: single agent.

### Task: Add the pin bump workflow

**File changes**:
- `xcl-website:.github/workflows/reference-bump.yml` (new) — triggers on `repository_dispatch` `[xcl-python-release]` and `workflow_dispatch` with `tag`; concurrency `reference-bump`, no cancel. Validate the tag, take an App token via `actions/create-github-app-token@v1` (secrets `RELEASE_APP_ID`, `RELEASE_APP_PRIVATE_KEY`), check out `main` with it, skip when the pin is already this tag or newer (`sort -V`), write the pin, `npm ci`, `npm run build`, commit as the App's bot user (as `xcl-python:.github/workflows/engine-bump.yml:889-895`), push to `main`. The App token is what makes the push start `deploy.yml`.

Complexity: Low. Token estimate: ~5k. Agent strategy: single agent.

### Task: Give the website the release bot's access

A person installs the release bot GitHub App on `jumppad-labs/xcl-website` with contents write, and adds `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY` as repository secrets. Checked by running `reference-bump.yml` by hand for the current tag, which should be a no-op and succeed.

### Task: Write the Python guide

**File changes**:
- `xcl-website:src/pages/using-xcl-with-python.mdx` (new) — `Shell`, `Hero`, `Prose`, `CtaBanner`, no `CodeTabs`. One program, built by section: install in a virtual environment with `pip install xcl-lang`; `xcl.Config`, a registered dataclass and function; the signed Docker plugin on `xcl.GitHubRegistry` (`xcl-plugin-docker` `v0.4.1`, check it is still the latest and installs against the latest release, key from `/keys/jumppad-labs-releases.asc`, note the Docker requirement); `apply`, state with `state_path` and a second run using `cfg.load()`; `cfg.find` and `cfg.find_by_type`; handling `xcl.ConfigError` with `xcl.print_error`; `xcl.logging_handler` and `xcl.prettylog.handler`; `cfg.print_entity`; `cfg.destroy`; the complete listing; asyncio section with `xcl.aio.Config` and when to use it (web services). Links: `/registries/`, `/github-registry/`, `/plugin-template/`, `/state-masking/`, `/events/`. Every API name is a `PyRef`. Python authoring of plugins is not built yet (`xcl.plugin` is absent), so mark it not yet supported if mentioned. Follow `decisions/xcl-is-its-own-language.md` for wording. Run the code against PyPI's latest in a clean venv before finishing, with Docker running.

Complexity: Medium. Token estimate: ~20k. Agent strategy: single agent.

### Task: Link the guide from the site

**File changes**:
- `xcl-website:src/components/Nav.astro:37` — guide second in Guides, after "Events and logging", so the Guides label still opens that first child (website gotcha).
- `xcl-website:src/pages/index.mdx:30-55` — after the Install `</CodeTabs>`, a line linking the guide and reference, outside the tabs so it shows for every reader.
- `xcl-website:README.md` — add both pages to the Pages table.

Complexity: Low. Token estimate: ~3k. Agent strategy: single agent.

### Task: Update the package README's links

**File changes**:
- `xcl-python:README.md:13-14` — `/api/python/` becomes `/reference/python/`; keep the guide link.
- `xcl-python:README.md:797-820` — one sentence that a release also updates the reference.

Complexity: Low. Token estimate: ~2k. Agent strategy: single agent.

### Task: Link the guide and reference from the xcl README

**File changes**:
- `xcl:README.md:215-225` — add the guide and reference to the documentation list.
- `xcl:README.md:241-247` — link them in "Other languages".

Complexity: Low. Token estimate: ~2k. Agent strategy: single agent.

### Task: Release the package with the reference asset

A person merges the `xcl-python` work with a `patch` label, then checks that the release's jobs all pass, including the install checks (which do not stop a release), and that the GitHub release has the reference asset.

### Task: Set the site's first pin

A person writes the released tag into `xcl-website:xcl-python-ref.txt` on the website's branch, checks a build passes, and merges so the deployment runs. Merge only after the release is on PyPI, per the release chain.

## Testing Strategy

- The API reference tool has unit tests beside it against temporary fixture packages, one assertion per test function, positive and negative separate.
- The check job runs the tool on `xcl` and runs those tests, in `xcl-python:.github/workflows/test.yml`.
- The site's guards are exercised by the build: missing data or a wrong version fail, and an unknown `PyRef` name fails under strict mode, in the reference check workflow.
- The guide, the workflows and the release path are checked by hand from the implementation test plan, per the manual items in the plan's Testing Approach.

## Project References

- Spec: `20261110084343-9ebf34a1-python-sdk-reference`; sibling `20261110084722-9e301981-typescript-sdk-reference`.
- Design documents: none.
- Knowledge: `architecture/release-chain.md`, `decisions/xcl-is-its-own-language.md`, `conventions/testing-and-mocking.md`, `conventions/dependencies.md`, `conventions/release-plan-after-implementation.md`, `conventions/config-files-are-xcl.md` (xcl); `gotchas/guides-dropdown-label-opens-first-child.md` (xcl-website); `gotchas/install-check-runs-after-publish.md`, `gotchas/test-module-basenames-unique.md`, `learnings/test-suite-in-a-spec-worktree.md` (xcl-python).
- Repo roots: `xcl-python` /home/nicj/code/github.com/jumppad-labs/xcl-python; `xcl-website` /home/nicj/code/github.com/jumppad-labs/xcl-website; `xcl` /home/nicj/code/github.com/jumppad-labs/xcl.

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

## Migration Notes

The package's README moves off the dead `/api/python/` URL. No code migrations. Merge order: `xcl-python` first (released), then the first pin and the website.

## Performance Considerations

The reference is one page, built at site build time, and the fetch adds a single download. A release's reference job adds under a minute.
