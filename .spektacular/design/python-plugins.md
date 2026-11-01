---
created_date: "2026-10-31"
document_status: draft
---

# Python plugins

How a plugin is written in Python, run while it is being developed, and shipped so that any xcl application can use it, whether that application is written in Go, Python or TypeScript. A Python plugin is an ordinary xcl plugin. It speaks plugin protocol 2. A Python application can bake it in. Any other application starts it as a separate process, either from source or installed from a GitHub release laid out by the asset contract (`plugin-release-assets-v2.md`).

Baking in matters as much as shipping. It is how a plugin is developed and tested: breakpoints in providers, end-to-end tests as plain `pytest`, real tracebacks, and no process to rebuild or restart. It is also how a Python application gives its own types behaviour.

This is a sketch, so we can agree on how it feels before planning. The choices marked **proposal** are listed again under open questions.

## Assumptions

- **Protocol 2 runs over stdin/stdout.** The plugin protocol's messages travel as length-prefixed frames on the plugin's standard input and output, the framing `xcl-core` already uses, with request ids so several calls run at once. Callbacks (logging, state) share the same pipe. A separate transport design settles this, and it replaces the gRPC and `XCL_HOST_ADDRESS` version of protocol 2 in `libxcl.md`. That design also replaces `libxcl.md`'s `plugin.serve()` over `grpcio`: a Python plugin needs no gRPC server and no native dependency of its own.
- **Types travel as schema v1**, derived from the plugin's dataclasses exactly as the Python SDK already derives them for types an application registers.
- **Baked in is protocol 2 over a second pipe.** See "One protocol, two pipes" below. This replaces the provider tasks `libxcl.md` designs with messages of their own.
- **Packaging is the asset contract's.** A Python plugin ships as a multi-platform container image, and optionally as frozen executables for each platform. There is no Python-source package kind.

## A plugin's repository

```
xcl-plugin-postgres/
  pyproject.toml               depends on xcl (>= the first protocol 2 release) and the plugin's own libraries
  uv.lock
  src/xcl_plugin_postgres/
    __init__.py                plugin = build_plugin()
    __main__.py                plugin.serve()
    entities.py                block types: dataclasses, no behaviour
    providers/
      database.py              one provider per resource type
    client/
      postgres.py              narrow interface over the backend library
  tests/
  examples/basic/main.xcl
  Dockerfile
  .github/workflows/release.yml
```

This is the Python counterpart of `plugin-layout.md`:

- **Entities** hold no behaviour and never import providers or clients.
- **Providers** take their clients through their constructor.
- **Clients** are the only code that imports the backend library.
- **The template:** a Python template repository, beside `xcl-plugin-template`, starts new plugins in this layout with the Dockerfile and release workflow in place.

## Entities

A block type is a plain dataclass, under exactly the rules the Python SDK already applies to the types an application registers: required and optional fields, `xcl.Sensitive[T]`, `xcl.Computed[T]`, nested blocks, lists and maps of blocks, label fields and `meta`.

```python
# entities.py
from dataclasses import dataclass, field

import xcl


@dataclass
class Database:
    host: str
    owner: str
    port: int = 5432
    password: xcl.Sensitive[str] | None = None
    extensions: list[str] = field(default_factory=list)
    connection_string: xcl.Computed[str] = None
    meta: xcl.Meta | None = None
```

The plugin's types reach the host as schema v1 derived from these classes, so a Go application sees `database "postgres" "main" {}` exactly as it sees a Go plugin's type.

## Providers

A provider is a class extending `xp.Provider[T]`, with the lifecycle methods of Go's `ResourceProvider`. Each method may be a plain function or `async def`; see "Sync and async" below.

**Why a class.** This is the shape Python libraries use for a stateful object with several related operations that share setup: Pulumi's Python dynamic providers (`class MyProvider(ResourceProvider)` with `create`, `read`, `update`, `delete` and `diff`), Airflow operators, Django class-based views. Decorators on functions, as in Kopf or FastAPI, suit independent event handlers. A provider is several operations sharing one client, so a class holds the client once, and tests build it with a test double through its constructor, as `plugin-layout.md` asks of a Go provider. It is also a base class rather than a structural `Protocol`: the provider inherits the default `changed`, as a Go provider embeds `DefaultChanged`.

