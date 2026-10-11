# Working context: 20261011130709-aa3eb933-python-support

## Problem and motivation
- Epic 20261010160411-19b965db-language-reference started as a docs reorg. User: "the core problem is that once we support multiple languages we are going to need to completely change the docs again. So I think we should just bite the bullet and add the other language support into this epic. The outcome of the language support and how that affects the api will drive what shape our docs need to be."
- Positioning: "I aim to create a language that builds on the readability of HCL for modern processing. I also intend to bring the library to Python, Javascript and other languages." Docs present the language first, Go library second.
- User: "let's start to add the spec for supporting python that is based on the design doc libxcl".

## Design
- design/libxcl.md (source `design`), status draft, user settling it (revised: out-of-process `xcl-core` deferred; protobuf carried through one C function, not RPC; codegen section added).
- Python = libxcl phases 1-4: schema v1; plugin protocol v2; Python plugin SDK out of process; CoreService + buf + coreservice + bindings/libxcl + Python binding + wheels (+ context-taking op methods in root). Node = phase 5. xcl-core + Wasm later.
- Design open questions relevant to Python: plugin distribution via GitHub registry; sync vs asyncio first; host-language functions not designed; ABI version independence; number `format` hint; schema v1 lacks `key` tag/`body` kind; toolchain versions.

## Interactions
- language-surface spec (20261011095738-fad05e41-language-surface, finished, not in the epic yet): plugin-hosted functions via a protocol RPC + app-registered functions; protocol v2 reworks the same protocol → order or fold. Named child blocks into Go maps → schema v1 must describe them.
- Docs specs come last and get revised; guide plan likely stale.
- xcl-python repo doesn't exist and isn't registered.

## Epic state (planning on hold)
- Guide plan final + summary section written. Reference plan paused at discovery. Earlier notes backed up in session scratchpad wc-before-python.md.

## Interview answers (python-support)
- See .spektacular/work/20261011130709-aa3eb933-python-support/interview.md. Key: Go plugins only, no Python providers; Python functions yes; sync + asyncio, docs lead sync; new xcl-python repo; PyPI; 3.10+; depends on language-surface; site docs later in the epic.
- User: 'this is all good, keep going until you are done' (after constraints) — draft remaining sections, review, commit, add both specs to epic without stopping per section.

## Done (2026-10-11)
- python-support spec finished and stored (design ref design/libxcl.md). Review: 14 of 15 fixes applied under the user's "keep going until you are done"; dropped "add no-reimplementation constraint" (covered by design ref). Split not offered (tightly coupled on the engine binding).
- Epic updated: overview rewritten for multi-language; specs language-surface → python-support → reference, guide (reference and guide depend on both). Reversible with `spektacular epic write`.
- Still to do later: revise reference spec ACs (locals dropped, module refs via outputs/Go Find, language-neutral); guide plan likely stale; register xcl-python repo once created; Python plugins/protocol v2 and Node are future specs.
