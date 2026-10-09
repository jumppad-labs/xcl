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
