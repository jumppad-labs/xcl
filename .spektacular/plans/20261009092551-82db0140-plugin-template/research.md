---
created_date: "2026-10-09"
document_status: final
closed_date: "2026-10-09"
---

# Research: 20261009092551-82db0140-plugin-template

## Alternatives considered and rejected

- **Grow the template inside the xcl repo (e.g. `example/plugin-template/`) instead of a separate GitHub template repository** — rejected: the spec's constraint "Delivered as a GitHub template repository" requires "Use this template" / `gh repo create --template`, which works only on a whole repository, and the release automation must run on tags of the plugin's own repo (the asset contract names the plugin after the repository).
- **A generator (`xcl new plugin`, cookiecutter, `gonew`)** — rejected by the spec constraint ("not from a generator").
- **Two templates, one in-process and one external** — rejected by the spec constraint ("one template for both ways of running"); the layout design already serves both from one module (`plugin.go` in the root package, `cmd/<plugin>/main.go` calling `plugins.Serve`), as `example/plugin/plugins/docker/main.go:15-17` shows for the external side and `registry/local.go:121-128` (`RegisterPlugin`) for the in-process side.
- **Reusing the Handlebars `template` plugin (`example/plugin/plugins/template/`) as the sample** — rejected: it imports `github.com/infinytum/raymond/v2` from the provider file directly (`template.go:18`), has no client layer or test double, and its `Changed` compares fields itself (`template.go:72-78`) rather than with `change.Within`; the layout design wants a backend client behind an interface with a Mockery double. The sibling docker-example spec keeps it in the example (file order only).
- **A backend that needs a service (HTTP API, Docker)** — rejected by the constraint "The template's sample needs no external service"; the Technical Approach steers to "files on disk".
- **A small host application inside the template for applying the sample** — rejected: the layout design names no such part, and the success metric "One shape everywhere" forbids parts in places the design does not name. The in-process and external registrations live in the `e2e/` tests (a place the design names), and the README quotes them and runs each with one make target.
- **An in-process vs external parity test** — explicitly a spec non-goal. The e2e suite runs the in-process lifecycle and the external lifecycle in separate test functions and never compares their results.
- **Generating the template from the xcl repo with a script / keeping a copy in xcl** — rejected: two copies drift; the spec makes the template a registered project repository so contract changes are planned into it directly.
- **GoReleaser for building, archiving, checksumming, signing and publishing releases** — rejected: the user dislikes it (user decision at review). `make dist` with `go build`, tar/zip and `sha256sum`, plus `gpg` and `gh` in the release workflow, produces the same contract; the release-asset design accepts any tool producing the same names and formats (`plugin-release-assets.md`, "Producing it").
- **`crazy-max/ghaction-import-gpg` for importing the signing key in the release workflow** — rejected: plain `gpg --batch --import` of the secret suffices and adds no third-party action.
- **golangci-lint for static checks** — rejected as heavier than needed; `go vet`, a `gofmt -l` check and `staticcheck` pinned and run through `go run` (the same "no install" pattern `example/plugin/Makefile:16-18` uses for Mockery) cover "static checks" with fewer moving parts.
- **Checking "entities stand alone" with `go list -deps` or an import-grepping test** — rejected by the knowledge entry `conventions/testing-and-mocking.md` (no test walks or greps repository source to enforce a rule). Instead a behavioural e2e test in its own package imports only xcl, the registry and `entities`, starts the plugin as a built binary, and reads applied resources from state into the entity types.

## Chosen approach — evidence

