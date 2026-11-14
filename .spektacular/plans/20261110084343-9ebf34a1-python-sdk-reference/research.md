---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Research: 20261110084343-9ebf34a1-python-sdk-reference

## Alternatives considered and rejected

- **Generate the reference on the website at build, from the PyPI sdist (website runs Griffe).** Would put a Python toolchain and two pinned Python libraries in `xcl-website`'s deploy (`xcl-website:.github/workflows/deploy.yml:27-39` only sets up Node) and a second copy of the "what is public" walk beside the doc comment check. Rejected: one walker, in xcl-python, serves both the check and the generator.
- **Self-contained HTML generator (pdoc).** Named as the simpler alternative in the spec. Rejected: it renders its own page, so the site's `Shell.astro` nav and footer would need post-processing, and the stable per-name anchors the guide links to would be pdoc's own shape.
- **Sphinx autodoc.** Imports the package, and the package's `xcl/_core` protos and engine are not needed for documentation; Griffe needs neither. Rejected.
- **Keep the check as a runtime pytest (extend `xcl-python:tests/unit/test_docstrings.py`).** `inspect` cannot see a docstring on a constant (`xcl-python:tests/unit/test_docstrings.py:47-49` says so and skips them), so `xcl.UNKNOWN`, `xcl.__version__` and the `xcl.highlight` formatters could never be checked, and the spec allows no exclusions. Rejected in favour of a CI lint that reads the source statically. A static pytest would break the xcl convention that no test parses the repository's own source (`conventions/testing-and-mocking.md`), so the check is a lint step in CI, not a test.
- **A runtime dependency on Griffe.** Forbidden by the spec's constraint. It goes in a `docs` extra only.
- **Add `aio` to `xcl.__all__` so `xcl.aio` is an export.** Changes `from xcl import *` and is a code change, which the constraint forbids. `xcl.aio` is included in the walk by name instead (`xcl-python:tests/unit/test_docstrings.py:42` already treats it as public).
- **Release bump as a pull request with auto-merge (the engine-bump pattern).** `xcl-website` has no branch protection and no required checks (`gh api .../branches/main/protection` returns 404), so `gh pr merge --auto` has nothing to wait for. Direct push after a passing build is the same safety with fewer parts.
- **A committed copy of the generated JSON in the website.** Every release would change a large file; the pin is one line.

- **Hard build failure for a reference link to a missing name.** Rejected by the user when the epic's plans were compared (2026-11-10). The bump's build gates the release, so an API rename would stop the reference updating. `PyRef` now matches the TypeScript plan's `TsRef`.

## Chosen approach — evidence

- `xcl-python:xcl/__init__.py:64-128` is the export list; `xcl-python:xcl/aio.py:501`, `xcl/mask.py:60`, `xcl/highlight/__init__.py:28`, `xcl/prettylog.py:284` give each exported module's list.
- Griffe 2.4.1 loads the package statically (`griffe.load("xcl", docstring_parser="sphinx", allow_inspection=False, resolve_aliases=True, resolve_external=False)`) and finds 63 top-level exports, 94 public names with submodules, and 75 members; `Class.all_members` includes inherited package members (`NotFoundError` shows `message, details, causes, tokens, code` from `Error`, `xcl-python:xcl/_errors.py:29`) and dataclass `__init__` is synthesised (`Problem`, `Event`). Prototype run on 2026-11-10 against the working tree.
- The same prototype found exactly 7 undocumented names: `xcl.__version__`, `xcl.UNKNOWN`, and `xcl.highlight.{PLAIN,TERMINAL,HTML,HTML_INLINE,SVG}`. All 162 existing doc comments parse through docutils 0.23 with no warning.
- `xcl-python:.github/workflows/release.yml:380-398` shows tag, wheels, publish, install-check, github-release; `xcl-python:.github/workflows/release.yml:519-530` and `:652-653` show the version is written into `xcl/_version.py` from the tag only during release, since a checkout holds `0.5.0.dev0` (`xcl-python:xcl/_version.py:14`). The reference job must do the same.
- `xcl-python:.github/workflows/engine-bump.yml` is the model for the pin-update workflow (App token, bot identity, idempotent no-op).
- `xcl:.github/workflows/release.yml:201-207` dispatches from a release job after the build; `xcl-python:.github/workflows/release.yml:763-786` is the job to follow.
- `xcl-website:.github/workflows/deploy.yml` runs `npm run build` on every push to `main`, so an npm `prebuild` script extends the build with no change to the deployment.
- `xcl-website:src/layouts/Shell.astro` carries nav and footer; `xcl-website:src/components/Nav.astro:9-48` is the nav list.
- `xcl-python:README.md:13-14` already links `/api/python/` and `/using-xcl-with-python/`, neither of which exists.

