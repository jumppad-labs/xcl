---
created_date: "2026-10-31"
document_status: draft
spec: 20261031080136-264e0eab-typescript-core-sdk
specs:
    - 20261031080136-264e0eab-typescript-core-sdk
---

# xcl for TypeScript: the `xcl-lang` npm package

The TypeScript SDK brings xcl to Node the way the Python package brought it to Python: it starts the bundled `xcl-core` engine and speaks `CoreService` to it over stdio, so nothing the engine does is reimplemented. This document fixes the SDK's public API and how the package is laid out and shipped. How entity types are defined is proposed in [typescript-entity-types.md](typescript-entity-types.md): decorated entity classes, under review. The engine, the protocol, the framing and the process lifecycle are those of [libxcl.md](libxcl.md) and are not repeated here; printing and highlighting are the TypeScript sections of [printing-and-highlighting.md](printing-and-highlighting.md).

Repository `jumppad-labs/xcl-typescript`. Package `xcl-lang` on npm. Node 22 and later, ES module only, Promise-only. Examples import names from `xcl-lang` directly.

## At a glance

```ts
import {
  Attribute,
  createConfig,
  Entity,
  GitHubRegistry,
  Int,
  LocalRegistry,
  Meta,
} from "xcl-lang";

@Entity("settings")
class Settings {
  @Meta()
  meta!: Meta;

  @Attribute(String)
  image!: string;

  @Attribute(Int)
  replicas = 1;
}

const github = new GitHubRegistry({ trustedKeys: [RELEASE_KEY] });
github.registerPlugin("jumppad-labs/xcl-plugin-docker", "v0.3.0");

const local = new LocalRegistry();
local.register(Settings);

await using config = await createConfig({
  registries: [local, github],
  statePath: ".xcl",
});

await config.apply("./infra");

const settings = await config.find(Settings, "settings.web");
console.log(settings.image, settings.replicas);

const network = await config.find("docker.network.main");
console.log(network.meta.id, network.attributes.subnet, network.attributes.dockerId);

await config.destroy();
```

`await using` closes the configuration when the block ends (TypeScript 5.2+ compiles it for Node 22; Node 24 and later run it natively). `config.close()` does the same by hand. `find` with a class returns an instance of it; `find` with only an address returns a generic `Entity`.

## Entity types

Entity types are proposed in [typescript-entity-types.md](typescript-entity-types.md): a class per type, named by `@Entity(type, subtype?)`, with a decorator on each field (`@Attribute`, `@Sensitive`, `@Computed`, `@Block`, `@Blocks`, `@NamedBlocks`, `@Label`, `@Meta`). Defaults come from field initializers, and lookups return instances of the class. That document also covers the alternatives considered (an entity builder, schema files, Zod) and the questions still open.

Two rules from it matter to the rest of this design:

- **Names.** Properties are camelCase in TypeScript and snake_case in configuration, state and entity JSON, converted by one rule in both directions: an upper-case letter becomes `_` and its lower-case form, and `_x` becomes `X`. `connectionString` is `connection_string`; `dockerId` reads `docker_id`. A property name that would not survive the round trip is refused at registration with `xcl.SchemaError`. The same rule applies to generic entities' attributes, variables passed in, outputs read back and diff paths.
- **Sensitive values.** A sensitive field holds an `xcl.Sensitive<T>`, whose `toString()`, `toJSON()` and `util.inspect` form are `(sensitive)` and whose `reveal()` returns the value. A looked-up entity's sensitive fields hold a `Sensitive` with no value, so `reveal()` returns `undefined`; real values never leave the engine through a lookup, as in Python.

## Registries

```ts
const local = new LocalRegistry();
local.register(App, Postgres);
local.registerFunctions(new Functions());
local.registerExternalPlugin("./bin/xcl-plugin-test");
local.registerPluginDirectory("~/.xcl/plugins");

const github = new GitHubRegistry({ trustedKeys: [RELEASE_KEY] });
github.registerPlugin("jumppad-labs/xcl-plugin-docker", "v0.3.0");
```

Registries are plain builders, holding no engine state, as in Python; a configuration takes a list of them.

### Functions

A function's signature cannot be read from TypeScript at runtime, so it is declared with `@Func`, as typescript-entity-types.md proposes. The method name is the function's name in configuration, and the method may be synchronous or return a Promise:

```ts
export class Functions {
  @Func([String], String)
  slugify(title: string): string {
    return title.toLowerCase().replaceAll(" ", "-");
  }
}

local.registerFunctions(new Functions());
```

```hcl
app "web" {
  image  = "nginx:1.27-alpine"
  region = slugify("US East")
}
```

