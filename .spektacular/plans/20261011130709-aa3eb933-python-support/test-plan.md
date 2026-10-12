---
created_date: "2026-10-11"
document_status: final
closed_date: "2026-10-11"
---

# Test plan: 20261011130709-aa3eb933-python-support

Manual checks for the success metrics and reviews the plan's Testing Approach left to a person, grounded in what was built. Everything else is covered by the automated suites: the Go suites in xcl (`go test -race ./...`, the nested `bindings/libxcl` module and its `cmd/parity` tests) and the Python suite in xcl-python (`make test`, 193 tests: unit, integration against the engine, Docker plugin, parity and asyncio).

## Prerequisites

Three human tasks of the plan are still open and gate the release checks below:

1. **Secrets on jumppad-labs/xcl** (task 15487d49): `XCL_RELEASE_GPG_KEY` (armoured private key of the jumppad-labs release key, fingerprint `15BDA6843A1FA0EAD1AA5190F775DA00AFD4B502`), `XCL_RELEASE_GPG_PASSPHRASE`, `XCL_PYTHON_DISPATCH_TOKEN` (fine-grained token, Contents: read and Actions: write on jumppad-labs/xcl-python).
2. **PyPI trusted publishing** (task fe0be452): the PyPI project `xcl-lang` owned by jumppad-labs, with trusted publisher owner `jumppad-labs`, repository `xcl-python`, workflow `release.yml`, environment `pypi`; and the `pypi` environment created in jumppad-labs/xcl-python.
3. **The release tag** (task 75d985e2): after the spec branches are merged, the orchestrator pushes `v0.4.0` on xcl (`git tag v0.4.0 && git push origin v0.4.0`), which runs the chain below.

## 1. Dry run of the libxcl release workflow (release-workflow task criteria)

- **What**: the four acceptance criteria of "Tagging xcl publishes signed engine builds for eight platforms", left unchecked in the plan: eight archives, a checksums file and its signature; each library reports the tag and the glibc builds need nothing newer than 2.28; the workflow dispatches xcl-python; a manual run without publishing builds every target.
- **How**: with the secrets in place, in jumppad-labs/xcl → Actions → "libxcl release" → Run workflow with `tag` = an existing tag (for example `v0.3.0`) and `publish` = false. Download the `release-assets` artifact. Run `gpg --import <(curl -fsSL https://xcl.dev/keys/jumppad-labs-releases.asc)` and `gpg --verify libxcl_0.3.0_checksums.txt.sig libxcl_0.3.0_checksums.txt`. On Linux: `tar -xzf libxcl_0.3.0_linux_amd64.tar.gz && objdump -T libxcl.so | grep -o 'GLIBC_[0-9.]*' | sort -V | tail -1`.
- **Expected**: the jobs `settings`, five `build-*` matrix entries, `build-windows-arm64`, two `build-macos` entries and `package` succeed; the artifact holds exactly `libxcl_0.3.0_{linux_amd64,linux_arm64,linux_amd64_musl,linux_arm64_musl,darwin_amd64,darwin_arm64}.tar.gz`, `libxcl_0.3.0_{windows_amd64,windows_arm64}.zip`, `libxcl_0.3.0_checksums.txt` and `.sig`; gpg reports a good signature from the jumppad-labs key; the newest glibc symbol version is `2.28` or lower; the `dispatch` job is skipped because `publish` was false.
- **Who / when**: the person who adds the secrets, before cutting v0.4.0.

## 2. Watch the first tagged release end to end (plan manual review)

- **What**: the plan's "watch the first tagged release end to end" review and the CI/release task's four criteria, left unchecked in the plan.
- **How**: push `v0.4.0` on xcl. In jumppad-labs/xcl → Actions → "libxcl release", watch the run for the tag; then in jumppad-labs/xcl-python → Actions → "Release", watch the run dispatched with the same tag (`repository_dispatch`, event `xcl-release`).
- **Expected**: the xcl release at https://github.com/jumppad-labs/xcl/releases/tag/v0.4.0 carries the eight `libxcl_0.4.0_*` archives, `libxcl_0.4.0_checksums.txt` and `.sig`. The xcl-python run is green through `install-check`: eight `wheels` jobs (each verifying the signature and checksum, checking the engine reports `v0.4.0`, building `xcl_lang-0.4.0-py3-none-<platform>.whl` and running the parity suite against it), `sdist`, `publish` (Trusted Publishing from the `pypi` environment) and eight `install-check` jobs that install `xcl-lang==0.4.0` with no Go or C compiler on the path and run `python -c "import xcl; xcl.Config().close()"` plus the engine version check. https://pypi.org/project/xcl-lang/0.4.0/ lists eight wheels tagged `py3-none-{manylinux_2_28_x86_64, manylinux_2_28_aarch64, musllinux_1_2_x86_64, musllinux_1_2_aarch64, macosx_11_0_x86_64, macosx_11_0_arm64, win_amd64, win_arm64}` and one sdist.
- **Who / when**: the person cutting the release, as it runs.