## Files examined

- xcl-python:xcl/__init__.py:64 — the public export list; `aio` is absent
- xcl-python:xcl/config.py:47 — class docstring style: prose with Sphinx roles and no field lists
- xcl-python:xcl/aio.py:52 — the asyncio client, same options as `xcl.Config`
- xcl-python:xcl/_version.py:14 — dev version in a checkout
- xcl-python:xcl/_errors.py:29 — `Error` members inherited by every error class
- xcl-python:xcl/registry.py:21 — `Registration` is returned but not exported, so it stays off the reference
- xcl-python:xcl/diff.py:131 — `Dependency` is in the module's list but not `xcl.__all__`
- xcl-python:tests/unit/test_docstrings.py — existing runtime check; skips constants and inherited members
- xcl-python:tests/plugins.toml — test plugin is `xcl-plugin-test` v0.3.1, test-only
- xcl-python:pyproject.toml:280-296 — extras with a comment giving the reason for each dependency
- xcl-python:Makefile — targets pattern for local checks
- xcl-python:.github/workflows/test.yml:206-225 — `check-package` and the `result` job that must list every job
- xcl-python:.github/workflows/release.yml — release job order
- xcl-python:README.md:13,42,605,797 — links, getting started, asyncio, releases section
- xcl-python:.github/scripts/install_check.py — shows a labelled dataclass needs `meta`
- xcl-website:package.json — npm scripts; no Python
- xcl-website:src/components/Nav.astro:9-48 — nav items and dropdown shape
- xcl-website:src/components/sections/Prose.astro — article styling
- xcl-website:src/config/site.ts — shared links
- xcl-website:src/pages/index.mdx:1-134 — home page, Install section tabs
- xcl-website:src/pages/events.mdx, github-registry.mdx — guide page shape and plugin registration
- xcl-website:.gitignore, README.md — generated paths and the Pages table
- xcl-website:public/keys/jumppad-labs-releases.asc — the key the guide fetches
- xcl:README.md:215-245 — documentation list and "Other languages" section
- xcl:.github/workflows/release.yml:201-207 — dispatch model

## External references

- Griffe (mkdocstrings) — https://mkdocstrings.github.io/griffe/ — static analysis of Python packages, `__all__`, inherited members and dataclass constructors.
- docutils — https://docutils.sourceforge.io/ — turns the package's reStructuredText doc comments into HTML, with custom roles for `:class:` and friends.

## Prior plans / specs consulted

- Spec 20261110084722-9e301981-typescript-sdk-reference — sibling in this epic that shares the Reference nav entry and wants the same page shape, so the JSON schema stays language-neutral.
- Spec 20261011084948-c995ed25-using-xcl-with-go — marked implemented, but no guide exists on the site or in its history; this guide is new work.
- Knowledge `architecture/release-chain.md` (xcl), `decisions/xcl-is-its-own-language.md` (xcl), `gotchas/guides-dropdown-label-opens-first-child.md` (xcl-website), `gotchas/install-check-runs-after-publish.md` (xcl-python), `learnings/test-suite-in-a-spec-worktree.md` (xcl-python), `gotchas/test-module-basenames-unique.md` (xcl-python).

## Open assumptions

- Assumes the `xcl-plugin-docker` release `v0.4.1` remains installable with the released package at publication; the guide task verifies it against PyPI's latest.
- Assumes the release bot GitHub App can be installed on `xcl-website` and its secrets added there (no secrets exist on that repo today).
- Assumes `docs` extras are not installed by default, so no user receives Griffe.
- Assumes the GitHub release asset is published before the website's pin moves, which the job order guarantees.

## Drafting assumptions

