# Docker plugin

An xcl plugin that creates real Docker networks and containers. It is the
external plugin the [plugin example](../../) loads, and it is laid out in the
standard xcl plugin layout, the same shape the plugin template uses. The
layout design (`plugin-layout.md`) is the shared reference for both.

It provides two block types:

- `docker "network" "<name>"`: a Docker bridge network named after the block,
  with an optional `subnet`. The plugin computes `docker_id`.
- `docker "container" "<name>"`: a Docker container named after the block,
  with `image`, optional `command`, `environment` and `init_script`, and one
  nested `network` block per network it joins. The plugin computes
  `docker_id` and `ip_address`.

The plugin is a Go module of its own,
`github.com/jumppad-labs/xcl/example/plugin/plugins/docker`.

## Layout

```
plugins/docker/
  plugin.go                       the Plugin type: Init builds the clients once and registers both types
  cmd/docker/main.go              serves the Plugin as a separate program
  entities/
    network.go                    the docker "network" block type
    container.go                  the docker "container" block type, with its nested network block
  providers/
    network.go                    the network provider
    container.go                  the container provider
    attachments.go                previous network attachments and network addresses, used by the container provider
    labels.go                     the labels every Docker object the plugin creates carries
    *_test.go                     strict-double unit tests; docker_test.go runs against a real engine
  client/
    docker/docker.go              the narrow interface over the Docker Engine SDK, with New and Ping
    docker/mocks/                 its Mockery-generated double
    containers/containers.go      the container task layer the providers use, built on client/docker
    containers/mocks/             its Mockery-generated double
  examples/basic/main.xcl         a sample configuration: a network and a container on it
  e2e/                            end-to-end tests: apply the sample, plan with no changes, destroy
  .mockery.yml
  Makefile
  README.md
  go.mod
```

- `entities/` holds the block types only. They import nothing but xcl's
  `types`, so a program that only reads what it applied imports `entities`
  alone, as the plugin example does.
- `providers/` holds the providers. Each is built by a constructor that takes
  the container task layer, so the unit tests build it the same way with a
  double. No provider imports a Docker library.
- `client/` is the only place the Docker libraries are imported.
  `client/docker` mirrors the SDK calls the plugin makes, and
  `client/containers` turns what the providers want done into those calls,
  in its own types.

## Commands

Run each from this directory:

| Command | What it does |
|---------|--------------|
| `make build` | builds the plugin binary, `build/docker-plugin` |
| `make test` | runs the unit, real-engine and end-to-end tests |
| `make generate` | regenerates the Mockery doubles in `client/*/mocks` |
| `make clean` | removes `build/` |

The real-engine tests in `providers/docker_test.go` and the end-to-end tests
in `e2e/` need a Docker engine, reached through `DOCKER_HOST` or the default
socket. They skip when none is reachable, so `make test` passes without one.

## Registering the plugin

The `Plugin` type lives in the module's root package, so a host can register
it in two ways.

In-process, compiled into the host:

```go
import (
	"github.com/jumppad-labs/xcl"
	"github.com/jumppad-labs/xcl/example/plugin/plugins/docker"
	"github.com/jumppad-labs/xcl/registry"
)

local := registry.NewLocal()
local.RegisterPlugin(&docker.Plugin{})

c, err := xcl.NewConfig(xcl.WithRegistry(local))
```

As a separate program, the binary `make build` produces, which xcl starts and
talks to over gRPC. This is how the plugin example uses it:

```go
local := registry.NewLocal()
local.RegisterExternalPlugin("plugins/docker/build/docker-plugin")

c, err := xcl.NewConfig(xcl.WithRegistry(local))
```

## Reading state

A program that only reads the blocks it applied imports `entities`, and none of
the providers or Docker libraries:

```go
import "github.com/jumppad-labs/xcl/example/plugin/plugins/docker/entities"

containers, err := xcl.FindByType[entities.Container](c, "docker", "container")
```