- Plugin contract today: `plugins/provider.go` (`ResourceProvider[T]`: `Init`, `Create`, `Destroy`, `Read`, `Update(ctx, resource, changes, dependencies)`, `Changed(ctx, old, new, changes, dependencies)`, `Functions`); `plugins/changed.go:24-49` (`DefaultChanged[T]` never answers replace).
- `change.Within(entity.Path{}.Attribute("x"))`: `entity/property_change.go:50-52`, `entity/path.go:107-109`; real use at `example/plugin/plugins/docker/resources/container.go:297` and `e2e/fixtures/recorder/recorder.go:136`.
- In-process registration: `registry.NewLocal().RegisterPlugin(p)` (`registry/local.go:121`); external: `RegisterExternalPlugin(path)` (`registry/local.go:131`); serving: `plugins.Serve(p)` (`plugins/serve.go:21`).
- Config operations for the e2e: `Apply(paths...)` (`config.go:353`), `Diff(paths, ...)` (`config_diff.go:34`), `Destroy()` (`config.go:420`), `WithStatePath` / `WithVariables` (`options.go:50,126`), `FindByType[T]` / `As[T]` (`query.go:62,180`).
- Plugin entities convert into a named Go type when queried (knowledge `gotchas/registered-types-convert-only-to-themselves.md`), which is what lets a state reader decode into `entities.Note` without the providers.
- Nested named struct fields in plugin types come out empty on the host (knowledge `gotchas/plugin-types-rebuilt-with-structof.md`) — the sample entity uses only plain fields.
- Mockery v3 config and pinned `go run` invocation to copy: `example/plugin/.mockery.yml`, `example/plugin/Makefile:16-18,97-99`.
- Strict double pattern (constructor injection, mock per test): `example/plugin/plugins/docker/resources/container_test.go`, `network_test.go`.
- CI shape to copy (setup-go, build, vet, test): `.github/workflows/go.yml`.
- Changelog format: `CHANGELOG.md` top entry `## <spec-name>`, prose, then `**Breaking:**` list.
- Docs locations: `docs/README.md` "Start here" list, `docs/plugin-developer-guide.md` ("The example provider" section at :856), `docs/plugins.md`; site nav `xcl-website/src/components/Nav.astro:18-22` (Guides), page pattern `xcl-website/src/pages/registries.mdx` (Shell layout, Hero, Prose).
- Release layout: the plugin is pure Go, so `CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -trimpath` cross-compiles all six platforms on one linux runner; `tar -czf -C <dir> .` and `zip` put the binary at the archive root; `sha256sum` writes exactly the contract's `<hex>  <name>` lines; `gpg --armor --detach-sign` writes the ASCII-armoured detached `<checksums>.sig`; `gh release create <tag> dist/*` uploads them with the workflow's `GITHUB_TOKEN`. go (via setup-go), tar, zip, sha256sum, gpg and gh are all on GitHub's `ubuntu-latest` runner. The archive and checksums names use the version without its leading `v` (`plugin-release-assets.md`) and the repository name, which is the plugin name the asset contract fixes.

## Files examined

- xcl:plugins/provider.go:1-140 — the ResourceProvider contract and method docs.
- xcl:plugins/changed.go:1-70 — DefaultChanged behaviour.
- xcl:plugins/serve.go:21-29 — Serve for external plugins.
- xcl:entity/property_change.go:50 — PropertyChange.Within.
- xcl:registry/local.go:1-160 — NewLocal, RegisterPlugin, RegisterExternalPlugin, option style.
- xcl:registry/registry.go:64-115 — InProcess and Executable starters.
- xcl:options.go — Config options (WithRegistry, WithStatePath, WithVariables).
- xcl:query.go:29-351 — Find/As/FindByType/All.
- xcl:example/plugin/plugins/docker/{main.go,plugin.go,client/client.go} — external plugin shape, client interface + real impl.
- xcl:example/plugin/plugins/template/{plugin.go,template.go} — Handlebars sample; not reused.
- xcl:example/plugin/{Makefile,.mockery.yml} — make targets and pinned Mockery.
- xcl:e2e/fixtures/externalplugin — external plugin fixture registering several types.
- xcl:.github/workflows/go.yml — CI style, Go versions (1.27.0 test, 1.25.0 minimum).
- xcl:go.mod — module `github.com/jumppad-labs/xcl`, go 1.25.0; tags v0.1.0 only.
- xcl:CHANGELOG.md:1-40 — entry format.
- xcl:docs/README.md, docs/plugin-developer-guide.md, docs/plugins.md — headings; no layout guide exists yet.
- xcl:README.md:95-140,457-505 — registering plugins docs.
- xcl-website:src/components/Nav.astro:18-22 — Guides nav.
- xcl-website:src/pages/registries.mdx — page pattern.
- xcl-website:Makefile, package.json — `npm run build`, `npx astro check`.

