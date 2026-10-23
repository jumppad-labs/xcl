# Working context: epic 20261023074219-4357d314-printing-and-pretty-output

## How the user wants this run

- **Unattended.** The user: "I do not want to be asked every thirty seconds about my feelings on every individual sections of the spec process. I would like you to do it, I can do something else then I review later." Drive every step from the conversation; the user reviews all specs afterwards. (hivecommons/spektacular#94.)
- Large documents: write and link, do not print in the terminal (hivecommons/spektacular#93).
- Docs ship inside each spec (xcl-website, README, PARITY.md); no separate docs spec.
- Keep process notes and user quotes out of spec text (reviewer flagged them as leaks); provenance lives here.

## Specs, in order

1. 20261023074223-e06f835f-event-data-level-enum (xcl-python) — done.
2. 20261023074627-cc23fc08-encode-saved-entity-during-operation (xcl core service, pin bump in xcl-python) — no dependencies.
3. printing-and-highlighting-core (xcl Go) — built to design/printing-and-highlighting.md.
4. printing-and-highlighting-python (xcl-python) — depends on 3.
5. python-prettylog (xcl-python `xcl.prettylog`) — depends on 1, 2, 4.
TypeScript is in the design only; no package yet.

## Decisions and the user's words

- EventDataLevel: "an enum that is on the top level package"; "Enum only". Name after Go's xcl.EventDataLevel. Explicit None refused (reviewer raised it; kept strict, matches enum only).
- Encode guard: user agreed to remove it from EncodeSavedEntity only, keep on EncodeEntity, after asking "I don't understand why the guard is there in the first place". Blocking in on_event deadlocks.
- Error report regression since v0.8.0 (ParserError removed); user: "half done ... provides half the utility".
- Highlight API: user dislikes renderer wrapping as non-idiomatic; wants renderers "a core feature for folks wanting to extend xcl"; custom colours, HTML, SVG. Design captured (final): native formatters, report tokens on every Error, chroma-style style strings, two-level kinds, HTML classes by default (from the vault-router project), HTMLInline, CSS from theme, no slog.LogValuer for now.
- Loggers: "make sure that the adaptions is a simple as possible for the end user".
- Pretty logger: like Go's example/prettylog, part of the xcl Python package, stdlib only.

## Facts

- xcl-python bundles the engine pinned in xcl-ref.txt (v0.11.1).
- libxcl design's lookups section says every encode is refused mid-operation; needs a revision once spec 2 lands (flag to user, needs their agreement).
- xcl-website problems page says "The report is not yet available from Python".

## Planning run (spek-plan-epic, 2026-10-23)

- Epic: 20261023074219-4357d314-printing-and-pretty-output, 6 specs, none planned at start.
- Started round 1: event-data-level-enum, encode-saved-entity-during-operation, printing-and-highlighting-core.
- Questions to the user and answers: (none yet)
- event-data-level-enum: DONE (summary kept in session scratchpad; rebuild from plan docs if lost). Reported knowledge candidates: py.typed gotcha, StrEnum isinstance gotcha, xcl-python release notes are generated (no changelog).
- encode-saved-entity-during-operation: DONE. Knowledge candidates: core service events are fire-and-forget (host state store holds an operation, useful for tests); child believes learnings/releases-tagged-by-hand.md is out of date with the automated release chain (raise with user, do not overrule).
- printing-and-highlighting-core: DONE. Knowledge candidates: errors cannot import highlight (use internal/report); example/prettylog tests force colour via CLICOLOR_FORCE so highlight.For cannot replace lipgloss check; Config.Validate returns *errors.ConfigError unwrapped.
- Started round 2: printing-tokens-over-the-engine.
- printing-tokens-over-the-engine: DONE. Flag for user: buf breaking rule change (FIELD_NO_DELETE -> UNLESS_NUMBER/NAME_RESERVED). Cross-plan risk: engine auto-bump into xcl-python breaks highlight=True tests until Python spec lands. Knowledge candidate: buf FIELD_NO_DELETE gotcha.
- Started round 3: printing-and-highlighting-python.
- printing-and-highlighting-python: DONE, but its plan parks a contradiction of spec criterion "Ordinary messages stay one line" (str(err) for multi-problem ConfigError stays one line per problem, matching Go). Put to user as a question; answer pending.
  Knowledge candidate: xcl-python CI regenerates stubs from the pinned xcl checkout, so an engine release that reserves/removes a field breaks the engine-bump PR until the Python code using it is removed in the same change.
- Started round 4: python-prettylog.
- Q (python printing): str(err) for multi-problem ConfigError? User: "Match Go" (one line per problem; criterion read as "report never in str(err)"). Applied to plan by child.
- python-prettylog: DONE (first pass). Q1 render inside handler? User: allow it (waiting on the operation is what's forbidden). Q2 reports above info? User: show reports ("no configuration" = no resource configuration). Child is applying both. Asyncio in-order delivery change left for the review.
- Knowledge candidates (prettylog): aio delivers events as concurrent tasks (out of order); Go %v float exponent + charmbracelet quoting rules; unsupported block yields parse error + closing operation error (report shown twice unless de-duplicated).
- Cross-plan decisions settled: encode release before tokens merges (fallback: ship with Python printing release); prettylog release adds GitHub notes line on asyncio ordering. Applied to plans; epic summary written; epic order added 3 deps (encode->event-data, core->event-data, core->encode).
- Remaining: end-of-planning review with user.
- Review: user accepted all plans ("Looks right"), incl. buf rule change, asyncio ordering, libxcl.md revisions. User asked to save all nine knowledge discoveries via spek-knowledge.
- Knowledge: nine entries written (xcl and xcl-python stores).
