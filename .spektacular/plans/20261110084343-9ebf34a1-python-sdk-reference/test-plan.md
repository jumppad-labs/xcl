---
created_date: "2026-11-13"
document_status: final
closed_date: "2026-11-13"
---

# Test plan: 20261110084343-9ebf34a1-python-sdk-reference

Manual procedures for the success metrics and reviews the plan's Testing Approach marks as manual. The automated parts
are the API reference tool's tests (`xcl-python: python -m pytest .github/scripts`), the doc comment check
(`make check-docs`, the `check-docs` job of `test.yml`), and the site's strict build (`reference-check.yml`).

Two procedures need a release that does not exist yet: run them after the release plan's steps (merge and release
`xcl-python`, then set `xcl-website:xcl-python-ref.txt` and deploy). The rest can run on the branch with a locally
generated reference:

```
# in xcl-python, with griffe 2.4.1 and docutils 0.23 installed from .github/scripts/requirements-docs.txt
python .github/scripts/api_reference.py --out /tmp/reference.json
# in xcl-website
npm ci
XCL_PYTHON_REFERENCE=/tmp/reference.json npm run dev     # http://localhost:4321/
```

## Success metrics

### 1. A Python developer can answer "what does this method take and return?" from the site

- **What to measure**: for each of five methods, whether the page answers what it takes, what it returns and what it
  raises without opening the package's source. Pass threshold: 5 of 5.
- **How**: open `http://localhost:4321/reference/python/` (or `https://xcl.dev/reference/python/` after deploy). Find
  `xcl.Config.find`, `xcl.Config.apply`, `xcl.LocalRegistry.register_type`, `xcl.aio.Config.find_one` and
  `xcl.prettylog.handler` from the index or by URL anchor (`#xcl.Config.find`). For each, note the signature, the
  parameter list with types and descriptions, the return value and the errors.
- **Expected result**: every one of the five shows its signature, each parameter's type and description, a return
  value where it returns something, and at least one raised error where the doc comment documents one.
- **Who / when**: a maintainer who did not write the doc comments, before the first release with the reference.

### 2. A new Python developer reaches a working apply against a Go plugin by following the guide alone

- **What to measure**: whether the guide's code, copied in order, applies against the Docker plugin. Pass: `python
  main.py apply` exits 0 and reports `applied 3 entities`.
- **How**: on a machine with a running Docker engine and Python 3.10 or later, in an empty directory, follow
  `/using-xcl-with-python/` from the top: create the virtual environment, `pip install xcl-lang`, write
  `infra/main.xcl`, download the key with `curl -O https://xcl.dev/keys/jumppad-labs-releases.asc`, copy the complete
  program into `main.py`, and run `python main.py apply`, `python main.py show`, `python main.py destroy`, then
  `python main_async.py`. Use only text from the page.
- **Expected result**: `apply` exits 0 with `applied 3 entities`; `show` prints `nginx:1.27-alpine 2 HELLO`, the
  network's subnet and id, the container's IP and the network as configuration text; `destroy` prints `destroyed`
  and `docker ps -a` lists no `web` container; `main_async.py` exits 0 with `applied 3 entities`.
- **Who / when**: someone new to xcl, against PyPI's latest `xcl-lang`, before the site's first pin merges, and again
  whenever the guide changes.

### 3. The published reference shows the latest package version within one hour of each release

- **What to measure**: the time from the `xcl-python` GitHub release's creation to xcl.dev showing that version on
  `/reference/python/`. Pass: under 60 minutes.
