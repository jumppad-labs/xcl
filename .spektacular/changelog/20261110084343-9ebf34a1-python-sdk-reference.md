---
created_date: "2026-11-13"
document_status: final
closed_date: "2026-11-13"
---

# Python SDK reference and guide

## What was built

- **A complete, generated Python reference.** `xcl-python` gained `.github/scripts/api_reference.py`, a tool built on Griffe and docutils that reads the package statically. It is the one definition of a public name: `xcl.__all__`, each exported module's `__all__`, and `xcl.aio`, with every constructor, method and property, including inherited ones. Class data attributes are listed but not checked. In check mode it fails on any public name or member without a doc comment, or with invalid reStructuredText. Otherwise it writes the API as language-neutral JSON (`schema`, `language`, `package`, `version`, sections of entries) with descriptions rendered to HTML and cross-references turned into links.
- **Complete doc comments.** Seven constants gained doc comments. `xcl.Config`, `xcl.aio.Config`, both registries and the module functions gained `:param:`, `:returns:` and `:raises:` fields. Only doc comments changed.
- **Checks and releases.** A required `check-docs` job in `test.yml`, and `make check-docs`, run the check and the tool's tests. It replaces the runtime docstring test. `release.yml` generates the reference for each tag before PyPI publishing, attaches `xcl-lang-<version>-reference.json` to the GitHub release, and dispatches `xcl-website` with the tag.
- **The reference on xcl.dev.** `xcl-website` pins the documented release in `xcl-python-ref.txt`. A prebuild script downloads that release's data into a gitignored `src/generated/` and refuses a missing file or a version mismatch; `XCL_PYTHON_REFERENCE` uses a local file instead. `/reference/python/` renders the data inside the site's layout: the version, an index, and one anchored entry per dotted name, with the two clients labelled. A Reference menu, a `PyRef` link component and a strict `reference-check.yml` build guard the links. `reference-bump.yml` moves the pin on each release, after the site builds, as the release bot.
- **A Python guide.** `/using-xcl-with-python/` builds one program from `pip install xcl-lang` to a working apply against the signed Docker plugin: state, lookups, error handling, logging, printing, destroy, then asyncio. It was tested against PyPI's latest release. It is linked from the Guides menu, the home page, the site README, the package README and the xcl README.

## Why it matters

Python developers had a long README but no API reference and no walkthrough. They can now look up any public name, with its signature, parameters, return value and errors, or follow one path from an empty program to a working one. The reference is generated from the package and follows each release with no hand edits.

## Deviations from the plan

- Criterion 1 of the tool task was reworded at the user's direction to the spec's definition of a member, so class data attributes are listed but not checked. The code did not change.
- The doc comments task took two goal-check rounds. Each attempt found errors further down the call chain. The user chose `keep_fixing`, and the engine start-up errors were placed on `engine_version`, both `Config` classes and `aio.Config.open`.
- The release bot access task was accepted on the user's confirmation. Criterion 1 (the App installed on `xcl-website`) stays unticked, because the App's repository selection could not be read. The first `reference-bump.yml` run after merge is the check.
- `Release the package with the reference asset` and `Set the site's first pin` were skipped at the user's decision. They are human release steps for after merge, in this order: release `xcl-python`, then set the pin and push `xcl-website`. Until the pin names a release with the asset, the website's build fails at the reference download.
- The plan's line numbers for `release.yml` and `engine-bump.yml` were stale. Changes were placed by job name.
