---
created_date: "2026-10-10"
document_status: final
closed_date: "2026-10-10"
---

# Test plan: 20261009092551-82db0140-plugin-template

These are the manual checks for the plugin template. The automated suite covers everything else: the files client, provider and plugin unit tests, the e2e lifecycle in-process and external, the replace and update plans, and the state reader. That suite runs with `make test` in the template repository.

Repositories, after merge: `jumppad-labs/xcl-plugin-template`, `jumppad-labs/xcl`, `jumppad-labs/xcl-website`.

## 1. First signed release of the template (deferred human task)

- **What:** the template's first version tag publishes a signed release in the release asset contract, with no manual steps. The signature must verify with gpg and with xcl's go-crypto verifier in the GitHub registry. The release was deferred from the implement run until after the merge.
- **How:**
  1. Commit the merged template files and push them to `jumppad-labs/xcl-plugin-template` `main`. Wait for the **CI** workflow (`check` and `dist` jobs) to go green.
  2. From a clone of `main`, run `make release VERSION=v0.1.0`. This pushes the tag `v0.1.0`.
  3. Wait for the **Release** workflow to finish, then run `gh release view v0.1.0 -R jumppad-labs/xcl-plugin-template`.
  4. Download the assets: `gh release download v0.1.0 -R jumppad-labs/xcl-plugin-template -D /tmp/rel`. Then run `cd /tmp/rel && sha256sum -c xcl-plugin-template_0.1.0_checksums.txt`.
  5. Run `curl -fsSLO https://xcl.dev/keys/jumppad-labs-releases.asc && gpg --import jumppad-labs-releases.asc && gpg --verify xcl-plugin-template_0.1.0_checksums.txt.sig xcl-plugin-template_0.1.0_checksums.txt`.
  6. From the xcl repository, run `XCL_GITHUB_LIVE_PLUGIN=jumppad-labs/xcl-plugin-template@v0.1.0 XCL_GITHUB_LIVE_KEY=<path to jumppad-labs-releases.asc> make test-github-live`. This is the opt-in live test, which checks that go-crypto verifies the real gpg signature.
- **Expected:**
  - The release `v0.1.0` exists and is not a draft. It holds exactly these files:
    - `xcl-plugin-template_0.1.0_{linux,darwin}_{amd64,arm64}.tar.gz`
    - `xcl-plugin-template_0.1.0_windows_{amd64,arm64}.zip`
    - `xcl-plugin-template_0.1.0_checksums.txt`
    - `xcl-plugin-template_0.1.0_checksums.txt.sig`
  - Every checksum line reports `OK`.
  - `gpg --verify` reports a good signature from `15BD A684 3A1F A0EA D1AA  5190 F775 DA00 AFD4 B502`.
  - `TestGitHubRegistryInstallsARealRelease` passes.
  - If the registry rejects the genuine signed release, stop and raise it. Do not loosen signing or verification.
- **Who / when:** a maintainer, right after the merge.

## 2. Install the release with the GitHub registry and apply the sample

- **What:** an application installs the released plugin from GitHub, with the trusted key supplied, and applies the sample. This closes the registry plan's real-GitHub check.
- **How:** in a scratch Go module requiring `github.com/jumppad-labs/xcl`, do the following:
  1. Build `registry.NewGitHub(registry.GitHubTrustedKeys(<contents of jumppad-labs-releases.asc>))`.
  2. Call `RegisterPlugin("jumppad-labs/xcl-plugin-template", "v0.1.0")`.
  3. Call `xcl.NewConfig(xcl.WithRegistry(gh), xcl.WithStatePath(<tmp>), xcl.WithVariables(map[string]any{"directory": <tmp out>}))`.
  4. Run `Apply` on the template's `examples/basic`, then `Diff`, then `Destroy`.
- **Expected:**
  - Apply succeeds and writes `<tmp out>/greeting.txt` containing `Hello from xcl`.
  - `Diff(...).Changed()` is 0.
  - Destroy removes the file.
  - The same run with a different trusted key fails with `xcl.ErrPluginVerification`.
- **Who / when:** a maintainer, after check 1.

## 3. Every README command works on a fresh repository from the template

- **What:** the README covers the whole workflow, and every command it shows succeeds unchanged.
- **How:**
  1. Run `gh repo create <you>/xcl-plugin-check --template jumppad-labs/xcl-plugin-template --private --clone && cd xcl-plugin-check`.
  2. Run `make build`, `make test`, `make generate` (then `git status` must show no changes), `make lint`, `make inprocess`, `make external` and `make dist VERSION=v0.0.0-snapshot`.
  3. With `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` set from a throwaway key made as the README's "Signing key" section shows, run `make release VERSION=v0.0.1`.
  4. Delete the repository afterwards.
- **Expected:**
  - Every command exits 0.
  - `make generate` leaves no diff.
  - `dist/` holds the six archives and the checksums file.
  - The release workflow publishes `v0.0.1` with a `.sig`.
  - With the `GPG_PRIVATE_KEY` secret deleted, a further tag fails at "Require the signing key" and nothing is published.
- **Who / when:** a maintainer, after the merge.

## 4. Time to a first plugin (success metric: under 15 minutes)

