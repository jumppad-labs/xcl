---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Context: 20261009092551-82db0140-plugin-template

## Current State Analysis

- The plugin template repository does not exist yet. It is registered in the project as `xcl-plugin-template` (provider git, source `git@github.com:jumppad-labs/xcl-plugin-template.git`), and its footprint is at `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template/.spektacular/`. Its root stays empty until the first human task creates the GitHub repo and makes that directory the clone.
- xcl's plugin contract today is in `plugins/provider.go` (`ResourceProvider[T]`) and `plugins/changed.go:24-49` (`DefaultChanged`). `Changed`/`Update` receive `[]entity.PropertyChange` and `[]entity.DependencyChange`, and `entity/property_change.go:50` has `Within`. In-process registration is `registry/local.go:121` and external is `registry/local.go:131`. Serving is `plugins/serve.go:21`.
- The closest worked plugin is `example/plugin/plugins/docker/` (plugin type, client interface plus Mockery double, constructor-injected providers, strict-double tests). The sibling plan `20261009102138-48e95432-docker-example-standard-layout` moves it to the standard layout. `example/plugin/plugins/template/` is a Handlebars plugin, unrelated to this template.
- Only `v0.1.0` of xcl is tagged, and it predates `entity.PropertyChange`.
- No written plugin layout guide exists in `docs/`: there are `docs/plugins.md`, `docs/plugin-developer-guide.md` and `docs/README.md`. The site has no plugin template page. Its Guides nav is at `xcl-website:src/components/Nav.astro:17-29`, and the GitHub registry plan adds `/github-registry/`.
- CI for xcl lives in `.github/workflows/go.yml` (Go 1.27.0 tests, 1.25.0 minimum build).

## Per-Task Technical Notes

Requirement-to-repo attribution:

| Requirement | Repo | Where |
|---|---|---|
| Ready-to-build starting plugin; runs in-process and separately; explicit change decision; told-only update; tests in the project's style; entity types stand alone; consistent file order | xcl-plugin-template | `plugin.go`, `cmd/notes/`, `entities/`, `providers/`, `client/files/`, `examples/basic/`, `e2e/`, `Makefile`, `.mockery.yml` |
| How to build, test and register it | xcl-plugin-template | `README.md` |
| Checked on every change | xcl-plugin-template | `.github/workflows/ci.yml`, `.github/workflows/current.yml` |
| Signed releases for every platform | xcl-plugin-template | `.goreleaser.yaml`, `.github/workflows/release.yml` |
| Kept up to date with the project | project registry (`.spektacular` config of xcl) | `spektacular repo add` |
| A standard plugin layout; documentation | xcl | `docs/plugin-layout.md`, `docs/README.md`, `docs/plugin-developer-guide.md`, `README.md`, `CHANGELOG.md` |
| Documentation site covers the template | xcl-website | `src/pages/plugin-template.mdx`, `src/components/Nav.astro`, `src/pages/registries.mdx`, `src/pages/examples/plugins.mdx` |

### Task: Scaffold the template module and its files client

**File changes** (all in the template repository):

