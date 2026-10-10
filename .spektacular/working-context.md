# Working context: 20261009102148-7d0b205b-github-releases-registry

Part of epic 20261009092551-82db0140-plugin-template ("Plugin standard and distribution"). The epic's other specs:
- plugin-template: the GitHub template repo; its release action publishes signed, multi-platform releases in this registry's format; it depends on this spec.
- docker-example-standard-layout.

## Motivation
- The user added this requirement while specifying the template: "we should add a Github releases remote registry".
- xcl's registry design (design:plugin-registries.md) already anticipates remote registries (`registry.NewRemote(...)`, `remote.RegisterPlugin("postgres", "1.2.3")`). Today only the local registry exists (`registry.NewLocal`, with `InProcess`/`Executable` plugin starters; third parties write their own registries).
- What this adds: xcl fetches a plugin binary for the current platform from a GitHub repo's release, verifies it, caches it, and starts it as an external plugin.

## User's answers
- Private repos: yes, via a GitHub token (from the environment).
- Cache: a default location (under the xcl home directory), with an option to override.
- Versions: pinned to exact versions.
- Verification: GPG signing. The registry verifies; the template's release action signs ("this should be in the template too").
- Docs must be updated (registry guide, README, website page on installing from GitHub).

## Shared contract
- A release-asset contract design (`plugin-release-assets.md`) was proposed, shared with the template spec: asset naming per OS/arch, checksum file, signature, version tags. Not yet written.

## Open
- How the registry knows which GPG public key(s) to trust (per plugin, per owner, or configured on the registry).
- Interview answers: key choice is up to the user (app supplies trusted keys); unsigned releases allowed when no key is configured; platforms linux/darwin/windows x amd64/arm64.
- Registry spec finished. Epic order: docker-example, registry, template (template depends on registry). Design plugin-release-assets.md is a draft pending user review. Decisions made for review: a release without checksums is refused; cached plugins are re-verified before each start; default cache under xcl home, per repo/version; GoReleaser naming; signature over the checksums file.

# Orchestrator: plan epic 20261009092551-82db0140-plugin-template (2026-10-09)
- Project root: /home/nicj/code/github.com/jumppad-labs/xcl
- Started children: 20261009102138-48e95432-docker-example-standard-layout, 20261009102148-7d0b205b-github-releases-registry
- All 3 plans DONE. Changelog disagreement settled (docker plan now adds entry). Summary written; epic order added registry -> docker-example. Next: end-of-planning review.
- Review done: registry plan wraps registry.Local; template repo created+pushed by user, registration repaired (repo.yaml source file ..), user marked repo as template; that task removed from plan. Repo xclconfig renamed to xcl by user; all plans + summary updated (ordering log section still says xclconfig, written only by epic order). User tagged xcl v0.1.1 and added GPG secrets: template plan drops the xcl-release and pin tasks, scaffold requires v0.1.1, signing-key task reduced to publishing the public key. Public key: shared jumppad-labs Ed25519 key (fpr 15BD A684 3A1F A0EA D1AA 5190 F775 DA00 AFD4 B502) to be served at xcl.dev/keys/jumppad-labs-releases.asc; key task is now an xcl-website agent task. No GoReleaser: template uses make dist + gpg + gh; design plugin-release-assets.md revised; plans + summary updated.
- Possible cross-plan disagreement: changelog (registry plan has a CHANGELOG task; docker plan leaves it to implement workflow).
- Q (plugin-template): register xcl-plugin-template repo during planning? A: yes, option A (name xcl-plugin-template, location /home/nicj/code/github.com/jumppad-labs/xcl-plugin-template).

# Orchestrator: implement epic 20261009092551-82db0140-plugin-template (2026-10-10)
- Order: docker-example -> github-releases-registry -> plugin-template (strictly sequential).
- Repos clean at start; user told about spek/<spec> branch commits.
- Worktrees created + child started: 20261009102138-48e95432-docker-example-standard-layout
- docker-example DONE + merged. Registry worktrees created + child started.
- registry DONE + merged. Template worktrees created + child started.
- Q (plugin-template): human task 'publish first signed release' — A: defer until after merge (option 1); release listed as first manual check in test plan.
- plugin-template DONE + merged. Epic implement complete.

# Repo add: xcl-plugin-docker (2026-10-10)
- User created xcl-plugin-docker from the xcl-plugin-template GitHub template to test the end-to-end process; code at /home/nicj/code/github.com/jumppad-labs/xcl-plugin-docker.
- (xcl-plugin-template registry metadata still stale: says GoReleaser; not fixed — user chose to add the new repo instead.)
- Name agreed: xcl-plugin-docker. User handed over description/role/tags. Purpose: maintained as an example remote plugin (Docker), installed via the GitHub releases registry.
- User confirmed registration (reusing the template-copied .spektacular folder, metadata to be replaced). Flagged: template's changelog record copied in; template ships its .spektacular folder.
- xcl-plugin-docker registered; repo.yaml metadata now Docker example. Template changelog record still present (offered deletion).

# Template footprint move (2026-10-10)
- User chose: move xcl-plugin-template's Spektacular files out of the template repo into the xcl project (placement "project"), so repos made from the template don't inherit .spektacular/. User OKs removing the folder from the template repo by hand (CLI can't delete repo-scoped changelog records).
- Template changelog record staged at .spektacular/tmp/xcl-plugin-template/changelog.out to rewrite into the new footprint.
- Also fix stale metadata (GoReleaser -> make dist + gpg + gh).
- Name kept xcl-plugin-template; metadata accepted, GoReleaser removed.
- User confirmed template re-registration (placement project).
- Template footprint moved to xcl/repos/xcl-plugin-template/.spektacular; changelog record rewritten there (identical); template repo's .spektacular git rm'd (uncommitted).