## 3. From `pip install xcl-lang` to a working apply by the README alone (success metric)

- **What**: a Python developer gets from install to an apply against a Go plugin following only the package README.
- **How**: on a clean machine (or a fresh virtualenv on a machine with Docker and no Go or C compiler on the path, for example `docker run -it --rm -v /var/run/docker.sock:/var/run/docker.sock python:3.12 bash` with the Docker CLI's socket mounted), follow https://github.com/jumppad-labs/xcl-python/blob/main/README.md from "Install": `pip install xcl-lang`, save the release key from https://xcl.dev/keys/jumppad-labs-releases.asc as `jumppad-labs-releases.asc`, create `infra/main.xcl` with the README's configuration, run the README's getting-started program, then the asyncio example.
- **Expected**: `pip install xcl-lang` succeeds; the program prints the settings and the network's address, subnet and Docker id, and `destroy` removes the container and network (`docker ps -a` and `docker network ls` show neither); the asyncio example prints `my-app`. Nothing outside the README was needed. Any step that needed a hint not in the README fails this check and is a README fix.
- **Who / when**: someone who did not write the package, after the first release is on PyPI.

## 4. Review `PARITY.md` against the Go API of the released tag (success metric)

- **What**: every operation and lookup in the Go API has a Python equivalent in both APIs, checked against the parity list at release.
- **How**: with xcl checked out at the tag, run `go doc github.com/jumppad-labs/xcl Config | grep -E '^func'` and `go doc github.com/jumppad-labs/xcl | grep -E '^func (Find|FindByType|FindOne|All|As|Decode|EncodeEntity)'`, and compare with the tables in https://github.com/jumppad-labs/xcl-python/blob/main/PARITY.md. For the Python side, `python -c "import xcl, xcl.aio, inspect; print(sorted(n for n in dir(xcl.Config) if not n.startswith('_'))); print(sorted(n for n in dir(xcl.aio.Config) if not n.startswith('_')))"`.
- **Expected**: every exported `Config` method and every package-level lookup function in the Go output appears in `PARITY.md`, either mapped to a `xcl.Config` and `xcl.aio.Config` method that the Python output lists, or in the "Not applicable" table with its reason. A Go method missing from the list fails the check.
- **Who / when**: the person cutting each release, before tagging (the test `test_sync_and_async_expose_same_surface` already holds the two Python surfaces equal).

## 5. Review the xcl CHANGELOG entry (plan manual review)

- **What**: the `## 20261011130709-aa3eb933-python-support` entry in xcl's `CHANGELOG.md`.
- **How**: open `CHANGELOG.md` at the top of the xcl repository.
- **Expected**: the entry sits above language-surface's, is prose, and describes the context-taking operations, the untyped lookups, `registry.TypeFromSchema` and schema v1, the `coreservice` package and `proto/xcl/core/v1`, the `bindings/libxcl` module and its release assets, and the `xcl-lang` package; it ends with `**Breaking:**` followed by `None.`; it names the release as v0.4.0.
- **Who / when**: a maintainer, before tagging.

## 6. Review the PyPI project page (plan manual review)

- **What**: the PyPI page shows the README with install, synchronous getting started and the asyncio section, in that order.
- **How**: open https://pypi.org/project/xcl-lang/ after the release.
- **Expected**: the page renders the README (`pyproject.toml` sets `readme = "README.md"`, and `twine check` passed in CI) with the sections "Install", "Getting started" and "Asyncio" in that order, the eight platforms and CPython 3.10+ named under Install, and the sidebar showing version 0.4.0, licence Apache-2.0 and the Source link to jumppad-labs/xcl-python.
- **Who / when**: the person cutting the first release, once PyPI shows it.
