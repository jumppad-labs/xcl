---
created_date: "2026-10-08"
document_status: draft
specs:
    - 20261008132354-4538504f-replacement-deps
---

# Replacement and dependency changes

## Goal

Let a provider say that a change can't be made in place and the resource must be **replaced** (destroyed and created again). Let every resource that depends on it decide what that means for itself. For example, a Docker network whose subnet changes must be replaced, and so must a container attached to it. A template that only reads the network's name only needs updating.

## The provider decides, through `Changed`

```go
// Change is what applying a new configuration needs for one resource
type Change int

const (
    NoChange Change = iota // leave the resource as it is
    Update                 // call Update in place
    Replace                // Destroy the resource, then Create it again
)

// DependencyChange is a resource this one depends on that the same apply
// will update or replace
type DependencyChange struct {
    Address string // i.e. "docker.network.app"
    Change  Change // Update or Replace
}

type ResourceProvider[T any] interface {
    // ...
    // Changed reports what applying new over old needs. dependencies lists
    // the resources this one references that the same apply will update or
    // replace; a dependency that is unchanged is not listed.
    Changed(ctx context.Context, old, new T, dependencies []DependencyChange) (Change, error)
}
```

- A provider returns `Replace` for a change it cannot make in place.
- A dependent receives every dependency that will change, with its outcome, and returns whatever that means for itself. A container on a replaced network returns `Replace`; a template reading a replaced network's name returns `Update`.
- `dependencies` holds only the resources this one directly references, not the whole apply. A dependency of a dependency reaches it only through the direct dependency's own decision.
- Deletion never appears: a resource removed from the configuration cannot still be referenced by a configured one, because validation fails first.

## How the core uses it

Applying becomes two passes over the dependency graph:

1. **Decide.** For each resource, in dependency order, call `Read` and then `Changed` with the decisions already made for its dependencies. Nothing is created, changed or destroyed. A resource not in state is a create, and one saved as failed is a replace, as today.
2. **Act.**
   - Destroy every resource being replaced, together with every resource being removed, dependents first.
   - Then, in dependency order, create the replaced and new resources and update the updated ones.

`Diff` and `plan` run the decide pass alone. A replaced resource shows `-/+`, with the reason when it came from a dependency:

```
-/+ docker "container" "web" {   # replaced because docker.network.app is replaced
```

The computed values of a dependency that is updated or replaced are "(known after apply)".

## What changes

- **Breaking**:
  - `ResourceProvider.Changed`'s signature and result change.
  - The typed adapter, the direct host and the gRPC protocol for external plugins (`plugins/proto`) carry `dependencies` and `Change`.
  - Every provider in the repo is migrated.
- **Example**: the Docker network provider returns `Replace` when `subnet` changes, and the container provider returns `Replace` when its network is replaced. `Update` no longer pretends to change a network.
- **Unchanged**: create, destroy and the order a configuration first applies in.

## Open

- Whether a provider may also return `Replace` from a failed `Update`. Not proposed.
- Create-before-destroy replacement, which some resources need for no downtime. Not proposed; replacement always destroys first.