## External references

- GnuPG (`gpg --batch --import`, `--pinentry-mode loopback --passphrase`, `--armor --detach-sign`) — imports the key from repository secrets and signs the checksums file in the release workflow.
- GitHub CLI `gh release create <tag> <files>` — publishes the release and its assets with the workflow's `GITHUB_TOKEN`.
- GitHub-hosted `ubuntu-latest` runner image — ships tar, zip, sha256sum, gpg and gh, so the release workflow installs nothing extra.
- GitHub template repositories ("Use this template", `gh repo create --template`) — the delivery mechanism the spec fixes.
- Mockery v3 (`github.com/vektra/mockery/v3`) — test doubles, already pinned at v3.8.0 in the example.
- staticcheck (`honnef.co/go/tools/cmd/staticcheck`) — static checks in CI, run pinned through `go run`.

## Prior plans / specs consulted

- Plan `20261009102148-7d0b205b-github-releases-registry` (final) — the registry that installs what this template publishes: `registry.NewGitHub(...)`, `RegisterPlugin("owner/repo","v1.2.0")`, `GitHubTrustedKeys(armored...)`, ProtonMail/go-crypto verification; its open question on key algorithm compatibility with the template's gpg-produced signature is answered here by choosing an RSA 4096 or Ed25519 key (both supported). Its site guide page is where this plan's docs link for installing.
- Plan `20261009102138-48e95432-docker-example-standard-layout` (being planned in parallel) working notes — it rebuilds the docker example to the layout, keeps the Handlebars template plugin layout as is, and explicitly leaves the written layout guide to this spec.
- Plan `20261008194959-b128e508-update-property-changes` (via CHANGELOG) — the current `Changed`/`Update` contract the template builds to.

## Open assumptions

- `jumppad-labs/xcl-plugin-template` is the template repository's name. It was registered during planning at `/home/nicj/code/github.com/jumppad-labs/xcl-plugin-template`, at the user's direction. The first (human) task creates it on GitHub and makes that directory the clone.
- The template's `go.mod` can require an xcl version containing the current contract. Only `v0.1.0` is tagged today and it predates `entity.PropertyChange`; until a release is cut the template pins a pseudo-version of a pushed xcl commit, and the final pin is an xcl release cut by a person.
- Declaring nothing but reading a plugin entity from state into `entities.Note` with `xcl.FindByType` works when the plugin is started from its binary (conversion path for plugin entities). If it does not, STOP and ask.
- A testify-generated Mockery mock with no expectation for a call fails the test on that call (strict double), as the docker example relies on.
- `make dist`'s archive and checksums names match the contract exactly (version without its leading `v`, `.zip` on windows, binary at the archive root). Verified by the `dist/` listing of the snapshot build in CI.
- The `ubuntu-latest` runner keeps shipping tar, zip, sha256sum, gpg and gh; if one disappears, the workflow installs it with `apt-get`.

## Drafting assumptions

### Template repository name (discovery)
- **Decision**: `jumppad-labs/xcl-plugin-template`, Go module `github.com/jumppad-labs/xcl-plugin-template`, cloned beside the other repos.
- **Rationale**: matches the `xcl-plugin-*` naming the local registry searches for and the asset contract's example names.
- **Rejected**: `xcl-template` (does not read as a plugin), a path inside the xcl repo (spec requires a template repository).