- **What:** a person new to the template gets from "Use this template" to a passing, applying plugin of their own using only the README. Target: under 15 minutes.
- **How:** give a developer who has not seen the template only its README. Time them from clicking **Use this template** until they have finished these steps:
  1. Done the README's "Make it yours" renames: module path, `PLUGIN`, root package and `cmd/<plugin>`.
  2. Run `make test`, `make inprocess` and `make external`, each passing.
- **Expected:** at most 15 minutes, with no help beyond the README.
- **Who / when:** a developer new to the template, before announcing it.

## 5. First real resource without guessing (success metric)

- **What:** a person adds a second resource by following only the README's "Add your own resource" section and the layout guide, without opening the xcl examples or source.
- **How:** ask a developer to add a second resource to a repository made from the template, using only `README.md` and `docs/plugin-layout.md` in the xcl repo. A small `notes "folder"` that creates a directory is enough. It needs the following:
  - an entity
  - a provider with its replace list
  - unit tests against a double
  - registration in `plugin.go`
  - a sample and an e2e test
- **Expected:**
  - `make test` passes with the new resource.
  - The developer reports that they did not need to open the xcl examples or source.
  - The new files sit where the layout guide says.
- **Who / when:** a developer new to the template, before announcing it.

## 6. The template stays current: first green scheduled run (success metric)

- **What:** the weekly workflow runs the suite against the latest xcl release, and goes green against a real release.
- **How:** in `jumppad-labs/xcl-plugin-template`, open Actions, choose **Latest xcl**, then **Run workflow** on `main`. Alternatively, wait for the Monday 06:00 UTC schedule.
- **Expected:**
  - The run is green.
  - Its "Use the latest xcl release" step prints the newest `github.com/jumppad-labs/xcl` release tag.
- **Who / when:** a maintainer, after the merge, and again after each xcl release.

## 7. One shape everywhere (success metric)

- **What:** the template tree, the layout guide and the site page all agree with the `plugin-layout.md` design.
- **How:** open the design (`spektacular design read --data '{"source":"design","path":"plugin-layout.md"}'`), `docs/plugin-layout.md` in xcl, `https://xcl.dev/plugin-template/` and the template's tree (`git ls-files` in the template). Then:
  1. Check that every plugin part in the template sits in a place the design names.
  2. Check that every location the guide names exists in the template.
- **Expected:**
  - No plugin part is in a place the design does not name.
  - Repository tooling (`.github/workflows/`, `LICENSE`, `.gitignore`) is outside the layout tree by the plan's decision.
  - Every location the guide and page name is present in the template.
- **Who / when:** a reviewer, before announcing the template.

## 8. CI reports failures on the change

- **What:** a push or pull request that breaks vet, formatting or a test fails the check on the change.
- **How:** on a branch of the template, push three commits, each reverted after its run:
  1. An unformatted Go file, for example `var  x=1` in `entities/`.
  2. A `go vet` error, for example a `fmt.Printf("%d", "s")` in `providers/note.go`.
  3. A failing assertion in `providers/note_test.go`.
  Open a pull request for one of them.
- **Expected:**
  - The `check` job fails at "Static checks" for the first two commits and at "Test" for the third.
  - The pull request shows the failed check.
- **Who / when:** a maintainer, after the merge.

## 9. File order review

- **What:** every template source file lists exported members first, and provider methods follow the lifecycle order.
- **How:** read `plugin.go`, `cmd/notes/main.go`, `entities/note.go`, `providers/note.go` and `client/files/files.go` in the template.
- **Expected:**
  - Exported types, vars and the constructor come before unexported helpers, vars and consts.
  - `providers/note.go` orders its methods `Init`, `Create`, `Read`, `Changed`, `Update`, `Destroy`, `Functions`.
- **Who / when:** a reviewer, at merge review.

## 10. The template is a registered project repository

- **What:** the template appears in the project's registered repositories with a description, and its root is on disk.
- **How:** run `spektacular repo list` from the xcl project root.
- **Expected:**
  - An `xcl-plugin-template` entry with a description and role, and a `root` that exists on disk and is a clone of `jumppad-labs/xcl-plugin-template`.
  - Note: the description still mentions GoReleaser, and this has been reported to the user separately.
- **Who / when:** a maintainer, after the merge.

## 11. Documentation review

- **What:** the site page, its navigation entry, the layout guide, the README links and the changelog entry are accurate.
- **How:**
  1. In `xcl-website`, run `npm ci && npm run build && npx astro check`, then `npm run dev`.
  2. Open `/plugin-template/` and check its content and its link to `/keys/jumppad-labs-releases.asc`.
  3. Check that the Guides menu lists "Plugin template".
  4. Check the links to the page from `/registries/` and `/examples/plugins/`.
  5. In xcl, read `docs/plugin-layout.md`, the links in `docs/README.md`, `docs/plugin-developer-guide.md` and `README.md` ("Writing a plugin"), and the top `CHANGELOG.md` entry.
- **Expected:**
  - The build and check pass with 0 errors.
  - The page and links render and resolve.
  - The text matches the template and the designs.
  - `https://xcl.dev/keys/jumppad-labs-releases.asc` serves the key, and `gpg --show-keys` shows the fingerprint `15BD A684 3A1F A0EA D1AA  5190 F775 DA00 AFD4 B502`.
- **Who / when:** a reviewer, at merge review, and again after the site deploys.