Parameters and results are strings, booleans, numbers, and arrays and records of those, as in Python. A name a builtin or another function uses throws `xcl.FunctionNameClashError` when the configuration is created. A result computed from a sensitive argument is held as sensitive.

## Creating a configuration

```ts
const config = await createConfig({
  registries: [local, github],
  variables: { region: "us" },
  statePath: "./.xcl",
  stateMask: mask.aes256gcm(key),
  onEvent: (event) => console.log(event.operation, event.phase, event.resourceId),
  eventData: EventDataLevel.Processed,
});
```

`variables` become `variable.<name>` in configuration. State goes to `statePath`, or to a custom `stateStore`, and `stateMask` encrypts sensitive values in it. `eventData` carries the entity on events, masked.

`createConfig` is asynchronous because it is the `CreateConfig` call: it starts the engine on first use, checks its version, derives every registered type's schema and fails there, rather than on the first operation, for a schema or function the engine refuses. `xcl.Config` is exported as a type only; there is no constructor to call.

### Custom state stores

Any object with these four methods, each of which may return a Promise:

```ts
interface StateStore {
  load(): Uint8Array[] | Promise<Uint8Array[]>;
  save(records: Uint8Array[]): void | Promise<void>;
  exists(): boolean | Promise<boolean>;
  clear(): void | Promise<void>;
}
```

Each record is one entity's JSON, as in Go and Python.

## Operations

Every operation returns a Promise and takes an optional `signal`:

```ts
await config.validate("./infra");
const diff = await config.diff(["./infra", "./extra"]);
await config.apply("./infra", { signal });
await config.load();
await config.destroy({ signal });
await config.close();
```

`paths` is a string or an array of strings. Operations on one configuration run one at a time, as in Go; starting a second while one runs throws `xcl.OperationInProgressError`.

### Cancellation

Aborting the `signal` sends `Cancel` to the engine. The engine stops the operation, saves the state of everything completed so far, and the Promise rejects with `xcl.CancelledError`, whose `cause` is the signal's `reason`. An already-aborted signal rejects at once, without starting the operation.

### Ctrl-C

The package never listens for SIGINT on its own. A program that wants Ctrl-C to cancel, as it does in Python, asks for it in one argument:

```ts
await config.apply("./infra", { signal: interruptSignal() });
```

`interruptSignal()` returns an `AbortSignal` that aborts on the next SIGINT. Its listener is added with `process.once` and removed when the operation it was passed to settles, so the program goes back to Node's default handling as soon as the operation ends, and a second Ctrl-C after the first exits as usual. Combined with a signal of the program's own through `AbortSignal.any`, the listener is removed when it fires or when the process exits.

Without it, Ctrl-C ends the program as Node does by default. The engine reads the end of its input, releases every configuration, which saves the state of an operation in progress to a state directory, and exits, so no engine is left running. A custom state store lives in the program that just ended and receives nothing more.

## Lookups

```ts
const web = await config.find(App, "app.web");
const network = await config.find("docker.network.main");
const databases = await config.findByType(Postgres, "database", "postgres");
const main = await config.findOne(Postgres, "database", "postgres");
const apps = await config.all(App);
const everything = await config.entities();
const count = await config.entityCount();
const outputs = await config.outputs();
```

With a class as the first argument, the result is an instance of that class, checked against its registration on this configuration; a class this configuration did not register throws `xcl.NotRegisteredError`. `findByType` and `all` return arrays of instances, `entities` returns `Entity` values, and `outputs` returns a plain object. With no class, the result is an `xcl.Entity`:

```ts
interface Entity {
  readonly meta: Meta;
  readonly attributes: Readonly<Record<string, unknown>>;
  as<T>(type: new () => T): T;
}
```

`attributes` has camelCase keys. `as` converts the entity to its registered class, or throws `xcl.NotRegisteredError`.

Errors: a missing address throws `xcl.NotFoundError`; `findOne` with several matches throws `xcl.NotUniqueError`; any lookup while an operation runs throws `xcl.OperationInProgressError`.

## Encoding entities as configuration

```ts
const web = await config.find(App, "app.web");

console.log(await config.encodeEntity(web));
console.log(await config.encodeEntity(web, { revealSensitive: true, includeComputed: true }));

const draft = new App();
draft.image = "nginx:1.27-alpine";
draft.meta = { name: "web" };

console.log(await config.encodeEntity(draft));
```

The four choices, all off by default, are those of Go's encoder and Python's keywords: `includeComputed`, `includeEmpty`, `revealSensitive` and `showReferences`. The engine renders the text, so it is byte-for-byte Go's.

