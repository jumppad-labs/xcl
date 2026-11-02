---
created_date: "2026-11-02"
document_status: draft
---

# Test plan: plugin release contract v2

These procedures cover the success metrics and manual reviews that the plan's Testing Approach marks as manual. Everything else is in the automated suites:

- xcl: the registry unit and integration tests, and the fake release server tests.
- Template, Docker and test plugins: the provider tests, the e2e tests and the CI `dist` job.
- xcl-python: the integration tests.

The releases this work produced are:

| Repository | Release |
|---|---|
| jumppad-labs/xcl-plugin-template-go | v0.1.0 |
| jumppad-labs/xcl-plugin-docker | v0.4.1 |
| jumppad-labs/xcl-plugin-test | v0.3.1 |

All are signed with the jumppad-labs release key, fingerprint `15BD A684 3A1F A0EA D1AA 5190 F775 DA00 AFD4 B502`. The key is published at https://xcl.dev/keys/jumppad-labs-releases.asc.

Common setup for the procedures below:

```shell
curl -fsSLO https://xcl.dev/keys/jumppad-labs-releases.asc
gpg --show-keys jumppad-labs-releases.asc   # compare with the fingerprint above
```

## Metric: a new release is produced from the template with one release command and the workflow

- **What to measure:** a release made from the template holds every asset the registry needs, with nothing written by hand:
  - `<name>-<version>-index.xcl`;
  - `<name>-<version>-index.xcl.sig`;
  - one `<name>-<version>-<os>-<arch>.zip` for each of linux, darwin and windows on amd64 and arm64.
- **How:**
  1. Create a repository from the template:

     ```shell
     gh repo create <you>/xcl-plugin-<name> --template jumppad-labs/xcl-plugin-template-go --private --clone
     ```

  2. Set `PLUGIN := xcl-plugin-<name>` in the Makefile.
  3. Add the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets.
  4. Run `make release VERSION=v0.0.1` on an up-to-date `main`.
  5. Once the workflow finishes, list the assets:

     ```shell
     gh release view v0.0.1 -R <you>/xcl-plugin-<name> --json assets --jq '.assets[].name'
     ```

- **Expected result:** 8 assets: the index, its `.sig` and 6 zips. Each zip holds the binary, `xcl-plugin.xcl`, `README.md` and `LICENSE` (`unzip -l`).
- **Who / when:** a maintainer, before announcing the template, and again after any change to the template's Makefile `dist` target or `release.yml`.
- **Recorded:** done for template v0.1.0 on 2026-11-01. It produced 8 assets, and the signature verified against the key above.

## Metric: pushing a version tag produces a verifiable release with no manual step

- **What to measure:** for each plugin repository, a pushed tag yields a release whose index signature verifies, whose zips match their checksums in the index, and which installs.
- **How:** for each repository and tag in the table above:
  1. Download the release and verify it:

     ```shell
     mkdir rel && cd rel
     gh release download <tag> -R <repo>
     export GNUPGHOME=$(mktemp -d)
     gpg --import ../jumppad-labs-releases.asc
     gpg --verify *-index.xcl.sig *-index.xcl
     for z in *.zip; do grep -q "$(sha256sum "$z" | cut -d' ' -f1)" *-index.xcl && echo "ok $z"; done
     ```

  2. Install it with xcl. From an xcl checkout at v0.13.1 or later:

     ```shell
     XCL_GITHUB_LIVE_PLUGIN=<repo>@<tag> XCL_GITHUB_LIVE_KEY=$PWD/../jumppad-labs-releases.asc \
       go test ./registry -run TestGitHubRegistryInstallsARealRelease -v
     ```

- **Expected result:**
  - `gpg` reports "Good signature" from the jumppad-labs key.
  - Every zip prints `ok`.
  - The live test passes.
- **Who / when:** a maintainer, after every plugin release. Tag from an up-to-date `main` (`git pull` before `make release`). Docker v0.4.0 and test v0.3.0 were tagged from a stale `main` and published the old layout.
- **Recorded:** done on 2026-11-01 and 2026-11-02, all on linux/amd64, for Docker v0.4.1 and the test plugin v0.3.1:
  - signatures verify;
  - checksums match;
  - the live test passes;
  - an install-and-restart check from the cache passes.