### Public names include `xcl.aio` (discovery)
- **Decision**: public names are `xcl.__all__`, each exported module's own `__all__`, and the module `xcl.aio` and its `__all__`, which is not in `xcl.__all__`.
- **Rationale**: the spec requires the asyncio client in the reference and forbids behaviour changes; the existing docstring test already treats it as public.
- **Rejected**: adding `aio` to `xcl.__all__`, which changes `from xcl import *`.

### Members are constructors, methods and properties; a class's doc comment documents its constructor (discovery)
- **Decision**: the check requires doc comments on names, methods and properties; `__init__` is documented by its class's doc comment. Inherited members come from the package's own classes, not `Exception` or `Enum`. Class data attributes (dataclass fields, enum members) are listed but not checked.
- **Rationale**: matches the spec's definition of a member and Python's documentation convention; a static tool cannot see into the standard library's source.
- **Rejected**: requiring `__init__` docstrings, which dataclasses cannot have.

### The doc comment check is a CI lint, not a pytest (discovery)
- **Decision**: a script run as its own job beside `check-package`; `tests/unit/test_docstrings.py` is removed because the lint covers more.
- **Rationale**: only static analysis sees constants' doc comments, and the xcl testing convention forbids tests that parse the repository's source.
- **Rejected**: extending the runtime test (cannot cover constants), adding a static pytest (against the convention).

### Chosen direction: one Griffe tool in xcl-python, release asset, site fetches by pin (architecture)
- **Decision**: `xcl-python` holds the walker and attaches JSON to its GitHub release; the site pins a tag, fetches it in `prebuild`, renders through Astro, and a dispatched workflow bumps the pin after a passing build.
- **Rationale**: one definition of "public" serves the check and the generator, the website gains no Python, and `deploy.yml` is unchanged.
- **Rejected**: generating on the website from the sdist, pdoc, a committed copy of the data (see research.md).

### Bump pushes to main rather than opening a pull request (architecture)
- **Decision**: push after a passing `npm run build`, as the release bot App.
- **Rationale**: the website has no required checks, so a PR with auto-merge has nothing to wait for; the build is the gate.
- **Rejected**: mirroring engine-bump's PR with auto-merge.

### Field lists added on the main API only (architecture)
- **Decision**: `:param:`, `:returns:`, `:raises:` on `xcl.Config`, `xcl.aio.Config`, the registries and module functions; prose elsewhere stays.
- **Rationale**: the spec requires a doc comment on everything and field detail "where documented"; the success metric is about methods developers call. Signatures always show parameter types and defaults.
- **Rejected**: field lists on all members, which adds noise on simple accessors.

### Conventions selected (architecture)
- **Decision**: `testing-and-mocking`, `dependencies` and `release-plan-after-implementation` applied; `config-files-are-xcl` considered and judged not to cover a documentation payload; the Go-specific and engine conventions dropped.
- **Rationale**: only those bear on this work.
- **Rejected**: applying the xcl-format rule to the reference data, which no xcl reader consumes.

### No automated test of the guide's code (testing_approach)
- **Decision**: a person runs the guide's code against the latest release.
- **Rationale**: it needs PyPI, Docker and a network, and extracting code from a page would be a test of documentation text.
- **Rejected**: an extraction-based CI test.

### First pin set by a person (tasks)
- **Decision**: the site's first pin is written by hand after the first release carrying the asset; every later release is automatic.
- **Rationale**: the site cannot name a release before it exists, and the first merge must be after the release.
- **Rejected**: an agent guessing the next tag, which an intervening release would make wrong.

### First pin placeholder (tasks)
- **Decision**: `xcl-python-ref.txt` starts at `v0.14.1`, which has no asset, so a build on the branch fails until the first-pin task runs or `XCL_PYTHON_REFERENCE` is set.
- **Rationale**: an honest failure is better than a silent placeholder; the release plan orders the merge after the pin.
- **Rejected**: leaving the file out, which would change the loader's error handling.

## Rehydration cues

- `spektacular spec file read 20261110084343-9ebf34a1-python-sdk-reference`
- `spektacular knowledge read` for `architecture/release-chain.md` (xcl) and `gotchas/guides-dropdown-label-opens-first-child.md` (xcl-website)
- Re-read `xcl-python:.github/workflows/release.yml`, `.github/workflows/engine-bump.yml`, `xcl/__init__.py`, and `xcl-website:src/components/Nav.astro`, `package.json`