Every entity is encoded on its own, because an instance's class names its type and a generic entity carries its metadata. An instance built by hand is encoded from its values, defaults included, and has no references to show. `encodeSavedEntity(record, options)` renders a saved record, as a state store holds it and events carry it. `encodeSavedEntity` works at any time, including from an `onEvent` handler while an operation runs; `encodeEntity` during an operation throws `xcl.OperationInProgressError`. Encoding a variable, an output, a module or a value with no configuration form throws `xcl.NotEncodableError`.

## Events

```ts
async function onEvent(event: Event): Promise<void> {
  console.log(event.operation, event.phase, event.resourceId);

  if (event.operation === "create" && event.phase === "success" && event.data) {
    console.log(await config.encodeSavedEntity(event.data, { includeComputed: true }));
  }
}
```

`event.entity(App)` returns the event's entity as an instance of `App`, or `undefined` when the event carries no data.

`xcl.Event` carries the fields of Go's `events.Event` in camelCase. `eventData` is `xcl.EventDataLevel.None` (the default), `Raw` or `Processed`, with the same masking as Go. A handler that returns a Promise is awaited before the engine's event task completes, as an async handler is in Python's asyncio API.

## Diffs

`config.diff()` returns an `xcl.Diff` with one `xcl.ResourceDiff` per resource, each holding `xcl.Change`s with `path`, `before`, `after`, `sensitive` and `unknown`. A value only known after apply is the sentinel `xcl.UNKNOWN`, a unique symbol, as in Python. `diff.summary` counts creates, updates, replacements and deletes.

## Errors

Every error the package throws is an `xcl.XclError`, with one subclass per error code, named as in Python:

```ts
try {
  await config.apply("./infra");
} catch (error) {
  if (error instanceof ConfigError) {
    for (const problem of error.problems) {
      console.error(`${problem.file}:${problem.line}: ${problem.summary}`);
    }
  } else if (!(error instanceof CancelledError)) {
    throw error;
  }
}
```

A `CancelledError` means the operation was stopped, with the state saved so far kept.

`XclError` has `code`, `message` (one line), `details` and `problems` (file, line, column, summary, detail, entity, field), all enumerable, so structured loggers record them without configuration. The engine's own failures are `xcl.EngineNotFoundError` (naming the platform package to install), `xcl.EngineVersionError` (naming both versions) and `xcl.EngineExitedError` (carrying the exit code or signal). Showing a problem's source lines, and `console.error(err)` printing the report, are part of printing, not this design.

## The engine

### Finding it

Each platform's `xcl-core` ships in its own package, listed in `xcl-lang`'s `optionalDependencies` with `os`, `cpu` and `libc` fields, the pattern esbuild uses for its executable:

| Package | Platform |
|---|---|
| `xcl-lang-linux-x64-gnu` | Linux x86-64, glibc |
| `xcl-lang-linux-x64-musl` | Linux x86-64, musl |
| `xcl-lang-linux-arm64-gnu` | Linux arm64, glibc |
| `xcl-lang-linux-arm64-musl` | Linux arm64, musl |
| `xcl-lang-darwin-x64` | macOS x86-64 |
| `xcl-lang-darwin-arm64` | macOS arm64 |
| `xcl-lang-win32-x64` | Windows x86-64 |
| `xcl-lang-win32-arm64` | Windows arm64 |