- `xcl-plugin-template:go.mod` (new) — `module github.com/jumppad-labs/xcl-plugin-template`, `go 1.25.0` (matches xcl's `go.mod:3`), require `github.com/jumppad-labs/xcl` at a pseudo-version of the pushed commit carrying the current contract (`entity.PropertyChange`, `Changed`/`Update` with `changes`), and `github.com/stretchr/testify` pinned. No `replace` directives.
- `xcl-plugin-template:LICENSE` (new) — Apache-2.0. The repository already exists on GitHub (public, `main`, marked as a template) and is checked out at the registered root; the uncommitted `.spektacular/repo.yaml` footprint change (source `provider: file`, `location: ..`) is committed with this task.
- `xcl-plugin-template:.gitignore` (new) — `build/`, `dist/`, `.xcl/`, `out/`.
- `xcl-plugin-template:client/files/files.go` (new) — package doc; exported first: `Files` interface (`Write`, `Chmod`, `Stat`, `Remove`, each taking `ctx`), `Info{Checksum string; Mode fs.FileMode}`, `Local` struct with `var _ Files = (*Local)(nil)`, `NewLocal()`, then the `Local` methods; unexported helpers (`checksum([]byte) string`) last. `Write` creates parent dirs (`os.MkdirAll`), writes, then `os.Chmod` (WriteFile only sets mode on create — same gotcha handled in `example/plugin/plugins/template/template.go:189-194`), returns `Info`. `Stat` reads the file, returns checksum and mode, wrapping `fs.ErrNotExist`. `Remove` treats `fs.ErrNotExist` as success. Each method checks `ctx.Err()` first.
- `xcl-plugin-template:client/files/files_test.go` (new) — tests against `t.TempDir()`, one function each: write creates parent directories and reports checksum and mode; write over an existing file sets the new mode; chmod changes the mode; stat reports checksum and mode; stat of a missing file wraps `fs.ErrNotExist` (negative, separate); remove deletes the file; remove of a missing file succeeds; a cancelled context is returned as an error (negative).
- `xcl-plugin-template:.mockery.yml` (new) — copy of `example/plugin/.mockery.yml` with the package `github.com/jumppad-labs/xcl-plugin-template/client/files`, interface `Files`.
- `xcl-plugin-template:client/files/mocks/mock_files.go` (generated) — `MockFiles`, via `make generate`.
- `xcl-plugin-template:Makefile` (new) — header comment; `MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0` (as `example/plugin/Makefile:16-18`); targets `build` (`go build ./...` and `go build -o build/$(PLUGIN) ./cmd/notes` once the program exists — add the binary line in the program task), `test` (`go test -race ./...`), `generate` (`$(MOCKERY)`), `clean`. `PLUGIN ?= $(notdir $(CURDIR))` is not used for the binary name; use `PLUGIN := xcl-plugin-template` with a comment telling authors it matches their repo name.

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add the note entity and its provider

**File changes**:

- `xcl-plugin-template:entities/note.go` (new) — package doc stating entities hold block types only and import nothing from providers/clients; `Note` as in Data Structures, with doc comments on every field saying which need a replace. Imports only `github.com/jumppad-labs/xcl/types`. Plain field types only (knowledge `gotchas/plugin-types-rebuilt-with-structof.md`).
- `xcl-plugin-template:providers/note.go` (new) — order per `plugin-layout.md` "File order": `ReplaceSettings` var (exported, doc: "edit this list to change what needs a replace"), `NoteProvider` type, `var _ plugins.ResourceProvider[*entities.Note]`, `NewNoteProvider(files files.Files) *NoteProvider`, then `Init` (logs ready), `Create` (computes path = `filepath.Join(Directory, Name)`, mode via `fileMode`, calls `files.Write`, sets `Path` and `Checksum`, logs via `plugins.Logger(ctx)` with `path`), `Read` (calls `files.Stat(old.Path)`; on `fs.ErrNotExist` returns `plugins.ErrNotFound`; else returns `new` with `Path` and `Checksum` from the real file — must not change configured fields, per `plugins/provider.go` Read doc), `Changed` (loop `changes`: `Within` any `ReplaceSettings` → `entity.Replace`; loop `dependencies`: `.Change == entity.Replace` → `entity.Replace`, with a comment "narrow this to the types your resource is built on"; `len(changes) > 0` → `entity.Update`; else `p.DefaultChanged.Changed(...)`), `Update` (walk `changes`: a change `Within` `content` → `files.Write` with new content and mode, set `Checksum`; else a change `Within` `mode` only → `files.Chmod`; never `Stat`; keep `Path`), `Destroy` (`files.Remove(Path)`), `Functions` (nil). Unexported helpers last: `fileMode(string) (fs.FileMode, error)` (octal parse, default 0644, same rules as `example/plugin/plugins/template/template.go:205-217`), `defaultMode` const.
- `xcl-plugin-template:providers/note_test.go` (new) — each test builds `NewNoteProvider(mocks.NewMockFiles(t))` (testify mock asserts expectations at cleanup and fails on unexpected calls). Separate functions: `TestCreateWritesTheNoteAndSetsComputedValues`; `TestCreateReturnsWriteError` (negative); `TestCreateRejectsAnInvalidMode` (negative, no calls); `TestReadFillsPathAndChecksumFromTheFile`; `TestReadReportsNotFoundForAMissingFile` (negative); `TestChangedReplacesWhenDirectoryChanges`; `TestChangedReplacesWhenNameChanges`; `TestChangedReplacesWhenADependencyIsReplaced`; `TestChangedUpdatesWhenContentChanges`; `TestChangedUpdatesWhenModeChanges`; `TestChangedDoesNotReplaceForAnUpdatedDependency`; `TestChangedDefersWhenNothingChanged`; `TestUpdateRewritesOnlyForAContentChange` (only `Write` expected; a `Stat` would fail); `TestUpdateChangesOnlyTheModeForAModeChange` (only `Chmod` expected); `TestUpdateReturnsWriteError` (negative); `TestDestroyRemovesTheFile`; `TestDestroyReturnsRemoveError` (negative). Changes are built as real `entity.PropertyChange{Path: entity.Path{}.Attribute("content"), Before: "a", After: "b"}`; dependencies as `entity.DependencyChange{Address: "notes.note.other", Change: entity.Replace}` (confirm field names in `entity/change.go`).

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent: entity, then provider, then tests (the tests depend on the provider's exact calls).

### Task: Add the plugin type, its program and the sample configuration

**File changes**:

- `xcl-plugin-template:plugin.go` (new) — package `notes` doc (what the plugin provides, how to rename it); `Plugin{plugins.PluginBase}`, `var _ plugins.Plugin = (*Plugin)(nil)`, `Init(logger, state)` builds `files.NewLocal()` once and calls `plugins.RegisterResourceProvider(&p.PluginBase, logger, state, "notes", "note", &entities.Note{}, providers.NewNoteProvider(filesClient))`, as `example/plugin/plugins/docker/plugin.go:28-61` does.
- `xcl-plugin-template:plugin_test.go` (new) — `TestInitRegistersTheNoteType`: calls `Init` with a no-op logger and nil/empty state as `example/plugin/plugins/template/plugin_test.go` does and requires the `notes`/`note` type is provided; asserts nothing was written (temp dir untouched is implicit — Local touches nothing at construction).
- `xcl-plugin-template:cmd/notes/main.go` (new) — doc comment; `func main() { plugins.Serve(&notes.Plugin{}) }` as `example/plugin/plugins/docker/main.go:15-17`.
- `xcl-plugin-template:examples/basic/main.xcl` (new) — `variable "directory" { default = "./out" }` and one `notes "note" "greeting" { directory = variable.directory  name = "greeting.txt"  content = "Hello from xcl" }` (check the exact variable reference syntax against `internal/test_fixtures/config/simple`).
- `xcl-plugin-template:Makefile` — `build` also produces `build/$(PLUGIN)` from `./cmd/notes`.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add the end-to-end and state-reader tests

**File changes**:

- `xcl-plugin-template:e2e/main_test.go` (new) — `TestMain` builds `../cmd/notes` once into a temp dir with `os/exec` (`go build -o`), stores the path; helpers `newInProcessConfig(t, stateDir)` (`registry.NewLocal(); local.RegisterPlugin(&notes.Plugin{})`) and `newExternalConfig(t, stateDir)` (`local.RegisterExternalPlugin(binaryPath)`), each `xcl.NewConfig(xcl.WithRegistry(local), xcl.WithStatePath(stateDir), xcl.WithVariables(map[string]any{"directory": outDir}))` (`options.go:22,50,126`). Keep the registration lines short and literal: the README quotes them.
- `xcl-plugin-template:e2e/inprocess_test.go` (new) — `TestInProcessAppliesPlansNoChangesAndDestroys`: `Apply("../examples/basic")`, require note file content, `Diff([]string{"../examples/basic"})` reports no changes (check the `diff.Diff` summary/counts API in `diff/`), `Destroy()`, require the file is gone.
- `xcl-plugin-template:e2e/external_test.go` (new) — `TestExternalAppliesPlansNoChangesAndDestroys`: the same steps through the built binary. No comparison with the in-process result (spec non-goal).
- `xcl-plugin-template:e2e/changes_test.go` (new) — `TestPlanReplacesTheNoteWhenItsNameChanges` and `TestPlanUpdatesTheNoteWhenItsContentChanges`: first apply the sample (real apply for state, knowledge `conventions/test-state-from-real-apply.md`), then `Diff` a copy of the configuration written to `t.TempDir()` with the edited setting, and require the resource's action is replace / update. Use the external binary (the shape a released plugin runs in).
- `xcl-plugin-template:e2e/stateonly/stateonly_test.go` (new, own package `stateonly`) — imports only `github.com/jumppad-labs/xcl`, `.../registry` and `.../entities` (no `notes`, `providers` or `client`); builds `../../cmd/notes` itself in `TestMain`; applies the sample through `RegisterExternalPlugin`, then `xcl.FindByType[entities.Note](c, "notes", "note")` (`query.go:180`; plugin entities convert into a named Go type per knowledge `gotchas/registered-types-convert-only-to-themselves.md`) and requires the note's name, content and computed path. If `FindByType` cannot convert, STOP and ask (open assumption).
- `xcl-plugin-template:Makefile` — targets `inprocess` (`go test -v -run TestInProcess ./e2e`) and `external` (`go test -v -run TestExternal ./e2e`), each with a comment saying what it shows.

**Complexity**: Medium
**Token estimate**: ~40k tokens
**Agent strategy**: Single agent; write `main_test.go` helpers first, then the four test files.

### Task: Write the template README

**File changes**:

- `xcl-plugin-template:README.md` (new) — sections in task order: what the template is (one `notes "note"` resource, in-process and external, signed releases); Start (`Use this template` / `gh repo create <you>/xcl-plugin-<name> --template jumppad-labs/xcl-plugin-template --clone`); Make it yours (module path in `go.mod` and imports, root package and `cmd/<plugin>` directory, `PLUGIN` in the Makefile, block type in `plugin.go`); Layout (the design tree with one line per part, linking xcl's `docs/plugin-layout.md`); Build (`make build`); Test (`make test`); Regenerate the double (`make generate`); Use it in-process (`make inprocess`, quoting the in-process registration lines from `e2e/main_test.go`); Use it as a separate program (`make external`, quoting `RegisterExternalPlugin`); The change decision (`ReplaceSettings`, how `Changed` reads); Updating from what you are told; Add your own resource (step list: entity file, client package and interface, `.mockery.yml` entry + `make generate`, provider file in lifecycle order with replace list, unit tests against the double, register in `plugin.go`, sample config and an e2e test); Tools and why (Mockery, staticcheck, GoReleaser — pinned versions, reason for each, per the dependencies convention). Release and install sections are added by the release automation task.

**Complexity**: Medium
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential; run every command it documents on a fresh copy before finishing.

### Task: Add the template's continuous integration

**File changes**:

- `xcl-plugin-template:.github/workflows/ci.yml` (new) — `on: push` (all branches) and `pull_request`; job `check` on `ubuntu-latest`: `actions/checkout@v4`, `actions/setup-go@v5` with `go-version-file: go.mod`, steps `go build ./...`, `go vet ./...`, `test -z "$(gofmt -l .)"` (prints offending files), `go run honnef.co/go/tools/cmd/staticcheck@<pinned 2025.x release> ./...`, `make test`. Shape follows xcl's `.github/workflows/go.yml:15-32`.
- `xcl-plugin-template:.github/workflows/current.yml` (new) — `on: schedule` (weekly cron) and `workflow_dispatch`; same setup, then `go get github.com/jumppad-labs/xcl@latest && go mod tidy`, `go vet ./...`, `make test`. Comment: "fails when the latest xcl release breaks the template — update the template".
- `xcl-plugin-template:Makefile` — target `lint` running the vet, gofmt and staticcheck steps locally, used by CI so local and CI checks match.

**Complexity**: Low
**Token estimate**: ~15k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Add signed release automation

**File changes**:

- `xcl-plugin-template:.goreleaser.yaml` (new) — `version: 2`; no `project_name` (defaults to the GitHub repo name, the plugin name per `plugin-release-assets.md`), with a comment saying so; `builds: - main: ./cmd/notes, binary: "{{ .ProjectName }}", env: [CGO_ENABLED=0], goos: [linux, darwin, windows], goarch: [amd64, arm64]`; `archives: - formats: [tar.gz], format_overrides: [{goos: windows, formats: [zip]}], name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}", files: [README.md, LICENSE]` (explicit name template so a GoReleaser default change cannot break the contract); `checksum: name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt", algorithm: sha256`; `signs: - artifacts: checksum, signature: "${artifact}.sig", args: ["--batch", "--yes", "--pinentry-mode", "loopback", "--passphrase", "{{ .Env.GPG_PASSPHRASE }}", "-u", "{{ .Env.GPG_FINGERPRINT }}", "--armor", "--output", "${signature}", "--detach-sign", "${artifact}"]`; `release: draft: false`; `changelog` default.
- `xcl-plugin-template:.github/workflows/release.yml` (new) — `on: push: tags: ["v*"]`; `permissions: contents: write`; checkout with `fetch-depth: 0`; setup-go from `go.mod`; `make test`; `crazy-max/ghaction-import-gpg@v6` with `gpg_private_key: ${{ secrets.GPG_PRIVATE_KEY }}`, `passphrase: ${{ secrets.GPG_PASSPHRASE }}` (id `gpg`; fails the job when the secret is empty, so nothing unsigned is published); `goreleaser/goreleaser-action@v6` with a pinned `version`, `args: release --clean`, env `GITHUB_TOKEN`, `GPG_FINGERPRINT: ${{ steps.gpg.outputs.fingerprint }}`, `GPG_PASSPHRASE`.
- `xcl-plugin-template:.github/workflows/ci.yml` — add a `snapshot` job: `goreleaser/goreleaser-action@v6` with `args: release --snapshot --clean --skip=sign,publish`, then a step listing `dist/` so the six archive names and the checksums file are visible in the log.
- `xcl-plugin-template:Makefile` — `release` target: requires `VERSION` (fails with usage when unset or not `v<semver>[-pre]`), runs `git tag $(VERSION) && git push origin $(VERSION)`; `snapshot` target running GoReleaser snapshot locally via `go run github.com/goreleaser/goreleaser/v2@<pinned>`.
- `xcl-plugin-template:README.md` — sections: Signing key (generate RSA 4096 or Ed25519 with `gpg --quick-generate-key`, export armoured private key into secret `GPG_PRIVATE_KEY`, passphrase into `GPG_PASSPHRASE`, publish the public key: commit nothing, upload to the GitHub account's GPG keys so it is served at `https://github.com/<user>.gpg`, and include it in the release notes/README for applications); Release (`make release VERSION=v0.1.0`; what the release contains, per the asset contract); Install (application side: `registry.NewGitHub(registry.GitHubTrustedKeys(publicKey))`, `RegisterPlugin("<owner>/<repo>", "v0.1.0")`, `xcl.WithRegistry`, linking xcl's GitHub registry guide).

**Complexity**: Medium
**Token estimate**: ~30k tokens
**Agent strategy**: Single agent, sequential; validate with `goreleaser check` and a local snapshot, then inspect `dist/` names against the contract.

### Task: Add the release signing key to the template

What the person does: generate a dedicated signing key (`gpg --quick-generate-key "xcl plugin template releases <...>" ed25519 sign never`, or RSA 4096), add repository secrets `GPG_PRIVATE_KEY` (`gpg --armor --export-secret-keys <fpr>`) and `GPG_PASSPHRASE` on `jumppad-labs/xcl-plugin-template`, and publish the armoured public key where the README says (the organisation/maintainer GitHub GPG keys and the README's "Verifying releases" block).

Checked by: `gh secret list -R jumppad-labs/xcl-plugin-template` shows both names; the public key is fetchable at the documented location.

### Task: Publish an xcl release with the current plugin contract

What the person does: once the epic's specs are merged to `main` in `jumppad-labs/xcl`, tag and publish a release newer than `v0.1.0` (e.g. `v0.2.0`) with release notes drawn from `CHANGELOG.md`. Only `v0.1.0` (and a stray `0.1.0`) exist today, and they predate `entity.PropertyChange`.

Checked by: `go list -m github.com/jumppad-labs/xcl@latest` resolves to the new tag.

### Task: Pin the template to the xcl release

**File changes**:

- `xcl-plugin-template:go.mod`, `xcl-plugin-template:go.sum` — `go get github.com/jumppad-labs/xcl@<release>` and `go mod tidy`.

Verify `make lint test` and the CI snapshot pass.

**Complexity**: Low
**Token estimate**: ~5k tokens
**Agent strategy**: Single agent, sequential execution.

### Task: Publish the template's first signed release

What the person does: on the template's `main` with CI green, run `make release VERSION=v0.1.0` (pushes the tag) and wait for the release workflow.

Checked by: the GitHub release `v0.1.0` exists and is not a draft; the manual checks in the test plan cover its contents and signature.

### Task: Document the standard plugin layout in the repository

**File changes**:

- `docs/plugin-layout.md` (new) — prose guide from `plugin-layout.md` (design): directory tree, what each part holds (entities, providers, client, plugin.go, cmd, examples, e2e), file order, deciding and updating (`Changed` explicit with `change.Within`, `Update` from `changes`/`dependencies` and `Before`, `Read` reports the real resource), tests, what the layout does not cover; "Start from the template" section linking `https://github.com/jumppad-labs/xcl-plugin-template` with the `gh repo create --template` command; release section pointing to the asset contract summary and the GitHub registry docs. Names only locations the design names.
- `docs/README.md:8-31` — add a "Plugin Layout" bullet to "Start here".
- `docs/plugin-developer-guide.md:1-12` — one sentence in the intro linking the layout guide and the template; `docs/plugin-developer-guide.md:856` "The example provider" — a sentence pointing to the template as the layout's worked example (keep the sibling docker-layout plan's edits intact when rebasing).
- `README.md:95-140` ("With plugins") or `README.md:225` ("Plugins" example) — a short "Writing a plugin" paragraph linking the template and the layout guide.
- `CHANGELOG.md:3` — new top entry `## 20261009092551-82db0140-plugin-template`: prose (standard layout written down in `docs/plugin-layout.md`; new template repository `jumppad-labs/xcl-plugin-template` with one resource, in-process and external, strict-double tests, e2e, CI and signed GoReleaser releases in the release asset contract), then `**Breaking:**` with `- None.`

No test reads these files (knowledge `conventions/testing-and-mocking.md`).

**Complexity**: Low
**Token estimate**: ~25k tokens
**Agent strategy**: Single agent, sequential; re-read the design and the template tree before writing.

### Task: Add the plugin template page to the documentation site

**File changes**:

- `xcl-website:src/pages/plugin-template.mdx` (new) — frontmatter `layout: ../layouts/Shell.astro`, `title: "Plugin template - xcl"`, `description`; `<Hero>` and `<Prose>` as `xcl-website:src/pages/registries.mdx:1-30`; sections: the standard layout (tree and one line per part), the change decision and told-only update in brief, start from the template (`Use this template` / `gh repo create --template`), build and test, run in-process and as a separate program, release (tag, signing key secret, what the release holds), install (link to `/github-registry/` from the registry plan); closing `<CtaBanner>` linking to the template repository.
- `xcl-website:src/config/site.ts` — add `templateURL = "https://github.com/jumppad-labs/xcl-plugin-template"`.
- `xcl-website:src/components/Nav.astro:17-29` — add `{ label: "Plugin template", href: "/plugin-template/" }` to Guides.
- `xcl-website:src/pages/registries.mdx:25` — a sentence linking the page for authors writing a plugin.
- `xcl-website:src/pages/examples/plugins.mdx` — a sentence linking the page as the place to start a plugin of your own.

Verify with `npm run build` and `npx astro check`.

**Complexity**: Low
**Token estimate**: ~20k tokens
**Agent strategy**: Single agent, sequential execution.

## Testing Strategy

- **Scaffold the template module and its files client**: files client tests against `t.TempDir()`, one behaviour per function, negative cases (missing file, cancelled context) separate. `make generate` yields no diff.
- **Add the note entity and its provider**: provider unit tests with a fresh `mocks.NewMockFiles(t)` per test, covering every `Changed` rule (each replace setting, replaced dependency, updated dependency alone, content and mode updates, no change) and every `Update` branch with only the expected calls. Unexpected calls fail the test. Errors are tested in separate functions.
- **Add the plugin type, its program and the sample configuration**: `Init` registers `notes "note"` without touching the disk. The program builds.
- **Add the end-to-end and state-reader tests**: in-process and external lifecycle (apply, `Diff` with no changes, destroy) in separate functions; the plan shows replace for a `name` change and update for a `content` change; the state reader in its own package queries `entities.Note`. All state comes from real applies.
- **Add the template's continuous integration / Add signed release automation**: no tests read workflow or GoReleaser files. Correctness is shown by CI runs, the snapshot `dist/` listing and the manual release checks.
- **Pin the template to the xcl release**: the full suite and lint pass against the release.
- **Docs tasks**: no tests (knowledge `conventions/testing-and-mocking.md`). The site build and `astro check` must pass, and a person reviews the content.
- Manual items (time to first plugin, first resource without guessing, the first green scheduled run, layout agreement, tagged signed release verified with gpg, install with the GitHub registry, README commands on a fresh copy, CI failure reporting, file-order review, repo registration, the site page in a browser) go to the implementation test plan.

## Project References

- Spec `20261009092551-82db0140-plugin-template`.
- Design `plugin-layout.md` (source `design`): the layout.
- Design `plugin-release-assets.md` (source `design`): the release asset contract.
- Plan `20261009102148-7d0b205b-github-releases-registry`: the installing side (`registry.NewGitHub`, `GitHubTrustedKeys`, site page `/github-registry/`).
- Plan `20261009102138-48e95432-docker-example-standard-layout`: the sibling example rebuild.
- Knowledge entries: `conventions/testing-and-mocking.md`, `conventions/code-style.md`, `conventions/dependencies.md`, `conventions/patterns-and-architecture.md`, `conventions/project-structure.md`, `conventions/development-standards.md`, `conventions/test-state-from-real-apply.md`, `conventions/never-modify-dependencies.md`, `gotchas/plugin-types-rebuilt-with-structof.md`, `gotchas/registered-types-convert-only-to-themselves.md`.
- Repo roots: xcl `/home/nicj/code/github.com/jumppad-labs/xcl`; xcl-website `/home/nicj/code/github.com/jumppad-labs/xcl-website`; xcl-plugin-template `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template` (not a clone until the first task is done).

## Token Management Strategy

| Tier | Token Budget | Agent Strategy |
|------|-------------|----------------|
| Low | ~10k | Single agent, sequential |
| Medium | ~25k | 2-3 parallel agents |
| High | ~50k+ | Parallel analysis, sequential integration |

All agent tasks are Low or Medium and run as one agent each, in dependency order. The xcl docs task and the template CI/release tasks can run in parallel once the e2e task is done.

## Migration Notes

No migration is needed: nothing in xcl changes. When the xcl release is published, the template moves from a pseudo-version to that release (its own task). The sibling docker-layout plan edits the same README and `docs/` guides, so whichever lands second rebases its doc edits.

## Performance Considerations

None of note. The e2e suite builds the plugin binary once per test run in `TestMain`, and the state-reader package builds its own copy once.