| Go | Python |
|---|---|
| struct implementing `ResourceProvider[T]` | class extending `xp.Provider[T]` |
| `var _ ResourceProvider[T] = (*p)(nil)` | the generic base class, checked by mypy or pyright |
| embedded `plugins.DefaultChanged[T]` | the default `changed` inherited from `xp.Provider` |
| `NewNetworkProvider(tasks)` constructor | `__init__(self, client)` |
| `Init(state, functions, log)` | optional `setup(self, ctx)` |
| `Create(ctx, n)`, `Read(ctx, old, new)`, ... | `create(self, ctx, n)`, `read(self, ctx, old, new)`, ... |
| `plugins.Logger(ctx).Info(...)` | `ctx.log.info(...)` |
| `RegisterResourceProvider(..., "docker", "network", &Network{}, provider)` | `plugin.register_type(Network, "docker", "network", provider=...)` |

```python
# providers/database.py
import xcl.plugin as xp

from ..client.postgres import Postgres
from ..entities import Database


class DatabaseProvider(xp.Provider[Database]):
    replace_on = ["host", "owner"]

    def __init__(self, client: Postgres):
        self.client = client

    async def setup(self, ctx: xp.Context) -> None:
        ctx.log.debug("provider ready")

    async def create(self, ctx: xp.Context, db: Database) -> Database:
        ctx.log.info("creating database", name=db.meta.name)
        await self.client.create_database(db.meta.name, db.owner, db.password.reveal())
        db.connection_string = self.client.dsn(db.meta.name)
        return db

    async def read(self, ctx: xp.Context, old: Database, new: Database) -> Database:
        if not await self.client.exists(old.meta.name):
            raise xp.NotFound()
        new.connection_string = old.connection_string
        return new

    async def update(
        self,
        ctx: xp.Context,
        db: Database,
        changes: list[xp.Change],
        dependencies: list[xp.DependencyChange],
    ) -> Database:
        for change in changes:
            if change.within("extensions"):
                await self.client.sync_extensions(db.meta.name, change.before, change.after)
        return db

    async def destroy(self, ctx: xp.Context, db: Database, *, force: bool) -> None:
        await self.client.drop_database(db.meta.name, force=force)
```

### What the methods mean

They mean exactly what the Go methods mean (`plugins/provider.go`):

- **`setup(ctx) -> None`** (optional) runs once when the plugin starts, before any other call, with `ctx.log` and `ctx.state`. It is Go's `Init`. It isn't called `init`, which would read as a slip for `__init__`. The constructor builds the provider, and `setup` is for work that needs the context, such as logging that it is ready.
- **`create(ctx, entity) -> entity`** creates the real resource and returns the entity with its computed fields set.
- **`read(ctx, old, new) -> entity`** reports the real resource. It returns `new` with identity and observed fields filled in from the real resource, and raises `xp.NotFound` when the resource is gone, so xcl creates it again. It never changes configured fields or the real world.
- **`changed(ctx, old, new, changes, dependencies) -> xp.ChangeKind`** answers `NO_CHANGE`, `UPDATE` or `REPLACE`. See below.
- **`update(ctx, entity, changes, dependencies) -> entity`** acts only on the changes it is given. It works out previous values from each change's `before` and never asks the backend what the resource was.
- **`destroy(ctx, entity, *, force) -> None`** removes the real resource. `force` is keyword-only.
- **`functions() -> list[Callable]`** (optional) lists the functions the provider offers to configuration. Each function's signature comes from its annotations, as for `LocalRegistry.register_function`.

### Deciding: `replace_on`

A provider lists the settings that can't change in place as `replace_on`, a class attribute of setting paths:

