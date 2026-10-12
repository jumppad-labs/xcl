---
created_date: "2026-10-12"
document_status: final
closed_date: "2026-10-12"
---

# Research: 20261011232013-da18c586-python-release-hardening

## Alternatives considered and rejected

### Option A: Ship the test plugin as assets of an existing xcl release

(e.g. extra assets on the `libxcl` release in jumppad-labs/xcl).

**Rejected**: the GitHub registry derives every asset name and the binary name from the repository name (`xcl:registry/github_assets.go`, `xcl-plugin-template:Makefile` "PLUGIN ... must match the name of your repository, because the GitHub registry installs a plugin by its repository name"). In jumppad-labs/xcl the plugin would have to be called `xcl` (`xcl_<v>_<os>_<arch>.tar.gz`, binary `xcl`), which the local directory pattern `xcl-plugin-*` would not start (`xcl-python:tests/integration/test_docker_plugin.py:84-100` relies on that pattern), and it would tie test-plugin releases to engine releases.

### Option B: Publish the template's own sample plugin (`notes "note"`) as the test plugin

(`xcl-plugin-template` already has a signed `release.yml`).

**Rejected**: the spec asks for a plugin *generated from* the template; adding operation recording and test-only sensitive fields to the template would put test scaffolding into every new plugin authors create from it.

### Option C: Keep Docker and gate it better

(e.g. fix `AF_UNIX` on Windows).

**Rejected by the spec**: "No Docker in the test suite"; the Docker plugin stays only as the documented example.

### Option D: Replace the gpg-generated untrusted key with a committed throwaway public key

(`xcl-python:tests/integration/test_github_tamper.py:166-184` generates one with gpg and skips without it).

**Rejected**: the spec's constraint keeps the tamper tests unchanged apart from the plugin they install. CI runner images all provide gpg (GnuPG on ubuntu/macOS, Git for Windows' gpg on the bash PATH), so the fixture is kept, and a missing gpg on a runner becomes an implementation-time STOP.

### Option E: Read "refused naming the key" as the existing wording

("signature <file> does not verify against any trusted key: openpgp: signature made by unknown entity", `xcl:registry/github_verify.go:98-103`).

**Rejected**: that message names no key, so the acceptance criterion would not literally hold. Adding the issuer key ID to the message keeps the existing text, so the unchanged tamper tests still pass, and satisfies the criterion.

### Option F: Drop TOML entirely

(hard-code the pinned release in Python).

**Rejected**: the spec's Technical Approach says "Read TOML with a fallback reader on Python 3.10".

### Option G: Record operations in a computed attribute of the entity

instead of a log file.

**Rejected**: destroy removes the entity from state, so destroy could not be read back; the log-file pattern of the existing recording fixture survives destroy and works across the plugin process boundary.

## Chosen approach — evidence

- Test plugin modelled on the existing recording fixture: `xcl:bindings/libxcl/testdata/plugins/recording/main.go` — appends one JSON line per lifecycle call to the file named by the block's `log` setting; `Output` is `xcl:"output,optional,computed"`.
- Sensitive field pattern: `xcl:e2e/fixtures/recorder/recorder.go:39` — `Secret types.Sensitive[string] \`xcl:"secret,optional"\``.
- Template layout, `make dist` asset contract and signed release workflow: `xcl-plugin-template:Makefile` (dist target), `xcl-plugin-template:.github/workflows/release.yml` (GPG_PRIVATE_KEY / GPG_PASSPHRASE secrets, `gh release create`), `xcl-plugin-template:entities/note.go`, `providers/note.go`, `plugin.go`, `cmd/notes/main.go`, `e2e/`.
- Asset contract the registry verifies: `xcl:registry/github_assets.go`, `xcl:registry/github_verify.go:83-105` (signature error text "signature <file> does not verify against any trusted key: openpgp: signature made by unknown entity").
- Python tests to port: `xcl-python:tests/integration/docker_helpers.py` (plugin_release, release_files, github_registry, installed_release, require_docker, AF_UNIX ping), `test_docker_plugin.py`, `test_github_tamper.py` (FakeGitHub, tamper tests), `tests/plugins.toml`.
- Masking checks to mirror: `xcl-python:tests/integration/test_sync.py:156-200` (redacted lookup, `state_mask=xcl.mask.aes256gcm(KEY)`, `on_event` + `event_data="processed"`).
- CI evidence (run 38174422045, 2026-10-11): py3.10 collection `ModuleNotFoundError: No module named 'tomllib'` in `test_docker_plugin.py`/`test_github_tamper.py`; every job `FileNotFoundError .../xcl-python/xcl/bindings/libxcl/testdata/parity/annotations/types.json` from `tests/unit/test_schema.py:13-15` (`parents[3] / "xcl"`); Windows: `socket has no attribute AF_UNIX` (docker ping), `exec: ".../build/plugins/xcl-plugin-slow": executable file not found` (fixture plugins built without `.exe`), `test_keyboard_interrupt_cancels_apply` `ValueError: Unsupported signal: 2`, parity program exit 3221225477 (access violation, in-process DLL — sibling spec); macOS: `test_sync_apply_does_not_stall_threads` "the counter thread ran only 25 times during a 1.53s apply"; Alpine: musl TLS relocation error loading libxcl.so (sibling spec).
- Single recorded xcl ref already exists: `xcl-python:xcl-ref.txt` (`f-libxcl`), read by both `ci.yml` jobs; `Makefile` builds engine and fixture plugins from `XCL_SRC`.

