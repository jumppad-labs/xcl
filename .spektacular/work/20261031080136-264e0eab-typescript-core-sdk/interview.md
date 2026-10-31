# Interview synthesis: TypeScript core SDK

## What is being built

The first spec of epic `20261031080131-8c2a58ec-typescript-sdk`: an xcl package for TypeScript and JavaScript on Node, in the new repository `jumppad-labs/xcl-typescript`, published to npm as `xcl-lang`. Like the Python package, it runs the same bundled `xcl-core` engine as a subprocess over stdio and reimplements nothing, so a Node program gets the same results as a Go program for the same configuration, plugins and state.

Who it is for: people building command-line tools, Jumppad-style tools and services in TypeScript/JavaScript on Node.

## From the sources (Python specs and README)

Feature set to match (Python's core, as it stands today):

- Install from the registry with no Go or C toolchain, on eight platforms (Linux x86-64/arm64 glibc and musl, macOS x86-64/arm64, Windows x86-64/arm64). Alpine non-negotiable.
- Entity types registered with a type name and optional subtype (`register_type(App, "app")`, `register_type(Db, "database", "postgres")`); required, optional, defaults, sensitive, computed, nested block, list of blocks, named blocks keyed by name, label fields, `meta` field, singletons (no label).
- Host functions callable from configuration; result filled in like a builtin; sensitive argument -> sensitive result; name clash error.
- Go plugins from a path, a plugin directory, the GitHub registry, with signature verification (tampered asset refused).
- Validate, diff, apply, load, destroy; variables.
- Lookups: entities, entity count, find (by address; class or generic), find by type, find one, all of a class, outputs. Unregistered types read generically (address + attributes by name).
- Encoding: encode a looked-up, hand-built or generic entity, and a saved record, with include computed / include empty / reveal sensitive / show references; byte-identical to Go; refused during an operation (saved-record encode allowed); errors for non-entity, variable, output, module, unencodable value, unregistered class.
- State directory or custom store (load/save/exists/clear); state masking (encryption of sensitive values).
- Events via handler with data levels NONE/RAW/PROCESSED and masking; event.entity() hydrates.
- One error class per error code with message, details, positions.
- Cancellation stops work in the engine.
- Engine: one per process, starts on first use, stops on last close or process exit, never orphaned; engine exit surfaces as an error and the next use starts fresh; engine stderr reaches the user; missing engine named; version mismatch refused.
- Release: package version equals xcl tag; xcl release dispatches an engine-bump to the SDK repo (xcl-python's engine-bump.yml pattern); xcl CI runs the SDK test suite (bindings.yml today calls xcl-python's test.yml).
- Tests use the xcl-plugin-test plugin, not Docker; parity fixtures compare against Go.
- README with install and getting started; doc comment on every public name; PARITY.md.

## From the user

- Separate epic, four specs; this one is the core SDK. Printing/highlighting/pretty logger = spec 2; Python SDK reference + guide = spec 3; TS SDK reference + guide + TS tabs on xcl.dev = spec 4.
- Entity types: first chose Zod, then rejected it at verification ("I really dislike that method of defining types with zod"; xcl types are "closer to orm entities"). User prefers decorated classes (Option 2); captured as a proposal in design typescript-entity-types.md for review with Erik.
- Node 22, 24, 26. Bun and Deno not promised.
- npm name `xcl-lang`; platform packages `xcl-lang-<os>-<arch>[-<libc>]` as optionalDependencies (esbuild pattern).
- Approved: Promise-only API (no sync variant), AbortSignal cancellation, ESM, npm provenance / trusted publishing, parity fixtures against Go.
- A design document `typescript-sdk.md` in source `design` with the API and code snippets; libxcl.md's Node section to be pointed at it.

## Node-specific behaviour (proposed by me, to confirm at sections)

- An idle engine must not keep the Node process alive: a program that forgets `close()` still exits.
- Ctrl-C (user chose option C): the SDK never listens for SIGINT unless asked; it ships an opt-in helper (e.g. `xcl.interruptSignal()`) returning an AbortSignal that fires on SIGINT and removes its listener when the operation ends. Without it, Node's default exit applies; the engine, on stdin EOF, releases configs and saves in-progress state (state directory; a custom store in the dead host gets nothing). Rejected: A (user wires it by hand, no helper), B (SDK auto-installs a listener during operations: a process-wide side effect).
- Works from worker threads and cluster workers (each gets its own engine; no fork(2) hazard in Node).
- Missing platform package (e.g. installed with `--omit=optional`) gives an error naming the package expected for this platform.
- camelCase in TypeScript, snake_case in configuration and on the wire, one conversion rule both ways (from libxcl.md).

## Ruled out

Sync API; TypeScript providers/plugins and a plugin SDK; codegen for plugin types; browsers and edge isolates; other schema libraries; a separate CommonJS build; the in-process shared library; printing/highlighting/pretty logger; xcl.dev docs.

## Cross-repo

- xcl-typescript: all SDK code (not yet in the spektacular registry; user cloned it).
- xcl: release.yml dispatches xcl-typescript as it does xcl-python; bindings CI runs the TS suite. No core-service change expected.
- xcl-plugin-test: used as-is by the TS tests.
- xcl-website: untouched here (spec 4).