- a top-level setting is written by name (`"host"`);
- a nested setting as a tuple (`("network", "subnet")`).

The default `changed`, inherited from `xp.Provider`, then answers:

1. `REPLACE` when a change falls within a `replace_on` path, whether its new value is known yet or not;
2. otherwise `UPDATE` when the old and new entity differ, compared as Go's `DefaultChanged` compares them (ignoring `meta`, `depends_on` and `disabled`);
3. otherwise `NO_CHANGE`.

**It ignores dependencies**, exactly as Go's `DefaultChanged` does. What a replaced or updated dependency means is specific to each provider. The Docker container provider, for example, reattaches to a replaced network but is replaced when anything else it depends on is. A provider with rules like that defines `changed`, and defers to the base class for the settings:

```python
class ContainerProvider(xp.Provider[Container]):
    replace_on = ["image", "command", ("network", "ip_address")]

    async def changed(self, ctx, old, new, changes, dependencies):
        for dependency in dependencies:
            if dependency.kind is xp.REPLACE and not is_network(dependency.address):
                return xp.REPLACE
        return await super().changed(ctx, old, new, changes, dependencies)
```

The settings that need a replace stay listed in one place, and a hand-written `changed` holds only the rules beyond them. Go providers work the same way, through `DefaultChanged.ReplaceOn` (`plugin-layout.md`).

### Sync and async

A provider's methods may be plain functions or `async def`, mixed freely in one provider. Developers write whichever suits their backend library, and nothing has to be wrapped.

- **`async def` methods** run on the plugin's event loop, so calls for unrelated resources overlap as the engine's parallel walk expects.
- **Plain methods** run on a thread pool, so a blocking call to a synchronous library (`psycopg`, `boto3`, `requests`) doesn't hold up other resources. A provider's plain methods may be called for several resources at once, as Go providers are, so a provider keeps any state it shares safe for threads.
- **The same in both pipes.** Baked in or as a separate process, the dispatcher detects which kind each method is and runs it the same way.
- **Cancellation.** An `async` method is cancelled with `asyncio.CancelledError`. A plain method can't be interrupted, so it checks `ctx.cancelled` between steps when it has a long loop.

### Context, changes and errors

- **`ctx.log`** has `debug`, `info`, `warn` and `error`, with keyword arguments as structured details. A message becomes an event naming the resource, the step and the plugin, as a Go provider's `plugins.Logger(ctx)` does.
- **`ctx.state`** has `get(address)` and `find(pattern)`, the `plugins.State` lookups.
- **Cancellation.** See "Sync and async" above.
- **`xp.Change`** has:
  - `path`, a tuple of attribute names, indexes and keys;
  - `within(*path)`, true when the change is at or under that path, i.e. `change.within("network", "subnet")`. It is Go's `change.Within(...)`;
  - `before` and `after`, plain JSON values or `None` when absent;
  - `unknown`, true when the new value is only known once the apply runs. `after` is then `xp.UNKNOWN`, and this happens only in `changed`;
  - `sensitive`. A sensitive change holds real values and masks them when printed.
- **`xp.DependencyChange`** has `address` and `kind` (`UPDATE` or `REPLACE`).
- **Errors.** Any other exception fails the call with its message, as a Go provider's error does.

## The plugin

```python
# __init__.py
import xcl.plugin as xp

from .client.postgres import Postgres
from .entities import Database
from .providers.database import DatabaseProvider


def build_plugin() -> xp.Plugin:
    plugin = xp.Plugin("xcl-plugin-postgres")

    client = Postgres.from_env()   # built once; never connects here
    plugin.register_type(Database, "database", "postgres", provider=DatabaseProvider(client))
    plugin.register_type(Grant, "database", "grant")   # config-only: no provider

    return plugin


plugin = build_plugin()
```

```python
# __main__.py
from . import plugin

plugin.serve()
```