## Files examined

- xcl-python:.github/workflows/ci.yml — matrix 6 OS x 5 Python, reads xcl-ref.txt, builds engine+plugins via make, sets XCL_SRC=xcl-src for pytest; test-musl job.
- xcl-python:.github/workflows/release.yml — wheels from xcl tag; runs parity + test_load against wheel; not touched here.
- xcl-python:Makefile:1-50 — XCL_SRC ?= ../xcl; `plugins` builds xcl-plugin-slow, xcl-plugin-recording, parity with no `.exe` suffix.
- xcl-python:pyproject.toml — requires-python >=3.10; `test` extra = pytest>=8 only.
- xcl-python:xcl-ref.txt — `f-libxcl`.
- xcl-python:tests/conftest.py — puts package root and tests/ on sys.path.
- xcl-python:tests/integration/conftest.py:17-19 — XCL_SRC hard-coded to `ROOT.parent / "xcl"`, ignores the env var.
- xcl-python:tests/parity/conftest.py:26 — honours XCL_SRC env, defaults to sibling.
- xcl-python:tests/unit/test_schema.py:13-15,56-59 — PARITY_TYPES via parents[3]/"xcl"; fails (not skips) when absent.
- xcl-python:tests/integration/docker_helpers.py:23,49 — `import tomllib`; Docker daemon ping via AF_UNIX.
- xcl-python:tests/integration/test_docker_plugin.py — 4 tests: registry/path/directory start + generic entity read (meta, references, links, line).
- xcl-python:tests/integration/test_github_tamper.py — FakeGitHub + 4 tests (tampered checksums, untrusted key, no keys installs unsigned, valid release installs); `other_key` generated by gpg.
- xcl-python:tests/plugins.toml — `[docker] repository/version v0.2.0`.
- xcl-python:tests/integration/test_sync.py:674-755 — stall test (ticks>=50) and SIGINT test.
- xcl-python:README.md:28-95,240-246 — Getting started uses Docker plugin (stays); Development section describes make targets.
- xcl:bindings/libxcl/testdata/plugins/recording/main.go — recording fixture, model for the test plugin.
- xcl:registry/github_verify.go:83-105 — untrusted-key error wording.
- xcl:.github/workflows/libxcl-release.yml — engine release; untouched.
- xcl-plugin-template:Makefile, release.yml, ci.yml, current.yml, go.mod (xcl v0.3.0) — standard layout and signed release.
- xcl-plugin-docker: tags v0.1.0..v0.2.0, go.mod xcl v0.3.0 — stays as documented example.
- xcl: tags up to v0.4.0 (on f-libxcl); `plugins/` unchanged since v0.3.0, so a plugin built against v0.4.0 speaks the same protocol.

## External references

- PEP 680 / `tomllib` (Python 3.11+) and `tomli` (the backport with the same API) — the 3.10 fallback reader, installed only with marker `python_version < "3.11"`.
- Python docs, `subprocess.CREATE_NEW_PROCESS_GROUP` + `signal.CTRL_BREAK_EVENT` / `signal.SIGBREAK` — how to deliver an interrupt to a child Python process on Windows (`send_signal(SIGINT)` is unsupported there).
- ProtonMail/go-crypto `openpgp/errors.ErrUnknownIssuer` — "openpgp: signature made by unknown entity", the text an untrusted-key refusal carries.

## Prior plans / specs consulted

- Spec 20261011130709-aa3eb933-python-support (via epic) — origin of the Docker-based plugin tests and xcl-ref.txt.
- Spec 20261011211634-a8895873-python-out-of-process-engine (sibling, planned in parallel) — owns the engine program, subprocess transport, musl and Windows x86-64 wheels, and so the Windows parity access violation and Alpine TLS failures.
- Epic 20261010160411-19b965db-language-reference — ordering: this spec depends only on python-support; docs specs depend on it.

