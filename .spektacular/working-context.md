# Working context: typescript-sdk-reference (spec 4 of the TypeScript SDK epic)

User request 2026-11-10: create the epic's remaining specs while away, then "just go straight to planning them. If we need to make changes then that will be quicker than me waiting to review the specs". No user questions in this run; assumptions listed for review afterwards.

Decision (2026-10-31): generated reference (TypeDoc) at xcl.dev/reference/typescript, a per-language guide section, CodeTabs gains a TypeScript tab. Depends on the TS core SDK and TS printing specs.

Learnings: CodeTabs has fixed Go and Python tabs, 107 boxes on 19 pages. The choice is page-wide data-lang, remembered in localStorage key xcl-lang; Head.astro only recognises "python".

# Epic planning run 2026-11-10: 20261031080131-8c2a58ec-typescript-sdk

User: create the remaining specs, then "just go straight to planning them" while away. Plan-epic covers all four specs, including the core SDK the others build on. Recorded depends_on from the epic overview (printing on core; TS reference on core and printing). Registered xcl-typescript (repo add). Assumptions to report: see final message.

All four plans done 2026-11-10. Added depends_on: TS reference on Python reference (its plan reuses the Python pipeline). Open before writing the epic summary: PyRef strict vs TsRef lenient (Step 6 disagreement), then `epic order`, then review.
Orchestrator-made decisions to report: entities() typed (option A); platform packages vs one-package; uncaught report plain in err.stack, printing spec amended.

Link strictness settled by the user (both lenient); Python plan updated, epic summary written, epic order run. epic order added Python reference -> core and -> printing, a false positive: those plans only cite xcl-python files as models and have no xcl-python tasks. Asking the user whether to unorder.
- User agreed: removed the false Python reference dependencies on core and printing via epic order unorder.

# Epic implement run 2026-11-10: 20261031080131-8c2a58ec-typescript-sdk

Started via /spek-implement-epic with no epic named; this is the only epic with work left. No dirty repos, so no commit question. User told code is committed on spek/<spec> branches. Main checkouts at start: xcl on f-python-plugin, xcl-python on fix-install-check, xcl-website and xcl-typescript on main.
- Python reference, task 1 goal check, criterion 1 (attributes not checked by the doc comment tool): user chose to correct the plan criterion to match the spec definition of a member, attributes listed but not checked. Code unchanged.
- Python reference, task 2 goal check exhausted on criterion 2 (engine start-up errors undocumented): user chose keep_fixing. Add the two engine errors to engine_version and both Config classes; if the reviewer then wants them on every method, the child comes back to the user. Leftover for the final report: staged goal-check files under .spektacular/tmp/<spec>/goal-check, child could not rm them.
- Python reference, human task "Give the website the release bot access": user chose to do the App and secrets themselves now; child waits at check_goal until the user says done, then records accept on their word. Two final human tasks (release, first pin) need the merge first.
- TypeScript core: user approved (1) typescript-sdk.md design amendment (entities returns registered class instance else generic Entity; drop ", as for xcl-lang on PyPI" from Releases) and (2) spec amend of cancellation criterion to "within two seconds, once any provider call in flight has returned".
- TypeScript core: design typescript-sdk.md and the spec Cancellation criterion amended (spec amend, approved by user). Children share the scratchpad and collided once; further children told to keep working files under .spektacular/tmp/<spec>/.
- TypeScript core, human task 23 (npm and GitHub setup): user chose accept the shortfall; setup goes in the release plan. Tell the user at the end: push xcl change only after xcl-typescript test.yml is on its main and XCL_TYPESCRIPT_DISPATCH_TOKEN exists.
- TypeScript core merged: xcl into f-python-plugin (c2860fd), xcl-typescript into main (ccee545). 22 agent tasks accepted; task 23 accepted shortfall. Nine spec checkboxes remain that need a real CI run or release. Children must use .spektacular/tmp/<spec>/ for working files.
- TypeScript printing: user approved the printing-and-highlighting.md TypeScript paragraph correction (uncaught report via err.stack). Applied with design author (kept final), added the design ref to the printing spec, recorded with spec amend.
- TypeScript printing merged into xcl-typescript main (0fd791a, from e76a9f9); no xcl commit (design change lives in the project store). 13/13 tasks, 1276 strict tests. Its repo changelog was written into the main xcl-typescript checkout as untracked .spektacular/ (not in the commit). Only the Python reference remains in flight, waiting on the user (App + secrets on xcl-website); TS SDK reference blocked on it.
- 2026-11-13 user tried the TS SDK and decided: new Config({...}) replaces createConfig (no alias), lazy CreateConfig on first call, optional setup() (documented why; Python aio calls it open()). Route: amend core spec (Constraints) + typescript-sdk.md design, then change code directly in xcl-typescript main checkout (uncommitted). Must be committed before the TS SDK reference spec starts.
- Config change done in xcl-typescript main checkout, uncommitted: Config class, lazy create, setup(), configWith internal; tests converted; 18 new unit tests; README/PARITY/example/install-check/doc comments. Strict suite 1294/1294. TS SDK reference plan context updated (guide uses new Config + setup note).
- 2026-11-13 user installed the release bot App on xcl-website and xcl-typescript; secrets RELEASE_APP_ID and RELEASE_APP_PRIVATE_KEY verified on xcl-website via gh secret list (xcl-typescript has none yet). Python reference child told to record accept on the user word for the human task.
- Python reference: user chose skip for the two release-time human tasks (release xcl-python, set the site first pin); they go in the release plan, in order: release xcl-python, then pin and push xcl-website.
- Python reference merged 2026-11-13: xcl into f-python-plugin (29b1942), xcl-python into fix-install-check (5ed49cf), xcl-website into main (6958de1). Status still in_progress (13/15: two skipped release tasks), so TS reference stays blocked. Do NOT restart the Python reference although status says ready. Asking user: unorder TS ref from Python ref, or release first.
- User chose to drop the dependency: epic order unorder removed TS reference -> Python reference. TS reference now ready, waiting on the user to commit the xcl-typescript Config/example changes (repo dirty) before creating its worktrees. Python reference shows ready but must not be restarted: only its two skipped release tasks remain.
- 2026-11-13 npm spam detection refused unscoped xcl-lang-win32-*; user chose @jumppad-labs/ scoped platform packages. Spec Constraints + typescript-sdk.md amended; xcl-typescript code/tests/workflows/README changed (uncommitted, with Config change). Unit 1060/1060; scoped install verified locally on linux-x64-gnu. Placeholder script rewritten for scoped names.
- User: SDKs must always be consistent. TS release process to match Python (own version line, release labels, auto-tag, labelled engine bump, engineVersion()). Spec (Requirements, AC, Constraints) + typescript-sdk.md amended. Helper agent launched to change xcl-typescript workflows/code (uncommitted).
- TS reference plan updated (context, plan) for the own-version-line release process. xcl-typescript committed on f-update (b7879b0), PR #1 labelled minor and building. Starting the TS reference.