- **`register_type(cls, type, subtype=None, provider=None)`** follows `LocalRegistry.register_type`, plus the provider. A type registered without a provider is config-only. A mistake in a single call raises at the call, as registration does in Go.
- **`register_function(fn)`** offers a plugin-level function, for one that belongs to no provider.
- **`serve()`** serves the plugin over protocol 2 on stdin/stdout until the host closes the pipe, then returns. Standard output belongs to the protocol, so `serve()` sends `print` and stray writes to stderr, where the host captures them as the plugin's output.

## One protocol, two pipes

A plugin is hosted one way, protocol 2, which reaches it through one of two pipes:

```
                     ┌── stdin/stdout ──▶ serve() ──┐
engine (protocol 2) ─┤                              ├─▶ dispatcher ─▶ providers
                     └── Poll/Complete ─▶ tasks ────┘
```

- **Separate process.** The engine writes protocol 2 frames to the plugin's standard input and reads them from its standard output. `serve()` feeds them to the dispatcher.
- **Baked in.** The engine sends the same protocol 2 messages as a task through the operation's existing `Poll`/`Complete` loop, the loop that already carries function calls, state-store calls and events (`xcl/_poll.py`). The task holds a protocol 2 request, the SDK hands it to the same dispatcher, and the protocol 2 response goes back with `Complete`. Callbacks a provider makes (logging, state lookups) travel the same way in the other direction.

What follows from this:

- **One set of messages.** The core defines no provider messages of its own. A baked-in provider task carries a protocol 2 request and response, so a protocol change lands in one place.
- **One dispatcher in each SDK.** Decoding entities, calling the provider, encoding the result, turning exceptions into errors and `xp.NotFound` into not-found, logging and cancellation all live in the dispatcher. Each pipe only reads requests and writes responses.
- **One behaviour.** The plugin SDK's test suite runs over both pipes, and a plugin that passes behaves the same baked in and as a process.
- **The same concurrency.** Baked-in tasks run concurrently by default: `async` methods on an event loop, plain methods on a thread pool. A plugin's resources are worked on in parallel either way. The synchronous `xcl.Config` doesn't make baked-in providers run one at a time.
- **In the engine,** baked in is one more `PluginHost` that carries protocol 2 over the operation's task queue, next to the host that carries it over a process's stdin and stdout. Ordering, the decide-then-act sequence, saving state after a failure, masking and per-call logging stay in the parser, as they do for every plugin.

Baking in only works within one language. A Go application can't bake in a Python plugin, and a Python application can't bake in a Go one. Across languages, a plugin is always a separate process.

## Running a plugin

The same `Plugin` object is used three ways, and the provider code doesn't change between them.

### Baked in, from a Python application

```python
from xcl_plugin_postgres import plugin

local = xcl.LocalRegistry()
local.register_plugin(plugin)
```

Nothing is started and nothing is packaged. It is the counterpart of Go's `local.RegisterPlugin(&template.TemplatePlugin{})`, and the recommended way to develop and test a plugin.

### An application's own types

A Python application that gives its own type behaviour registers the type with a provider:

```python
local.register_type(App, "app", provider=AppProvider(api))
```

This is a one-type plugin baked in, using the same `xp.Provider` class, the same dispatcher and the same pipe. A type registered without a provider stays config-only, as today.

### As a separate process, from source

Any application, in any language, can run the plugin from its source tree. The same call in each SDK:

From a Go application:

```go
local.RegisterExternalPlugin("uv", "run", "--project", "../xcl-plugin-postgres", "-m", "xcl_plugin_postgres")
```

From a Python application:

```python
local.register_external_plugin(["uv", "run", "--project", "../xcl-plugin-postgres", "-m", "xcl_plugin_postgres"])
```

- **One feature, two spellings.** Go's `RegisterExternalPlugin` is the method on `registry.Local`. The Python SDK's `register_external_plugin` records the same entry and passes it to the engine, which starts it with the Go code. A plugin registered either way is started identically.
- **A command, not a path.** Both now take a command and its arguments instead of a single path to a binary, so a plugin runs from its source tree with whatever starts it. A compiled plugin is a command with no arguments, so registering a binary reads as it does today: `RegisterExternalPlugin("./bin/xcl-plugin-docker")`.
- **Only the command.** There is no working-directory option. Every tool that starts a plugin can be pointed at a directory itself (`uv run --project`, `go -C`, `npm --prefix`), and leaving the option out keeps the two signatures identical.

