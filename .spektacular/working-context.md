# Working context: plan for 20261016064106-8931613a-python-encode-entity

## Workflow
- Plan workflow started 2026-10-16 against spec 20261016064106-8931613a-python-encode-entity
  (final). Spec references design `design:libxcl.md` (Lookups section to update).
- auto_commit is off.

## Carried from the spec session
- User's words: "We need to be able to use the Encode entity from python with the same
  features that we can from the go api. It should use the shared library where possible
  and minimize any custom python code."
- Repos: xcl (engine / core proto / libxcl), xcl-python (SDK, PARITY.md, README),
  xcl-website (Python guide + API reference). Not other bindings.
- Breaking changes to core-service messages are fine (xcl unreleased).

## Presentation note
- Over remote control, text sent alongside a dialog was not visible to the user. Present
  drafts as plain chat text and ask for confirmation in text, not via AskUserQuestion.

## Plan progress
- overview: spec read.
- discovery: repos xcl, xcl-python, xcl-website. Design libxcl.md resolved and read.
  Interview notes absent (normal). Relevant knowledge: custom-marshaljson-changes-internal-hops,
  xcl-tags-gate-what-reaches-cty, save-state-through-encodeforstate, expression-text-lost-at-parse,
  links-are-attribute-paths, secret-fixture-for-sensitive-tests (registered.Secret), testing conventions
  (no table tests, no tests reading docs). Research agents launched (Go, Python, docs+prior specs).
- discovery research.md + assumptions.md written to .spektacular/work/<plan>/.
  Key findings: Go encoder is encode.go (options IncludeComputed/IncludeEmpty/RevealSensitive/
  ShowReferences; highlight is a separate step). coreservice EncodeSavedEntity ignores options today.
  Lookup JSON carries the marker; Go shows real values because it encodes the stored pointer.
  No Python instance->record serialiser exists; write as mirror of _schema.hydrate.
  Unmerged earlier attempt branch origin/spek/20261015072147-b41720ce-python-encoder (9c5dc1f) is
  reference only (wrong reveal semantics, old base).
  xcl-website has NO Python guide / API reference page; encoding docs are configuration-text.mdx tabs.
- USER ANSWERS (discovery):
  1. Website: treat configuration-text.mdx Python tabs (+ events.mdx mentions) as the Python guide;
     no API reference page exists, none is created.
  2. Event entities: engine fills by meta.id for every record, so a kept event entity with reveal
     shows the engine's current real value; differs from Go for masked event data; document it.
- architecture: chosen = new EncodeEntity RPC + shared EncodeOptions msg; Go public
  (*Config).EncodeEntityRecord (decode, fill-by-meta.id on reveal, encodeEntity) in encode_record.go;
  Python encode_entity + _schema.record_of; kwargs include_computed/include_empty/reveal_sensitive/
  show_references/highlight; parity `encode` section (16 combos, looked_up + saved, hand_built list).
- components drafted.
- data_structures drafted (proto EncodeOptions/EncodeEntity*, Go EncodeEntityRecord, py encode_entity/record_of, parity encode section).
- implementation_detail drafted.
- dependencies drafted.
- testing_approach drafted (metric -> parity behavioural; 4 manual doc reviews).
- milestones drafted: M1 engine (xcl), M2 Python API (xcl-python), M3 parity (both), M4 website.
- tasks drafted: 13 tasks (11 agent, 1 human release, ids from plan task-id). Titles avoid apostrophes/dots for anchors.
- open_questions: one (plugin-type identity for fill).
- out_of_scope drafted.
- assemble: staged plan/context/research templates in .spektacular/tmp/<plan>/.
- verification: passed (sections, 13 ids/repos, anchors ok, no commands in plan.md).
- write_plan: plan.md committed.
- write_context: context.md committed.
- write_research: research.md committed; work dir removed. Next: walkthrough (read docs back via plan file read).
- walkthrough: beat 1 (approach) accepted by user.
- walkthrough: beat 2 (tasks) accepted.
- walkthrough: beat 3 (out of scope) accepted.
- walkthrough: beat 4 (assumptions) accepted; user signed off ('it looks good'). Advancing to finished.
- FINISHED: plan approved and marked final (2026-10-16). Next: spek-implement.
