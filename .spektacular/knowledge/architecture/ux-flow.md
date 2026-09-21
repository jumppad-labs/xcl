---
tags: [architecture, config, query, public-api]
---

# XCL User Experience Flow

## Overview

This document describes how end users interact with the XCL package to parse and manage configuration.

## Entry Point

The main entry point for end users is the `Config` type in the root `xcl` package.

```go
import "github.com/jumppad-labs/xcl"
```

## Basic Usage Flow

### 1. Create a Config

```go
config := xcl.NewConfig(
    xcl.WithStateStore(stateStore),
    xcl.WithPlugin(myPlugin),
    xcl.WithVariables(map[string]any{"env": "production"}),
)
```

### 2. Apply Configuration

Parse and apply configuration files, executing plugin lifecycle methods:

```go
err := config.Apply("./config/main.xcl")
if err != nil {
    log.Fatal(err)
}
```

### 3. Query the Configuration

Ask the configuration for what you want, as your own Go type, in a single call.

```go
// one entity, by its address
db, err := xcl.Find[PostgreSQL](config, "resource.postgres.main")

// every entity of a kind: segment one is the kind, segment two the variety
dbs, err := xcl.FindByType[PostgreSQL](config, "resource", "postgres")

// the one you expect there to be exactly one of
ingress, err := xcl.FindOne[Ingress](config, "resource", "ingress")

// every entity of a Go type, without naming it as a string
all, err := xcl.All[PostgreSQL](config)
```

Values the configuration publishes are read the same way, and come back as the
value rather than the declaration that produced it:

```go
url, err := xcl.Find[string](config, "output.web_database")
published := config.Outputs() // all of them, keyed by address
```

Everything declared can be enumerated without naming a type, and an entity from
that enumeration converts in one call:

```go
for _, entity := range config.Entities() {
    typed, err := xcl.As[PostgreSQL](entity)
}
```

An empty result and an unanswerable question are never the same thing. A
well-formed query matching nothing returns an empty result and a nil error; a
query that cannot be answered returns an error matched by identity:
`ErrNotFound`, `ErrUnknownType`, `ErrNotTypeable`, `ErrNotRegistered`,
`ErrTypeMismatch`, `ErrNotAnEntity`, `ErrNotUnique`. Each wraps a detail type
recoverable with `errors.As` — `NotUniqueError.Count` reports how many matched.

**Two spellings, one implementation.** The same surface exists as methods on
`*Config` (`config.Find[T](addr)` and so on). Generic methods need Go 1.27 and
this project supports 1.25.0, so the method form is excluded below 1.27 by a
build constraint while the package-level functions compile everywhere. The
method form is the idiomatic one; **every runnable example uses the function
form** so what a reader copies works on the oldest supported toolchain. Both
delegate to one implementation and cannot differ. `As` is a function in both
worlds, since it takes no config for a method to hang off.

### 4. Validate Without Applying

Check whether a configuration is valid, without acting on it. Validation
creates, changes and removes nothing, decodes no resource bodies and reaches
no provider:

```go
err := config.Validate("./config/main.xcl")
if err != nil {
    log.Fatal(err)
}
```

A nil error means the configuration is valid. Otherwise the returned error is
an `*errors.ConfigError` collecting **every** problem found, each naming what
is wrong and the file and position it occurs at:

```go
var ce *errors.ConfigError
if errors.As(err, &ce) {
    for _, problem := range ce.Errors {
        fmt.Println(problem)
    }
}
```

Validation runs three stages in order, and each reports everything it finds
before the next is considered:

1. **Structure** — configuration too malformed to check any further.
2. **References** — anything a resource refers to must be defined somewhere
   in the configuration. Resolution spans files and reaches into modules.
3. **Properties** — a reference's trailing property path must name properties
   the referenced type actually has.

A later stage is skipped when an earlier one found problems, since checking
properties on a reference that resolves nowhere would only report consequences
of a problem already reported.

`Apply` runs the same validation first and refuses to proceed if it fails, so
nothing is created, changed or removed unless the configuration is valid.

### 5. Destroy Resources

Remove all resources in state:

```go
err := config.Destroy()
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     End User Code                            │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  config := xcl.NewConfig(...)                               │
│  config.Apply("config.xcl")                                 │
│  db, _ := xcl.Find[T](config, "resource.type.name")         │
│  all, _ := xcl.FindByType[T](config, "resource", "type")    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    xcl Package (Public API)                  │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Config          - Main orchestrator, the only query surface │
│  Find/FindByType/FindOne/All - typed lookups (+ methods)     │
│  As              - convert an enumerated entity to a type    │
│  Entities/EntityCount/Outputs - untyped enumeration          │
│  Err… sentinels  - every way a lookup can fail, errors.Is    │
│  ConfigOption    - Functional options for Config            │
│  ConfigError     - Collects every problem found by validation │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Internal Packages                           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  internal/parser    - HCL parsing, DAG building             │
│  internal/schema    - Go/CTY type conversion                │
│  internal/resources - Built-in resource types               │
│  internal/functions - Built-in HCL functions                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                  Supporting Packages                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  state/            - State management and persistence        │
│  plugins/          - Plugin interfaces and registry         │
│  types/            - Resource base types and helpers        │
│  errors/           - Custom error types                      │
│  logger/           - Logging abstractions                    │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Key Design Decisions

### 1. Config is the Only Entry Point

- Users never directly interact with Parser
- Parser is internal implementation detail
- Config orchestrates parsing, state management, and plugin execution

### 2. Config is the Only Public Query Surface

- The lookups take `*Config`, never `*State`; users need know nothing of State
- They scan `Config.Entities()`, so the typed and untyped views agree by
  construction
- The `state` package is store-and-retrieve only. Its remaining `Find…` methods
  are vestigial and slated for removal — see
  `architecture/config-is-the-public-query-surface.md`

### 3. State is Managed Internally

- State persistence is handled by `StateStore` interface
- Users configure storage via `WithStateStore()` option
- State transitions are managed by Config during Apply/Destroy

## Resolved Questions

These were open while `Querier[T]` existed. The query-api-v2 work settled all
three, and they are kept rather than deleted so the reasoning is not
re-litigated.

1. **Should the query surface accept an interface rather than concrete Config?**
   Moot. There is no `Querier`. The lookups are package-level generic functions
   taking `*Config`, plus methods on `*Config` above Go 1.27. Nothing binds a
   type at construction, so there is nothing for an interface to abstract.

2. **How should tests query resources without circular dependencies?**
   By not needing to. The lookups live in the root package and scan
   `Config.Entities()`. Where a test genuinely needs both the parser and the
   root package it uses an external test package, which is Option A — see
   `state/file_state_store_apply_test.go`, in `package state_test` for exactly
   this reason.

3. **What is the minimal interface for the query surface?**
   Not an interface at all. The shared address matcher takes a plain `[]any`
   (`internal/resources.Match`), so the public surface and the parser both use
   it without either depending on the other, and the storage layer needs no
   knowledge of addresses.
