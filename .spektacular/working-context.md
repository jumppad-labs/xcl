# Working context: plan 20261008194959-b128e508-update-property-changes

## Plan workflow status
- Spec read (overview step). Spec decisions below carry into planning.


## Problem
When a provider's `Changed` answers `entity.Update` (for example plain `DefaultChanged`), the core calls `Update(ctx, resource T)` with only the new configuration (plus computed values carried from the decide-pass Read). The provider cannot tell which properties changed or what their old values were, so it cannot write targeted in-place update code.

Motivating use case: the Docker example's container network block changes from `app` to `backend`, and the provider can hot swap networks. Assume `Changed` is just `DefaultChanged`, so it answers Update. The plan already shows `~ network[0].name = "app" -> "backend"`, but `Update` sees only `backend` and has no way to know to disconnect `app`.

User's words: "you call update with the new values. But what has changed, I don't know, so how could I write some code which would modify the network settings?"; "This is a lot of work to update a single property, I have to make api calls to fetch data when I already have this information in the type. I just need to know which properties have changed, and what the old value is."

## Decisions
- `Update` receives the list of changed properties: path, old value, new value. Sketch: `Update(ctx context.Context, resource T, changes []entity.<PropertyChange>) (T, error)`. The type name is a placeholder.
- The change type lives in the public `entity` package, beside `entity.Change` and `entity.DependencyChange`, and not in `diff`. User: "I think you are right about having the change on the entity package too."
- Breaking changes to the Go contract and the plugin protocol are fine. User: "I am all good with breaking changes right now, we are so close to release".
- Scope is the Update change list only. The plugin scaffold is a separate GitHub issue, #8, probably a GitHub template.

## Details raised (to settle in the spec)
- Compute the list at act time, from the saved copy against the configuration decoded with real values, so it never holds unknowns.
- Sensitive values: carry the real before and after values to the provider. It already sees real values; masking is a rendering concern.
- Path matching: `diff.Path` is structured (attribute, index and key steps). A friendlier match helper beats comparing strings like "network[0].name".
- External plugins need the change list on the gRPC `UpdateRequest`.

## Alternatives rejected
- **Reconcile in `Update` against the real resource** (inspect Docker): extra API calls, and each provider redoes a comparison the core already did.
- **Read the previous copy through `plugins.State`**: it is a back door around the contract, and `GRPCState.Get` returns "resource deserialization not implemented" for external plugins.
- **`Update(ctx, old, new)`**: smaller, but every provider still has to write its own comparison. The user wants to be told which properties changed and their old values.
- **A shared default `Changed` that replaces on replaced dependencies**: dropped by the user in favour of the scaffold issue.

## Context
This builds on the shipped spec 20261008132354-4538504f-replacement-deps (unchanged/update/replace, decide-then-act, `entity` package), merged into `f-diff`.

## Interview answers (spec workflow)
- Changed AND Update both receive the property change list; Update also receives the dependency list.
- Example: container network hot swap is an in-place update (no replace on network blocks or on a replaced network); new `init_script` fed by a second template ("init"), init template replaced → container replaced; network Destroy force-detaches containers.
- No cascade delete: dangling reference stays a validation error (user corrected themselves).
- Website docs updated too. Scaffold = issue #8 (out of scope).
- User said 'just run to the end' after constraints: draft remaining spec sections without per-section confirmation.
- Spec verified by fresh reviewer; applied fixes (logs in sensitive rule, not-yet-known values requirement, init-script producer wording, trimmed duplicates, no-syntax-change constraint). Spec committed to store.
- Split check: not offered (tightly coupled; example + docs are supporting work).

## Plan learnings (discovery done)
- Target repos: xclconfig (core, plugins, examples, docs) and xcl-website (replacement.mdx, examples/plugins.mdx, maybe diff.mdx). No formal design refs on the spec; the governing design `replacement-and-dependency-changes.md` was read.
- Changes are computed with the plan's `resourceChanges` (saved vs configured), before `adapter.Changed` in refresh, and recomputed at act time in `update` from the real decode plus `l.previous`. Dependencies for Update come from `dec.dependencies`.
- `diff.Path` moves to `entity`, with diff aliases. New `entity.PropertyChange` has real values, a Sensitive flag, and self-masking print/log/JSON forms. Values are JSON-normalised on both the in-process and external paths.
- Docker example gaps: no NetworkDisconnect in the client interface, the Update methods are no-ops, network Destroy doesn't detach, the template has no mode. Existing tests assert replace-on-network semantics and must be rewritten.
- Website pages replacement.mdx:195 and examples/plugins.mdx:433 claim the Docker Update is a no-op, so they must be rewritten.
- Architecture chosen: Option A (one shared comparison per pass; contract first with empty lists; then core; then example; then docs). Signatures: Changed(ctx, old, new, changes, deps), Update(ctx, resource, changes, deps). Proto: ChangedRequest.changes=6; UpdateRequest.changes=4, dependencies=5.
- Components drafted. A new e2e recording provider (in-process plus external) proves parity. The template gains an optional mode for the executable init script.
- Testing approach drafted; all 4 success metrics are behavioural; 2 manual reviews (Makefile walkthroughs, docs review).
- Tasks drafted (12 ids, in tasks_plan.md). Example variants live in config dirs. The recorder parity fixture is a shared package in e2e/fixtures/recorder.
- Assembled and staged the three docs to .spektacular/tmp/<plan>/ (assembly script in the session scratchpad).
- Verification passed: sections present, 12 unique task ids, context anchors match, commands removed from plan.md.
- All three docs committed to the plan store; work dir removed. Next: walkthrough (read the docs back with plan file read).
- Walkthrough done; user signed off on the plan with no changes.
