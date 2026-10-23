---
created_date: "2026-10-23"
document_status: final
closed_date: "2026-10-23"
---

# Plan: 20261023074223-e06f835f-event-data-level-enum

<!-- Metadata -->
<!-- Created: 2026-10-23T08:19:11Z -->
<!-- Commit: xcl-python 1aee25c, xcl-website f82b715 -->
<!-- Branch: main (xcl-python), main (xcl-website) -->
<!-- Repository: git@github.com:jumppad-labs/xcl-python.git, jumppad-labs/xcl-website -->

## Overview

Python programs choose how much resource data xcl attaches to events by typing a bare word for `event_data`, which nothing checks until the program runs and which editors cannot complete. This plan adds `xcl.EventDataLevel`, a top-level enum named and shaped like Go's, makes both `xcl.Config` and `xcl.aio.Config` accept only its members, and ships the package as typed, so a mistake is caught by the developer's tools before it runs. Python developers get completion and early errors, and their programs read the same as Go's. The documentation, the parity list and the release notes show the new form.

## Conventions

- **Testing & Mocking: never use table-driven tests** — each refused value (`"processed"`, `"verbose"`, an integer, `None`) and each accepted level gets its own test function for each configuration flavour, with no `pytest.mark.parametrize` table. The xcl-python suite follows the same rules as xcl.
- **Testing & Mocking: never mix positive and negative tests in one function** — the accepted-level tests, the refused-value tests and the two mypy snippets (member accepted, string flagged) are separate functions.
- **Testing & Mocking: favour verbosity over abstraction** — tests spell out the config, the call and the assertion inline rather than sharing a level-looping helper.
- **Testing & Mocking: never write tests that inspect repository files** — no test reads README.md, PARITY.md or the website pages to check they use the enum. Those are checked by human review. The mypy test runs a type checker over a snippet written to a temporary directory, which exercises the installed package's typing surface rather than reading repository text.
- **Testing & Mocking: locate tests with the source they test** — the unit tests go in `tests/unit/` (`test_config.py` for the option, a new `test_typing.py` for the type-checker check), and the engine-backed tests go in `tests/integration/test_sync.py` and `test_aio.py`, next to the existing event-data tests.
- **Dependencies: prefer the standard library and document every third-party dependency** — the enum is the standard library's `enum.StrEnum`. The one new dependency, `mypy`, is test-only, and `pyproject.toml` carries a comment saying why.
- **Include unit tests for all business logic** — the shared validator and its enum-to-wire mapping are unit-tested through both configuration flavours against the fake transport.

## Architecture & Design Decisions