### Sample resource: a note written to a file (discovery)
- **Decision**: one resource, block type `notes "note"`, backed by a `client/files` package (narrow `Files` interface + local filesystem implementation + Mockery double). `directory` and `name` need a replace; `content` and `mode` update in place.
- **Rationale**: the spec steers to "files on disk"; a note gives both outcomes of the change decision and an update that needs only the reported changes.
- **Rejected**: reusing the Handlebars template plugin (direct library import, no client layer), an HTTP-backed sample (needs a service).

### In-process and external registration live in e2e (discovery)
- **Decision**: no host application in the template; `e2e/` holds the in-process and external lifecycle tests, each its own function, and the README quotes their registration code and runs each with one make target.
- **Rationale**: the layout names no host-app location and the success metric forbids parts in unnamed places; separate tests are not the parity test the spec excludes.
- **Rejected**: a `cmd/host` program (unnamed part), one test comparing both modes (spec non-goal).

### Chosen direction: separate template repository built to the layout design (architecture)
- **Decision**: new repo `jumppad-labs/xcl-plugin-template` with the design's tree; filesystem-backed `notes "note"` sample; e2e holds both registrations; `make dist` + gpg + gh in GitHub Actions; docs in xcl `docs/` and one site page.
- **Rationale**: the spec fixes the template-repository delivery, one template for both modes, no external service, Actions CI and secret-held key; the designs fix layout and asset names; this is the only shape left that satisfies all of them.
- **Rejected**: a generator, two templates, a copy inside the xcl repo, a host program in the template, GoReleaser (see research.md).

### Replace on any replaced dependency (architecture)
- **Decision**: the sample `Changed` answers replace for any dependency reported as replaced, with a comment showing where to narrow it to the types a resource is built on.
- **Rationale**: the sample has one resource type, so there is no narrower "built on" type to name; the spec's Technical Approach requires the rule to be shown.
- **Rejected**: omitting the dependency rule (contradicts the Technical Approach), inventing a second resource type to depend on (spec asks for one resource).

### Mode-only update changes the mode only (architecture)
- **Decision**: `Update` rewrites the file only for a `content` change and only changes the mode for a `mode`-only change; the strict double proves no other call is made.
- **Rationale**: gives the told-only rule a visible branch that depends on which settings changed, which a single "rewrite everything" update would not show.
- **Rejected**: always rewriting the file (does not demonstrate acting on the reported changes).

### Release packaging with make dist + gpg + gh, no GoReleaser (user decision at review) (architecture)
- **Decision**: `make dist VERSION=vX.Y.Z` builds `./cmd/notes` for linux, darwin and windows × amd64 and arm64 with `CGO_ENABLED=0 go build -trimpath`, packs each binary at the root of `<name>_<version>_<os>_<arch>.tar.gz` (`.zip` on windows) and writes `<name>_<version>_checksums.txt` with `sha256sum`, all in `dist/`; CI runs it with a snapshot version on every push and pull request; the tag-triggered release workflow fails before publishing when `GPG_PRIVATE_KEY` is empty, runs `make dist` for the tag, imports the key with `gpg --batch --import`, signs the checksums with `gpg --armor --detach-sign`, and publishes `dist/*` with `gh release create` using `GITHUB_TOKEN`; `make release VERSION=...` stays the one command that tags and pushes.
- **Rationale**: the user rejected the third-party release tool at review; go, tar/zip, sha256sum, gpg and gh are already on the runner and produce the asset contract exactly, with nothing extra to pin.
- **Rejected**: GoReleaser (user dislikes it), `crazy-max/ghaction-import-gpg` (plain gpg suffices).

