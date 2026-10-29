---
created_date: "2026-10-26"
document_status: final
closed_date: "2026-10-26"
---

# Event data levels as an enum in Python

## What was built

Python programs now choose how much resource data xcl attaches to events with `xcl.EventDataLevel`, an enum exported from the top of the `xcl` package. Its name and its members, `NONE`, `RAW` and `PROCESSED`, match Go's `xcl.EventDataLevel`. `xcl.Config` and `xcl.aio.Config` take `event_data: xcl.EventDataLevel = xcl.EventDataLevel.NONE` and accept only a member. A string, even one spelled exactly like a level, raises a `TypeError` that names `xcl.EventDataLevel` and shows the member to pass instead. So do an integer, `None` and any other value. The check runs before anything else in either constructor, so a refused value creates no configuration in the engine and starts no polling thread. The enum is a `StrEnum` whose values are the old words, so reprs and logs stay readable. The check is on type, not equality, because a `StrEnum` member compares equal to its string. The wire is unchanged: each member sends the same protocol level its word did.

The package now ships a PEP 561 `py.typed` marker, so mypy and pyright read its annotations in users' programs. They flag a string passed as the level and accept the member.

Documentation follows the code:
- The xcl-python README's events example uses `xcl.EventDataLevel.PROCESSED`.
- A new "Migrating from string event data levels" section explains the break.
- The parity list pairs Go's `WithEventData(level)` and `xcl.EventDataLevel` with the Python enum.
- On xcl.dev, the events page, the configuration-text page and the configuration-only and plugins examples use the enum.

Tests cover the new behaviour:
- Unit tests check each member's wire value, the default, and each refused value for both configuration flavours.
- Engine-backed tests check the raw, none, left-out and processed levels.
- A test runs mypy over user-style programs against the installed package.

## Why it matters

Until now the level was a bare word that nothing checked until the program ran, and editors could not complete it. Now a developer picks a level from a list, their tools catch a mistake before the program runs, and Python and Go programs read the same: `event_data=xcl.EventDataLevel.PROCESSED` beside `xcl.WithEventData(xcl.EventDataProcessed)`.

## Deviations from the plan

- The internal request builder takes the protocol's own `EventDataLevel` type rather than a plain `int`, so the call into the generated message type-checks without an ignore.
- The asyncio refusal tests assert that no engine call is made at all, which is stronger than the planned "no CreateConfig call".
- The human task "Release xcl-python with the breaking change in its notes" was taken out of the run. The user chose this, because the release needs this change merged first. It is recorded in the plan as a descoped requirement: a person merges the xcl-python pull request with the `minor` label, then adds a line to the generated GitHub release notes saying string levels are refused, with `event_data="processed"` → `event_data=xcl.EventDataLevel.PROCESSED`. The test plan carries this as a manual check, along with a pyright check that was verified locally.
