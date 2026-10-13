---
created_date: "2026-10-13"
document_status: final
closed_date: "2026-10-13"
---

# Test plan: 20261011084948-c995ed25-using-xcl-with-go

No automated tests were written (spec non-goal, and the convention against tests that read repository files). Every success metric and review below is manual. The release under test is the one pinned in `xcl-website:src/config/release.json`: `v0.5.3` (`xcl-lang==0.5.3`).

Setup for the site checks, in xcl-website:

```
npm ci
npm run api-docs     # needs Go 1.25+, Python 3.10+, git and network access
npm run build
npm run preview      # serves dist/ at http://localhost:4321/
```

## Success metrics

### SM1: every Go API topic removed from the README has a home on xcl.dev

- **What**: the README sections cut in `bdba483` ("Querying a configuration", "State and Destroy") are covered.
- **How**: open `/using-xcl-with-go/`. Confirm sections exist for: finding by address (`xcl.Find`), by type (`xcl.FindByType`, `xcl.FindOne`, `xcl.All`), untyped (`FindEntitiesByType`, `Entities`), outputs (`types.Output`, `Outputs()`), lookup errors (`ErrNotFound`, `*NotUniqueError`), apply (`ApplyContext`), state with `Load` (the `status` command) and `Destroy`.
- **Pass**: every topic listed has a section with code.
- **Who / when**: reviewer, before merging to xcl-website main.

### SM2: getting-started questions are answered by a link to a guide

- **What**: each common question maps to one section of the matching guide.
- **How**: for each of install, declare a type, register a function, apply, find, keep state, destroy, use a plugin (Python), asyncio (Python), find the answering section in `/using-xcl-with-go/` or `/using-xcl-with-python/` and copy its anchor link (for example `/using-xcl-with-python/#use-asyncio`).
- **Pass**: every question has an anchor link into the right guide.
- **Who / when**: reviewer, before merging.

## Manual reviews

### R1: follow the Go guide top to bottom

- **Where**: an empty directory outside every repo, Go 1.25 or later.
- **How**: copy only the code `/using-xcl-with-go/` shows, in order: the shell block, `main.xcl`, `types.go`, `registry.go`, `config.go`, `check.go`, `apply.go`, `main.go`, then each "replace" edit, `find.go`, `outputs.go`, `lookups.go`, the temporary replica database (and its removal), the second `main.go`, and the `destroy` case. Run `go run .` after each step that says what it prints, then `go run . status` and `go run . destroy`.
- **Pass**: each run prints exactly the lines the page shows (`applied 4 entities`, `app.web is named web-store-prod`, `databases: 1 of 4 entities`, `expected one database, found 2` with the replica, `loaded 4 entities from state`, `destroyed everything in ./.xcl`), and the files equal the final listing. `gofmt -l .` and `go vet .` print nothing.

### R2: follow the Python guide top to bottom

- **Where**: an empty directory outside every repo, CPython 3.10 or later, a running Docker engine (or podman with `DOCKER_HOST` set), no `guide` network or container left from an earlier run.
- **How**: copy only what `/using-xcl-with-python/` shows, in order: the virtualenv and `pip install xcl-lang==0.5.3`, `main.xcl`, the parts of `main.py`, the `curl` for the release key, each `main` replacement, the replica, the commands version and the `destroy` branch. Run `python main.py` at each step, then `python main.py status`, `python main.py destroy`, then `python main_async.py`, then `python main.py destroy`.
- **Pass**: each run prints the lines the page shows (`applied 6 entities`, `the container guide has the address 10.74.0.2`, `expected one database, found 2` with the replica, `loaded 6 entities from state`, `destroyed everything in .xcl`). `docker ps` lists `guide` after an apply and not after a destroy. `main_async.py` prints `the event loop ticked N times during the apply` with N at least 1, then `applied 6 entities` and `app.web is named web-store-prod`. The files equal the final listing.

### R3: the guides are reachable

- **Where**: the built site (`npm run preview`), and `xcl:README.md`.
- **How**: on any page, open the Guides dropdown; click the Guides label; on `/`, check the closing banner and the Documentation section; in the xcl README, follow both Documentation links (after deploy).
- **Pass**: Guides lists "Using XCL with Go" then "Using XCL with Python" first, and its label opens `/using-xcl-with-go/`; the banner has a Go (primary) and a Python button; Documentation reads Language reference, Go guide, Python guide, Go API reference, Python API reference; every link opens the right page.

### R4: the guides link out to pages that exist