## Metric: both plugins install from their new releases, from Go and Python, verified against the published key, on every CI platform

- **What to measure:** Docker v0.4.1 and the test plugin v0.3.1 install and verify on linux, darwin and windows. Linux is covered by xcl-python's CI (`XCL_TEST_STRICT=1`) and by the live test above. darwin and windows are checked by hand.
- **How:** on a darwin/arm64 machine and a windows/amd64 machine, run the live test from step 2 of the previous metric for both releases. In an xcl-python checkout on the same machine, run the plugin-release tests:

  ```shell
  make engine plugins
  GITHUB_TOKEN=$(gh auth token) XCL_TEST_STRICT=1 python -m pytest tests/integration/test_test_plugin.py tests/integration/test_github_tamper.py
  ```

- **Expected result:** the live test passes for both releases on each machine, and the pytest run reports no failures or skips.
- **Who / when:** a maintainer with a Mac and a Windows machine, before the xcl-python release that carries engine v0.13.1.
- **Recorded:** not yet run on darwin or windows.

## Review: documentation describes the new layout, with no old-layout wording left

- **What to look at:**
  - xcl: `docs/plugins.md` (GitHub registry section) and `docs/plugin-layout.md` (Releasing), plus README line 183.
  - Template: `README.md`, release section.
  - Docker plugin and test plugin: `README.md`.
  - xcl-python: `README.md` (Getting started, Local plugins, Errors).
  - xcl-website pages: `/github-registry/`, `/plugin-template/`, `/registries/`, `/replacement/` and the home page.
- **What to look for:**
  - The index, the zips, the checksums in the index, and a signature over the index.
  - Either separator (`-` or `_`).
  - The cache holding the index, the signature, the zip and the extracted files.
  - Pins of Docker v0.4.1, the test plugin v0.3.1 and template v0.1.0.
  - No `tar.gz`, `checksums.txt`, or signature over a checksums file.
  - No pin of Docker v0.3.0 or v0.4.0, or of the test plugin v0.2.0 or v0.3.0.
- **Passing:** every page describes the new layout, and none describes or pins the old one.
- **Known issue to fix:** the Docker plugin's README pins v0.4.0. It should say v0.4.1.
- **Who / when:** a maintainer, before the website deploy and the xcl-python release.

## Review: the getting-started example creates and destroys its network and container against the Docker plugin's new release

- **What to look at:** the Docker example in xcl's README and in xcl-python's README "Getting started", both pinning Docker v0.4.1. The runnable form is xcl's `example/remoteplugin`.
- **How:** with Docker running, from an xcl checkout:

  ```shell
  go build -o /tmp/xcl-docker ./example/remoteplugin
  cd "$(mktemp -d)"
  /tmp/xcl-docker
  ```

  Build outside the checkout, so the tracked `example/remoteplugin/remoteplugin` binary isn't overwritten. Then run the Python README's Getting started program against the same Docker daemon.
- **Passing:**
  - Apply creates the `docker.network` and `docker.container` resources, and `docker ps` / `docker network ls` show them while they exist.
  - Destroy removes both, and no xcl-labelled containers or networks are left.
- **Who / when:** a maintainer, before the xcl-python release.
- **Recorded:** the Go example passed against Docker v0.4.1 on 2026-11-01 (apply, then destroy). The Python program has not been run by hand yet.

## Review: the website builds and the changed pages read correctly in a browser

- **How:**

  ```shell
  cd xcl-website
  npm ci
  npm run build
  npm run preview
  ```

  Then open `/github-registry/`, `/plugin-template/`, `/registries/`, `/replacement/` and `/`.
- **What to look for:**
  - The sample index and the cache listing render as code blocks.
  - The plugin-template page's link to "Installing plugins from GitHub" lands on "What a release must contain".
  - "Use the template" opens `jumppad-labs/xcl-plugin-template-go`.
  - The replacement page's Docker code shows `ReplaceOn` and is titled with `xcl-plugin-docker`.
- **Passing:** the build completes, and each page renders with working links and no broken layout.
- **Who / when:** a maintainer, before the website deploy.
- **Recorded:** `npm run build` completed on 2026-11-02 (23 pages). The browser check has not been done yet.
