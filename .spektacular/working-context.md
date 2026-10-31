# Working context: 20261031080136-264e0eab-typescript-core-sdk

Epic: 20261031080131-8c2a58ec-typescript-sdk (created 2026-10-31).

## Motivation (user's words)

- "start looking at building a typescript language similar to the python one. We should create a spec that defines the features based on previous specs for python."
- "We should also create a design document that would show the api and some code snippets."
- "One thing that I find is missing from the python is the SDK docs, they don't seem to exist. We have examples but no SDK so we should look at that too"

## Decisions (user answers)

- Organisation: new epic "TypeScript SDK" with four specs: (1) TypeScript core SDK (this spec), (2) TS printing, highlighting and pretty logger, (3) Python SDK reference and guide on xcl.dev, (4) TS SDK reference and guide on xcl.dev + TypeScript tabs in CodeTabs.
- Entity types described with **Zod** (user: "seems like it has gained traction in the industry ... better to use a standard rather than invent"). Rejected: own xcl schema builder, decorated classes. Zod 4 as peer dependency (metadata registry / introspection).
- Runtimes: Node 22, 24, 26. Bun/Deno not promised. (Node 20 EOL April 2026.)
- SDK docs (for spec 3/4): generated reference (TypeDoc for TS, equivalent for Python) published on xcl.dev/reference/{typescript,python} + per-language guide section on xcl.dev; CodeTabs gains TS tab.
- npm package name: `xcl-lang` (unscoped `xcl` taken on npm; `@xcl` scope unverifiable). Engine packages `xcl-lang-<os>-<arch>[-<libc>]`.
- Repo: `jumppad-labs/xcl-typescript`, cloned at /home/nicj/code/github.com/jumppad-labs/xcl-typescript (LICENSE + README only). NOT yet in the spektacular registry — suggest registering with spek-manage-repos before planning.
- User approved the synthesis: Promise-only API, AbortSignal cancellation, ESM, bundled xcl-core subprocess, esbuild-style optionalDependencies, npm provenance/trusted publishing, version == xcl tag, parity fixtures vs Go.
- Design doc: `typescript-sdk.md` in source `design` (user explicitly asked for it). libxcl.md's Node section (xcl-node, @xcl/core) is stale — update it to point at the new doc.

## Learnings

- Python today: plain dataclasses + `local.register_type(App, "app")` (the `@xcl.entity`/`xcl.Resource` form was removed). `meta: xcl.Meta | None` field, `xcl.field(label=True)`, singletons when no label.
- Python lookups: entities(), entity_count(), find(cls|None, addr), find_by_type, find_one, all(cls), outputs(), encode_entity, encode_saved_entity (4 choices: include_computed, include_empty, reveal_sensitive, show_references).
- Python custom state store: load/save/exists/clear. Events: EventDataLevel NONE/RAW/PROCESSED; event.entity().
- Printing design (printing-and-highlighting.md) already sketches TS surface: printError/formatError, cfg.printEntity, entityTokens, printDiff, XclError util.inspect.custom, events.toLogger(pino) — that's spec 2.
- libxcl.md Node section: child_process.spawn, frames read on event loop, optionalDependencies with os/cpu/libc, camelCase<->snake_case single rule, Promise-only, cluster/fork spawn so no fork problem.

## Interview round 3 (2026-10-31)

- Ctrl-C: user chose option C, an opt-in `xcl.interruptSignal()`-style helper; the SDK installs no SIGINT listener unless asked. Requirement + two ACs added.
- Gaps 2 (idle engine doesn't keep Node alive, workers, missing platform package error) and 3 (xcl release.yml dispatches xcl-typescript; bindings.yml runs TS suite) were proposed; user didn't object. Confirm at section steps.
- User expected to see the spec in the store; explained it is assembled from working files at verification. Show full drafts in chat at each section step.
- Overview and Requirements confirmed by the user. User then said: "I would much rather just check everything at once" — draft remaining sections without per-section confirmation; present the whole spec once at verification.
- Design doc drafted at .spektacular/tmp/typescript-sdk.md (NOT yet written to the store). Present it with the full spec at verification; on approval: `spektacular design author --data '{"source":"design","path":"typescript-sdk.md"}' --spec 20261031080136-264e0eab-typescript-core-sdk --from .spektacular/tmp/typescript-sdk.md`, then `design ref add`, and revise libxcl.md's "Node and TypeScript" section to point at it (present the replaced section).
- Constraints: dropped "only runtime dep is protobuf" (moved to Technical Approach) and "Go/Python behaviour must not change" (to Non-Goals) — not user-imposed. Added async-only and no-SIGINT-unless-asked constraints (user decisions).
- Verification step: reviewer findings triaged and applied to working files (staged spec at .spektacular/tmp/spec_template.md). Full package (spec + design draft + libxcl diff) shown to user.
- User then REJECTED the Zod approach: "I really dislike that method of defining types with zod"; asked for three alternatives, then said all previews "look horrible" and asked for a professionally formatted markdown comparison. User's framing: "XCL Types are closer to orm entities than anything". Asked why entities don't use Zod / how React does it.
- Wrote .spektacular/work/20261031080136-264e0eab-typescript-core-sdk/entity-type-options.md: Option 1 entity builder (Drizzle-style, recommended), Option 2 decorated classes (TypeORM-style), Option 3 xcl schema files + codegen (Prisma-style, needs core feature), Zod for reference. Awaiting the user's choice; the Zod decision in spec (constraints, requirements wording "schema", TA, AC "Zod schema") and design draft must be revised once chosen. Nothing written to stores yet.
- 2026-10-31: User prefers Option 2 (decorated classes), finds the others "visually repulsive". Asked for a design doc to share with Erik and for the specs saved as draft so they can open a draft PR.
- Done: design typescript-entity-types.md (draft; proposal = decorated classes, alternatives = builder, schema files, Zod; 4 open questions) and typescript-sdk.md (draft, reworked to classes, no Zod) authored and referenced from the spec along with libxcl.md. Spec written to store as document_status draft with Zod removed (neutral "type" wording; constraint points at typescript-entity-types.md, "draft until decided"). Epic overview updated.
- Workflow deliberately left at the verification step (split/finish not run) so it can resume after Erik's review; working files kept. libxcl.md NOT revised (user never approved those edits; its Node section still says xcl-node/@xcl/core).
- xcl-typescript repo still not in the spektacular registry.
