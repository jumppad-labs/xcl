---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# Test plan: xcl for TypeScript

Everything here is a check a person makes. The behavioural success metrics (Go parity, event-loop delay, first configuration under one second) are covered by the test suites.

**Prerequisite for the release checks:** the npm and GitHub setup of task "Set up npm publishing and GitHub secrets" must be done first. It was not done during implementation.

## Manual success metrics

### A developer reaches a working apply from the README alone

- **What to measure:** one developer who has not seen the package, following `README.md` only, gets a successful apply against a Go plugin. Pass: the program prints the entity with no help and no step outside the README.
- **How:** on a machine with Node 22 or later and no Go, in an empty directory, follow "Install" and "Getting started", taking the release key from `https://xcl.dev/keys/jumppad-labs-releases.asc`. Run with `npx tsc` and `node build/main.js`, or as the README describes.
- **Expected result:** the program prints `test.item.second two <output of first>` and exits 0, and `.xcl` holds the state.
- **Who / when:** a developer new to the package, once the first release is on npm.

### Every Go operation and lookup has a TypeScript equivalent

- **What to measure:** the parity record against the Go API of the bundled tag. Pass: no exported `*Config` method or package-level lookup is missing from `PARITY.md`.
- **How:** in an xcl checkout at the tag in `xcl-ref.txt`, run `grep -hE '^func \(c \*Config\) [A-Z]' *.go | grep -v _test` and `go doc . | grep ^func`, and compare each with `PARITY.md`. Utilities that are neither an operation nor a lookup (`HashString`, `ReadFileLocation`) are out of scope.
- **Expected result:** every operation and lookup has a row or a "Not applicable" entry. Rows marked planned are the printing spec's.
- **Who / when:** the release manager, at each release.

### No orphaned engine after a Node program exits

- **What to measure:** open issues on `jumppad-labs/xcl-typescript` reporting an `xcl-core` process left running. Pass: zero.
- **How:** `gh issue list --repo jumppad-labs/xcl-typescript --search "orphan OR \"left running\" xcl-core"`.
- **Expected result:** no open issue describing an orphaned engine.
- **Who / when:** the maintainer, monthly after the first release.

## Manual reviews

### The first release works end to end

- **Look at:** cut an xcl release tag and watch `xcl-typescript`'s `engine-bump.yml`, the merged pull request, `tag.yml` and `release.yml`.
- **Pass:** with no manual publish, nine packages appear on npm at the tag's version with provenance (`npm view xcl-lang@<version> dist.attestations`), and `node_modules/xcl-lang-<platform>/bin/xcl-core --version` prints the tag.
- **Who / when:** the release manager, first release. This is the first real exercise of the workflows, which have never run on GitHub.

### A parity-breaking change fails its run

- **Look at:** a throwaway pull request to xcl that changes an entity's rendered text (for example in the encoder), and a throwaway pull request to `xcl-typescript` that changes a value in `src/hydrate.ts`.
- **Pass:** the xcl pull request's `bindings` check and the package's `result` check each fail, naming a fixture and the first differing path. Close both unmerged.
- **Who / when:** the maintainer, after the bindings job is live.

### A clean install on macOS and Windows, offline after install

- **Look at:** a fresh macOS machine and a fresh Windows machine with no Go or C compiler, on Node 22, 24 and 26.
- **How:** `npm install xcl-lang`, then disconnect the network and run a script that imports the package as an ES module, calls `createConfig()` and `close()`.
- **Pass:** the script exits 0 with the network off, on each machine and Node version.
- **Who / when:** the release manager, first release. The test workflow covers these platforms only once it has run; none has been exercised yet.

### Ctrl-C on Windows

- **Look at:** a console program on Windows running a slow apply with `xcl-plugin-slow` (`build/plugins`, built with `make plugins`).
- **How:** press Ctrl-C mid-apply in a program using `{ signal: interruptSignal() }`, and in one without it.
- **Pass:** opted in, the apply rejects with `CancelledError`, the program's own code then runs, and completed entities are in state. Not opted in, the program ends as Node does by default and completed entities are in the state directory. In both, no `xcl-core.exe` remains in Task Manager.
- **Who / when:** a developer on Windows, before the first release. Automated signal tests are POSIX only.

### The npm page

- **Look at:** `https://www.npmjs.com/package/xcl-lang` and `examples/getting-started/main.ts`.
- **Pass:** the page shows the README with install and a getting started applying a configuration with a Go plugin, and the README's example matches the checked example.
- **Who / when:** the release manager, first release.

### The parity record

- **Look at:** `PARITY.md`.
- **Pass:** it lists every Go operation and lookup with its TypeScript equivalent, as in the metric above.
- **Who / when:** the release manager, each release.

### Doc comments

- **Look at:** `src/index.ts` and every declaration it exports, or the generated `dist/index.d.ts` in an editor.
- **Pass:** every public export shows a doc comment.
- **Who / when:** a reviewer, before the first release.

### Package managers and C libraries

- **Look at:** fresh projects on Linux glibc, Alpine (`node:22-alpine`, on x86-64 and arm64), macOS and Windows.
- **How:** install with npm, pnpm, yarn and bun, then check `node_modules/` for the platform package and run a script that creates a configuration. Then install with `npm install --omit=optional`.
- **Pass:** each manager picks the right package, including musl on Alpine, and with optional dependencies omitted the first use throws `EngineNotFoundError` naming the package and `npm install <package>`.
- **Who / when:** the release manager, first release.
