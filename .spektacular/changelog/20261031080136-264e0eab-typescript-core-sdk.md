---
created_date: "2026-11-10"
document_status: final
closed_date: "2026-11-10"
---

# xcl for TypeScript and Node

## What was built

The `xcl-lang` npm package brings xcl to Node. It runs the same bundled `xcl-core` engine Go and Python use, so a Node program reads, checks and applies configuration exactly as a Go program would, with the same Go plugins.

In `xcl-typescript`:

- A byte transport that alone knows about the engine process (one engine per process and per worker thread, released from the event loop whenever idle, reporting an engine's death to every waiting call), a typed client built from the generated `CoreService` descriptor, and the hand-written API above them.
- `createConfig`, the five operations, lookups, typed errors with one class per code, variables, outputs and diffs.
- Decorated entity classes that work under both standard and legacy decorators, schema v1 derived at registration, hydration into class instances, TypeScript functions callable from configuration, and encoding entities as configuration text.
- Events, custom state stores, state and event masks, cancellation through `AbortSignal`, and opt-in Ctrl-C.
- Eight platform packages, a reusable test workflow across every platform and Node version, bump, tag and release workflows, a parity suite against Go's results on the shared fixtures, a README, and a parity record.

In `xcl`: the release workflow also dispatches `xcl-typescript`, and the bindings gate runs and requires the TypeScript suite. The SDK design document was corrected.

## Why it matters

TypeScript and JavaScript developers could not use xcl. People building command-line tools and services in TypeScript now can, describing their configuration's types as classes, and long-running work never stalls the rest of their program. Printing, the pretty logger and the SDK reference follow in later specs of the epic.

## Deviations from the plan

- **The npm and GitHub setup was not done.** It is a person's task, and the user accepted the shortfall. All three of its criteria remain unmet until then, so no release can publish and xcl's dispatch job fails for want of its secret.
- **No workflow has run on GitHub and nothing is on npm.** All testing ran on Linux x86-64, with the suites passing 876 of 876. macOS, Windows, arm64 and Node 24 are unexercised until the first CI run. The test plan lists what a person must check.
- **The spec's Cancellation criterion was amended**, with the user's approval, to "within two seconds, once any provider call in flight has returned", because the engine lets a provider call finish. The design's `entities` and Releases text was corrected, also with approval.
- `optionalDependencies` are written into `package.json` by the release script rather than kept in the repository, because unpublished versions make `npm ci` refuse the lock file.
- `EngineExitedError` carries the status as `exitCode`, since `code` is the stable error code.
- A missing required field is reported by `apply`, not `validate`, because the engine's `Validate` parses without decoding, as from Go and Python.
- TypeScript is pinned at 5.9.3. The goal check's snapshot was not written for the error-classes task, because a helper in a shared scratchpad was overwritten; nothing changed during that review.
