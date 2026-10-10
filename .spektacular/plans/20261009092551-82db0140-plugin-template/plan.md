---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Plan: 20261009092551-82db0140-plugin-template

<!-- Metadata -->
<!-- Created: 2026-10-09T15:12:05+01:00 -->
<!-- Commit: a000ffa81afd341da8e391aef4f58e076138789c -->
<!-- Branch: f-plugin-registries -->
<!-- Repository: github.com/jumppad-labs/xcl -->

## Overview

This plan publishes the standard xcl plugin layout as a GitHub template repository, `jumppad-labs/xcl-plugin-template`. A plugin made from it has one file-backed resource. Unchanged, it builds, tests and applies, runs both registered inside an application and as a separate plugin program, and publishes GPG-signed releases for every platform in the layout the GitHub Releases registry installs. It gives plugin authors a working, idiomatic starting point instead of hand-copied examples. The project gets one documented layout, written up in its guides and on the documentation site, that the template, the examples and later jumppad ports all follow.

## Conventions

- **Testing & Mocking (testify `require`, Mockery, no table-driven tests, positive and negative cases in separate functions, tests beside their source, no tests that read repository files)** — the spec makes these the template's own test rules; every provider and client behaviour gets its own test function beside its source, and no test asserts on the README, docs, CHANGELOG, CI workflows or GoReleaser config (those are human-reviewed), nor greps imports to prove the entity types stand alone.
- **Code style (gofmt, go vet, descriptive names, `any`, semantic import grouping)** — applies to all template Go code, and `gofmt`/`go vet` run in the template's CI.
- **Dependencies: prefer the standard library, document third-party reasons, pin versions** — the files backend uses only `os`, `io/fs`, `crypto/sha256`; the template's only third-party modules are xcl and testify, with Mockery, staticcheck and GoReleaser pinned to exact versions and run through `go run` or a pinned action, each reason given in the README.
- **Patterns & architecture: dependency injection, `context.Context`** — the provider takes its `Files` client through its constructor so tests inject the strict double; provider methods honour the `ctx` they are given.
- **Project structure: `/cmd` for main applications** — matches the layout design's `cmd/<plugin>/main.go`; the design's tree governs everything else in the template.
- **Development standards: structured logs** — the provider logs through `plugins.Logger(ctx)` with key/value pairs (`path`, `checksum`), as the existing examples do.
- **Generate test state with a real apply** — the e2e "next plan has no changes", destroy and state-reader tests get their state from a real first apply, never a hand-written state file.
- **Never modify dependency packages** — the template uses xcl as a published module, never a patched copy or a `replace` to a local checkout in what it commits.
- Deliberately dropped: database/external services (no database or pooled service), shared test helpers in `internal/testutil` (an xcl-repo rule; the template's test helpers stay in its own `_test.go` files), shared `errors` package (no new shared error types in xcl), assert ordering on graph parents (the template has one resource and asserts no ordering).

## Architecture & Design Decisions

The plugin template is a new GitHub template repository, `jumppad-labs/xcl-plugin-template` (Go module `github.com/jumppad-labs/xcl-plugin-template`), built exactly to the `plugin-layout.md` design and publishing exactly the `plugin-release-assets.md` contract. It is created on GitHub, marked as a template and registered with the project as repo `xcl-plugin-template` in the first task, so every later task is attributed to it, as the spec's Technical Approach asks. The rest of the work lands in `xcl` (a written layout guide in `docs/`, links from the README and the existing guides, the changelog entry) and `xcl-website` (one new guide page and its nav entry). The plugin contract is used as it is today: nothing in `xcl`'s Go code changes.

**One sample resource that runs with nothing else.** The template is a plugin for one domain, `notes`, with one resource, `notes "note" "<name>"`, that writes a note to a file. Following the design tree: `plugin.go` in the root package `notes` holds `notes.Plugin`, whose `Init` builds the files client once (without touching the disk) and registers the type with `providers.NewNoteProvider(files)`; `cmd/notes/main.go` serves the same type with `plugins.Serve`; `entities/note.go` holds the block type only (configured `directory`, `name`, `content`, `mode`; computed `path` and `checksum`) and imports nothing from the plugin; `providers/note.go` holds the provider; `client/files/` holds the narrow `Files` interface (write, change mode, remove, stat), its local-disk implementation and its Mockery double in `client/files/mocks/`, and is the only package that touches the filesystem. A plain filesystem backend is the "simple local implementation" the Technical Approach names, so the template builds, tests and applies with no service running, while still showing a backend behind an interface with a strict double. Every source file keeps exported types, vars and the constructor first, provider methods in the design's lifecycle order (`Init`, `Create`, `Read`, `Changed`, `Update`, `Destroy`, `Functions`), and unexported helpers last.

**The explicit decision and the told-only update.** `Changed` is explicit, as the design and the spec's Technical Approach fix it: one exported list, `ReplaceSettings`, names the settings that cannot change in place (`directory` and `name`, which move the file); `Changed` answers replace for any change `Within` one of them or for any dependency reported as replaced (commented as the place an author narrows this to the types their resource is built on), update when anything else changed, and defers to `DefaultChanged` otherwise. `Update` works only from what it is told: a `content` change rewrites the file, a `mode`-only change only changes the mode, and it never stats or reads the file to rediscover the old state; the new checksum is kept as a computed value so the next decision needs no lookup. `Read` reports the real file (`path`, `checksum`) and returns `ErrNotFound` when it is gone. Unit tests build the provider through its constructor with a strict Mockery double, so a stat made by `Update` fails the test, which is how the acceptance criterion "a call made to rediscover previous state fails the test" is met.

**Both ways of running, without a host program.** The design names no place for a host application, and the "One shape everywhere" metric forbids a part in an unnamed place, so the in-process and external registrations live in `e2e/`: one test registers `&notes.Plugin{}` with `registry.NewLocal().RegisterPlugin`, another builds `cmd/notes` and registers the binary with `RegisterExternalPlugin`; each applies `examples/basic/main.xcl`, requires the next `Diff` to report no changes, and destroys, in its own test function (not the parity comparison the spec excludes). A separate package, `e2e/stateonly`, imports only xcl, the registry and `entities`, applies through the built binary, and reads the notes back into `entities.Note` with `xcl.FindByType`, which is the behavioural proof that the entity types stand alone (the knowledge base forbids proving it by grepping imports). The README quotes the registration code from these tests and gives one make target each for building, testing, regenerating the Mockery double (pinned and run without installing, as the plugin example does), the in-process run, the external run and tagging a release.

**Checks and signed releases.** GitHub Actions, as the Technical Approach fixes: a CI workflow on every push and pull request builds, runs `go vet`, a `gofmt` check and pinned `staticcheck`, runs the whole suite including `e2e`, and runs a GoReleaser snapshot build so every platform build and the release config are checked on each change; a scheduled workflow re-runs the suite against the latest xcl release (the "stays current" metric). A release workflow on `v*` tags imports the GPG key held as repository secrets and runs GoReleaser, whose defaults give the contract's archive and checksum names (`project_name` is left to default to the repository name, which is the plugin name the contract fixes, so a repository made from the template names its own assets), with `format_overrides` for Windows `.zip`, linux/darwin/windows × amd64/arm64, and a `signs` step writing an ASCII-armoured detached `<checksums>.sig`. A missing key fails the workflow, so an unsigned release is never published. Rejected alternatives — a generator, two templates, a template inside the xcl repo, a host program, hand-rolled release scripts, import-grepping checks — are recorded with evidence in `research.md#alternatives-considered-and-rejected`.

## Component Breakdown

- **Template repository (new repo, `xcl-plugin-template`)** — a GitHub repository marked as a template and registered with the project. Owns everything an author gets from "Use this template": the plugin module, its tests, tooling, CI and release automation. Its name is the plugin's name, which the release automation uses for every asset.
- **Plugin type (new, template root package)** — `notes.Plugin`. Owns building the files client once, without touching the disk, and registering the `notes "note"` type with a provider constructed around that client. Imported by an application registering the plugin in-process, and served unchanged by the plugin program.
- **Plugin program (new, template)** — the separate-program entry point. Owns nothing but serving the plugin type over xcl's existing plugin protocol, so in-process and external use run identical code.
- **Note entity (new, template)** — the `notes "note"` block type: configured directory, name, content and mode, plus the computed path and checksum. Owns the shape of a note only; it depends on nothing in the plugin, so an application reading notes from state needs only this package and xcl.
- **Note provider (new, template)** — owns the note's lifecycle: create writes the file, read reports the real file (or that it is gone), the explicit change decision (one list of replace-only settings, replace for a replaced dependency, update for any other change, otherwise the default), an update that acts only on the reported changes, and destroy. Depends only on the files client interface and the entity, built through a constructor so tests inject a strict double.
- **Files client (new, template)** — the plugin's only backend layer: a narrow interface over writing, changing the mode of, removing and inspecting a file, its local-disk implementation and the shapes it returns. The only place the filesystem is touched; never depends on the provider or the entity.
- **Files test double (new, generated, template)** — the Mockery-generated strict double of the files client interface, regenerated with one command. Used by the provider's unit tests so any call the reported changes do not require fails the test.
- **Sample configuration (new, template)** — one example configuration with a note whose directory is a variable, applied by the end-to-end tests and the README's commands.
- **End-to-end suite (new, template)** — builds the plugin program, then applies the sample configuration, confirms the next plan reports no changes and destroys it: once with the plugin registered in-process and once registered as a separate program, each in its own test. A separate state-reader test, depending only on xcl and the entity package, reads applied notes back from state into the entity type.
- **Build tooling (new, template)** — the make targets an author uses (build, test, generate, in-process and external runs, release), the Mockery configuration, and pinned versions of Mockery and the static-check tool run without installing them.
- **Continuous integration (new, template)** — GitHub Actions on every push and pull request: build, static checks, the full suite and a snapshot release build of every platform; plus a scheduled run against the latest xcl release.
- **Release automation (new, template)** — GoReleaser configuration and a tag-triggered GitHub Actions workflow that imports the signing key from repository secrets and publishes archives for linux, darwin and windows on amd64 and arm64, the checksums file and its armoured detached signature, exactly as the release asset contract names them; it fails rather than publish unsigned.
- **Template README (new, template)** — the author's guide: start from the template, make it yours, build, test, regenerate doubles, run in-process and as a separate program, add a resource of your own, publish the signing key, release, and install with the GitHub registry.
- **Plugin layout guide (new, `xcl` docs)** — the written standard layout: the tree, what each part holds, file order, the change decision and told-only update, the test shape, and where to start from the template. Linked from the docs index, the plugin developer guide and the README.
- **Changelog (changed, `xcl`)** — one entry for this spec recording the layout guide and the template.
- **Documentation site page (new, `xcl-website`)** — a guide page on the standard layout and starting, releasing and installing a plugin from the template, in the Guides navigation and linked from the registries and plugin example pages; it links to the GitHub registry guide for installing.
- **Plugin contract and local registry (existing, unchanged)** — `ResourceProvider`, `DefaultChanged`, `entity.PropertyChange`, `plugins.Serve`, `registry.NewLocal` and xcl's query functions are used as they are today.

## Data Structures & Interfaces

All new types live in the template repository. No type, interface or wire format in `xcl` changes: the plugin uses `plugins.ResourceProvider[T]`, `plugins.DefaultChanged[T]`, `entity.PropertyChange`, `entity.DependencyChange`, `plugins.ErrNotFound` and `plugins.Serve` as they are.

**Entity, `entities` package.** The block type `notes "note" "<name>"`. Plain fields only (no nested named structs, which the host's type rebuild drops); no methods, no imports beyond `types`:

```go
// Note is a note written to a file, the block type notes "note"
type Note struct {
    types.ResourceBase `xcl:",remain"`

    Directory string `xcl:"directory" json:"directory"`        // replace when changed
    Name      string `xcl:"name" json:"name"`                  // file name, replace when changed
    Content   string `xcl:"content" json:"content"`            // update in place
    Mode      string `xcl:"mode,optional" json:"mode,omitempty"` // octal, default "0644", update in place

    Path     string `xcl:"path,optional,computed" json:"path,omitempty"`         // directory joined with name
    Checksum string `xcl:"checksum,optional,computed" json:"checksum,omitempty"` // sha256 of what was written
}
```

**Backend client, `client/files` package.** The narrow interface the provider depends on, its local-disk implementation, and the client's own result type (it never uses entity types):

```go
// Files is the filesystem the note provider uses
type Files interface {
    Write(ctx context.Context, path string, content []byte, mode fs.FileMode) (Info, error)
    Chmod(ctx context.Context, path string, mode fs.FileMode) error
    Stat(ctx context.Context, path string) (Info, error) // wraps fs.ErrNotExist when missing
    Remove(ctx context.Context, path string) error       // a missing file is not an error
}

// Info is what the client reports about a file
type Info struct {
    Checksum string      // lowercase hex sha256 of the content
    Mode     fs.FileMode
}

type Local struct{}          // the real implementation, over os
func NewLocal() *Local       // touches nothing until called
```

`mocks.MockFiles` is generated from `Files` by Mockery (testify template, `mocks` package beside it) and is the strict double the provider tests use.

**Provider, `providers` package.**

```go
// ReplaceSettings are the note settings that cannot change in place
var ReplaceSettings = []entity.Path{
    entity.Path{}.Attribute("directory"),
    entity.Path{}.Attribute("name"),
}

type NoteProvider struct {
    plugins.DefaultChanged[*entities.Note]
    // files files.Files (unexported)
}

var _ plugins.ResourceProvider[*entities.Note] = (*NoteProvider)(nil)

func NewNoteProvider(files files.Files) *NoteProvider
// Init, Create, Read, Changed, Update, Destroy, Functions — in that order
```

The decision contract: `Changed` returns `entity.Replace` when any change is `Within` a path in `ReplaceSettings` or any dependency's change is `entity.Replace`; `entity.Update` when `changes` is non-empty; otherwise `DefaultChanged.Changed`. `Update` reads previous values only from each change's `Before`.

**Plugin type, root package `notes`, and program.**

```go
type Plugin struct{ plugins.PluginBase }
var _ plugins.Plugin = (*Plugin)(nil)
func (p *Plugin) Init(logger logger.Logger, state plugins.State) error // registers "notes","note"
```

`cmd/notes/main.go` calls `plugins.Serve(&notes.Plugin{})`.

**Serialization boundaries.** The entity crosses xcl's existing JSON/gRPC plugin boundary unchanged (JSON tags above, saved in xcl state); nothing new is added to the plugin protocol. The sample configuration is `.xcl` text with a `variable "directory"`. Release artifacts follow `plugin-release-assets.md` byte for byte: `<repo>_<version>_<os>_<arch>.tar.gz` (`.zip` on windows) holding `<repo>`/`<repo>.exe` at the root, `<repo>_<version>_checksums.txt` in `sha256sum` format, and `<repo>_<version>_checksums.txt.sig`, ASCII-armoured and detached. The release workflow reads two repository secrets, the armoured private key and its passphrase.

## Implementation Detail

**A new kind of deliverable: a repository authors copy.** Everything in the template is written to be read and copied, so it favours the obvious over the clever: one resource, one backend, one sample configuration, doc comments that say what an author changes and why, and no helpers that hide a lifecycle step. The template is the first place the standard layout exists in full, so its shape *is* the layout: a reader who opens it sees the design's tree and nothing else beyond repository tooling (Makefile, Mockery and GoReleaser config, GitHub workflows, licence).

**Patterns followed from the existing examples.** The plugin follows the docker example's proven shape — a plugin type whose `Init` builds clients once and registers each type with a constructor-built provider, a narrow client interface with a real implementation and a Mockery double, provider unit tests that inject the double through the constructor — and xcl's local registry for both registrations. Mockery is configured and run the way the plugin example runs it (pinned, through `go run`, so nothing is installed), and CI follows the xcl repository's workflow (checkout, set up Go, build, vet, test).

**Patterns the template introduces.**

- *Replace rules as data.* The settings that need a replace are one exported list of setting paths at the top of the provider file, matched with `change.Within`; `Changed` is a short, fixed sequence (replace-only settings, replaced dependencies, any change, default). An author changes the list, not the logic.
- *Update branches on what changed.* `Update` walks the reported changes and does the least work each one needs, taking previous values only from `Before`, and keeps what a later decision needs (the checksum) as a computed value, so no step ever looks the resource up to find out what it was.
- *A test for each rule, in its own function.* The provider tests are a readable list — create writes, read fills computed values, read of a missing file reports not found, each replace setting replaces, a replaced dependency replaces, an updated dependency alone does not replace, a content change updates, no change defers, a content update writes only, a mode update changes only the mode, destroy removes — each against a fresh strict double, positive and negative cases apart.
- *Registration lives in tests.* With no host program, the e2e tests are the canonical examples of registering the plugin in-process and as a separate program, and the README quotes them; the state-reader test sits in its own package so its build graph proves the entity package stands alone.
- *Release by tag only.* Releasing is pushing a tag; the workflow, the signing key in secrets and GoReleaser do the rest. The same GoReleaser config runs unsigned as a snapshot in CI so a broken release config fails on the change that broke it, not on release day.

**Documentation shape.** The template README is task-ordered (start, make it yours, build, test, generate, run both ways, add a resource, release, install), one command per task. The `xcl` layout guide is the prose form of the layout design, pointing to the template as its worked example; the plugin developer guide, docs index and README point to it rather than repeating it. The site page summarises the same layout and the author's path from "Use this template" to an installed plugin, and defers to the GitHub registry guide for the application side, so no page restates another's detail.

**Keeping it current.** The template is a registered project repo, so later plans that change the plugin contract attribute work to it; a scheduled CI job runs the suite against the latest xcl release so drift shows up without anyone remembering to look.

## Dependencies

- **Design `plugin-layout.md` from the `design` design source** — the settled plugin layout this plan builds the template and the layout guide to: the tree, what each part holds, file order, the explicit `Changed`, the told-only `Update`, and the test shape. No change needed.
- **Design `plugin-release-assets.md` from the `design` design source** — the settled release layout the template's GoReleaser configuration publishes: tag form, archive, checksums and signature names and formats, binary at the archive root, platforms. No change needed.
- **Plan `20261009102148-7d0b205b-github-releases-registry` (final; must land before the template's install docs are finished)** — provides `registry.NewGitHub`, `RegisterPlugin("owner/repo", "vX.Y.Z")` and `GitHubTrustedKeys`, which the template README and the site page show for installing a released plugin, and the site's GitHub registry guide this plan's page links to. The template's code and tests do not depend on it.
- **Sibling plan `20261009102138-48e95432-docker-example-standard-layout` (independent)** — rebuilds the docker example to the same layout and leaves the written layout guide to this plan; nothing needs to land first, but both edit the README and `docs/` guides, so whichever lands second rebases its doc edits.
- **xcl module (`github.com/jumppad-labs/xcl`, existing, unchanged)** — the plugin contract (`plugins`, `entity`, `types`, `logger`), `registry.NewLocal`, `Config` operations and query functions. The template must require a version that contains the current contract (`entity.PropertyChange`, the `changes` parameter): only `v0.1.0` is tagged and it predates that, so the template pins a pseudo-version of a pushed xcl commit until **a person cuts an xcl release** with the epic's changes, then pins that release.
- **GitHub (external service)** — hosts the template repository (marked as a template), runs GitHub Actions, holds the signing key as repository secrets and serves releases. Creating the repository under `jumppad-labs` and adding the secrets are tasks for a person.
- **GoReleaser v2 (external tool, pinned)** — builds, archives, checksums and signs releases; run by `goreleaser/goreleaser-action` in the release workflow and as a snapshot in CI.
- **`crazy-max/ghaction-import-gpg` (GitHub Action, pinned)** — imports the GPG key from secrets for GoReleaser's `signs` step.
- **GnuPG key (external, made by a person)** — an RSA 4096 or Ed25519 signing key, both of which the registry's OpenPGP library verifies; its public key is published for applications to trust.
- **Mockery v3 (`github.com/vektra/mockery/v3`, pinned, run with `go run`)** — generates the files client double, as in the plugin example.
- **staticcheck (`honnef.co/go/tools`, pinned, run with `go run`)** — static checks in CI.
- **testify (`github.com/stretchr/testify`, pinned)** — `require` and the mock runtime the generated double uses.
- **Go standard library** — `os`, `io/fs`, `path/filepath`, `crypto/sha256` for the files backend; `os/exec` to build the plugin program in the e2e suite.
- **`xcl-website` repo (existing)** — gains one page and a nav entry; its build and `astro check` must still pass.
- **`xcl` docs (existing)** — `docs/README.md`, `docs/plugin-developer-guide.md`, `README.md`, `CHANGELOG.md` gain links and an entry; a new `docs/plugin-layout.md`.

## Testing Approach

**Kinds of tests (all in the template repository).**

- **Provider unit tests** — the most coverage, because the change decision and the told-only update are what authors copy. Each builds the note provider through its constructor with a fresh strict Mockery double of the files client and covers one rule: create writes and fills `path` and `checksum`; read fills computed values from the real file; read of a missing file returns `ErrNotFound`; a change within each replace-only setting replaces; a replaced dependency replaces; an updated dependency with no setting changes does not replace; a `content` or `mode` change updates; no change defers to the default; a content update makes only the write call; a mode-only update makes only the mode call; destroy removes. Positive and negative cases are separate functions.
- **Files client tests** — the local implementation against a temporary directory: write reports the right checksum and mode, chmod changes the mode, stat of a missing file wraps `fs.ErrNotExist`, remove of a missing file succeeds.
- **Plugin type test** — `Init` registers `notes "note"` without touching the filesystem.
- **End-to-end tests** — build the plugin program once per run; then, in separate test functions, register the plugin in-process and as a separate program, apply the sample configuration into a temporary directory, require the next `Diff` to report no changes, require the note file to exist with the configured content, and destroy, requiring the file gone. Two more e2e functions edit the applied configuration (via variables or a second sample in the test's temp dir) and require the plan to show the note replaced for a `name` change and updated for a `content` change, satisfying the "editing a setting that needs a replace replaces" criterion end to end. State always comes from a real first apply.
- **State-reader test** — in its own package depending only on xcl, the registry and `entities`: applies through the built plugin program, then reads the notes back from state into `entities.Note` with xcl's typed query and checks their values.
- **CI** runs build, `go vet`, the `gofmt` check, staticcheck, the whole suite (unit and e2e, no external service needed) and a GoReleaser snapshot build of all six platforms, on every push and pull request.

All tests follow the project rules the spec adopts: testify `require`, Mockery doubles, no table-driven tests, one behaviour per function with positive and negative cases apart, tests beside the code they test. No test reads the README, docs, CHANGELOG, workflow or GoReleaser files; those are reviewed by a person.

**Load-bearing assertions.** A fresh template builds, passes, applies, plans no changes and destroys with nothing else running, both in-process and as a separate program; the replace-only settings replace and every other setting updates, in unit tests and in a real plan; `Update` makes exactly the calls its reported changes need, and any lookup of previous state fails the test; an application with only `entities` reads applied notes from state; the release config builds every supported platform on every change.

**Success metrics.**

- **Time to a first plugin (under 15 minutes from the README alone)** — **Manual — captured in the implementation test plan** (a person new to the template times themselves from "Use this template" to a passing, applying plugin of their own).
- **First real resource without guessing** — **Manual — captured in the implementation test plan** (a person adds a second resource following only the README and the layout guide, without opening the xcl examples or source).
- **The template stays current** — behavioural: the scheduled CI workflow runs the full suite against the latest xcl release; its first green run against a real xcl release is **Manual — captured in the implementation test plan**.
- **One shape everywhere** — **Manual — captured in the implementation test plan** (a reviewer checks the template tree, the layout guide and the site page against `plugin-layout.md`: no part in a place the design does not name, and every location the guide names exists in the template). Not automated, because the knowledge base forbids tests that walk or read the repository's own files.

**Manual reviews.**

- **Manual — captured in the implementation test plan**: push a version tag to a repository created from the template with its signing key configured; confirm the release has an archive for each of the six platforms, the checksums file and the `.sig`, named per `plugin-release-assets.md`, with no manual steps, and that the signature verifies against the published public key.
- **Manual — captured in the implementation test plan**: install that release with the GitHub registry (trusted key supplied) and apply the sample, closing the registry plan's real-GitHub check.
- **Manual — captured in the implementation test plan**: create a new repository from the template (through the web or the GitHub CLI), and run every README command (build, test, generate, in-process run, external run, release) unchanged; each succeeds.
- **Manual — captured in the implementation test plan**: push a change with a deliberate failure (vet, format, test) to a branch and confirm CI reports it on the pull request.
- **Manual — captured in the implementation test plan**: read every template source file and confirm exported members come first and provider methods follow the lifecycle order.
- **Manual — captured in the implementation test plan**: confirm the template repository appears in the project's registered repositories, with a description and a root on disk.
- **Manual — captured in the implementation test plan**: build the documentation site, check the new page and its nav entry in a browser, and review the layout guide, README links and changelog entry for accuracy.

**Deliberate gaps.** No in-process vs external parity test (spec non-goal). No automated check of file order or layout conformance (review only, per the knowledge base). Release signing is not exercised in CI (the snapshot build skips signing; the tagged release is the manual check). Cross-platform binaries are built in CI but only run on linux.

## Milestones & Tasks

### Milestone 1: A new plugin builds, tests and applies straight from the template

**What changes**: Plugin authors can start a plugin from a GitHub template repository instead of copying examples. A repository created from it, with nothing changed and nothing else running, builds, passes its unit and end-to-end tests, and applies, re-plans with no changes and destroys its sample note, both registered inside an application and run as a separate plugin program, each with one command. The template follows the standard layout, decides between update and replace explicitly, updates only from what it is told, and its entity types can be used on their own. The template is a registered project repository, so later contract changes are planned into it.

**Validation point**: On a fresh clone of the template, with no external service running, the build, test, regenerate-double (no diff), in-process and external commands all succeed, and the repository appears in the project's registered repositories with its root on disk.

#### - [ ] Task: Scaffold the template module and its files client
**Id:** 007d02bd-a1f8-4511-b83e-fa7377519094
**Repo:** xcl-plugin-template
**Depends on:** none
**Execution:** agent

Sets up the Go module and the plugin's only backend layer: a narrow files client interface with its local-disk implementation and its tests, the Mockery configuration and generated strict double, and the Makefile's build, test and generate targets. This gives the provider something to depend on and its tests a double to use, all without any external service.

*Technical detail:* [context.md#task-scaffold-the-template-module-and-its-files-client](./context.md#task-scaffold-the-template-module-and-its-files-client)

**Acceptance criteria**:
- [ ] The module builds and the files client's tests pass against a temporary directory.
- [ ] Regenerating the double with one command produces no changes.
- [ ] The files client package is the only place the filesystem is touched and depends on nothing else in the plugin.

#### - [ ] Task: Add the note entity and its provider
**Id:** e6bd58de-df37-49ce-88e6-b91644ce6b07
**Repo:** xcl-plugin-template
**Depends on:**
- 007d02bd-a1f8-4511-b83e-fa7377519094 — Scaffold the template module and its files client
**Execution:** agent

Adds the `notes "note"` block type and the provider that writes, reads, updates and removes the note's file. The provider's change decision is explicit, with the settings that need a replace listed in one place, and its update acts only on the changes it is given. Unit tests against the strict double cover every rule, so any extra call fails a test.

*Technical detail:* [context.md#task-add-the-note-entity-and-its-provider](./context.md#task-add-the-note-entity-and-its-provider)

**Acceptance criteria**:
- [ ] Changing the directory or name of a note is decided as a replace, and changing its content or mode as an update, each shown by its own unit test.
- [ ] A replaced dependency is decided as a replace, and an updated dependency with no setting changes is not.
- [ ] The update makes only the calls its reported changes require; a test fails if it looks the file up.
- [ ] The entity package depends on nothing in the plugin and on no backend library.
- [ ] Every file lists its exported members first, and the provider's methods follow the layout's lifecycle order.

#### - [ ] Task: Add the plugin type, its program and the sample configuration
**Id:** 0a5785bc-fa98-4bac-96f0-e247a497fc03
**Repo:** xcl-plugin-template
**Depends on:**
- e6bd58de-df37-49ce-88e6-b91644ce6b07 — Add the note entity and its provider
**Execution:** agent

Adds the plugin type an application registers in-process, the program that serves the same type as a separate plugin, and the sample configuration with one note whose directory is a variable. Together they make the plugin usable both ways from one module.

*Technical detail:* [context.md#task-add-the-plugin-type-its-program-and-the-sample-configuration](./context.md#task-add-the-plugin-type-its-program-and-the-sample-configuration)

**Acceptance criteria**:
- [ ] The plugin type registers the note type without touching the filesystem, shown by a unit test.
- [ ] The plugin program builds to a binary named after the plugin.
- [ ] The sample configuration declares one note and needs nothing but a directory to write to.

#### - [ ] Task: Add the end-to-end and state-reader tests
**Id:** 27d251cd-adc6-43f9-962d-42a2f6d64b4a
**Repo:** xcl-plugin-template
**Depends on:**
- 0a5785bc-fa98-4bac-96f0-e247a497fc03 — Add the plugin type, its program and the sample configuration
**Execution:** agent

Adds end-to-end tests that apply the sample configuration, confirm the next plan reports no changes and destroy it, once with the plugin registered in-process and once run as a separate program, plus tests that a plan shows a replace for a name change and an update for a content change. A separate state-reader test reads applied notes from state using only the entity types. Make targets run the in-process and external tests with one command each.

*Technical detail:* [context.md#task-add-the-end-to-end-and-state-reader-tests](./context.md#task-add-the-end-to-end-and-state-reader-tests)

**Acceptance criteria**:
- [ ] The sample applies, re-plans with no changes and destroys cleanly when registered in-process, and again when run as a separate program, with no external service.
- [ ] A plan after changing the note's name shows it replaced, and after changing its content shows it updated.
- [ ] A test that depends only on xcl and the entity types reads the applied notes from state.
- [ ] The in-process run and the external run each have one make command.

#### - [ ] Task: Write the template README
**Id:** ac83d914-869d-44a3-ac37-55a42374d985
**Repo:** xcl-plugin-template
**Depends on:**
- 27d251cd-adc6-43f9-962d-42a2f6d64b4a — Add the end-to-end and state-reader tests
**Execution:** agent

Writes the README an author follows from "Use this template" to a plugin of their own: making it theirs, building, testing, regenerating the double, registering and running it in-process and as a separate program, and adding a resource of their own, one command per task, with the layout explained in place. Releasing and installing are added with the release automation.

*Technical detail:* [context.md#task-write-the-template-readme](./context.md#task-write-the-template-readme)

**Acceptance criteria**:
- [ ] The README shows one command each for build, test, regenerating the double, the in-process run and the external run, and each command works on a fresh copy.
- [ ] The README shows the in-process and separate-program registration code exactly as the end-to-end tests use it.
- [ ] The README walks an author through renaming the plugin and adding a second resource without needing the xcl examples or source.

### Milestone 2: The template checks every change and publishes signed releases

**What changes**: Every push and pull request to the template (and to every repository made from it) is built, statically checked, tested and given a snapshot release build, with failures reported on the change; a weekly run checks it against the latest xcl release. Pushing a version tag publishes a GitHub release with the plugin built for linux, darwin and windows on amd64 and arm64, a checksums file and a GPG signature, named exactly as the GitHub Releases registry installs them, with no manual steps, and never an unsigned one. The README explains releasing, publishing the public key and installing the plugin with the GitHub registry.

**Validation point**: CI is green on the template's main branch; a tagged release of the template carries the six archives, the checksums file and an armoured signature that verifies against the published public key, and installs through the GitHub registry.

#### - [ ] Task: Add the template's continuous integration
**Id:** 4fe70fb5-cd5b-48c8-b96b-7670d77f3520
**Repo:** xcl-plugin-template
**Depends on:**
- 27d251cd-adc6-43f9-962d-42a2f6d64b4a — Add the end-to-end and state-reader tests
**Execution:** agent

Adds GitHub Actions workflows that build, run static checks and run the whole test suite on every push and pull request, and a weekly workflow that runs the suite against the latest xcl release, so the template, and every plugin made from it, checks itself and drift from xcl shows up on its own.

*Technical detail:* [context.md#task-add-the-templates-continuous-integration](./context.md#task-add-the-templates-continuous-integration)

**Acceptance criteria**:
- [ ] Every push and pull request runs a build, `go vet`, a formatting check, staticcheck and the full test suite, and a failure in any of them fails the check on the change.
- [ ] A scheduled workflow runs the suite against the latest xcl release and can also be started by hand.

#### - [ ] Task: Add signed release automation
**Id:** 7eddb09e-52fe-482e-b7d8-912ce6629d98
**Repo:** xcl-plugin-template
**Depends on:**
- 4fe70fb5-cd5b-48c8-b96b-7670d77f3520 — Add the template's continuous integration
- ac83d914-869d-44a3-ac37-55a42374d985 — Write the template README
**Execution:** agent

Adds the GoReleaser configuration and a tag-triggered workflow that imports the signing key from repository secrets and publishes the release asset contract exactly: an archive per platform, the checksums file and its armoured signature. CI gains a snapshot build of the same configuration, a make command tags a release, and the README explains creating and publishing the signing key, releasing, and installing the plugin with the GitHub registry.

*Technical detail:* [context.md#task-add-signed-release-automation](./context.md#task-add-signed-release-automation)

**Acceptance criteria**:
- [ ] A snapshot build on every change produces archives for linux, darwin and windows on amd64 and arm64 and a checksums file, named as the release asset contract requires, with the binary at each archive's root.
- [ ] A pushed version tag runs a release that signs the checksums file with an armoured detached signature, and fails instead of publishing when the signing key is missing.
- [ ] The README shows one command to release and explains making the key, adding the secrets, publishing the public key and installing the plugin with the GitHub registry.

#### - [ ] Task: Add the release signing key to the template
**Id:** 3517eb9d-6f32-4f02-b513-fe33dcfdf929
**Repo:** xcl-plugin-template
**Depends on:**
- 7eddb09e-52fe-482e-b7d8-912ce6629d98 — Add signed release automation
**Execution:** human — needs a GPG signing key and access to the repository's secrets

Creates the GPG key that signs the template's releases, stores the armoured private key and passphrase as repository secrets, and publishes the public key where applications can fetch and trust it, as the README describes.

*Technical detail:* [context.md#task-add-the-release-signing-key-to-the-template](./context.md#task-add-the-release-signing-key-to-the-template)

**Acceptance criteria**:
- [ ] The template repository holds the signing key and passphrase as secrets under the names the release workflow reads.
- [ ] The public key is published at the location the README names.

#### - [ ] Task: Publish an xcl release with the current plugin contract
**Id:** d5751259-087b-4020-8aed-c524ccab2163
**Repo:** xcl
**Depends on:** none
**Execution:** human — tagging and publishing an xcl release is a release action outside the repository's code

Tags and publishes an xcl release that contains the plugin contract the template builds to (the changed settings given to `Changed` and `Update`), once the epic's work is on the main branch. The template can then require a real release rather than a commit, and its weekly check has a latest release to run against.

*Technical detail:* [context.md#task-publish-an-xcl-release-with-the-current-plugin-contract](./context.md#task-publish-an-xcl-release-with-the-current-plugin-contract)

**Acceptance criteria**:
- [ ] An xcl release newer than v0.1.0, containing the current plugin contract, is published and resolvable as a Go module version.

#### - [ ] Task: Pin the template to the xcl release
**Id:** b8211ebb-2eb0-492c-a1e3-6815b380c285
**Repo:** xcl-plugin-template
**Depends on:**
- d5751259-087b-4020-8aed-c524ccab2163 — Publish an xcl release with the current plugin contract
- 4fe70fb5-cd5b-48c8-b96b-7670d77f3520 — Add the template's continuous integration
**Execution:** agent

Moves the template's xcl requirement from a development commit to the published xcl release, so a repository made from the template builds against a real version and the weekly check compares like with like.

*Technical detail:* [context.md#task-pin-the-template-to-the-xcl-release](./context.md#task-pin-the-template-to-the-xcl-release)

**Acceptance criteria**:
- [ ] The template requires the published xcl release and all its checks pass against it.

#### - [ ] Task: Publish the template's first signed release
**Id:** ee6af539-3526-4182-8f27-1c95ac3f0254
**Repo:** xcl-plugin-template
**Depends on:**
- 3517eb9d-6f32-4f02-b513-fe33dcfdf929 — Add the release signing key to the template
- b8211ebb-2eb0-492c-a1e3-6815b380c285 — Pin the template to the xcl release
**Execution:** human — pushing a release tag publishes a release, an action outside the repository's code

Pushes the template's first version tag, so a real signed release in the contract's layout exists for authors to see and for the GitHub registry to be checked against.

*Technical detail:* [context.md#task-publish-the-templates-first-signed-release](./context.md#task-publish-the-templates-first-signed-release)

**Acceptance criteria**:
- [ ] A signed release of the template exists on GitHub for its first version tag.

### Milestone 3: The documentation explains the standard layout and starting from the template

**What changes**: The project's guides describe the standard plugin layout — the tree, what each part holds, file order, the change decision, told-only updates and the test shape — and point to the template as its worked example; the README and plugin developer guide link to it; the changelog records it. The documentation site gains a guide page on the layout and on starting, releasing and installing a plugin from the template, reachable from its Guides navigation.

**Validation point**: The documentation site builds and type-checks with the new page in its navigation, and a reviewer confirms the layout guide, site page and template agree with `plugin-layout.md`.

#### - [ ] Task: Document the standard plugin layout in the repository
**Id:** eb96f101-c793-47ff-901e-69d0f0a90aff
**Repo:** xcl
**Depends on:**
- 27d251cd-adc6-43f9-962d-42a2f6d64b4a — Add the end-to-end and state-reader tests
**Execution:** agent

Adds a plugin layout guide to the project's docs describing the standard layout, what each part holds, file order, the explicit change decision, told-only updates and the test shape, with the template as its worked example. Links it from the docs index, the plugin developer guide and the README, and adds the changelog entry for this spec.

*Technical detail:* [context.md#task-document-the-standard-plugin-layout-in-the-repository](./context.md#task-document-the-standard-plugin-layout-in-the-repository)

**Acceptance criteria**:
- [ ] The docs have a guide to the standard plugin layout naming every location the template has, and nothing the layout design does not name.
- [ ] The docs index, the plugin developer guide and the README link to the guide and to the template.
- [ ] The changelog has one entry for this spec describing the layout guide and the template.

#### - [ ] Task: Add the plugin template page to the documentation site
**Id:** 3b1ea449-9aaa-432e-8f5d-d966341b47f7
**Repo:** xcl-website
**Depends on:**
- eb96f101-c793-47ff-901e-69d0f0a90aff — Document the standard plugin layout in the repository
- 7eddb09e-52fe-482e-b7d8-912ce6629d98 — Add signed release automation
**Execution:** agent

Adds a documentation site page explaining the standard plugin layout and how to start a plugin from the template, build and test it, release it signed and install it with the GitHub registry. The page is added to the Guides navigation and linked from the registries and plugin example pages.

*Technical detail:* [context.md#task-add-the-plugin-template-page-to-the-documentation-site](./context.md#task-add-the-plugin-template-page-to-the-documentation-site)

**Acceptance criteria**:
- [ ] The site has a page covering the layout and starting, releasing and installing a plugin from the template.
- [ ] The page is reachable from the Guides navigation and from the registries and plugin example pages.
- [ ] The site builds and type-checks.

## Open Questions

- **Can a state reader that registers no plugin type for `notes "note"` decode applied notes into `entities.Note`?** This depends on how xcl's typed query treats a plugin entity when the plugin is loaded from its binary, which only running it shows. The planned state-reader test registers the binary and queries into `entities.Note`. If the conversion fails, or the test can only pass by importing the plugin's root package, STOP and ask the user. Do not weaken the "entity types stand alone" criterion.
- **Does the GPG signature that GoReleaser's `signs` step produces verify with the GitHub registry's OpenPGP library?** This depends on the key the person creates and on the first real release. If the registry install in the manual checks rejects a genuine signed release, STOP and ask the user. Do not loosen signing or verification.

There are no other uncertainties that have to wait for implementation.

## Out of Scope

- **An in-process vs external parity test in the template** — a spec non-goal. The template runs each mode in its own e2e test and never compares the results. xcl's own e2e suite already covers parity.
- **A hosted plugin registry or marketplace** — a spec non-goal. Installing from GitHub releases belongs to the GitHub Releases registry plan (`20261009102148-7d0b205b-github-releases-registry`).
- **Porting jumppad's resources** — a spec non-goal. This plan writes down the layout those resources will follow, but ports nothing.
- **Jumppad's engine-level behaviour** (implicit image cache, registry merging) — a spec non-goal, left to a future port.
- **Rebuilding the docker plugin example to the layout** — owned by the sibling plan `20261009102138-48e95432-docker-example-standard-layout`. This plan only writes the layout guide that plan refers to.
- **Any change to the plugin contract or to xcl's Go code** — the spec forbids contract changes. This plan changes only docs and the changelog in `xcl`.
- **A generator or a rename tool for the template** — the spec says no generator. The README lists the renames by hand.
- **Automated layout or file-order conformance checks** — the knowledge base forbids tests that walk or grep the repository's files. Reviewers check both by hand.
- **Running the cross-platform binaries on darwin and windows in CI** — CI builds them in the snapshot release but runs tests on linux only.