### Release, CI and tooling files sit outside the layout tree (architecture)
- **Decision**: `.github/workflows/*`, `.gitignore` and `LICENSE` are repository tooling, not plugin parts, so they do not breach the "no part in a place the design doesn't name" metric.
- **Rationale**: the spec itself requires CI and release automation, which GitHub Actions locates at a fixed path; the design's tree already lists tooling files (`.mockery.yml`, `Makefile`) at the root.
- **Rejected**: folding release and CI into the Makefile only (GitHub Actions requires `.github/workflows`).

### Conventions selected (architecture)
- **Decision**: testing & mocking, code style, dependencies, patterns, project structure (`/cmd`), structured logs, real-apply state, never modify dependencies; dropped database, internal/testutil, errors package, graph-parent ordering.
- **Rationale**: the dropped ones govern code this plan does not write or rules local to the xcl module.
- **Rejected**: listing every convention.

### Files client takes a context and returns its own Info (data_structures)
- **Decision**: every `Files` method takes `ctx` and returns client-owned types (`Info`), never entity types.
- **Rationale**: the layout design says client packages own their data shapes and never depend on entities; `ctx` keeps the shape authors copy for real network backends.
- **Rejected**: a context-free filesystem interface (less representative of real backends), returning `*entities.Note` (breaks the layout rule).

### Release signing secrets named GPG_PRIVATE_KEY and GPG_PASSPHRASE (data_structures)
- **Decision**: the release workflow reads the armoured private key and passphrase from secrets `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE`.
- **Rationale**: plain, descriptive names the release workflow's `gpg --batch --import` and signing steps read, and the names the secrets already exist under; the spec only says the key is a repository secret.
- **Rejected**: other names (no benefit).

### Pseudo-version until an xcl release is cut by a person (dependencies)
- **Decision**: the template's go.mod pins a pseudo-version of a pushed xcl commit carrying the current contract, and a person cuts an xcl release so the final pin (and the "latest release" scheduled check) has something to resolve.
- **Rationale**: `v0.1.0` predates `entity.PropertyChange`, which the template's `Changed`/`Update` need; cutting a release is a maintainer decision.
- **Rejected**: a `replace` to a local checkout (cannot be committed in a template), building to the `v0.1.0` contract (spec: "the contract as it is today").

### Signing key algorithm (dependencies)
- **Decision**: RSA 4096 or Ed25519 key, chosen by the person who makes it.
- **Rationale**: both are verified by ProtonMail/go-crypto, closing the registry plan's open question.
- **Rejected**: leaving the algorithm unstated.

### End-to-end replace/update plan checks (testing_approach)
- **Decision**: add e2e functions asserting the real plan shows replace for a `name` change and update for a `content` change.
- **Rationale**: the acceptance criterion speaks of "the plan" showing the outcome, beyond the unit tests.
- **Rejected**: unit tests only (would not show the plan output an author sees).

### Template repo registered during planning (tasks)
- **Decision**: per the user's answer (option A), `xcl-plugin-template` was registered during planning with `spektacular repo add` (provider git, source `git@github.com:jumppad-labs/xcl-plugin-template.git`). The clone step failed because the GitHub repo doesn't exist yet, so the registry entry has an empty root until the first human task creates the repo, makes the existing footprint directory the clone and repairs the registration.
- **Rationale**: the plan store refuses tasks attributed to an unregistered repo, and the spec wants work attributed to the template repo.
- **Rejected**: attributing template tasks to xcl.

## Rehydration cues


- `spektacular spec file read 20261009092551-82db0140-plugin-template`
- `spektacular design read --data '{"source":"design","path":"plugin-layout.md"}'` and `... "path":"plugin-release-assets.md"`
- `spektacular plan file read 20261009102148-7d0b205b-github-releases-registry plan`
- `spektacular knowledge always-applied --tier repo --filter xcl --filter xcl-website`
- Re-read `example/plugin/plugins/docker/` (client + tests), `plugins/provider.go`, `registry/local.go`, `.github/workflows/go.yml`, `xcl-website/src/components/Nav.astro`.
