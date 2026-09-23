# Interview: 20260923095659-references-and-secrets

## What this is

Three gaps in what `xcl.EncodeEntity` / `xcl.EncodeSavedEntity` put in their output, all found
while implementing `20260922132517-hcl-encoding-helpers`. Two were deferred out of that plan by the
user's explicit decision; the third was already Out of Scope there and the user asked for it here.

The unifying frame: **the text should show what the user wrote, and should not show what they would
not want shown.**

## Who it is for

Developers embedding xcl who want to show a person what a configuration declares — the same
audience as the conversion feature it builds on. The output is for display and for reading, not for
reprocessing; that framing was settled in the previous spec and does not change here.

## The three parts

### 1. Write the reference, not the value it resolved to

Today every reference is written as the literal value it resolved to, so a field the user wrote as
`networkobj = resource.network.onprem` comes back as a dumped object. The user wants the option to
see the reference instead.

**Settled:**
- **Link resolution stays on by default.** Output keeps showing values, which is the user's stated
  reason for wanting the field visible at all: it "visibly shows me the result of the config".
  Asking for references is the opt-in.
- **Provenance is recorded in saved state**, so `EncodeEntity` and `EncodeSavedEntity` can both
  write references and continue to produce identical text. This deliberately lifts the previous
  spec's constraint that the stored format must not change.
- **No backwards compatibility is required.** The user was explicit: "We don't need to worry about
  backwards compatibility." State written by earlier versions does not have to load.

**Why it is not possible today:** the parser collects references per attribute and then flattens
them, discarding which field each came from. `Meta.Links` records *what* was referenced, never
*which field referenced it*. Recording that is the enabling work.

### 2. Stop overwriting a user-set property with computed data

The user considers the current behaviour a design mistake, in their words: "I honestly feel this is
a mistake and that we should never overwrite a user set property in the backend."

`ResourceBase.DependsOn` is public and user-settable, but xcl writes its own resolved references
into it during parsing, so after a parse the two are mixed and indistinguishable. The private
property that should hold the computed set already exists — `Meta.Links` — but the dependency graph
is currently built by copying `Links` into `DependsOn` and reading `DependsOn` back, so the mirror
cannot simply be removed without moving the graph onto `Links` first.

**The visible consequence:** the encoder cannot write `depends_on` at all today, because it cannot
tell a hand-written entry from a derived one. It currently omits it entirely. Once `DependsOn`
holds only what the user wrote, it can be written faithfully.

**Risk to respect:** this changes how the dependency graph is built, and the graph orders both
create and destroy. It needs its own tests rather than riding on the encoder's.

### 3. Do not show secrets

Encoded output shows every value exactly as held, including passwords. This became more visible
with the previous feature: example terminal output now prints `password = "password"` by default,
where before a secret was only reachable through the API. Tracked at
https://github.com/jumppad-labs/xcl/issues/1.

**Settled:**
- **A field is marked with an `xcl:"...,sensitive"` tag option**, a third alongside the existing
  `computed` and `key`. Type authors declare it once on the struct, and plugin types carry it
  through their schema automatically, exactly as `computed` already does. Nothing like it exists in
  the codebase today.
- **Sensitive values are masked by default**, and revealing them is the opt-in. A newly tagged
  field is therefore protected the moment it is tagged, and example output stops printing
  passwords without anyone having to remember to ask.
- **Scope is display output only.** In the user's words: "this is just for log output right now."

## Scope boundaries

**Explicitly out, and why:**

- **Encrypted state.** The user raised it unprompted and wants it, but as its own spec: "I am
  wondering if we should add encrypted state, I am thinking we should but add it as a separate
  spec." State continues to store real values in plaintext — xcl needs them to work.
- **Masking the raw JSON on `Event.Data`.** At the raw and processed levels an event carries the
  whole resource as JSON, secrets included. That path is unchanged here. This is a known gap, left
  deliberately rather than overlooked; it belongs with the encrypted-state conversation.
- **Backwards compatibility with existing state files.** Ruled out by the user.
- **Making the output reprocessable.** Settled in the previous spec: the text is for display. That
  does not change, and reinstating read-back is not in scope.

## Repos

- **xclconfig** (`/home/nicj/code/github.com/jumppad-labs/xcl`) — all three parts live here: the
  parser change that records provenance, the dependency-graph change, the tag option, and the
  encoder.
- **xcl-website** (`/home/nicj/code/github.com/jumppad-labs/xcl-website`) — the `/configuration-text/`
  guide documents the current behaviour in detail and the `/events/` page documents the data levels.
  The user asked for both to be updated. No new pages; the example pages also need updating if the
  output they quote changes.

## What success looks like

- A developer can ask for references and get back the addresses they wrote, from either entry point,
  with both producing the same text.
- A user-written `depends_on` survives a parse and appears in the output; xcl's own resolved
  references never appear there.
- A field tagged sensitive never appears in output in the clear unless explicitly asked for, and the
  bundled examples stop printing a password.