The event data level becomes a public enum, `xcl.EventDataLevel`, and both configuration flavours accept only its members. The enum is an `enum.StrEnum` (available from Python 3.11, the package's minimum) with the members `NONE`, `RAW` and `PROCESSED`, whose values are today's wire words `"none"`, `"raw"` and `"processed"`. It is declared in `xcl-python:xcl/events.py`, beside `Event`, which is where Go keeps its counterpart (`events.DataLevel`, re-exported as `xcl.EventDataLevel` in `xcl:events.go:21-41`), and it is imported and listed in `__all__` in `xcl-python:xcl/__init__.py`, so `from xcl import EventDataLevel` works without a submodule import. Its docstrings follow Go's comments for the three constants, so the two languages describe the levels in the same words.

All checking and encoding sits in one private helper in `xcl-python:xcl/config.py`, which replaces the bare-word map `EVENT_DATA_LEVELS`. The helper takes the value given for `event_data`, refuses it with a `TypeError` unless `isinstance(value, EventDataLevel)`, and otherwise returns the matching protocol value (`core_pb2.EVENT_DATA_NONE`, `_RAW` or `_PROCESSED`). Both `xcl.Config.__init__` and `xcl.aio.Config.__init__` call it **first**, before they resolve a client, build the class map or (for the asyncio flavour) start the polling executor. So a refused value creates no configuration in the engine, starts nothing and leaks nothing. The check has to be on type, not equality: a `StrEnum` member compares and hashes equal to its string, so `"processed" in {...}` or a dict lookup would wave a correctly spelled bare word through. The error message names `xcl.EventDataLevel`, gives a member as an example and shows the refused value with its type, so the reader knows what to pass instead. Both signatures become `event_data: EventDataLevel = EventDataLevel.NONE`. The option is not `Optional`, so leaving it out still means none, and an explicit `None` is refused like any other wrong type. The wire is unchanged: the protocol already carries its own enum (`xcl-python:xcl/_core/core_pb2.pyi:10-17`), so xcl and the engine are untouched.

For a type checker to see the new annotation in a user's program, the package must declare that it is typed. The plan adds an empty `xcl-python:xcl/py.typed` (PEP 561), which the existing wheel and sdist rules already pick up. With it in place, mypy and pyright flag `event_data="processed"` as a `str` passed where `EventDataLevel` is expected, and accept the member. The plan proves this in the test suite by running mypy over two small snippets in a subprocess against the installed package, with `mypy` added to the `test` extra that CI already installs. Pyright is checked by hand. Documentation follows the code: the package README's example and its new short migration note (worded without a bare-string literal, so the docs criterion's search stays clean), the `xcl.Config` docstring, the `PARITY.md` configuration-options row (which pairs Go's `WithEventData(level)` / `xcl.EventDataLevel` with `event_data=xcl.EventDataLevel...`), and the four xcl-website pages that show or describe the option (`events.mdx`, `configuration-text.mdx`, `examples/configuration-only.mdx`, `examples/plugins.mdx`). The breaking change ships as a `minor` release of xcl-python. Its generated GitHub release notes get one added line, written by a person, saying string levels are refused and showing `event_data="processed"` becoming `event_data=xcl.EventDataLevel.PROCESSED`.

Two other shapes were set aside, and research.md#alternatives-considered-and-rejected gives the evidence. A plain `enum.Enum` with integer (protocol) values would make reprs and logs unreadable and expose the wire numbering. Re-exporting the generated protobuf enum would give `int` wrappers named `EVENT_DATA_*`, which is not a Python enum and does not read like Go. Accepting bare words alongside the enum is ruled out by the spec. A `ValueError` (today's behaviour) and an `xcl.Error` subclass were both set aside for a plain `TypeError`, which is the Python convention for an argument of the wrong type and matches the configuration's other built-in argument checks. Following the testing conventions, every behaviour gets its own verbose test function rather than a parametrised table, positive and negative cases are never mixed, and no test reads the README, `PARITY.md` or the website. Those are checked by review.

## Component Breakdown

- **Event data level enum (new, xcl-python).** `xcl.EventDataLevel` is the public, named set of event data levels: `NONE`, `RAW` and `PROCESSED`, whose string values are the words the option took until now. It owns the vocabulary and the per-level descriptions, which follow Go's. It lives in the events module next to `Event`, as Go's does, and the package's top level exports it. A new component is justified: no existing public type names the levels.
- **Event data option check (changed, xcl-python).** The synchronous configuration module held a public map from bare words to protocol values. A private helper replaces it. The helper accepts only an `EventDataLevel` member, refuses anything else with a `TypeError` that names the enum, and returns the protocol value the request carries. It is the one place that checks and encodes the option, and both configuration flavours use it.
- **Synchronous configuration, `xcl.Config` (changed, xcl-python).** Its `event_data` keyword is typed as the enum and defaults to `EventDataLevel.NONE`. It calls the option check before it does anything else, so a refused value never reaches the engine. Its request builder now carries the protocol value the check returned. Its docstring describes the option in terms of the enum.
- **Asynchronous configuration, `xcl.aio.Config` (changed, xcl-python).** It has the same keyword, type and default, and it runs the same check first, before it starts its polling executor or resolves a client. It still builds its request through the synchronous configuration's builder.
- **Typed-package marker (new, xcl-python).** An empty PEP 561 marker shipped inside the package tells mypy and pyright to read the package's own annotations in a user's program. The option's type only reaches users through this marker. The existing wheel and sdist rules package it with no build change.
- **Test dependencies (changed, xcl-python).** The `test` extra gains `mypy`, so the suite can run a type checker over snippets that use the installed package. CI already installs that extra.
- **Package documentation: README and parity list (changed, xcl-python).** The README's events example uses the enum. A short migration note says a string is now refused and names the member to pass instead. It has no bare-string code, so a search of the README for a string level finds nothing. The parity list's configuration-options row pairs Go's `WithEventData(level)` and `xcl.EventDataLevel` with `event_data=xcl.EventDataLevel...`.
- **xcl.dev pages (changed, xcl-website).** Four pages show the option: the events page's Python tab and its prose list of levels, the configuration-text page's prose, and the Python tabs of the configuration-only and plugins examples. All of them change to the enum and keep the wording the library docs use.
- **Release notes (changed, outside the repositories).** The GitHub release of the xcl-python version that ships this change is generated from pull-request titles. A person adds one line to it saying string levels are refused, with the string-to-enum example. The pull request carries the `minor` release label, the pre-1.0 bump for a breaking change.
- **Engine and protocol (unchanged, xcl).** The protocol already carries the level as its own enum, so xcl, the engine and the generated messages stay as they are.

## Data Structures & Interfaces

**`xcl.EventDataLevel` (new public enum).** The named set of event data levels, the Python counterpart of Go's `xcl.EventDataLevel`. It is a string-valued enum, so a member's value is the word the option used to take and its repr reads `<EventDataLevel.PROCESSED: 'processed'>`.

```python
class EventDataLevel(enum.StrEnum):
    NONE = "none"            # no resource data on any event (the default)
    RAW = "raw"              # the resource as it was before the provider ran, on every lifecycle event
    PROCESSED = "processed"  # on a success event, the resource as state records it, masked
```

**The `event_data` keyword of `xcl.Config` and `xcl.aio.Config` (changed contract).**

```python
def __init__(self, *, ..., event_data: EventDataLevel = EventDataLevel.NONE, ...) -> None
```

Only an `EventDataLevel` member is accepted. Anything else is refused with a `TypeError` before any other work is done, and no configuration is created. That covers any `str`, including one spelled like a member, any `int`, an explicit `None` and any other object. The message names `xcl.EventDataLevel`, gives a member as an example and shows the refused value and its type, for example:

```
event_data must be an xcl.EventDataLevel, such as xcl.EventDataLevel.PROCESSED, not 'processed' (str)
```

**The internal option check (changed, private).** It replaces the public map `xcl.config.EVENT_DATA_LEVELS` from bare words to protocol values. It is a private function that both flavours call, mapping a member to the protocol's own `EventDataLevel` value (`EVENT_DATA_NONE`, `EVENT_DATA_RAW`, `EVENT_DATA_PROCESSED`). It is not part of the public API.

**Serialization boundary (unchanged).** `CreateConfigRequest.event_data` keeps the protocol enum and its numbering, so the engine receives exactly what it received for the same level before. No protocol message, field or generated file changes.

**Typed-package marker (new file, no contents).** `py.typed` inside the package declares that the package's inline annotations are authoritative for type checkers (PEP 561).

## Implementation Detail

**A new pattern for the package: options typed as an enum.** Until now every option of the configuration has been a plain Python value (a bool, an int, a str, a mask object). The event data level is the first that takes a dedicated enum, and it sets the shape any later conversion would follow, though converting other options is out of scope. The shape has four parts. The enum is a `StrEnum` named after Go's type. It is exported from the package's top level. The keyword is annotated with it and defaults to a member. A runtime check refuses anything that is not a member, by type, because a `StrEnum` member is equal to its string.

**Check first, then build.** Both configuration constructors gain one line at the very top that validates and encodes the level. Today the check sits in the middle, after the client, the class map, the functions and (for the asyncio flavour) the polling executor have been set up. Moving it first means a refused level costs nothing and leaves nothing behind. The existing `state_path`/`state_store` check stays where it is: moving it is unrelated to this spec. The encoded protocol value is passed into the shared request builder, so the builder no longer looks anything up.

**One check shared by both flavours.** The asyncio configuration already imports helpers from the synchronous configuration module and builds its request through the synchronous builder. The new check follows that existing pattern: a private function in the synchronous module that both constructors call. No new module is introduced.

**What a developer sees.** Code that passed `event_data="processed"` now fails at construction with a `TypeError` that names `xcl.EventDataLevel` and shows the member to use instead. A type checker reports the same mistake at the call site before the program runs. Editors complete `xcl.EventDataLevel.` to the three members. Reprs and logs still show the readable word, and Python and Go programs now read alike: `event_data=xcl.EventDataLevel.PROCESSED` beside `xcl.WithEventData(xcl.EventDataProcessed)`.

**The package becomes typed for its users.** Shipping the PEP 561 marker means every public annotation in the package, not only this option, is now read by users' type checkers. That is the intended effect, and the new typing test exercises it end to end against the installed package.

**Existing patterns followed.** The enum's docstring describes each member the way the Go constants' comments do, as the package's other public names mirror Go's documentation. Tests follow the suite's existing split. Unit tests run against the fake transport and assert on the serialised request and the calls made. Integration tests apply a one-resource configuration against the real engine and inspect the delivered events. Each case gets its own test function.

## Dependencies

- **Design documents this plan was built on: none.** The spec references no design documents.
- **Python standard library `enum.StrEnum`** (runtime, Python 3.11+). Provides the string-valued enum. The package already requires 3.11, so nothing changes.
- **The protocol's `EventDataLevel` enum in the generated CoreService messages** (runtime, existing). Carries the level to the engine. It is used as it is, with no regeneration and no xcl change.
- **Go's `xcl.EventDataLevel` and `xcl.WithEventData`** (reference, existing, xcl). These set the name, the members and the member descriptions the Python enum mirrors, and the Go side of the parity row. No change.
- **mypy** (new test-only dependency, added to the `test` extra). Runs the type-checker test over user-style snippets. It is not a runtime dependency, and pyproject records why it is there.
- **The xcl-python release flow** (existing: release label on the PR, auto-tag on merge, GitHub release with generated notes). Ships the change as a minor version. The only addition is a person editing the published release notes, with no change to the workflows.
- **Upstream specs or plans that must land first: none.** This spec has no dependencies in its epic. It can be implemented on xcl-python's and xcl-website's main lines against the engine already pinned in `xcl-ref.txt`.
- **Downstream: `20261023075328-741138ee-python-prettylog`** depends on this spec and will use `xcl.EventDataLevel.PROCESSED`. That spec and the Python printing specs may also edit the xcl-website example pages this plan touches, the configuration-only and plugins examples. Whichever lands second rebases onto the first.

## Testing Approach

Three kinds of test cover the change. Each follows the xcl-python suite's existing layout and the project's testing conventions: one behaviour per test function, no parametrised tables, and positive and negative cases always in separate functions.

**Unit tests against the fake transport (most coverage).** The option check is the only logic this change adds, so it gets the most tests, each run through both configuration flavours.
- The enum imports from the package's top level and has exactly three members, `NONE`, `RAW` and `PROCESSED`, whose values are `"none"`, `"raw"` and `"processed"`.
- Each member puts the matching protocol value on the `CreateConfig` request, for the synchronous and the asynchronous configuration. Leaving the option out sends the none level.
- Refusal: the string `"processed"`, the string `"verbose"`, an integer and an explicit `None` each raise `TypeError` whose message names `xcl.EventDataLevel`, and no `CreateConfig` call is made. There is one test function per value per flavour.
- The existing docstring check keeps working with the new public name, and the enum carries its own docstring.

**Integration tests against the real engine.** These prove a member reaches the engine and changes the events delivered. They apply a one-resource configuration, as the existing event tests do.
- Processed, synchronous and asynchronous: the create success event's entity is the created resource. The existing processed-level tests switch to the enum and carry this guarantee for both flavours, along with every other existing processed test (sensitive redaction, per-config classes).
- Raw: the create events carry the resource's data, as the raw level does.
- Option left out, and the none level: no event carries any resource data. Each has its own test.

**A type-checker test.** It runs mypy in a subprocess over two snippets written to a temporary directory, so mypy resolves the installed package, as it would in a user's program.
- Passing `"processed"` as `event_data` to `xcl.Config` and to `xcl.aio.Config` is reported as an error on each of those lines, naming `EventDataLevel`.
- The same snippet passing `xcl.EventDataLevel.PROCESSED` type-checks cleanly.
- Together these guarantee that the typed-package marker ships and that the annotation is what users' tools see.

**Deliberate gaps.**
- No test reads the README, the parity list or the xcl.dev pages, because the project forbids tests that inspect repository files. Those are reviewed by a person.
- The engine and protocol are unchanged, so no Go tests are added.
- The raw-level guarantee "matches what the string `"raw"` delivered before this change" is carried two ways. The unit tests prove `RAW` puts the same protocol value on the request the string did. The integration test proves that value delivers resource data on events.

**Success metrics.** The spec defines no post-delivery success metric; its acceptance criteria are all covered above or by the manual checks below.

**Manual checks.**
- **Manual — captured in the implementation test plan:** pyright run over the same two snippets flags the string and accepts the member. mypy is automated; pyright is not added as a dependency.
- **Manual — captured in the implementation test plan:** review that the package README and every xcl.dev page that shows or describes the event data option use `xcl.EventDataLevel` members, with no bare string level left, and that the README's migration note says strings are refused and names the members, without a bare-string code example.
- **Manual — captured in the implementation test plan:** review that the parity list's event data row names Go's `xcl.EventDataLevel` (with `WithEventData`) and the Python enum.
- **Manual — captured in the implementation test plan:** check that the GitHub release notes of the xcl-python version that ships this change contain a line saying string levels are refused, with the `event_data="processed"` → `event_data=xcl.EventDataLevel.PROCESSED` example.

## Milestones & Tasks

### Milestone 1: Python programs choose event data from `xcl.EventDataLevel`

**What changes**: A Python program imports `xcl.EventDataLevel` from the package's top level and passes `NONE`, `RAW` or `PROCESSED` as `event_data` to `xcl.Config` or `xcl.aio.Config`. Each member attaches exactly the event data its word attached before, and leaving the option out still means none. A plain string is refused at construction with a `TypeError` naming the enum, even one spelled exactly like a level. So are an integer, an explicit `None` and anything else that is not a member. Nothing reaches the engine and no configuration is created. The package now declares itself typed, so mypy and pyright flag a string passed as the level in a user's program and accept the member. Editors complete the three members.

**Validation point**: xcl-python's unit tests pass. They cover the enum's members, each member's wire value for both flavours, the default, and each refused value for both flavours with no configuration created. The mypy test flags the string snippet and passes the member snippet. The sync and asyncio integration suites pass against the pinned engine, with the processed, raw, none and left-out event tests.

#### - [ ] Task: Add the EventDataLevel enum and accept only its members
**Id:** a437ae0f-6f47-452c-be33-5b43f0e4985a
**Repo:** xcl-python
**Depends on:** none
**Execution:** agent

Adds the public `xcl.EventDataLevel` enum, with `NONE`, `RAW` and `PROCESSED` whose values are today's words, and exports it from the package's top level. It replaces the bare-word map with one private check that both `xcl.Config` and `xcl.aio.Config` run before anything else. The check accepts only a member and refuses every other value with a `TypeError` naming the enum, so a refused value creates no configuration and starts nothing. The default becomes `EventDataLevel.NONE`. Existing tests that passed the string `"processed"` switch to the member, and new unit tests cover the members, the wire value of each level, the default and every refused value, for both flavours.

*Technical detail:* [context.md#task-add-the-eventdatalevel-enum-and-accept-only-its-members](./context.md#task-add-the-eventdatalevel-enum-and-accept-only-its-members)

**Acceptance criteria**:
- [ ] `from xcl import EventDataLevel` works, and the enum has exactly three members, `NONE`, `RAW` and `PROCESSED`, with values `"none"`, `"raw"` and `"processed"`.
- [ ] Each member, given to either configuration, sends the engine the same protocol level the matching string sent before, and leaving the option out sends none.
- [ ] Giving either configuration the string `"processed"`, the string `"verbose"`, an integer or an explicit `None` raises a `TypeError` whose message names `xcl.EventDataLevel`, and no configuration is created in the engine.
- [ ] The `xcl.Config` docstring describes the option in terms of the enum, and the enum's docstring describes each level as Go's documentation does.
- [ ] No test or library code in the package still passes a bare string as the event data level, and the existing processed-level integration tests pass using the member.

#### - [ ] Task: Prove each level against the engine
**Id:** bec34d15-ac04-481c-a27b-35029e2897a8
**Repo:** xcl-python
**Depends on:**
- a437ae0f-6f47-452c-be33-5b43f0e4985a — Add the EventDataLevel enum and accept only its members
**Execution:** agent

Adds integration tests that apply a one-resource configuration against the real engine and check what each level puts on the delivered events. The processed level, for both flavours, is already covered by the existing entity tests switched to the member in the previous task. This task adds the raw level, the none level and the option left out, so every level's effect on events is pinned down end to end.

*Technical detail:* [context.md#task-prove-each-level-against-the-engine](./context.md#task-prove-each-level-against-the-engine)

**Acceptance criteria**:
- [ ] With `EventDataLevel.RAW`, the create events for the applied resource carry that resource's data.
- [ ] With the option left out, no delivered event carries any resource data.
- [ ] With `EventDataLevel.NONE`, no delivered event carries any resource data.
- [ ] With `EventDataLevel.PROCESSED`, the create success event's entity is the created resource, for both `xcl.Config` and `xcl.aio.Config`.

#### - [ ] Task: Declare the package typed and check the option with mypy
**Id:** 0ecb27ac-960f-401c-9d68-60f3f6284ca1
**Repo:** xcl-python
**Depends on:**
- a437ae0f-6f47-452c-be33-5b43f0e4985a — Add the EventDataLevel enum and accept only its members
**Execution:** agent

Ships the PEP 561 marker that tells type checkers to read the package's own annotations. Without it, a user's mypy run cannot see that `event_data` takes the enum. Adds mypy to the test dependencies with a note saying why. Adds tests that run mypy over two small user-style programs against the installed package: one passes the string and must be flagged, the other passes the member and must be clean.

*Technical detail:* [context.md#task-declare-the-package-typed-and-check-the-option-with-mypy](./context.md#task-declare-the-package-typed-and-check-the-option-with-mypy)

**Acceptance criteria**:
- [ ] The built wheel and sdist contain the typed-package marker.
- [ ] mypy run over a program passing `"processed"` as `event_data` to `xcl.Config` and to `xcl.aio.Config` reports an error on each of those lines that names `EventDataLevel`.
- [ ] mypy run over the same program passing `xcl.EventDataLevel.PROCESSED` reports no errors.
- [ ] The test dependencies include mypy, with a comment explaining it is used only to check the package's typing as users see it.

### Milestone 2: Documentation, parity list and release notes show the enum

**What changes**: Every place a reader learns about the event data option shows the enum. In xcl-python, that is the package README (with a short note that strings are refused and the matching member is passed instead) and the parity list's configuration-options row, which now pairs Go's `xcl.EventDataLevel` with the Python enum. On xcl.dev, it is the events page and the configuration-text page, and the Python code of the configuration-only and plugins examples. The change ships as a minor xcl-python release. That release's notes say plain string levels are no longer accepted and show the change from `event_data="processed"` to `event_data=xcl.EventDataLevel.PROCESSED`.

**Validation point**: A search of the README, the parity list and the website sources for the event data option finds no bare string level. The website builds. The published GitHub release of the shipping xcl-python version carries the breaking-change line with the migration example.

#### - [ ] Task: Show the enum in the package README and parity list
**Id:** 1f94d121-4e53-4194-9bab-a2b29f16a79c
**Repo:** xcl-python
**Depends on:**
- a437ae0f-6f47-452c-be33-5b43f0e4985a — Add the EventDataLevel enum and accept only its members
**Execution:** agent

Updates the README's events example to pass `xcl.EventDataLevel.PROCESSED`. Adds a short migration note. It says `event_data` now takes only an `xcl.EventDataLevel` member, that a string is refused, and that code which passed a word passes the member of the same name instead. It is worded without a bare-string literal, so a search of the README for one finds nothing; the string-to-enum example itself goes in the release notes. Changes the parity list's event data row to pair Go's `WithEventData(level)` and `xcl.EventDataLevel` with the Python keyword and enum. That way the Python and Go documentation read the same.

*Technical detail:* [context.md#task-show-the-enum-in-the-package-readme-and-parity-list](./context.md#task-show-the-enum-in-the-package-readme-and-parity-list)

**Acceptance criteria**:
- [ ] The README shows no bare string event data level, and its events example uses an `xcl.EventDataLevel` member.
- [ ] The README has a migration note stating that string levels are refused and that the member of the same name is passed instead, with no bare-string example.
- [ ] The parity list's event data row names Go's `xcl.EventDataLevel` and `WithEventData`, and the Python `xcl.EventDataLevel` with its three members.

#### - [ ] Task: Show the enum on the xcl website pages
**Id:** a5c9e273-cc56-4cd5-a241-ee35aadadda8
**Repo:** xcl-website
**Depends on:**
- a437ae0f-6f47-452c-be33-5b43f0e4985a — Add the EventDataLevel enum and accept only its members
**Execution:** agent

Changes every Python snippet and every description of the event data option on xcl.dev to the enum. That covers the events page's Python tab and its prose list of levels, the configuration-text page's prose, and the Python code of the configuration-only and plugins examples. The Go tabs already use `xcl.EventDataProcessed` and stay as they are.

*Technical detail:* [context.md#task-show-the-enum-on-the-xcl-website-pages](./context.md#task-show-the-enum-on-the-xcl-website-pages)

**Acceptance criteria**:
- [ ] No page on the site shows or describes a bare string event data level.
- [ ] The events page names the three members of `xcl.EventDataLevel` where it lists the Python options.
- [ ] The site builds with the changed pages.

#### - [ ] Task: Release xcl-python with the breaking change in its notes
**Id:** 324e4716-bbe5-4874-86b2-563067aa3c4f
**Repo:** xcl-python
**Depends on:**
- a437ae0f-6f47-452c-be33-5b43f0e4985a — Add the EventDataLevel enum and accept only its members
- bec34d15-ac04-481c-a27b-35029e2897a8 — Prove each level against the engine
- 0ecb27ac-960f-401c-9d68-60f3f6284ca1 — Declare the package typed and check the option with mypy
- 1f94d121-4e53-4194-9bab-a2b29f16a79c — Show the enum in the package README and parity list
**Execution:** human — releasing the package and editing its published GitHub release are actions outside the repo

Merges the xcl-python pull request with the `minor` release label, the pre-1.0 bump for a breaking change, so the tag workflow publishes the next minor version. Once the release exists, adds a line to its generated notes. The line says plain string event data levels are no longer accepted and shows the one-line change from `event_data="processed"` to `event_data=xcl.EventDataLevel.PROCESSED`.

*Technical detail:* [context.md#task-release-xcl-python-with-the-breaking-change-in-its-notes](./context.md#task-release-xcl-python-with-the-breaking-change-in-its-notes)

**Acceptance criteria**:
- [ ] The xcl-python pull request merged with the `minor` label, and the next minor version is published.
- [ ] That version's GitHub release notes contain a line stating that string event data levels are refused, with the string-to-enum migration example.

## Open Questions

There are no open questions left for implementation time. The one uncertainty found during research was whether mypy would resolve the editable install and flag a bare string once the typed-package marker ships. It was settled by an experiment during planning, recorded in research.md under Open assumptions. If CI's install turns out to behave differently from the local editable install, the implementer must STOP and ask, rather than point mypy at the source tree.

## Out of Scope

- **New event data levels.** Only `NONE`, `RAW` and `PROCESSED` exist, matching Go (spec Non-Goals).
- **Turning other string-valued options into enums.** This plan sets the pattern, but each other option would be its own change (spec Non-Goals).
- **A TypeScript counterpart.** There is no TypeScript package yet (spec Non-Goals).
- **Any change to the engine, its protocol or the Go API.** The protocol already carries the level as its own enum, and xcl is untouched (spec Non-Goals).
- **A deprecation period or string fallback.** Strings are refused immediately, as the spec's Constraints require.
- **Changing the release workflow to write release notes from a file.** The breaking-change line is added by hand to the generated notes of this one release.
- **An entry in xcl's CHANGELOG.** Nothing in xcl changes, and xcl-python keeps no changelog file. The README migration note and the release notes carry the break.
- **Fixing type errors the new typed-package marker may surface elsewhere in users' programs or in the package's own annotations.** Only the `event_data` option's typing is checked. Any other annotation problem a user's type checker reports would be a separate fix.
- **Moving the configuration's other argument checks** (such as `state_path` with `state_store`) ahead of client setup, or changing their exception types.
