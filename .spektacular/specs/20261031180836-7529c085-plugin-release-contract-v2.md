---
created_date: "2026-10-31"
document_status: final
closed_date: "2026-10-31"
designs:
    - source: design
      path: plugin-release-assets-v2.md
---

# Feature: 20261031180836-7529c085-plugin-release-contract-v2

<!--
  OVERVIEW
  A concise 2-3 sentence summary of the feature. Answer three questions:
    1. What is being built?
    2. What problem does it solve?
    3. Who benefits and why does it matter?
  Avoid implementation details — this should be readable by any stakeholder.
-->
## Overview

Plugins installed from GitHub releases move to a new release layout that works for a plugin written in any language, with each release described by one index listing the native builds and container image it offers. Installs are verified end to end and fetch only what the machine needs. This is the first step towards plugins written in Python, TypeScript and other languages, so plugin authors in any language and the applications that install their plugins get one way to publish and install. It doesn't depend on Go.

<!--
  REQUIREMENTS
  Specific, testable behaviours the feature must deliver.
  Format: bold title on the checkbox line, detail indented below.
  Rules:
    - Use active voice: "Users can...", "The system must..."
    - Each requirement should be independently verifiable
    - Focus on WHAT, not HOW — avoid prescribing implementation
    - Keep each item atomic — one behaviour per line
-->
## Requirements

- [ ] **Releases are described by an index**
  The registry installs a plugin release only when the release carries an index describing the packages it offers. A release without an index is refused with an error saying it isn't a plugin release in the supported layout.
- [ ] **Incomplete indexes are refused before any download**
  The registry refuses a release whose index names a different plugin or version from the one requested, or breaks the checksum rule. It refuses before downloading any build.
- [ ] **A verified signature vouches for the whole release**
  When the application trusts signing keys, the registry installs a release only when its index carries a signature that verifies against one of them. That one verified signature vouches for every build in the release.
- [ ] **Only the build this machine needs is downloaded**
  The registry chooses the native build for this machine's operating system and architecture, and downloads only that build.
- [ ] **Downloaded builds are verified**
  A downloaded build whose checksum doesn't match the index is refused and not installed.
- [ ] **Each build agrees with its index**
  The registry refuses a build whose own description disagrees with the index: a different plugin name, version or protocol, or a platform the index doesn't list for it.
- [ ] **Unsupported machines get a clear error**
  When the release offers nothing this machine can run, the install fails with an error naming the platforms the release offers and the platform this machine has.
- [ ] **Release descriptions are static**
  The registry refuses an index or build description that uses expressions (variables, functions or references) instead of literal values.
- [ ] **New package kinds don't break older registries**
  The registry ignores a package kind it doesn't know and chooses among the kinds it does know, so a later release can offer new kinds without breaking existing applications.
- [ ] **Cached plugins are verified on every start**
  An installed plugin is used from the cache without contacting GitHub, and is verified again (including its signature, when keys are trusted) every time it starts.
- [ ] **Container entries are validated but not yet run**
  The registry validates a release's container entry and refuses a release whose container entry lists a platform other than Linux. It doesn't choose or start container images. A release offering only a container fails on this machine with the "nothing this machine can run" error.
- [ ] **Plugin authors can produce a release with one command**
  Plugin authors using the plugin template can produce and publish a complete release in the new layout, signed when a key is available, with a single release command and the release workflow.
- [ ] **The existing plugins are re-released**
  The Docker plugin and the test plugin each publish a new release in the new layout, built with the updated template tooling.
- [ ] **Users of the existing plugins move to the new releases**
  Every test, example and document that pins a release of the Docker plugin or the test plugin pins its new release, across the core library, the Python SDK and the documentation site.
- [ ] **Documentation describes the new layout**
  The plugin, registry and plugin-template documentation, in the repository and on the documentation site, describe the new release layout and no longer describe the old one.

<!--
  CONSTRAINTS
  Hard boundaries the solution must operate within. These are non-negotiable.
  Format: one bullet point per constraint.
  Examples:
    - Must integrate with the existing authentication system
    - Cannot introduce breaking changes to the public API
    - Must support the current minimum supported runtime versions
  Leave blank if there are no constraints.
-->
## Constraints