- **How**: after an `xcl-python` release (the first is the release plan's own), note the release's `created_at` from
  `gh release view <tag> -R jumppad-labs/xcl-python --json createdAt`. Watch the `Reference bump` run on
  `jumppad-labs/xcl-website` (`gh run list -R jumppad-labs/xcl-website -w reference-bump.yml`), then the `Deploy to
  GitHub Pages` run it starts. Then open `https://xcl.dev/reference/python/` and read the version badge.
- **Expected result**: the badge reads `version <released version>`, `xcl-python-ref.txt` on `main` holds the tag in a
  commit by the release bot, and the badge changed less than 60 minutes after `created_at`.
- **Who / when**: the maintainer cutting the release, on the first automatic bump (the second release that carries the
  reference asset; the first pin is set by hand).

## Manual reviews

### 4. The reference page shows the nav and footer, labels both clients, shows the version, and every internal link opens

- **Where**: `/reference/python/` on the dev server or a built `dist/reference/python/index.html`.
- **Look for**: the site's navigation at the top and its footer at the bottom; `xcl.Config` badged `synchronous` and
  `xcl.aio.Config` badged `asyncio`; a `version <x>` badge equal to the `version` field of the data file in
  `src/generated/python-reference.json`; clicking ten links in descriptions and in the index (including one dotted
  anchor such as `#xcl.Config.__init__`) lands on the named entry.
- **Passes when**: all four hold.

### 5. Every name in the export list has an entry, with members

- **Where**: `xcl-python:xcl/__init__.py` (`__all__`), `xcl/aio.py`, `xcl/mask.py`, `xcl/highlight/__init__.py`,
  `xcl/prettylog.py`, and the reference page.
- **Look for**: every name in each `__all__` (and `xcl.aio`) has an entry; each class has its constructor, methods and
  properties, inherited ones marked "inherited from" (for example `xcl.NotFoundError` lists the members of
  `xcl.Error`).
- **Passes when**: no exported name or public member is missing.

### 6. A doc comment changed in a local package appears in a site built against the regenerated data

- **Where**: an `xcl-python` checkout and the `xcl-website` dev server.
- **Look for**: change the first sentence of `xcl.Config.apply`'s doc comment in `xcl/config.py`, rerun
  `python .github/scripts/api_reference.py --out /tmp/reference.json`, restart `XCL_PYTHON_REFERENCE=/tmp/reference.json
  npm run dev`, and open `/reference/python/#xcl.Config.apply`.
- **Passes when**: the entry shows the changed sentence with no edit to `xcl-website`. Revert the doc comment after.

### 7. A real release moves the pin and updates the site with no hand commit

- **Where**: `jumppad-labs/xcl-website` after an `xcl-python` release that follows the first pin.
- **Look for**: the release's `Dispatch the website` job succeeded; a `Reference bump` run built the site and pushed a
  commit to `main` changing only `xcl-python-ref.txt`, authored by the release bot; `Deploy to GitHub Pages` ran for
  that commit. Also run `Reference bump` by hand (`gh workflow run reference-bump.yml -R jumppad-labs/xcl-website -f
  tag=<current pin>`) and check it ends with "nothing to do".
- **Passes when**: the pin moved and the site deployed with no human commit, and the repeat dispatch changed nothing.
  This also confirms the release bot App can push to `xcl-website`, the criterion of the access task left unticked.

### 8. Every example in the guide behaves as the guide says, and anything marked not yet supported is unsupported

- **Where**: `/using-xcl-with-python/` and a clean virtual environment with PyPI's latest `xcl-lang`.
- **Look for**: besides procedure 2, change `replicas` to `"two"` and check `python main.py apply` prints the report the
  page shows and exits 1; run the page's `xcl.prettylog.handler` snippet and check readable lines with each entity's
  configuration; check `python -c "import xcl.plugin"` fails, matching "Writing a plugin in Python is not yet supported".
- **Passes when**: every example behaves as written.

### 9. The guide meets the wording rules

- **Where**: `xcl-website:src/pages/using-xcl-with-python.mdx` and the rendered page.
- **Look for**: `grep -Eni 'superset|interpolation|terraform|resource' src/pages/using-xcl-with-python.mdx` prints
  nothing; the opening describes XCL as a configuration language of its own; blocks are called entities throughout.
- **Passes when**: all three hold.

### 10. The navigation, home page and both READMEs link to the reference and guide, and each link opens the right page

- **Where**: the site's navigation (Guides, Reference), the home page's Install section, `xcl-python:README.md` (links
  section and "Releases and the engine pin"), and `xcl:README.md` ("Documentation" and "Other languages").
- **Look for**: Guides lists "Using XCL with Python" second and its label still opens `/events/`; Reference > Python
  opens `/reference/python/`; the home page's line after the Install tabs links both, visible with either language tab
  chosen; each README link opens the matching xcl.dev page once deployed.
- **Passes when**: every link opens the right page.
