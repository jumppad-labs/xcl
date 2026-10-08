# Working context: 20261008132354-4538504f-replacement-deps

## Problem
Found while testing the new `plan` command in example/plugin: changing the Docker network's `subnet` is planned and applied as an in-place **update**. `networkProvider.Update` (example/plugin/plugins/docker/resources/network.go:102) is a no-op, so state records the new subnet while Docker keeps the old network, and the container `web` (attached to it) is never touched. User: "we concentrated on a computed property changing but what we did not handle is when a property changes and that property is consumed by a linked resource which also needs to change ... if we change the ip then the docker container also needs to be destroyed and re-created."

Core gaps (internal/parser/lifecycle.go):
- Provider `Changed(ctx, old, new) (bool, error)` can only say changed/not; changed always means Update. Replace exists only for resources whose last apply failed (diff.ActionReplace).
- Dependents are only marked updated when they read a *computed* value of a pending resource (unknownPaths). The container reads `docker.network.app.meta.name`, never computed, so nothing cascades.

## Direction chosen by the user
- User: "the changed property for network would return Replace or something signaling to other resources that something they depend on is going to be completely replaced. They then have to decide if this means they will be replaced. Again this could be handled by Changed plugin function."
- Dependents are told about dependencies that will **update or replace** (user chose this over replace-only). Delete never cascades: a removed resource can't still be referenced (validation fails).
- Rejected: struct tag `xcl:",replace"` (ForceNew-style) — user preferred provider-decided via Changed; "any reference cascades" and Terraform value-based cascade rules — superseded by dependents deciding themselves.
- Design stored: design/replacement-and-dependency-changes.md (source "design"), authored and agreed. It is binding: Change enum NoChange/Update/Replace, DependencyChange{Address, Change}, Changed(ctx, old, new, dependencies) (Change, error); apply = decide pass then act pass (destroy replaced+removed dependents-first, then create/update in dependency order); diff/plan = decide pass only, `-/+ ... # replaced because <dep> is replaced`; out of scope: create-before-destroy, Replace from failed Update.
- Breaking API is acceptable (previous spec set that precedent; this design lists the Changed signature break).

## Side notes
- example/plugin/config/alt.xcl (user's file) sits next to main.xcl; `apply ./config` reads both and declares app/web/welcome twice — breaks example tests/smoke/make run. Needs moving (e.g. config-alt/).
- Plugin SDK registration (PluginBase.RegisterType / RegisterResourceProvider return errors) is a separate open question, not this spec.
- Spec name: replacement-deps (user). No epic (user didn't pick one).
- Merges: user wants work merged back into the branch they are on (currently f-diff).

## Interview answers
- Website: document both the plugin-author Changed contract and plan/diff -/+ output.
- alt.xcl becomes a demo config in its own directory (e.g. example/plugin/config-subnet/), used by example tests and docs.
