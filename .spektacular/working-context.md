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

## Implement run (spek-implement-epic, 2026-10-26)

- Epic: 20261023074219-4357d314-printing-and-pretty-output; 6 specs planned, 0 implemented at start; dirty: false.
- xcl main checkout is on branch f-python-prettylogger (not main); xcl-python and xcl-website on main.
- Spektacular log.level set to verbose at the user's request (debug is not a valid value).
- Each plan has human release tasks (specs 1, 2, 5, 6); release order constraint: spec 2's xcl release before spec 4 merges to xcl main.
- Questions to the user and answers: (none yet)
- Started spec 1 child (event-data-level-enum), worktrees under .spektacular/worktrees/20261023074223-e06f835f-event-data-level-enum/, span 4488470995b4ae4cb320a32012a990f9-237a8f5bb2fefa2a.
- Q (spec 1, human release task blocks the run: workflow has no step to finish a human task). User: "Follow-up, all specs" — option A for specs 1, 2, 5 and 6: move each release task out of the Milestones & Tasks checklist into **Descoped requirements** (done by a person after the merge), keep it in the manual checks, and carry on without stopping. Tell each later child this up front.
- Q (commit approval; user's global CLAUDE.md forbids commits unless asked). User: "Yes, all specs" — each child may commit its spec on spek/<spec> at the finished step (message without Claude-Session trailer), and the orchestrator merges each finished spec locally into xcl f-python-prettylogger, xcl-python main, xcl-website main. No push, no PRs. Tell each later child this up front.
- event-data-level-enum: DONE and MERGED (xcl-python 60bb3ec, xcl-website e550204 on spek branch, merged). Knowledge: xcl-python learnings/test-suite-in-a-spec-worktree.md (new), gotchas/py-typed-marker.md (updated). Outstanding for a person: minor-labelled xcl-python release + release-notes line.
- Note for children: commit message must include a `Spec: <spec>` line (CLI refuses otherwise).
- Started spec 2 child (encode-saved-entity-during-operation), span 899a2a5483be4163db97c7c81d02b2ec-0269468e9c140916; told it the release-descope and commit decisions.
- Q (spec 2, task 4: plan requires revising design:libxcl.md; child may not write designs). User approved the staged revision; orchestrator stored it with `design author --spec`. User also chose for LATER specs: apply planned design revisions the spec/plan already requires without stopping, and show the full change in the progress message; anything changing a design's intent still stops and asks.
- encode-saved-entity-during-operation: DONE and MERGED (xcl b685b86, xcl-python 6e5549e, xcl-website b312ce5). Descoped: xcl release + "run Python suites against released engine". xcl-ref.txt stays v0.11.1, so new Python tests fail against it until the engine bump. Knowledge: xcl gotchas/rpc-comments-not-in-generated-go.md, xcl gotchas/builtin-records-not-encodable.md (prettylog must skip variable/output records), xcl-python gotchas/aio-state-tasks-run-beside-handlers.md.
- Started spec 3 child (printing-and-highlighting-core), span c744933e1c2a0a5d50794eb962c3c8bc-892c4f16cb0e210a; told it release/commit/design-revision decisions.
- printing-and-highlighting-core: DONE and MERGED (xcl 41ab110, xcl-website fc3efd0). No descopes. Note: core service source/source_start details now carry context lines (Python receives them). Knowledge: xcl architecture/reference-outputs-are-a-contract.md, gotchas/style-strings-cannot-reset.md (design style syntax can't express reset), learnings/loggers-and-error-reports.md.
- Started spec 4 child (printing-tokens-over-the-engine), span 988ce278b1acb55dfd4867c68896e502-c482bc14fc264280.
- spec 4: stored planned libxcl.md revision (Error.report, ConvertTheme, diff_tokens, encode tokens + reserved highlight, Labelled text section, buf rule swap) under the pre-approval; diff kept at scratchpad/libxcl-spec4.diff.
- printing-tokens-over-the-engine: DONE and MERGED (xcl 89fa062). Knowledge: xcl gotchas/buf-field-no-delete.md updated. Release constraint recorded in test plan/changelogs/CHANGELOG.md. CI buf 1.50.0 not yet run (local 1.27.2).
- Started spec 5 child (printing-and-highlighting-python), span f609eddec5bb893b011d3f133a74644d-1e9f3caeac0a7df5.
- printing-and-highlighting-python: DONE and MERGED (xcl 9a9fb36, xcl-python 56675a6, xcl-website 984e06c). Descoped release task; xcl-ref.txt stays v0.11.1 (pin bump must land with this branch, CI regenerates stubs). Knowledge: xcl-python gotchas/theme-typing-mapping-keys.md (new), learnings/test-suite-in-a-spec-worktree.md (updated: GITHUB_TOKEN, XCL_SRC), gotchas/error-event-report-once.md (new).
- Started spec 6 child (python-prettylog), span 4dfb8c2d2dd102b5ef30197da72d4eb6-630ce842558402f0.
- python-prettylog: DONE and MERGED (xcl 326872f, xcl-python 14b14f9, xcl-website 13ddcd5). Descoped release task. Knowledge (xcl-python): updated aio-events-out-of-order, aio-state-tasks-run-beside-handlers, matching-go-log-lines, test-suite-in-a-spec-worktree; new gotchas/test-module-basenames-unique.md.
- EPIC COMPLETE: 6/6 implemented and merged; no worktrees left. Nothing pushed.
