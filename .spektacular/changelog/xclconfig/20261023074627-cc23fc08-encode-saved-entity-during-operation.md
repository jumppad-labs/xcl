---
created_date: "2026-10-26"
document_status: draft
project: xclconfig
spec: 20261023074627-cc23fc08-encode-saved-entity-during-operation
plan: 20261023074627-cc23fc08-encode-saved-entity-during-operation
---

# Render saved records while an operation runs

Programs that drive xcl's engine from another language can now turn a resource's saved record into configuration text while an apply, validate, diff, destroy or load is still running, including from their event handler. The text is the same as after the operation, and the operation itself is unchanged. Rendering a looked-up or edited entity, and every other lookup, is still refused while an operation runs. Go programs are unaffected.

> Derived from project xclconfig, spec/plan 20261023074627-cc23fc08-encode-saved-entity-during-operation. See the project-level record for the full feature.

## What changed in this repo

- The core service's `EncodeSavedEntity` handler no longer consults the lookup guard. It keeps its empty-record check, now shared with `EncodeEntity` as `requireEntity`. `EncodeEntity` keeps the guard. Changed: `coreservice/lookups.go`, plus a comment in `coreservice/operations.go`.
- `proto/xcl/core/v1/core.proto` documents `EncodeSavedEntity` as allowed during an operation. The change is comment-only: buf generates only messages, so no generated code changed.
- `coreservice/operations_test.go` adds these tests:
  - a render during each of the five operations;
  - bad-record errors during an operation;
  - equality of the text rendered during and after an apply;
  - an apply that is unchanged by rendering;
  - refusals of looked-up and edited entities from the handler.

  It removes the old refusal test.
- `config_plugin_lifecycle_test.go` shows that an external plugin keeps running while a handler renders during an apply.
- `CHANGELOG.md` has this spec's section. Breaking: none.

## Why

Python tools, starting with the planned pretty logger, need to show each resource as it is created. The engine refused every encode during an operation, although the saved-record render reads none of the entities the guard protects.

## Follow-up

Release this change with a release-size label on the pull request. Cut it before printing-tokens-over-the-engine merges to main, so that xcl-python's engine bump can take it.