- **How**: in each guide, follow every link: the language reference pages, `/registries/`, `/github-registry/`, `/plugin-template/` (Go), `/examples/plugins/`, `/state-masking/`, `/events/`, `/language/functions/#functions-from-your-application`, and the API reference (`/api/go/` from the Go guide, `/api/python/` from the Python guide).
- **Pass**: every link opens an existing page, and each guide's API link goes to its own language's reference.

### R5: the Go API reference

- **Where**: `/api/go/`.
- **Pass**: the page lists `Config`, `NewConfig`, `Find`, `FindByType`, `FindOne`, `All`, `ErrNotFound` and `NotUniqueError` with their doc comments; the package list in the header and the Directories table reach `registry` (`Local`, `NewLocal`, `RegisterType`, `RegisterFunction`) and `types` (`ResourceBase`, `Output`); no `internal`, `mocks`, `e2e`, `plugins/testing`, example, protobuf or `coreservice` package is a page; the heading reads "Go API reference · github.com/jumppad-labs/xcl v0.5.3" and links pkg.go.dev; links to the standard library and left-out packages go to pkg.go.dev.

### R6: the Python API reference

- **Where**: `/api/python/`, `/api/python/aio/`, `/api/python/mask/`.
- **Pass**: `/api/python/` lists `Config`, `LocalRegistry`, `GitHubRegistry`, `entity`, `Resource`, `Error`, `NotFoundError`, `NotUniqueError` and `ConfigError` with their docstrings; `/api/python/aio/` shows `xcl.aio.Config`; `/api/python/mask/` the masks; the header's module list and in-text links move between the three pages; the heading reads "Python API reference · xcl-lang 0.5.3" and links PyPI.

### R7: the references sit inside the site

- **How**: view `/api/go/`, `/api/go/registry/`, `/api/python/` and `/api/python/aio/` at desktop width and at 375px wide (browser device mode).
- **Pass**: the site header, nav, footer and dark palette surround the content; signatures are readable and code is highlighted; at 375px the nav does not overflow and "Features" and "API" are hidden; the footer's "Go API" and "Python API" open the references in the same tab.

### R8: the API nav dropdown and home page entries

- **Pass**: "API" follows "Language" and lists Go then Python; its label opens `/api/go/`; the home page's Documentation section ends with the Go and Python API references.

### R9: one release pin

- **How**: compare the hero badge on `/` with the two reference headings and the install lines of both guides. Then change `src/config/release.json` to another tag, run `npm run api-docs && npm run build`, compare again, and restore the file.
- **Pass**: badge, both headings and both install lines show the same release before and after the change. A tag with no matching `xcl-lang` on PyPI, such as `v0.5.0`, makes `npm run api-docs` exit non-zero.

### R10: the build guards

- **How**: move `generated/` away, run `npm run build` (succeeds, prints "API reference fragments missing: run npm run api-docs"), then `CI=1 npm run build` (fails with the same message); restore `generated/`.
- **Pass**: as stated.

### R11: the deploy workflow (after merge; needs the user's push)

- **How**: push xcl-website to main, or run "Deploy to GitHub Pages" by hand from the Actions tab.
- **Pass**: the run sets up Go and Python, "Generate API references" succeeds, the build succeeds and xcl.dev serves `/api/go/`, `/api/python/`, `/using-xcl-with-go/` and `/using-xcl-with-python/`.

### R12: not-yet-supported claims

- **Pass**: the Go guide's claim that method-form lookups need Go 1.27 holds (the methods are in `query_methods_go127.go`, behind a `go1.27` build constraint, at v0.5.3); the Python guide's claim that plugins are written in Go for now holds (xcl-lang has no plugin or provider API).

### R13: terminology

- **How**: search both guide pages for "superset", "interpolation", "Terraform" and "resource".
- **Pass**: none of the first three appears; "resource" appears only inside `types.ResourceBase` (Go) and `xcl.Resource` (Python); each guide calls XCL a configuration language of its own.

### R14: own language only

- **Pass**: the Go guide shows only Go, XCL and shell code, and the Python guide only Python, XCL and shell code; neither uses language tabs.

### R15: the xcl CHANGELOG entry

- **Pass**: `xcl:CHANGELOG.md` has one `## 20261011084948-c995ed25-using-xcl-with-go` entry at the top, in prose, followed by **Breaking:** and None.

### R16: the xcl-python README

- **Pass**: it links `https://xcl.dev/api/python/` and `https://xcl.dev/using-xcl-with-python/`, and both open once xcl.dev is deployed.