`xcl-lang` resolves the package for `process.platform` and `process.arch`, and on Linux for the C library the running Node was built against (glibc when Node's report names a glibc version, musl otherwise). A package manager that ignores the `libc` field installs both Linux packages; the C library check picks the right one. When none is installed (`--omit=optional`, `--no-optional`, a lockfile made on another platform), the first call throws `xcl.EngineNotFoundError` naming the package and the command that installs it. `XCL_CORE_PATH` overrides the lookup with a path to an `xcl-core` binary, for a source build of xcl and for the SDK's own tests; nothing else changes which engine runs, and nothing is downloaded.

### Running it

The transport follows libxcl.md's lifecycle with Node's tools:

- `child_process.spawn(xclCore, ["--serve"], { stdio: ["pipe", "pipe", "inherit"] })` on first use, one engine per process (per worker thread, since each has its own module graph), shared by every configuration created there.
- Frames are read on the event loop, so a `Poll` that waits for seconds blocks nothing.
- When no call is outstanding the child and its pipes are `unref`ed, so an idle engine never keeps Node alive; a script that forgets `close()` still exits, and the engine reads the end of its input and exits after it.
- The last `close()` ends the engine with SIGTERM and a bounded wait.
- If the engine exits with calls outstanding, each rejects with `xcl.EngineExitedError`; the next `createConfig` starts a fresh engine.
- Cluster workers and `child_process.fork` start new processes, each with its own engine; Node has no `fork(2)` hazard.

## Package layout

```
xcl-typescript/
  package.json          xcl-lang: "type": "module", "exports",
                        dependencies @bufbuild/protobuf, optionalDependencies xcl-lang-*
  src/
    index.ts            the public API
    config.ts           createConfig, Config, operations, lookups, encoding
    registry.ts         LocalRegistry, GitHubRegistry
    decorators.ts       Entity, Attribute, Block, Label, Meta, Func and the rest
    schema.ts           decorated classes -> schema v1
    hydrate.ts          entity JSON <-> instances; the name rule
    events.ts           Event, EventDataLevel
    diff.ts             Diff, ResourceDiff, Change, UNKNOWN
    errors.ts           XclError and one subclass per code
    mask.ts             state masks
    interrupt.ts        interruptSignal
    internal/
      transport.ts      Transport: call(handle, method, bytes) -> { status, bytes }
      subprocess.ts     finds, starts and frames xcl-core
      client.ts         typed client over the generated messages
      poll.ts           the poll loop; runs function, event and state tasks
      core/             generated from xcl's protos by protoc-gen-es (committed)
  npm/                  the eight platform package manifests; CI drops xcl-core into each
  test/
  PARITY.md
  README.md
```

The three layers are libxcl.md's: a byte transport that alone knows about the process, a typed client built from the generated `CoreService` descriptor, and the hand-written API. Tests below the API run against a fake transport with no engine.

## Releases

The npm version equals the xcl tag it bundles, as for `xcl-lang` on PyPI. xcl's release workflow dispatches an engine bump to `xcl-typescript` beside the one it sends `xcl-python`; the bump pull request pins the new tag, and when it merges the package is tagged and the nine packages are published with npm trusted publishing and provenance. Each platform package takes its `xcl-core` from the signed release assets of that xcl tag. xcl's bindings CI runs the TypeScript suite against the engine built from each change, as it runs Python's.

## Parity with Go and Python

| Go | Python | TypeScript |
|---|---|---|
| `xcl.New(options...)` | `xcl.Config(...)` | `await xcl.createConfig({...})` |
| `RegisterType[T](r, type, subtype)` | `local.register_type(App, "app")` | `@Entity("app")` and `local.register(App)` |
| `RegisterFunction` | `local.register_function(slugify)` | `@Func` and `local.registerFunctions(functions)` |
| `Validate` / `Apply` / `Diff` / `Load` / `Destroy` | `validate` / `apply` / `diff` / `load` / `destroy` | the same, awaited, with `{ signal }` |
| `...Context(ctx, ...)` | asyncio task cancellation | `AbortSignal` |
| `Find[T](c, address)` / `FindResource` | `find(cls, address)` / `find(None, address)` | `find(App, address)` / `find(address)` |
| `FindByType`, `FindOne`, `All`, `Entities`, `EntityCount`, `Outputs` | `find_by_type`, `find_one`, `all`, `entities`, `entity_count`, `outputs` | `findByType`, `findOne`, `all`, `entities`, `entityCount`, `outputs` |
| `EncodeEntity`, `EncodeSavedEntity` | `encode_entity`, `encode_saved_entity` | `encodeEntity`, `encodeSavedEntity` |
| `types.Sensitive[T]` | `xcl.Sensitive[T]` | `xcl.Sensitive<T>` |
| `WithEventHandler`, `WithEventData` | `on_event`, `event_data` | `onEvent`, `eventData` |
| `WithStateStore`, `WithStateMask` | `state_store`, `state_mask` | `stateStore`, `stateMask` |

`PARITY.md` in the package carries the full list and is checked at each release.

## Not in this design

- Printing errors, resources and diffs, themes, formatters and the pretty logger: the TypeScript sections of printing-and-highlighting.md, built by the next spec in the epic.
- Providers written in TypeScript, and TypeScript plugins served over plugin protocol v2.
- Generating entity classes from a plugin's types through `DescribeTypes`, the equivalent of the generator libxcl.md sketches for Python.
- A synchronous API, a CommonJS build, browsers and edge isolates.

## Open questions

- How entity types are defined: proposed in typescript-entity-types.md, under review.
- Whether the name rule should keep a way to name a configuration attribute that does not follow it (an escape hatch like Python's `xcl.field(name=...)`), or refuse such names outright. The draft refuses them.
- Whether `interruptSignal()` should also cover SIGTERM, for programs run under a process manager.
- Whether npm's handling of the `libc` field in `optionalDependencies` is reliable across the npm, pnpm, yarn and bun versions in use, or whether the C library check has to carry the selection on its own.