- **The release layout is the design `plugin-release-assets-v2.md`.** Asset names, the shape of the index and the build description, checksum and digest forms, the signature and the install sequence are as that design fixes them. The planner works within it and raises any disagreement with the user rather than designing around it. (A design decision settled with the user.)
- **No backwards compatibility.** Releases in the first layout (`plugin-release-assets.md`, now superseded) are not installed, and no code path reads them.
- **Release descriptions are xcl files, read through xcl itself.**
- **No checksum, no install.** Every native build has a checksum in the index and every container image is pinned by digest, or the release is refused.
- **This work supports only two package kinds:** native executables and container images. No package kind runs source code on a runtime installed on the host.
- **Plugin releases are GitHub releases.**
- **The plugin protocol doesn't change.** Existing plugins keep speaking protocol 1.
- **The registry's public API doesn't change.** Registering a plugin by repository and tag, trusted keys, tokens and the cache directory are configured exactly as today, in Go and through the Python SDK.
- **Signatures stay optional.** An application without trusted keys installs unsigned releases. With trusted keys, an unverified release is never installed.
- **Rebuilding the existing plugins must not change their behaviour.** For the same setting changes, they replace and update exactly the resources they did before.
- **Project test rules apply.** Unit tests use testify `require`, never table-driven tests, and never mix positive and negative cases in one test function.

<!--
  ACCEPTANCE CRITERIA
  The specific, binary conditions that define "done".
  Format: bold title on the checkbox line, verifiable detail indented below.
  Each criterion must be:
    - Independently verifiable (pass/fail, not subjective)
    - Traceable back to a requirement above
    - Testable by someone who didn't write the code
-->
## Acceptance Criteria

- [ ] **A release without an index is refused**
  Registering a plugin whose pinned release has builds but no index fails when plugins load. The error names the repository and tag and says the release isn't in the supported layout. Nothing is written to the plugin cache.
- [ ] **An incomplete index is refused before download**
  For a release whose index omits a checksum on one executable platform, names a container image by tag instead of digest, or names a different plugin or version than requested, plugins fail to load with an error naming the problem, and no build asset is requested from the server.
- [ ] **A signed release installs with trusted keys**
  With a trusted key supplied, a release whose index signature verifies against it installs and its plugin starts.
- [ ] **An unsigned or wrongly signed release is refused with trusted keys**
  With a trusted key supplied, a release with no index signature, or a signature from another key, fails to load with a signature error, and no build asset is requested.
- [ ] **Signatures are not required without trusted keys**
  With no trusted keys, an unsigned release installs and its plugin starts.
- [ ] **Only this machine's build is fetched**
  Installing a release offering builds for four platforms requests exactly one build asset from the server: the one for the running operating system and architecture.
- [ ] **A tampered build is refused**
  When the server returns a build whose bytes don't match the index checksum, plugins fail to load with a checksum error naming the build, and the plugin isn't cached.
- [ ] **A build that disagrees with its index is refused**
  When a build's own description names a different plugin, version, protocol or platform from its index entry, plugins fail to load with an error naming the mismatch.
- [ ] **No matching platform gives a clear error**
  Installing a release that offers no build for the running platform fails with an error listing the platforms the release offers and the running platform.
- [ ] **Expressions are refused**
  An index or build description that uses a variable, function call or reference in place of a literal fails to load, with an error pointing at the expression.
- [ ] **Unknown kinds are ignored**
  A release whose index carries an extra package kind the registry doesn't know, alongside a valid native build, installs that native build without error.
- [ ] **The cache is used and re-verified**
  A second load of the same pinned plugin makes no requests to the server. If the cached build is altered on disk between loads, the next load fails with a verification error.
- [ ] **Container-only releases don't install yet**
  A release offering only a valid, digest-pinned container image fails to install with the "nothing this machine can run" error. A release offering a container alongside native builds installs the native build.
- [ ] **A container entry with a non-Linux platform is refused**
  A release whose container entry lists a platform other than Linux fails to load with an error naming that platform, and no build asset is requested.
- [ ] **The template produces a complete release**
  Running the template's release build for a version produces one zip per supported platform, each with its description at the root and the binary marked executable, and an index whose checksums match the zips. With a key available it also produces a signature that verifies over the index.
