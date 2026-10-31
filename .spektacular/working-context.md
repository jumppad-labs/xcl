# Working context: plugin-release-contract-v2

## Problem and motivation

The user wants xcl plugins written in languages other than Go: a Python plugin run from Go, Python from Python, TypeScript from Python, and so on. Today plugins are compiled Go binaries, released under `plugin-release-assets.md`: a per-platform tar.gz/zip with the binary at the root, a checksums file and an optional signature. The user's words: "I think it makes sense that plugins should be loadable in docker containers which bundle the runtime and plugin", and "maybe we define a plugin as a zip, it contains resources and a manifest which describes execution". The first step agreed was the release asset contract. The Python plugin SDK is designed separately (`python-plugins.md`) and depends on a protocol 2 transport design that isn't written yet, so it is not part of this spec.

## Settled design (binding): `plugin-release-assets-v2.md`

- Two package kinds only: `executable` (native binary per platform, any language) and `container` (multi-platform image). The user removed source packages ("let's keep it simple, container, executable"). A source kind was explored (host runtime, package managers, a `source` kind with a command) and dropped as too complex. A later kind can be added because readers skip unknown kinds.
- Release index `<name>_<version>_index.xcl` (generated, static, literals only), written in xcl. The user: "we really should use xcl for all our config files". `plugin {}` singleton, `executable {}` and `container {}` singleton blocks, `platform "<os>" "<arch>"` children. The user rejected package names ("I don't see why we need native and image").
- Checksums live in the index: `checksum = "sha256:<hex>"` on each executable platform; the container image is pinned by digest. The user: "no checksum, no work". An index missing either is refused. No separate checksums file, and the signature `<name>_<version>_index.xcl.sig` is over the index.
- Zips have `xcl-plugin.xcl` at the root, with `plugin` and `executable { protocol, command, platform }`.
- No backwards compatibility. The user: "No backwards compatibility, we rebuild the two existing plugins" (xcl-plugin-docker, xcl-plugin-test). A release without an index is refused.
- Default selection order: executable, then container (only when a container runtime is present).
- Not covered: the protocol/transport (separate design), how a container is started and its access (mounts etc.), source packages, types in the index, OCI.

## Already done in this branch (commits 9e9893d, 348ec83)

- The gohcl fix: nested blocks in a list are named by all their labels (the design's "depends on" item).
- `DefaultChanged.ReplaceOn` (related, from the Python design discussion). The xcl-plugin-docker and template providers are to adopt it "later" ("we can do the plugin later"). That work is natural to fold into the rebuild.

## Open items for planning (from the design's "Depends on")

- A literals-only parse for the index and manifest: a parser option or a check after parsing.
- Tolerating unknown kinds: today an unregistered top-level block is refused.

## Decisions taken while the user was AFK

- The user asked to "commit changes and build the spec and the plan" and went AFK. This spec covers only the asset contract, because the Python plugins design depends on the unwritten transport design.
- The in-progress TypeScript spec workflow (at verification) was displaced with `spec new --force`. Its state.json is committed at cdd4de8, with a backup in the session scratchpad. It must be restored afterwards.
- The spec stands alone; it was not added to the old plugin-template epic.

## Verification review (applied while the user was AFK)

A fresh-eyes reviewer raised 24 findings. All were applied as hygiene fixes that don't change any decision:
- trimmed the overview;
- removed the mechanism and duplication from the requirements;
- removed the package-order clause, because only native builds are chosen in this spec;
- added a rule that refuses a container entry with a non-Linux platform;
- moved the ReplaceOn detail to the technical approach, and the behaviour-preservation guarantee to the constraints;
- added the constraints "GitHub releases only" and "protocol doesn't change";
- added a criterion that the docs describe the new layout;
- dropped the success metrics that duplicated requirements.
