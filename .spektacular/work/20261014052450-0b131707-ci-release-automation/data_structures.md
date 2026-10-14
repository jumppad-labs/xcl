No program types are added. The contracts are the interfaces between workflows, one dispatch payload, the package's version stamp, and repository settings.

**Release labels.** A fixed set of four PR labels, identical in every participating repo: `major`, `minor`, `patch`, `no-release`. Exactly one per PR. The override label `breaks-bindings` exists only in xcl.

**Release-label check (reusable workflow).** No inputs; reads the calling PR's labels. Its only output is pass/fail.

**Auto-tag (reusable workflow).**
```
inputs:  none (reads the merge commit of the calling push)
secrets: RELEASE_APP_ID, RELEASE_APP_PRIVATE_KEY
outputs: tag    — e.g. "v0.7.1", empty for no-release
         bump   — "major" | "minor" | "patch" | "none"
```
Next version: semver from the highest `v*` tag; `major` while below 1.0 yields `1.0.0`.

**Binding test suite (reusable workflow, one per language).**
```
inputs: binding-ref — the binding repo's ref to test (default: main)
        xcl-ref     — the xcl ref to build the engine from (default: the binding's pin)
result: one final job whose conclusion is the suite's result
```
This is the contract a new language must implement to join the Bindings check.

**Release dispatch payload (xcl → each binding, event type `xcl-release`).**
```
client_payload: { tag: "v0.8.0", bump: "major" | "minor" | "patch" }
```
`bump` is new; `tag` is unchanged.

**Engine pin.** `xcl-ref.txt` in a binding repo holds one xcl tag (or, transiently in a bump PR, the new tag). It is the only source of the engine version for CI, the release download, and the runtime check.

**Package version stamp (xcl-python).** The version module carries two values:
```
__version__         = package version, from the xcl-python tag at release
__engine_version__  = the pinned engine tag at release; a fixed development value otherwise
```
The development engine build is stamped with the same development value, so local and CI runs pass the check. The client compares the engine's reported version with `__engine_version__`; `EngineVersionError` keeps its shape, with the expected side now the pin.

**Repository settings (per repo).** Labels as above; branch protection on `main` requiring the release-label check and the repo's test gates (in xcl also the `bindings` gate); auto-merge allowed; secrets `RELEASE_APP_ID` and `RELEASE_APP_PRIVATE_KEY`.
