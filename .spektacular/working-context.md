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