### Installed from a release

From a Go application:

```go
github.RegisterPlugin("acme/xcl-plugin-postgres", "v1.2.0")
```

From a Python application:

```python
github.register_plugin("acme/xcl-plugin-postgres", "v1.2.0")
```

The release offers a container, plus executables if the plugin publishes them. The registry chooses and starts it as the asset contract describes. The application can't tell the plugin is written in Python.

## Shipping a plugin

The template's release workflow builds and pushes the image for `linux/amd64` and `linux/arm64`, then writes and signs the index with the image's digest:

```dockerfile
FROM python:3.12-slim
COPY --from=ghcr.io/astral-sh/uv:0.5 /uv /bin/uv
WORKDIR /plugin
COPY . .
RUN uv sync --frozen --no-dev
ENTRYPOINT ["/plugin/.venv/bin/python", "-m", "xcl_plugin_postgres"]
```

- **Dependencies** are resolved from `uv.lock` when the image is built. Native libraries, such as `psycopg`'s `libpq`, are installed in the image, so nothing is installed on the machine that uses the plugin.
- **Frozen executables** (PyInstaller or Nuitka, one CI runner per OS) can be added to a release from the same code, for hosts without a container runtime or for plugins that must reach the host. The template leaves them out to begin with.

## Testing a plugin

- **Unit tests** build each provider with its constructor and test doubles of its client, and call its methods directly. They cover the `replace_on` and `changed` rules, and show that `update` makes only the calls its changes require.
- **End-to-end tests** bake the plugin into an `xcl.Config` and run as plain `pytest`, with breakpoints and tracebacks working. They apply the examples, confirm the next plan reports no changes, and destroy them, as Go plugins' `e2e/` does.
- **The released form.** The release workflow runs the same end-to-end tests once more against the built image, through `register_external_plugin(["docker", "run", "-i", "--rm", "<image>"])`, so the process pipe and the packaging are tested too.

## The SDK package

The plugin SDK is part of the `xcl` package, imported as `xcl.plugin`. There is no separate plugin package. Each language has one xcl package, which holds both the application API and the plugin API (`xcl` on PyPI, and the TypeScript SDK's single npm package likewise), so the project maintains one package for each language, not two.

- **One way to write entities.** Entity classes, `xcl.Sensitive`, `xcl.Computed`, `xcl.Meta`, `xcl.field` and schema v1 derivation are shared by applications and plugins without moving between packages.
- **Loose version requirements.** A plugin requires a minimum `xcl` version, the first release whose protocol it speaks (i.e. `xcl>=0.6`), never an exact one. A baked-in plugin shares its application's environment, and a loose minimum lets it bake into any application on a newer `xcl`.
- **The cost.** A plugin carries the `xcl-core` engine binary it never runs, about 20 MB in each image or frozen executable, and installs a wheel built for its platform. That is a size cost, not a correctness one, and it is accepted in exchange for one package per language.

## Typed access from an application

Entity classes are never published as a package of their own, and an application isn't told to install a plugin's package just to import its classes. Either would add packages to version and keep in step with every plugin.

An application reads a plugin's types one of two ways. Both work the same whatever language the plugin is written in:

- **Untyped:** `cfg.find(None, address)` returns an `xcl.Entity`, a read-only mapping with `.meta`, as it does for a Go plugin today.
- **Typed:** the application generates classes from the plugin's schema v1 with the SDK's generator (`python -m xcl generate`, see `libxcl.md`). The generated classes live in the application's own code and are regenerated when the application moves to a new plugin version.

A Python application that bakes a plugin in already imports its package, so it uses the plugin's own classes directly.

## Open questions

None at present. The decisions above were settled in review on 2026-10-31.