- [ ] **The new plugin releases install end to end**
  The core library's live registry test, and the Python SDK's integration tests, install the new releases of the test plugin from GitHub, verify them against the published signing key, and apply configuration through them.
- [ ] **The Docker plugin works from its new release**
  The documented getting-started example, pinned to the Docker plugin's new release, creates and destroys its network and container.
- [ ] **The docs describe the new layout**
  The plugin, registry and plugin-template documentation, in the repository and on the documentation site, each describe the release index, the per-platform builds, checksums in the index, and the optional signature over the index.
- [ ] **No old-layout references remain**
  Neither the repository docs nor the documentation site describe the old layout's per-platform tar.gz archives, its checksums file, or a signature over the checksums file, and no test, example or doc pins a release of the Docker or test plugin made in the old layout.
- [ ] **Existing plugin behaviour is unchanged by the rebuild**
  For the same setting changes as before the rebuild, the Docker plugin and the template's plugin still replace the resources they replaced and update the ones they updated.

<!--
  TECHNICAL APPROACH
  High-level technical direction to guide the planning agent. Include:
    - Key architectural decisions already made
    - Preferred patterns or technologies if known
    - Integration points with existing systems
    - Known risks or areas of uncertainty
  Format: one bullet point per direction/steer.
  Leave blank if you want the planner to propose the approach.
-->
## Technical Approach

- **Decode the release descriptions as config-only xcl documents.** Make each package kind its own block type and platforms nested blocks with two labels. These already decode, thanks to the nested-block fix in commit 9e9893d. Whether expressions are refused through a parser option or a check after parsing, and how unknown top-level kinds are skipped instead of refused, are left to planning.
- **Move signature checking to the index.** Signature checking moves from the checksums file to the index, reusing the existing OpenPGP verification. The index carries every build's checksum and the image digest, which is why one signature covers the whole release.
- **Release in order.** The template tooling changes first. The test plugin and Docker plugin then publish their new releases. Only after that do the core library's live test, the Python SDK's pinned test release and the docs move to them. The core library's own registry tests keep using a fake release server, so they don't wait on a published release.
- **Existing Go plugins list protocol 1** in their indexes (`protocol = [1]`).
- **Fold `ReplaceOn` into the rebuild.** The Docker plugin and template providers move their hand-written replace checks to `DefaultChanged.ReplaceOn` while they're being rebuilt. Where a provider's hand-written `Changed` also answers update for changes the JSON comparison ignores, as the Docker network does for `depends_on`, keep that rule in its `Changed` so behaviour is preserved.

<!--
  SUCCESS METRICS
  How you will know the feature is working well after delivery. Be specific:
    - Quantitative: "p99 latency < 200ms", "error rate < 0.1%"
    - Behavioural: "users complete the flow without support intervention"
  Format: one bullet point per metric.
  Leave blank if not applicable.
-->
## Success Metrics

- A new plugin release is produced and published from the template with one release command and the release workflow, with no hand-written files.
- Pushing a version tag to a plugin built from the template produces a verifiable release with no manual step in between.
- Both existing plugins install from their new releases, from Go and from Python, verified against the published signing key, on every platform the core library's CI runs on.

<!--
  NON-GOALS
  Explicitly state what this spec does NOT cover. This is as important as
  the requirements — it prevents scope creep and sets clear expectations.
  Format: one bullet point per exclusion.
  Examples:
    - "Mobile support is out of scope (tracked in #456)"
    - "Internationalisation will be addressed in a follow-up spec"
  Leave blank if there are no explicit exclusions to call out.
-->
## Non-Goals

- **Starting container plugins.** How a container plugin is started (mounts, network, user, what it can reach) and the transport it speaks are separate designs.
- **Plugin protocol 2 and its stdin/stdout transport.** These are a separate design.
- **Plugins written in Python or TypeScript.** The Python plugin SDK (`python-plugins.md`) and a TypeScript counterpart follow in their own specs, once the transport is designed.
- **The types a plugin provides in the index.** The registry still learns a plugin's types by running it. Fetching only the plugins a configuration uses, and generating code from an index, are later work.
- **Publishing plugins to OCI registries.**
- **A command-line tool for building or publishing plugins.** The template's release tooling is the supported way to produce a release. Any other tool producing the same layout also works.
