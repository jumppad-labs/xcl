- [ ] **Install from npm, no toolchain**
  Users can install xcl for TypeScript from npm on every supported platform without installing Go or any other toolchain, and without anything downloaded at install time or on first use.
- [ ] **A typed ES module**
  The package is imported as an ES module on every supported Node version and is fully typed for TypeScript.
- [ ] **Entity types defined in TypeScript**
  Developers can describe an entity type in TypeScript and register it with a type name and an optional subtype, which become the block's labels and address, as in Go.
- [ ] **Type definitions map to configuration**
  Optional fields, defaults, sensitive and computed fields, nested blocks, lists of blocks, named blocks collected by name, label fields, the block's metadata and singletons are all expressed in the type definition, and configuration is checked against it as it is for Go types.
- [ ] **TypeScript names, configuration names**
  Properties are camelCase in TypeScript and snake_case in configuration, converted by one rule in both directions.
- [ ] **TypeScript functions callable from configuration**
  Developers can register a TypeScript function, synchronous or asynchronous, and configuration can call it. The result is filled in exactly like a built-in function's. Registering a function under a name a built-in or another function already uses is refused.
- [ ] **Go plugins usable from TypeScript**
  A Node program can use Go-compiled plugins from a local path, a plugin directory or the GitHub registry, with release signatures verified as they are for Go programs.
- [ ] **Every operation available**
  A Node program can validate, diff, apply, load and destroy configuration, with the same results a Go program gets for the same configuration, plugins and state.
- [ ] **Variables passed in**
  A Node program can supply variable values when it creates a configuration.
- [ ] **Entities read back as typed values**
  A Node program can find entities by address, by type, exactly one of a type, all of a registered type, and every entity, and gets values of their registered type. Entities of types it hasn't registered, such as a Go plugin's types, are readable generically.
- [ ] **Outputs and diffs readable**
  A Node program can read a configuration's outputs, and a diff's changes, as JavaScript values.
- [ ] **Entities encoded as configuration**
  A Node program can turn a looked-up entity, a hand-built entity, a generic entity and a saved record into xcl configuration text, with the same four choices Go offers (include computed, include empty, reveal sensitive, show references), and gets the same text Go writes.
- [ ] **Encoding during an operation**
  Encoding a looked-up entity is refused while an operation is running on its configuration; a saved record can be encoded at any time, including from an event handler during an operation.
- [ ] **State location and custom stores**
  A Node program can keep state in a directory, or in its own store written in TypeScript whose methods may be asynchronous, and can have sensitive values encrypted in state.
- [ ] **Events delivered to TypeScript**
  A Node program can receive the engine's events through a handler, which may be asynchronous, with the same event data levels and masking a Go program has.
- [ ] **Errors as error classes**
  Failures are thrown as JavaScript errors, one class for each kind of error, each distinguishable by its class, carrying the message, the details and any configuration positions.
- [ ] **The event loop keeps running**
  The event loop keeps serving other work while an operation runs.
- [ ] **Cancellation stops the work**
  Aborting an operation through the platform's standard abort signal stops it in the engine, rather than leaving it running in the background, and keeps the state saved so far.
- [ ] **Ctrl-C cancels when asked**
  Users can opt in, per operation, to Ctrl-C cancelling that operation while keeping the state saved so far.
- [ ] **Ctrl-C without opting in**
  Without opting in, Ctrl-C ends the program as Node does by default, and the state of the operation in progress is still saved.
- [ ] **One engine per process**
  Every configuration in a process shares one engine, started on first use.
- [ ] **No engine left behind**
  No engine is left running after the last configuration is closed, after the process exits, or after the process is killed.
- [ ] **An idle engine never holds the process**
  A Node program that has finished its work exits, even if it never closed its configurations.
- [ ] **Engine failures are reported, not hidden**
  If the engine exits unexpectedly, every call waiting on it fails with an error carrying the exit status; the next use starts a fresh engine and the host process survives. The engine's own diagnostics reach the user's standard error.
- [ ] **Clear error when the engine is missing**
  If the engine package for the platform is not installed, the first use fails with an error naming the package expected for this platform and how to install it.
- [ ] **Engine version checked**
  The package refuses to run against an engine whose version doesn't match its own, with an error naming both versions.
- [ ] **Worker threads and cluster workers**
  Configurations work from worker threads and from cluster or child-process workers.
- [ ] **Releases kept in step**
  Every xcl release produces a TypeScript package of the same version, carrying the engine built from that release, without anyone publishing it by hand.
- [ ] **Parity breaks caught before release**
  A change to xcl, or to the package, that makes the package's results differ from Go's for the same configuration is caught before it is released.
- [ ] **Package documentation**
  The package ships a README covering install and getting started, a parity record against the Go API, and a doc comment on every public export.
