---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Render saved records while an operation runs

## What was built

Through xcl's engine, programs written in other languages than Go can now render a resource's saved record as configuration text while an operation is still running, including from their event handler. Before, the engine refused every encode call until the operation ended.

- **xcl (engine).** The core service's `EncodeSavedEntity` no longer checks whether an operation is running. It reads none of the configuration's entities: it types the record through the plugin catalog and renders it. So it is accepted during Validate, Apply, Diff, Destroy and Load. `EncodeEntity` and every other lookup still refuse with `operation_in_progress`, because they read entities an operation replaces when it ends. An empty, invalid or unrecognised record keeps its specific error (`invalid_argument`, `invalid_saved_data`, `unregistered_type`) during an operation. The protocol documents the call as allowed during an operation. The changelog has this spec's section, and the other-language design `design:libxcl.md` was revised: its error table, threading rules, service block and encode paragraph.
- **xcl tests.** These core-service tests prove the behaviour:
  - a render during each of the five operations, each held at its first host state task;
  - bad-record errors during an operation;
  - text from a create success handler that equals the text after the apply;
  - an apply that is unchanged when its handler renders every created record;
  - refusals of looked-up and edited entities from the handler.

  A root-package test shows that an external plugin keeps running while a handler renders during an apply, and is stopped after it. That was the spec's flagged risk, and no library change was needed.
- **xcl-python.** No package code changed: both clients already exposed `encode_saved_entity`. New integration tests cover the synchronous and asyncio clients. They render from a create success handler during an apply with a Python state store, and check:
  - the during/after text equality;
  - the unchanged apply;
  - `OperationInProgressError` for looked-up and edited entities;
  - `InvalidSavedDataError` and `UnregisteredTypeError` for bad records.

  Docstrings and the README now say when each encoder may run, with an example that prints each created resource beneath its success event.
- **xcl-website.** The configuration-text page and the events page now say that saved records render at any time, including from an event handler during an operation, and that looked-up entities do not. They include Go and Python handler examples.

## Why it matters

Tools built on xcl in Python, starting with the planned pretty logger, can show each resource's configuration as it is created rather than all at the end. Go programs already could. No buffering is needed, and the operation runs exactly as before.

## Deviations from the plan

- **Release tasks descoped.** The human task to release xcl, and the agent task to re-run the Python suites against the released engine, are follow-up work done by a person after the merge. They are recorded in the plan's descoped list and in the test plan, including the epic's release-order constraint: cut this xcl release before printing-tokens-over-the-engine merges to xcl main. Until xcl-python's engine pin names that release, its new tests fail against the pinned `v0.11.1` engine.
- **No development pin.** `xcl-ref.txt` was not pointed at the xcl feature branch. The tests ran locally against an engine, library and plugins built from the xcl worktree.
- **Protocol comment.** The rpc comment does not reach the generated Go code, because buf generates only messages. The criterion was reworded.
- **External-plugin test.** The test applies two dependent widgets, so events follow the rendered create success. With one widget the apply can release the plugins during the render.
- **Unchanged-apply test.** The core-service test also renders the variable and the output, and asserts `not_encodable` for them.
- **Design revision.** It was staged by the implementer and stored by the orchestrator after the user approved it.
- **Website.** A Go handler example was added beside the Python one.
