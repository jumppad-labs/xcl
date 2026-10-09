# Plugin Developer Guide

A provider owns the lifecycle of one resource type. xcl decides *when* to
call it; the provider decides *what* each call means for the real resource.
This guide is about writing the provider side correctly.

The reference implementation is the example provider in
[`plugins/example/pkg/person/provider.go`](../plugins/example/pkg/person/provider.go),
with its resource type in
[`resource.go`](../plugins/example/pkg/person/resource.go) next to it. For how
providers are hosted and registered, see [Plugin Architecture](plugins.md).

## The contract

A provider implements `plugins.ResourceProvider[T]`
([`plugins/provider.go`](../plugins/provider.go)), where `T` is a pointer to
your resource struct:

```go
type ResourceProvider[T any] interface {
    Init(state State, functions ProviderFunctions, logger logger.Logger) error
    Create(ctx context.Context, resource T) (T, error)
    Read(ctx context.Context, old T, new T) (T, error)
    Changed(ctx context.Context, old T, new T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
    Update(ctx context.Context, resource T, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (T, error)
    Destroy(ctx context.Context, resource T, force bool) error
    Functions() ProviderFunctions
}
```

`Init` is called when the provider is registered, which happens when xcl
loads the plugin at the start of the first `Validate`, `Apply`, `Destroy` or
`Load`. For an in-process plugin that is once. An external plugin's process
runs only while an operation is using it, xcl starts it for each operation
and stops it when the operation is done, so its `Init` runs again for every
operation and the provider keeps nothing in memory between them. Use it to keep the state and functions it is given and to set up
any clients. The logger it is given is plugin scoped: it is for messages
written outside a provider call, such as in `Init` itself. Don't keep it for
the lifecycle methods; log from those through the call's context (see
[Logging from a provider](#logging-from-a-provider)).
`Functions` returns the functions the provider exposes to other providers.
The rest of this guide is about the lifecycle methods.

## The two copies of a resource

Every call on a resource that already exists works with two copies of it:

- **`old`**: the resource as saved to state by the last apply. It holds
  everything the provider returned last time, including identity (IDs
  assigned at create time) and observed values.
- **`new`**: the resource decoded from the current configuration. It holds
  what the user wrote, plus the computed values xcl carried over from `old`
  (see [Computed fields](#computed-fields)). Observed and derived values are
  empty until `Read` fills them in.

Keep these straight and the rest follows.

`entity` is the package `github.com/jumppad-labs/xcl/entity`
([`entity/change.go`](../entity/change.go)). It holds the answer `Changed`
gives, `entity.Change`, and the two lists `Changed` and `Update` are given:
the settings that changed, `[]entity.PropertyChange`
([`entity/property_change.go`](../entity/property_change.go)), and the
dependencies that change with the resource, `[]entity.DependencyChange`; see
[`Changed`](#changedctx-old-new-changes-dependencies-entitychange-error).

## Kinds of field

| Kind | Example | Who sets it | Where it comes from on the next apply |
|---|---|---|---|
| Config | `image`, `path`, `port` | the user, in HCL | the configuration |
| Identity | container ID, cloud ARN | the provider, in `Create` | carried over from `old` (a computed field) |
| Observed | `running`, `ip_address` | the provider, in `Create`/`Read`/`Update` | `Read` writes it into `new` |
| Derived | a file's `checksum` | the provider, in `Create`/`Read`/`Update` | `Read` writes it into `new` |

Identity fields are why `Read` gets `old`: you can't always find the real
resource from configuration alone. Any field the user must not set, identity
in particular, should be marked computed.

## Sensitive fields

A field holding a password, token or other secret is declared with the type
`types.Sensitive[T]`:

```go
type Database struct {
    types.ResourceBase `xcl:",remain"`

    Username string                  `xcl:"username" json:"username"`
    Password types.Sensitive[string] `xcl:"password" json:"password"`
}
```

Users set it exactly as they set any other field. xcl shows it only as
`(sensitive)` in logs, events, errors and printed output, and keeps the real
value in state. Your provider receives the real value and returns it unchanged
unless you change it, and a change to it is detected like any other.

A plugin type is rebuilt on the host from its schema, and Go cannot build a
generic type at run time, so a plugin type may use only these instantiations:
`types.Sensitive[string]`, `[int]`, `[int64]`, `[float64]`, `[bool]`,
`[[]string]` and `[map[string]string]`. A plugin type with any other fails to
load, with an error naming the type and the field. Rebuild an external plugin
against the version of xcl it is loaded by, so its type names match the
host's.

Read the real value with `Reveal()`, only where it is needed, such as when
connecting to the service the provider manages:

```go
func (p *DatabaseProvider) Create(ctx context.Context, db *Database) (*Database, error) {
    conn, err := p.client.Connect(db.Username, db.Password.Reveal())
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    // ...
    return db, nil
}
```

Once you call `Reveal()`, the value is an ordinary value: it is no longer
protected and must not be logged or otherwise emitted. Do not pass it to
`plugins.Logger(ctx)`, put it in an error, or copy it into a field that is not
sensitive. Passing the sensitive value itself is safe: logging `db.Password` or
`db` writes `(sensitive)`.

## The lifecycle

Every apply runs in two halves
([`internal/parser/parser.go`](../internal/parser/parser.go),
[`internal/parser/lifecycle.go`](../internal/parser/lifecycle.go)). First xcl
**decides** what happens to every resource, without creating, updating or
destroying anything. Then it **acts** on those decisions: it destroys, then
creates and updates. A plan (`Config.Diff`) runs exactly the same decide pass
and stops there, so a plan and the apply that follows it agree.

### Decide

xcl walks the resources in dependency order, so every resource is decided
after everything it depends on. For each one it looks up the resource's entry
in the state saved by the last apply:

```
not in the previous state         -> create

saved as failed or destroy_failed -> replace (its last apply failed)
                                     no provider call

saved as created or updated:
    carry computed values from old onto new
    result, err = Read(old, new)
        ErrNotFound -> reset new to the configured copy -> create
        other error -> the apply fails, nothing has been changed
    changes = the settings that differ, saved copy against configured copy
    change, err = Changed(old, result, changes, dependencies)
        error           -> the apply fails, nothing has been changed
        entity.NoChange -> unchanged
        entity.Update   -> update
        entity.Replace  -> replace
```

`changes` lists the settings that differ between the saved copy and the
configuration, worked out by the same comparison the plan shows, before `Read`
is called. `dependencies` lists the resources this one depends on that the
same apply will update or replace; see
[`Changed`](#changedctx-old-new-changes-dependencies-entitychange-error). A
status xcl does not recognise is treated like `failed`.

A resource whose configuration uses a value that is only known once the apply
has run, such as a computed field of a dependency being created, updated or
replaced, is still read and asked: xcl puts the saved value in place of the
unknown one, and lists the setting in `changes` with `Unknown` set. Its
inputs are about to change, so it is decided at least an update even when
`Changed` answers `entity.NoChange`; `Changed` can still answer
`entity.Replace`.

Every `Read` and `Changed` call of the apply happens in this half. An error
from either fails the apply before anything is created, updated or destroyed:
no resource is marked failed and the saved state is left as it was.

### Act: destroy

Every resource decided replace, and every resource that is in the previous
state but no longer in the configuration, is destroyed with its saved copy,
before anything is created or updated. Destroys run dependents first, the
reverse of dependency order, and the state is saved after each one, so a
replaced network is destroyed after the container attached to it.

```
Destroy(old)
    error -> keep old, status destroy_failed, the apply stops
```

If a `Destroy` fails, the saved copy is kept, because it holds the identity
needed to try again, the resource is saved as `destroy_failed`, and the apply
stops without creating or changing anything. The next apply decides it
replace again (or removes it again, if its block is gone) and tries the
destroy first.

### Act: create and update

xcl then walks the resources in dependency order again, decoding each with
real values as its dependencies are created and updated, and follows its
decision:

```
create or replace -> Create(new)                          -> status created
update            -> changes = the comparison run again, with real values
                     Update(new, changes, dependencies)   -> status updated
unchanged         -> keep what Read returned and the previous status
```

A replaced resource was destroyed in the previous phase, so it is created like
a new one. `Update` gets the configuration as decoded now, with the computed
values `Read` returned while deciding. Its `changes` come from the plan's
comparison run again now that every value is known, and its `dependencies`
are the list recorded while deciding, the same list `Changed` was given; see
[`Update`](#updatectx-resource-changes-dependencies-t-error). Nothing is
called for an unchanged resource: what `Read` returned is saved and the
status stays as it was (`created` or `updated`).

When `Read` reports `ErrNotFound`, the computed values carried over from
`old` are dropped along with the real resource: `Create` gets the resource as
configured.

### Builtin types

`variable`, `output` and `module` resources have no provider. xcl handles
them itself and makes no provider calls for them.

## Methods

### `Create(ctx, resource) (T, error)`

Called when the resource is not in the previous state, when `Read` returned
`ErrNotFound`, and for a replaced resource, after its `Destroy`.

- **Input**: the resource as configured. Computed fields are empty.
- **May change**: computed, observed and derived fields.
- **Returns**: the resource with identity, observed and derived fields set.
  This is what is saved and what `old` will be next time.

### `Read(ctx, old, new) (T, error)`

Called only for resources saved as `created` or `updated`, so `old` is never
nil. Look up the real resource and fill in `new` from it.

- **Input**: `old` is the saved copy; use its identity fields to find the
  real resource. `new` is the configured copy with the saved computed values
  already carried over.
- **May change**: identity, observed and derived fields on `new`.
- **Returns**: `new`, filled in. This goes to `Changed`. Its computed values
  go to `Update` if `Changed` answers update, and it is saved as is if
  `Changed` answers no change.

The rules:

- Never change a configured field. If you overwrite `image` with what is
  actually running, the user's change is lost.
- Never record values that change on their own (see the callout below).
- Never change the real resource. `Read` only looks: it must not create,
  change or remove anything.
- Return `plugins.ErrNotFound` when the real resource no longer exists. xcl
  creates it again.
- Any other error fails the apply before anything is changed. xcl does not
  guess.

> [!IMPORTANT]
> **Never record values that change on their own in `Read`.** Uptime,
> last-seen timestamps, counters and anything else that moves without the
> resource changing will differ from the saved copy on every apply. `Changed`
> then reports a change and xcl calls `Update` on every apply, forever.
> Record only values that change when the resource really changes.

### `Changed(ctx, old, new, changes, dependencies) (entity.Change, error)`

Decides what applying the configuration needs for the resource. `old` is the
saved copy and `new` is what `Read` returned, so it holds both configuration
edits and drift in the real resource. `changes` lists the settings that
changed and `dependencies` the dependencies that change with it. It must not
change anything.

It answers one of three `entity.Change` values:

| Answer | What the apply does |
|---|---|
| `entity.NoChange` | leaves the resource as it is; nothing is called |
| `entity.Update` | calls `Update` to change the resource in place |
| `entity.Replace` | calls `Destroy` with the saved copy, then `Create` |

Most providers should not write this. Embed `plugins.DefaultChanged[T]`
([`plugins/changed.go`](../plugins/changed.go)):

```go
type ContainerProvider struct {
    plugins.DefaultChanged[*Container]
    // ...
}
```

`DefaultChanged` compares the JSON form of both copies, ignoring xcl's own
resource metadata (`meta`, `depends_on` and `disabled`). It answers
`entity.Update` when they differ and `entity.NoChange` when they don't. It
ignores both `changes` and `dependencies` and never answers `entity.Replace`.
Because `Read` has put reality into `new`, one comparison catches both kinds
of change:

| | `old` (last applied) | `new` (config + Read) | result |
|---|---|---|---|
| nothing changed | `running=true` | `running=true` | `entity.NoChange` |
| container stopped | `running=true` | `running=false` | `entity.Update` |
| config changed | `image=a` | `image=b` | `entity.Update` |
| file contents changed | `checksum=x` | `checksum=y` | `entity.Update` |

#### The changed settings

`changes` lists the settings of the resource that differ between the last
apply and the configuration, each as an `entity.PropertyChange`
([`entity/property_change.go`](../entity/property_change.go)):

```go
type PropertyChange struct {
    Path      Path // where the setting is, for example network[0].name
    Before    any  // the previous value, nil when the setting was added
    After     any  // the new value, nil when it was removed or is Unknown
    Unknown   bool // the new value is only known once the apply runs
    Sensitive bool // the setting is sensitive; Before and After are still real
}
```

- **The plan's own list.** xcl works the list out with the same comparison of
  the saved copy against the configured copy that the plan shows, so a
  provider is told exactly the settings the plan lists, with the same paths.
  It describes the configuration: drift that `Read` finds in the real
  resource is not in it, it is in the difference between `old` and `new`.
- **`Unknown` only while deciding.** When `Changed` is called, a new value
  that is only known once the apply runs, such as a computed field of a
  dependency that is being created or replaced, has `Unknown` set and a nil
  `After`. By the time `Update` runs, every value is real.
- **Real values at `Update`.** `Update` is given the comparison run again
  with every value known, including values produced earlier in the same
  apply. A setting that was unknown while deciding stays listed with its real
  value, even when it turns out the same as before, so the list always
  matches what the plan showed.
- **Sensitive values are real.** A sensitive setting's change holds its real
  values in `Before` and `After`, so the provider can act on them. The change
  masks them itself: printing it, logging it or marshalling it to JSON shows
  `(sensitive)` in their place. Once you take a value out of the change, treat
  it as you would the result of `Reveal()`; see
  [Sensitive fields](#sensitive-fields).
- **Plain JSON values.** `Before` and `After` hold `string`, `float64`,
  `bool`, `nil`, `[]any` or `map[string]any`, never your Go types: a number
  is a `float64` and a nested block is a `map[string]any`. They are the same
  whether the plugin runs in-process or as a separate program.
- **Lists by position, maps by key.** A list setting is compared element by
  element at its position, so a change is at `network[0].name`, `command[2]`
  or `network[0].aliases[1]`. A map entry is at its quoted key, such as
  `environment["LOG_LEVEL"]`. An added or removed list element or map key is
  one change holding the whole value, with a nil `Before` when it was added
  and a nil `After` when it was removed.

A `Path` ([`entity/path.go`](../entity/path.go)) is a list of steps, built
with `Attribute`, `Index` and `Key`. Match a change with `At`, for a change
exactly at a path, or `Within`, for a change at a path or anywhere inside it:

```go
networks := entity.Path{}.Attribute("network")

for _, change := range changes {
    if change.Within(networks) {
        // network, network[0], network[0].name, network[1].aliases[0], ...
    }
}
```

An empty list means none of the resource's settings changed.

#### The dependency list

`dependencies` lists the resources this one depends on that the same apply
will update or replace, sorted by address, each as an
`entity.DependencyChange`:

```go
type DependencyChange struct {
    Address string        // for example "docker.network.app"
    Change  entity.Change // entity.Update or entity.Replace, never entity.NoChange
}
```

- **Direct only.** A resource depends on what it references and what it names
  in `depends_on`. The dependencies of a dependency are not passed on: a
  resource is told about what it depends on, not about what that depends on.
- **Update and replace only.** A dependency that is left unchanged is not
  listed, and neither is one being created for the first time; a resource
  that starts referencing a new resource has changed configuration anyway.
- **Through outputs, variables and modules.** Resources without a provider,
  such as an `output`, a `variable`, a `module` or a registered config-only
  type, are looked through to the provider-backed resources behind them. A
  resource inside a module is told about the changing resources the module
  block's `variables` reference.

An empty list means nothing this resource depends on is changing. `Update` is
given the same list.

#### Overriding `Changed`

Define `Changed` on your provider when a plain comparison is wrong for your
type: a setting the real system cannot change in place, a dependency whose
replacement leaves the resource broken, or two values that differ as text
but mean the same thing. Answer what you know and defer to the embedded
`DefaultChanged` for the rest. The Docker container in the plugin example
([`example/plugin/plugins/docker/resources/container.go`](../example/plugin/plugins/docker/resources/container.go))
does both, deciding only from what it is told:

```go
// replaceSettings are the settings Docker fixes when it creates a container,
// a change to any of them needs a new container
var replaceSettings = []entity.Path{
	entity.Path{}.Attribute("image"),
	entity.Path{}.Attribute("command"),
	entity.Path{}.Attribute("environment"),
	entity.Path{}.Attribute("init_script"),
}

func (p *containerProvider) Changed(ctx context.Context, old *Container, new *Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	for _, change := range changes {
		for _, setting := range replaceSettings {
			if change.Within(setting) {
				return entity.Replace, nil
			}
		}
	}

	for _, dependency := range dependencies {
		if dependency.Change == entity.Replace && !isNetworkAddress(dependency.Address) {
			return entity.Replace, nil
		}
	}

	if len(changes) > 0 {
		return entity.Update, nil
	}

	for _, dependency := range dependencies {
		if isNetworkAddress(dependency.Address) {
			return entity.Update, nil
		}
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}
```

Docker fixes a container's image, command, environment and mounts when it
creates it, so a change within `image`, `command`, `environment` or
`init_script` is a replacement. So is a replaced dependency that isn't a
Docker network: the init script is usually `template.init.destination`, and
when that template is replaced the container that mounts it is replaced too.
Any other change to its own settings, such as a `network` block added,
removed or given new aliases, answers update, and so does a network it
depends on being updated or replaced: `Update` moves the running container
between networks. A dependency that is only updated, such as the init
template rendering new content to the same file, is left to `DefaultChanged`,
which leaves the container alone.

The network it is attached to
([`network.go`](../example/plugin/plugins/docker/resources/network.go))
cannot move to a new address range in place, so it answers replace when its
subnet changes:

```go
func (p *networkProvider) Changed(ctx context.Context, old *Network, new *Network, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error) {
	if old.Subnet != new.Subnet {
		return entity.Replace, nil
	}

	return p.DefaultChanged.Changed(ctx, old, new, changes, dependencies)
}
```

Changing the subnet therefore replaces the network and updates the container,
which `Update` attaches to the new network, keeping the same container. The
plan names why the container is updated:

```
# docker.container.web will be updated because docker.network.app is replaced
~ docker "container" "web" {}
```

See [the plugin example](../README.md#plugins).

If you find yourself computing things in `Changed`, move that work into
`Read`. An error from `Changed` fails the apply before anything is changed.

### `Update(ctx, resource, changes, dependencies) (T, error)`

Called only when `Changed` answered `entity.Update`. It is never called for
`entity.NoChange` or `entity.Replace`.

- **Input**: the resource as configured now, with the computed values `Read`
  returned while deciding. `changes` lists the settings that changed since
  the last apply, with every value real; see
  [The changed settings](#the-changed-settings). `dependencies` is exactly the
  list `Changed` was given.
- **May change**: computed, observed and derived fields.
- **Returns**: the resource with observed and derived fields set to the new
  reality (`running=true` after a restart), so that the next `Read` sees no
  difference. This is what is saved. The provider is authoritative over its
  computed values: one it leaves out of its result, such as an `omitempty`
  address it cleared, is cleared, not kept from the last apply. The same holds
  for `Create` and `Read`.

`resource` is the configuration as it is now; `changes` is how it got there.
Put each change's `Before` back to rebuild what the resource looked like
after the last apply, without asking the real system. The Docker container
does this to move a running container between networks in place. It rebuilds
the previous `network` blocks from the changes within `network`
([`attachments.go`](../example/plugin/plugins/docker/resources/attachments.go)),
disconnects the networks that went away or whose aliases changed, connects
the new ones, connects again the networks whose dependency was replaced, and
reads the new address once if anything moved:

```go
func (p *containerProvider) Update(ctx context.Context, c *Container, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Container, error) {
	previous, err := previousAttachments(c.Networks, changes)
	if err != nil {
		return nil, fmt.Errorf("unable to work out the previous networks of container %s: %w", c.Meta.Name, err)
	}

	moved := false

	for _, attachment := range previous {
		current, ok := findAttachment(c.Networks, attachment.Name)
		if ok && equalStrings(current.Aliases, attachment.Aliases) {
			continue
		}

		// a network that was removed or rebuilt has already detached it
		err := p.client.NetworkDisconnect(ctx, attachment.Name, c.DockerID, true)
		if err != nil && !dockerclient.IsErrNotFound(err) {
			return nil, fmt.Errorf("unable to disconnect container %s from network %s: %w", c.Meta.Name, attachment.Name, err)
		}

		plugins.Logger(ctx).Info("disconnected container from network", "name", c.Meta.Name, "network", attachment.Name)
		moved = true
	}

	connected := map[string]bool{}
	for _, attachment := range c.Networks {
		before, ok := findAttachment(previous, attachment.Name)
		if ok && equalStrings(before.Aliases, attachment.Aliases) {
			continue
		}

		if err := p.connect(ctx, c, attachment); err != nil {
			return nil, err
		}

		connected[attachment.Name] = true
		moved = true
	}

	for _, dependency := range dependencies {
		if dependency.Change != entity.Replace || !isNetworkAddress(dependency.Address) {
			continue
		}

		attachment, ok := findAttachment(c.Networks, networkName(dependency.Address))
		if !ok || connected[attachment.Name] {
			continue
		}

		if err := p.connect(ctx, c, attachment); err != nil {
			return nil, err
		}

		connected[attachment.Name] = true
		moved = true
	}

	if !moved {
		return c, nil
	}

	inspect, err := p.client.ContainerInspect(ctx, c.DockerID)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect container %s: %w", c.Meta.Name, err)
	}

	c.IPAddress = firstAddress(c, inspect)

	return c, nil
}
```

The hot swap has a companion in the network's `Destroy`
([`network.go`](../example/plugin/plugins/docker/resources/network.go)).
Docker refuses to remove a network that still has containers attached, and a
replaced network is destroyed before its dependents are updated, so `Destroy`
inspects the network and force-disconnects every attached container before
removing it. The container's `Update` then attaches it to the network that
replaces it, which is why it connects again to every replaced network.

The init script is the other half of the container's rules. A replaced
`template.init` dependency, for example because its destination moved,
replaces the container, since the script is mounted when Docker creates it.
An updated one, which renders new content to the same file, leaves the
container alone.

### `Destroy(ctx, resource, force) error`

Removes the real resource. It is called:

- by `Config.Destroy`, for every resource in the saved state;
- during an apply, for a resource that was removed from the configuration;
- during an apply, for a resource decided replace: one `Changed` answered
  `entity.Replace` for, or one saved as `failed` or `destroy_failed`.

- **Input**: always the saved copy, never the configured one. After a failed
  `Create`, the saved copy may hold no identity at all, because `Create`
  never returned one.
- **Returns**: only an error. `force` asks for a quick destroy that doesn't
  wait for graceful shutdown; xcl always passes `false` today.

Whether destroying a resource that no longer exists is an error is your
decision. Any error you return fails the destroy, and the resource is saved
as `destroy_failed`: a replacement does not create it again, and `Config.Destroy`
keeps it, and everything it depends on, in the state for the next attempt.
Treating "already gone" as success is usually right, since xcl only needs
the resource to be gone.

## Computed fields

A computed field is owned by the provider. Mark it with the `computed`
option in an `xcl` struct tag, and make it optional in its `hcl` tag:

```go
PersonID string `xcl:"person_id,optional,computed" json:"person_id,omitempty"`
```

The option lives in its own `xcl` tag because `gohcl` rejects options it
does not know in the `hcl` tag.

What xcl does with computed fields
([`internal/parser/computed.go`](../internal/parser/computed.go)):

- **Users can't set them.** Setting a computed field in configuration is a
  validation error naming the resource and the field path, for example
  `resource 'resource.container.web' sets computed field 'network.assigned_address'`.
  It is reported before any provider is called.
- **They must be optional.** A computed field without `xcl:",optional"` is a
  validation error, since no configuration could satisfy it.
- **Saved values are carried over.** Before `Read`, xcl copies the computed
  values from the saved copy onto the configured copy. This works at any
  depth: nested blocks, pointer blocks, and lists and maps of blocks.
- **They survive unchanged applies.** Because the values are carried over,
  a provider whose `Read` adds nothing still keeps them, and `Changed` sees
  the same values on both copies.
- **The provider's result is authoritative.** Before xcl decodes what
  `Create`, `Read` or `Update` returned, it clears the resource's computed
  values, so a computed value the provider leaves out of its result, such as
  an `omitempty` address it cleared, is cleared rather than kept from the
  last apply. Return every computed value you want kept.
- **Dependents can reference them.** Other resources can use a computed
  value, such as `resource.container.web.container_id`, and see the value
  the provider set.

### Pairing list elements with `key`

For a list of blocks, xcl has to decide which saved element goes with which
configured element. Mark the fields that identify an element with
`xcl:"key"`. Elements are paired by the values of their key fields; when the
element type has no key fields, they are paired by position. Map elements are
paired by map key. An element present on only one side gets nothing carried
over.

An example, a container with network attachments whose address is assigned
by the network:

```go
type Container struct {
    types.ResourceBase `xcl:",remain"`

    Image string `xcl:"image" json:"image"`

    // set by the provider in Create
    ContainerID string `xcl:"container_id,optional,computed" json:"container_id,omitempty"`

    Networks []NetworkAttachment `xcl:"network,block" json:"networks,omitempty"`
}

type NetworkAttachment struct {
    // identifies the attachment, so saved and configured elements pair up
    // even when the user reorders the blocks
    Name string `xcl:"name,key" json:"name"`

    // set by the provider when the container joins the network
    AssignedAddress string `xcl:"assigned_address,optional,computed" json:"assigned_address,omitempty"`
}
```

Without the key, reordering the `network` blocks would carry each address
onto the wrong attachment.

## Don't change configured values

`Create`, `Read` and `Update` must only change computed, observed and
derived fields. After each of these calls, xcl compares what went in with
what came back
([`internal/parser/configured_check.go`](../internal/parser/configured_check.go)).
For every non-computed field the provider changed, it emits a warn log event
to the application's event receiver: `Phase` `log`, `Source` `core`, the
resource in `ResourceID`, the call in `Operation` (`create`, `read` or
`update`), and in `Meta` the level `warn`, the message
`provider changed a configured value` and the field path under `field`.
Written by `events.SlogHandler`, it reads:

```
level=WARN msg="provider changed a configured value" source=core operation=create phase=log resource=resource.container.web type=container.web file=... field=image
```

The apply continues. Fields whose configuration references another resource
or a module are not reported, since their value comes from elsewhere.

The warning points at a real problem. The changed value is what gets saved,
so on the next apply the saved copy no longer matches the configuration,
`Changed` reports a change, and xcl calls `Update`. A provider that rewrites
a configured value causes an update on every apply.

## Errors

### `ErrNotFound`

`plugins.ErrNotFound` ([`plugins/errors.go`](../plugins/errors.go)) is the
only error with a special meaning, and only from `Read`. xcl checks for it
with `errors.Is`, so you can wrap it:

```go
return nil, fmt.Errorf("container %s: %w", old.ContainerID, plugins.ErrNotFound)
```

It keeps its meaning when the provider runs in a separate plugin process.
Error values don't survive gRPC, so `ReadResponse` carries a `not_found`
field ([`plugins/plugin.proto`](../plugins/plugin.proto)) and the host turns
it back into `ErrNotFound`.

### Any other error

An error from `Read` or `Changed` comes while the apply is still deciding. It
fails the apply before anything is created, updated or destroyed: no resource
is marked failed and nothing is saved, so the next apply decides again from
the same state.

An error from `Create` or `Update` marks the resource `failed`, and one from
`Destroy` marks it `destroy_failed`; either fails the apply. On the next apply
a resource saved as either is replaced, without asking its provider:
destroyed with its saved copy, then created.

### What happens to the rest of the apply

A failed `Destroy` stops the apply before anything is created or changed; see
[Act: destroy](#act-destroy). After a failed `Create` or `Update`:

- Resources that depend on the failed resource are skipped.
- Resources that don't depend on it still complete.
- The state is saved anyway, so the next apply picks up where this one
  stopped:
  - resources that were reached are saved with their new values and status;
  - the failing resource is saved as `failed`;
  - resources that existed before but were not reached keep their previous
    entry;
  - new resources that were not reached are left out.

`Config.Apply` saves this state and then returns the error. A configuration
that doesn't parse or validate saves nothing, since no provider was called,
and neither does an apply that fails, or is cancelled, while deciding.

## Statuses

xcl records a resource's status in `Meta.Status`
([`types/status.go`](../types/status.go)). These are the only values it sets:

| Status | Meaning |
|---|---|
| `created` | the provider created the resource |
| `updated` | the provider updated the resource |
| `failed` | creating or updating the resource failed, including the create of a replacement; it is replaced on the next apply |
| `destroyed` | never saved: a destroyed resource is removed from the state |
| `destroy_failed` | destroying the resource failed, including the destroy of a replacement; the destroy is tried again by the next `Destroy`, or the next apply (removed again if its block is gone, replaced if it is still configured) |

`Meta` belongs to xcl. Don't set it, and don't confuse `Meta.Status` with
your own observed fields, like a container's `running`.

## Ordering

Resources are processed in dependency order. By the time `Create` or
`Update` is called for a resource, everything it references has already been
through its own lifecycle, and the references hold the values those providers
returned. While deciding, `Read` and `Changed` are called after everything
the resource references has been decided, but before anything is acted on: a
reference to a value only known after the apply holds the saved value
instead (see [Decide](#decide)).

Destroys run in the reverse order: children first, the create order reversed,
built from the links each resource saved. `Destroy` is called
for a resource only after everything that depends on it has been destroyed,
so it is never called for a parent once a child's destroy has failed.
Resources that don't depend on each other are destroyed in parallel.

## Logging from a provider

A provider writes no output. Log through `plugins.Logger(ctx)`, the logger
xcl puts in each call's context:

```go
func (p *ContainerProvider) Create(ctx context.Context, c *Container) (*Container, error) {
    log := plugins.Logger(ctx)

    id, err := p.client.Run(ctx, c.Image)
    if err != nil {
        return nil, err
    }

    log.Info("started container", "container_id", id)
    c.ContainerID = id

    return c, nil
}
```

Each message becomes a log event on the application's event stream. xcl has
already bound the logger to the resource, its type, the file it was declared
in and the step, and names your plugin as the event's `Source`, so pass only
the details of the message itself. The same code works in-process and in an
external plugin; for an external plugin, detail values cross the process
boundary as text. Outside a provider call, `plugins.Logger(ctx)` emits
nothing; use the plugin scoped logger from `Init` there. See
[Plugin logging](plugins.md#plugin-logging).

Log a sensitive field as the field itself, which writes `(sensitive)`, never as
the value `Reveal()` returns; see [Sensitive fields](#sensitive-fields).

## Events

Each provider call fires a `start` event, a `log` event for each message the
provider logs during it, and then a `success` or `error` event, with the
operation name `create`, `read`, `changed`, `update` or `destroy`. Because an
apply decides before it acts, every `read` and `changed` event of an apply
comes before its first `destroy`, `create` or `update` event. A replacement
has no operation of its own: it is a `destroy` then a `create` for the same
resource. See
[Parser & Resource Lifecycle](parser-lifecycle.md#events-parseroptionsemit).

## The example provider

[`plugins/example/pkg/person/provider.go`](../plugins/example/pkg/person/provider.go)
follows everything in this guide:

- It embeds `plugins.DefaultChanged[*Person]` and defines its own `Changed`
  on top: a person's ID is derived from their name, so a change to
  `first_name` or `last_name` answers `entity.Replace`, and anything else is
  left to `DefaultChanged`, passing on the `changes` and `dependencies` lists
  it was given:

  ```go
  func (p *ExampleProvider) Changed(ctx context.Context, old *Person, new *Person, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (entity.Change, error)
  ```

- `Update` takes the same two lists and needs neither:

  ```go
  func (p *ExampleProvider) Update(ctx context.Context, person *Person, changes []entity.PropertyChange, dependencies []entity.DependencyChange) (*Person, error)
  ```

- `Person.PersonID` is a computed field, set in `Create`.
- `Read` returns `plugins.ErrNotFound` when the saved email is
  `missing@example.com`. This is a sentinel for demonstration; a real
  provider would look the resource up by its identity.
- It never changes a configured field.