## Open assumptions

- The test plugin lives in a new repository, `xcl-plugin-test` (user's decision, 2026-10-12), registered with Spektacular at /home/nicj/code/github.com/jumppad-labs/xcl-plugin-test. The local directory holds only the `.spektacular` footprint and is not yet a git repository; a person creates jumppad-labs/xcl-plugin-test from the template on GitHub and turns the local directory into its checkout before implementation.
- jumppad-labs release key secrets (GPG_PRIVATE_KEY, GPG_PASSPHRASE) can be given to the new repository by a person.
- gpg is on PATH for pytest on every CI runner (Windows via Git for Windows in the bash shell).
- The issuer key ID can be read from the detached signature packet (go-crypto `packet.Signature.IssuerKeyId`) when verification fails with `ErrUnknownIssuer`.
- The sibling spec's transport change does not itself fix the Windows `.exe` fixture builds, the Windows SIGINT test or the macOS stall-test threshold; if it does, the matching tasks here become no-ops.
- Files shared with the sibling plan (ci.yml, Makefile, tests/integration/conftest.py, tests/parity/conftest.py) will need ordering by the epic orchestrator.

## Drafting assumptions

### Keep the gpg-generated untrusted key (verification)
- **Decision**: the tamper tests' `other_key` fixture keeps generating its key with gpg; CI relies on gpg being present on every runner image.
- **Rationale**: the spec's constraint keeps the tamper tests unchanged apart from the plugin they install; gpg ships on all GitHub runner images used.
- **Rejected**: a committed throwaway public key (would change the tamper tests beyond the plugin, contradicting the constraint without the user's agreement) — kept as the fallback named in Open Questions.

### Name the untrusted signing key in the engine's refusal (verification)
- **Decision**: extend xcl's untrusted-key verification error with the signature's issuer key ID, keeping the existing wording; add a Go unit test and a new Python registry test; leave the existing tamper tests unchanged.
- **Rationale**: satisfies the acceptance criterion "refused naming the key" literally while honouring the constraint that the existing tamper tests stay unchanged apart from the plugin.
- **Rejected**: treating the existing message as naming the key (it names only the signature file).

### Chosen direction: new xcl-plugin-test repo + plugin-agnostic Python harness (architecture)
- **Decision**: generate jumppad-labs/xcl-plugin-test from the template with one block type `test "item"` (log, value, input, sensitive secret, computed output), released signed at v0.1.0; xcl-python tests install it from the GitHub registry in place of xcl-plugin-docker; xcl changes only to name the signing key in its untrusted-key refusal.
- **Rationale**: the registry names assets after the repository, so a separate repo is the only clean home; the recording fixture already proves the log-file pattern across the plugin process boundary. Repo registration confirmed by the user.
- **Rejected**: assets on the xcl release; reusing the template's sample plugin; keeping Docker (see research.md alternatives).

### Block type and field names of the test plugin (architecture)
- **Decision**: `test "item" "<name>"` with `log`, `value`, `input`, `secret`, `output`; log records `{"operation","address"}` as JSON lines.
- **Rationale**: mirrors the recording fixture the Python suite already uses; short, obviously test-only names.
- **Rejected**: recording operations into a computed state attribute (lost on destroy).

### Single XCL_SRC resolver + strict mode in CI (architecture)
- **Decision**: `tests/xcl_source.py` resolves XCL_SRC (env, else sibling ../xcl) for every test; `XCL_TEST_STRICT=1` in CI makes harness skips fail.
- **Rationale**: the spec requires skips naming what to set locally, and the success metric requires zero e2e/registry skips in CI; strict mode makes CI prove it instead of silently skipping.
- **Rejected**: leaving per-conftest resolution (already diverged once); counting skips from pytest output in a CI script.

### GITHUB_TOKEN in the CI test step (architecture)
- **Decision**: pass `GITHUB_TOKEN: ${{ github.token }}` to pytest in ci.yml.
- **Rationale**: ~30 matrix jobs install the release from GitHub; unauthenticated API calls are limited to 60/hour per IP and runners share IPs. The engine already reads GITHUB_TOKEN (xcl registry/github.go:242-252).
- **Rejected**: caching the release between jobs (more workflow machinery).

### Extra CI failures covered here (architecture)
- **Decision**: fix Windows `.exe` fixture builds, the Windows interrupt test and the macOS stall-test threshold in this plan; leave the Windows parity access violation and Alpine musl load to the sibling spec.
- **Rationale**: confirmed in scope by the user; they are harness faults, independent of transport.
- **Rejected**: skipping those tests on the failing platforms (would breach "green everywhere").

### Conventions selected (architecture)
- **Decision**: apply testing-and-mocking, testing-provider-changed-and-update, assert-ordering-on-graph-parents, test-state-from-real-apply, dependencies, code-style, development-standards; drop xcl-internal conventions and the Go project-structure one in favour of the template layout.
- **Rationale**: those are the ones that bear on a Go plugin plus Python test changes; the rest concern xcl internals not touched.
- **Rejected**: listing every loaded convention.

### Milestone order; switching xcl-ref.txt to main left as a follow-up (milestones)
- **Decision**: harness fixes first (independent of the plugin), then the plugin release, then the Docker swap. Actually switching `xcl-ref.txt` to `main` is not a task here; the plan only guarantees (and manually checks) that the switch is one change.
- **Rationale**: the spec's requirement and acceptance criterion are that switching is a single change, not that it happens in this spec; f-libxcl merges only after the whole epic (including the documentation specs that depend on this spec), so a task gated on that merge would leave this plan unfinishable and deadlock the epic order.
- **Rejected**: a human-gated milestone 4 to merge f-libxcl and flip the ref (blocks the plan on the epic's own completion).

### Strict mode name and scope (tasks)
- **Decision** (revised by the user's cross-plan decisions, 2026-10-12): `XCL_TEST_STRICT=1` fails a harness skip only for something the CI job is meant to provide. Skips for sources, fixture plugins and the plugin release fail on every job. Missing-engine skips go through the sibling's `tests/engines.py::skip_missing_engine(route, reason)`, whose `expected_routes()` reads `XCL_TEST_ENGINES`: `subprocess` on every job, plus `inprocess` only on the Linux glibc x86-64 job that builds libxcl. Route-marker skips (`subprocess_only` / `inprocess_only`) are never strict failures. The sibling introduces both variables and their CI/release settings; this plan only extends them. Ordinary `skipif` platform markers are untouched.
- **Rationale**: makes the zero-skip success metric enforceable in CI without counting skips in a script.
- **Rejected**: `pytest -W error`-style global strictness (would also fail legitimate platform skips).

### Stall-test calibration (tasks)
- **Decision**: compare ticks during apply to an idle baseline on the same runner (≥ 50%) instead of a fixed 50.
- **Rationale**: keeps the test's meaning (threads are not stalled) while tolerating coarse timers on macOS runners.
- **Rejected**: lowering the fixed threshold (still machine-dependent); skipping on macOS (breaches green-everywhere).

### Test plugin xcl version (tasks)
- **Decision**: the test plugin requires the latest xcl tag at implementation time (v0.4.0 today).
- **Rationale**: the plugin protocol is unchanged since v0.3.0, so any recent tag works with the f-libxcl engine; the latest tag keeps the template's weekly "latest xcl" workflow green.
- **Rejected**: pinning v0.3.0 as the template does (older for no reason).

### Cross-plan decisions settled by the user (2026-10-12)
- **CHANGELOG**: the xcl task adds one `## 20261011232013-da18c586-python-release-hardening` entry at the top of xcl's CHANGELOG.md, prose ending `**Breaking:**` / `None.`; checked by manual review, never by a test.
- **Strict mode**: fails a harness skip only for what the job is meant to provide; route skips for engines the job doesn't build stay skips (see the revised strict-mode entry above).
- **Windows interrupts**: CTRL_BREAK_EVENT with CREATE_NEW_PROCESS_GROUP is the agreed method in both plans. The sibling's `xcl-core` discards Ctrl-C/Ctrl-Break through `signal.Notify` (not `signal.Ignore`) and runs in the host's process group, so the Python host drives cancellation. The sibling plan points its Windows interrupt coverage at this test.
- **Engine signal**: the sibling's `XCL_TEST_ENGINES` values (`subprocess`, plus `inprocess` on the Linux glibc x86-64 job) and its `skip_missing_engine` are used; this plan defines no engine reader of its own.

## Rehydration cues

- `spektacular spec file read 20261011232013-da18c586-python-release-hardening`
- `spektacular repo list`; xcl-python root /home/nicj/code/github.com/jumppad-labs/xcl-python
- Re-read xcl-python tests/integration/docker_helpers.py, test_github_tamper.py, test_docker_plugin.py, tests/unit/test_schema.py, tests/integration/conftest.py, Makefile, .github/workflows/ci.yml
- Re-read xcl bindings/libxcl/testdata/plugins/recording/main.go and xcl-plugin-template Makefile + .github/workflows/release.yml
- `gh run view 38174422045 --repo jumppad-labs/xcl-python` for the CI failures
