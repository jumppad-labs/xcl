**Two passes, one decision.** The main change in the parser is that `Apply` stops being one walk that reads and acts as it goes. It becomes "decide, then act", and `Diff` becomes "decide, then report".

A reader sees three thin entry points over one sequence: parse and validate, run the decide walk, then hand the decision record to a reporter (diff) or an actor (apply). The walk's mode no longer means "diff or apply". It means "decide or act", and the branches on mode stay where they are today: which lifecycle step runs, how a body is decoded, and which events fire.

The decide pass emits the read and changed lifecycle events. The act pass emits destroy, create and update. Each resource's event sequence is unchanged. Across resources, every read and changed event now comes before the first destroy, create or update, which is exactly the "nothing changes until every decision is made" guarantee, and it can be seen in the event stream.

**The diff recorder grows up rather than being replaced.** The recorder already holds the cross-resource knowledge the decide pass needs: pending entities and unknown paths. It gains one decision per entity and becomes the decision record. The existing pattern carries over unchanged: a mutex-guarded store written from concurrent walk callbacks, relying on the DAG guarantee that a parent is recorded before its children run.

Dependency lists are derived from the record at the moment a resource is decided. No second graph traversal is needed and no new ordering guarantee is introduced.

**Refresh stays the single provider-facing decision step.** It already isolates Read + `Changed` behind a small outcome type. The outcome grows a replace case, and refresh gains the dependency list as an input. Apply's old "refresh, then update" collapses into the decide pass. The act pass never calls Read or `Changed`.

This is the refactor that lets the in-walk rebuild go away. Failed-status replacements and provider-decided replacements become the same record entry, a replace, and are executed by the same destroy phase.

**The destroyer is reused as the destroy phase.** Today apply already runs the destroyer over removed resources before walking. The destroy phase simply widens its target list to include replaced resources. The destroyer's reverse graph, per-resource state saves and `destroy_failed` marking are inherited unchanged. Its existing rule that a failed destroy blocks its own dependencies' destroys still holds.

A replaced resource destroyed successfully is dropped from working state, then recreated by the act walk like any new resource, so its fresh saved copy has no stale computed values.

**Unknown inputs get saved values plus a floor.** The diff decoder already records unknown paths and substitutes placeholders. For the copy handed to a provider in the decide pass, the placeholders are replaced by the saved values at those paths. The provider then sees a meaningful "what you had, plus what's known to change", together with the dependency list that tells it why.

The core then raises a NoChange answer to Update for such a resource. This is the one place the core overrides a provider, and it never overrides towards Replace, so "the plugin decides whether to replace" holds.

**Contract migration is mechanical and lands first.** The `Change` vocabulary and the new signatures go through every layer in one change, with the core temporarily treating `Replace` as today's rebuild and passing no dependencies. Because nearly every provider embeds `DefaultChanged`, most of the migration is that one method. Hand edits are confined to the recording test plugin, the gRPC conversion, the generated code (regenerated, never hand-edited) and the callers that asserted booleans.

The proto is regenerated with generator versions pinned to those already recorded in the generated files.

**Providers express replace rules as small, readable overrides.** The example providers each gain a `Changed` method that reads as a list of rules: "if subnet differs, replace", "if any dependency is replaced, replace", "otherwise defer to `DefaultChanged`". This is the pattern the documentation teaches plugin authors.

Each rule gets its own unit test next to the provider, with no table-driven tests. Mocked Docker clients are untouched, because `Changed` needs no client.

**Renderer change is a phrase, not a layout change.** `diff.Resource` carries the reason. The renderer's existing per-action phrase function branches on it, so the output layout, markers and colours are unchanged. That keeps the user's choice of the comment-line-above layout and every existing render test except the replace phrase.

**Patterns followed and introduced.**
- Followed:
  - the walk-mode pattern and the recorder pattern from the diff work;
  - the destroyer for ordered destroys;
  - `callProvider` for provider calls and their events;
  - real-apply test state with `TestPlugin`;
  - e2e plan-versus-apply comparison through public packages;
  - examples using only public packages.
- Introduced:
  - the decision record as the contract between the two passes;
  - provider-side `Changed` overrides that read the dependency list.
