# Working context: spec for 20260923095659-references-and-secrets

## Status
- Spec workflow started 2026-09-23. Name chosen by the user: `references-and-secrets`.
- Step: interview done → overview. Synthesis is in
  `.spektacular/work/20260923095659-references-and-secrets/interview.md`; draft every section from it.

## User decisions from the interview (2026-09-23) — all binding
1. **Link resolution stays ON by default**, references are the opt-in. Reason in the user's words:
   the resolved value "visibly shows me the result of the config".
2. **Field→reference provenance is recorded in saved state**, so both entry points can write
   references and keep producing identical text. This **deliberately lifts** the previous spec's
   "the stored format must not change" constraint.
3. **No backwards compatibility required** — user's words: "We don't need to worry about backwards
   compatibility." Old state files do not have to load. Removes any migration requirement.
4. **Secrets are marked with an `xcl:"...,sensitive"` tag option**, a third alongside `computed` and
   `key`. Verified: nothing like it exists in the codebase today.
5. **Sensitive values are masked by default**, revealing is the opt-in, so a newly tagged field is
   protected immediately.
6. **Secret scope is display output only** — user's words: "this is just for log output right now."
7. **Docs**: update the existing `/configuration-text/` guide and the `/events/` page. No new pages.

## Follow-up the user wants as its OWN spec (do not fold in)
**Encrypted state.** Raised unprompted: "I am wondering if we should add encrypted state, I am
thinking we should but add it as a separate spec." Offer to start it once this spec is finished.
Related known gap to mention there: `Event.Data` at raw/processed carries the whole resource as
JSON, secrets included, and that path is deliberately unchanged by this spec.

## Where this came from
All three items were found while implementing `20260922132517-hcl-encoding-helpers`, which added
`xcl.EncodeEntity` / `xcl.EncodeSavedEntity`. Two were deferred out of that plan **by the user's
explicit decision**, and recorded in its Out of Scope; the third was already Out of Scope there.
That plan's `## Changelog` section holds the full findings if more detail is needed.

## The three items

### 1. Resolve links — write the reference, not the value it resolved to
The encoder writes every reference as the literal value it resolved to. A field written as
`networkobj = resource.network.onprem` comes back as a dumped object.

**The user's framing, in their words:** they want the value visible because it "visibly shows me the
result of the config", but "augmented with a link if it came from interpolation". They proposed a
**resolve-links flag**: with it on, `networkobj = { ... }`; with it off,
`networkobj = resource.blah`. They also noted the scenario "might not be a computed block — the
scenario I was describing was something that the user could author".

**Blocker, verified in code:** `getDependentResources` (`internal/parser/parser.go:1119-1132`)
collects references per attribute but flattens them into one `[]string` and discards `a.Name`:

```go
for _, a := range b.Body.Attributes {
    refs, err := processExpr(a.Expr)
    references = append(references, refs...)   // a.Name is thrown away
}
```

So `Meta.Links` records *what* was referenced, never *which field* referenced it. The parser must
record field→reference provenance before any of this is possible.

**Constraint carried over:** the previous spec required the stored state format not change, and
`EncodeSavedEntity` works from a state record. If references must survive into saved data, that
touches the stored format. Restricting references to `EncodeEntity` only avoids that but breaks the
"both entry points give identical text" guarantee the last feature established and tested. **This
tension is unresolved and needs a decision in this spec.**

### 2. DependsOn / Links — stop overwriting a user-set property with computed data
**The user considers this a design mistake, in their words:** "I did not realize we used the public
property to store the privately computed links. I honestly feel this is a mistake and that we should
never overwrite a user set property in the backend. We should probably use a different private
property which merges in the depends_on values. Originally I think we did have one."

That private property already exists: **`Meta.Links`**. `types.AppendUniqueDependency`
(`types/resource_helpers.go:89-108`) mirrors it into the public `DependsOn` under a comment reading
`// Also update DependsOn for backwards compatibility`.

**It is NOT vestigial.** `dag.go:62-74` copies every `Links` entry into `DependsOn` via
`AppendUniqueDependency`, then reads `DependsOn` back through `getResourceDependencies` to build the
graph. So the mirror is the DAG's transport and cannot simply be deleted.

**The contained fix:** point `getResourceDependencies` (`internal/parser/util.go:506`) at
`Meta.Links`, then drop both the `dag.go` copy loop and the mirror write. `Links` already holds
user-authored entries (`parser.go:1105`), so nothing is lost. Everything else load-bearing already
reads `Links` (`validate.go:71,116`, `context.go:38`, `parser.go:1170`). `DependsOn`'s only other
readers are `logger/pretty_printer.go` and `encode.go`.

**Risk:** it changes DAG construction, which orders create and destroy. Needs its own tests.

**Consequence for the encoder:** `depends_on` is currently **not written at all**, because after
parsing it mixes hand-written and derived entries indistinguishably. Once fixed it can be written
faithfully — a one-line change in `trimBookkeeping` in `encode.go`.

### 3. Secrets
Encoded output shows every value exactly as held, including passwords. Example terminal output now
prints `password = "password"` by default, which is more prominent than before: it used to be
reachable only through the API. Tracked at https://github.com/jumppad-labs/xcl/issues/1 and listed
as Out of Scope in the previous plan. The user asked for it here: "we can add the secrets stuff to
that too".

## Framing to confirm during the interview
Items 1 and 2 share a root cause — xcl does not track what the user wrote versus what it derived.
Item 3 is a different mechanism (marking fields sensitive, probably a struct tag option like the
existing `computed`). Worth confirming the user wants one spec rather than two, and if one, that the
unifying frame is "the text shows what the user wrote, and does not show what they would not want
shown".

## Repos
- `xclconfig` → `/home/nicj/code/github.com/jumppad-labs/xcl` — all three items live here.
- `xcl-website` → `/home/nicj/code/github.com/jumppad-labs/xcl-website` — the
  `/configuration-text/` guide and `/events/` already document the current behaviour and would need
  updating. The example pages quote the library's example code directly.

## Standing preferences
- Don't commit unless asked.
- Go conventions: testify `require`, NEVER table-driven, never mix positive and negative tests in
  one function, favour verbose readable tests, `any` over `interface{}`.
- Store files (specs, plans, changelog, knowledge, design) go through the `spektacular` CLI only.
- The HCL fork in `internal/xcl` is ours but MPL-2.0: keep the HashiCorp header, add
  "Modifications Copyright (c) Jumppad Labs", record changes in `internal/xcl/UPSTREAM.md`.
- **The IDE/LSP diagnostics in this environment are stale and routinely wrong.** Trust
  `go build`/`go vet`/`go test`, not the diagnostics panel.
