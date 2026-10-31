- [ ] **Clean install on every platform**
  On a fresh machine for each of the eight platforms, with no Go or C compiler, `npm install xcl-lang` succeeds without network access beyond the npm registry, and, with the network then disconnected, a script that imports the package as an ES module and creates and closes a configuration runs without error on Node 22, 24 and 26. The README's getting-started example type-checks in a strict TypeScript project, and a lookup's result has its registered type.
- [ ] **Alpine install and apply**
  In a fresh `node:22-alpine` image on x86-64 and on arm64, installing the package and running a program that registers a type, applies a configuration, reads the entity back and closes succeeds.
- [ ] **Registered type decodes config**
  With a type registered as `app`, `app "web" { image = "nginx" }` validates and applies, and finding `app.web` returns a value of that type with `image === "nginx"`. A type registered as `database` with subtype `postgres` decodes `database "postgres" "main" { … }` and is found at `database.postgres.main`.
- [ ] **Type rules enforced**
  For a type with a required field, an optional field with a default, a sensitive field, a computed field, a single nested block, a list of `volume` blocks and named `listener` blocks:
  - leaving out the required field fails validation at the block's position;
  - leaving out the optional field gives the default;
  - the sensitive value is masked in events and state;
  - the computed field can't be set in config;
  - the nested block is decoded;
  - two `volume` blocks give an array of two items in order;
  - two `listener "…"` blocks give a record keyed by their names.
- [ ] **Labels, metadata and singletons**
  A type with a label field receives the block's label in it; a type with a metadata field receives the block's address in it; a type with neither is a singleton, written without labels and found by its type alone.
- [ ] **Name conversion**
  A property `connectionString` is written `connection_string` in configuration, and the value read back is on `connectionString`.
- [ ] **Function called**
  With `slugify` registered, `name = slugify("My App")` applies and the entity holds `"my-app"`; with an async function registered, its resolved value is filled in. Registering a name a builtin uses fails with the name-clash error.
- [ ] **Go plugin from path, directory and registry**
  A Node program applies config whose entities are handled by the test plugin, loaded from a local path, from a plugin directory and from the GitHub registry. A tampered release asset is refused with a signature error, and an asset signed by an untrusted key is refused naming the key.
- [ ] **Same results as Go**
  For the shared fixture configurations, the TypeScript and Go programs produce the same validation errors, the same diff, the same entities and outputs after apply, and the same state file contents. A load in a new process reads back the same entities in both, and a destroy removes every entity and empties the state in both.
- [ ] **Variables supplied**
  Passing `region: "us"` when creating the configuration makes an entity that uses `variable.region` hold `"us"`.
- [ ] **Lookups return typed entities**
  For a configuration with two `database "postgres"` blocks, one `database "mysql"` block and one `app` block: finding by type `database`, `postgres` returns the two postgres entities; finding exactly one of type `app` returns the app; finding all of the registered postgres type returns the two postgres entities; and finding every entity returns all four plus any variables and outputs, each of its registered type.
- [ ] **Unregistered types readable**
  Entities of a Go plugin's type, with no type registered, are returned with their address and attributes readable by name.
- [ ] **Outputs and diff**
  After apply, outputs are returned as a plain object of JavaScript values. A diff after changing one attribute reports that attribute's old and new values.
- [ ] **Encoding matches Go**
  For each entity the parity fixtures cover (looked-up, hand-built, generic and saved record) and each combination of the four encoding choices, the TypeScript text is byte-for-byte equal to Go's encoder output. Encoding a non-entity, a variable, an output, a module, an unencodable value or a value of an unregistered type throws the matching error; encoding a looked-up entity while an operation runs throws the operation-in-progress error, and encoding a saved record from an event handler during the operation succeeds.
- [ ] **State directory and custom store**
  With a state directory set, an apply writes state there and a load in a new process reads it back. An object implementing the store methods, with async methods, receives the state on apply, and a later load in a new process reads it back. With state encryption enabled, sensitive values in what the store receives are encrypted.
- [ ] **Events received**
  An event handler receives an event for each entity's start and finish during an apply; with event data on, sensitive values in event data are redacted, and the event's entity can be read as a typed value. An async handler is awaited.
- [ ] **Typed errors**
  A missing address throws the not-found error, asking for exactly one entity of a type when several exist throws the not-unique error, and invalid config throws the config error, which lists each problem with its file and line. Each is an `instanceof` its own class and of the base xcl error class.
- [ ] **Event loop not blocked**
  While an apply runs a slow plugin operation, a timer on the same event loop keeps firing with a delay under 50 ms, measured by Node's event-loop delay monitor.
- [ ] **Cancellation**
  Aborting an apply's `AbortSignal` mid-way makes it reject with the cancellation error within two seconds of the abort, the entities completed before it are in state, and the test plugin records no operation started after the abort.
- [ ] **Ctrl-C with the interrupt signal**
  Sending SIGINT to a program mid-apply that passed the package's interrupt signal makes the apply reject with the cancellation error, the entities completed before it are in state, the program's own code after the rejection runs, and once the apply has ended the program has no SIGINT listener left from the package.
- [ ] **Ctrl-C without it**
  Importing the package and running an apply adds no SIGINT listener to the process. Sending SIGINT to a program mid-apply that did not pass the interrupt signal ends the program as Node does by default, the entities completed before it are in the state directory, and no engine process is left running.
- [ ] **One engine per process, cleaned up**
  Creating three configurations in one process starts one engine process; closing the last one leaves no engine process running; a script that never calls `close()` still exits when its work is done, leaving no engine running; killing the host process leaves no engine process running within five seconds. Each of these holds on all eight platforms.
- [ ] **Engine death is surfaced**
  Killing the engine process while an apply is in progress makes the apply reject with the engine-exited error carrying the signal or exit status; the host process continues, and a new configuration created afterwards works.
- [ ] **Engine diagnostics shown**
  Output the engine writes to its standard error appears on the host program's standard error.
- [ ] **Missing engine named**
  With the platform engine package removed, the first use throws an error naming that package and how to install it.
- [ ] **Version mismatch refused**
  Running the package against an engine of a different version throws an error naming both versions.
- [ ] **Workers**
  A configuration created inside a worker thread, one created inside a cluster worker and one created inside a child-process worker each apply successfully.
- [ ] **Release publishes matching package**
  An xcl release tag leads, with no manual publishing step, to `xcl-lang` and its eight platform packages being published to npm at the same version with provenance, and that version's engine reports the tag.
- [ ] **Parity breaks caught**
  A change to xcl runs the TypeScript package's tests, including the Go parity fixtures, against the engine built from that change, and a change that makes the TypeScript results differ from Go's fails that run.
- [ ] **Docs present**
  The npm page shows the README with install and a getting started that applies a configuration; the parity record lists every Go operation and lookup with its TypeScript equivalent; every public export has a doc comment.
