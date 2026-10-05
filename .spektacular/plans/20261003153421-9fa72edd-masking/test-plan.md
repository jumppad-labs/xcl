---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Test plan: 20261003153421-9fa72edd-masking

Automated behavioural tests cover the masking package, the wire encoder, the saved-entity reader, encrypted state, the plaintext warning, event masking, the leak suite and the examples. The procedures below are the manual checks the plan's Testing Approach lists.

## 1. Zero occurrences of a known secret in state and event data (success metric)

- **What to measure**: occurrences of a known secret in any state file or captured output, with an encryption masker for state and default event settings. Threshold: 0.
- **How** (from the `xclconfig` repo root):
  1. `go test -count=1 ./...` must pass. It includes `TestLeakSuiteStateFileHoldsNoSecret`, the `config_state_mask_test.go` and `config_event_mask_test.go` suites, and the example tests.
  2. Run each example with a key and capture everything it writes:
     ```
     export XCL_STATE_KEY=$(openssl rand -base64 32)
     export DB_PASSWORD=manual-check-s3cret-42
     (cd example/appconfig && go run . ./config) > /tmp/appconfig.out 2>&1
     (cd example/plugin && make build && go run . ./config ./build/external) > /tmp/plugin.out 2>&1
     (cd example/configonly && go run . ./config) > /tmp/configonly.out 2>&1
     grep -c 'manual-check-s3cret-42\|pg-s3cret-example\|pg-an4lytics-example' /tmp/appconfig.out /tmp/plugin.out /tmp/configonly.out
     ```
     The examples keep state in a temporary directory removed when they exit; to inspect the state file too, temporarily point `stateDir` in each `main.go` at a fixed directory, rerun, and `grep -r` that directory for the same strings, then revert.
  3. Without `XCL_STATE_KEY`, rerun the appconfig and plugin examples and confirm stderr shows the warning `sensitive values are stored unencrypted in state; use xcl.WithStateMask to encrypt them`; the configonly example shows none.
- **Expected result**: step 1 passes; every grep count in step 2 is 0 (and the state file contains `xcl_masked`); step 3 shows the warning for the two examples with passwords only.
- **Who / when**: the reviewer of this change, before merging the epic.

## 2. Issue jumppad-labs/xcl#1 can be closed (success metric)

- **What to measure**: every area issue #1 lists (events, plugin log details, state, printed output) is covered.
- **How**: `gh issue view 1 --repo jumppad-labs/xcl`; for each area, find the covering tests: events (`config_event_mask_test.go`, `TestLeakEventsAtRawDataLevel`, `TestLeakEventsAtProcessedDataLevel`), plugin log details (`TestLeakInProcessPluginLogDetails`, `TestPluginLogDetailsRedactedWithEventMaskingOff`), state (`config_state_mask_test.go`, `TestLeakSuiteStateFileHoldsNoSecret`), printed output (`TestLeakPrinterTable`, `TestLeakPrinterTree`, `TestLeakPrinterCard`, `TestLeakPrinterJSON`).
- **Expected result**: every area maps to at least one passing test; then close the issue with a comment linking the merged change.
- **Who / when**: the maintainer, once the epic is merged.

## 3. Review: documentation site pages

- **Where**: in `xcl-website`, `npm ci && npm run dev`, then open `/events/` (section "Sensitive values in event data") and `/state-masking/`.
- **What to look for**: the events section covers the default redact envelope, choosing a masker with `xcl.WithEventMask`, turning masking off with `xcl.WithNoEventMask()`, and that errors and log details always show `(sensitive)`. The state masking page covers configuring `xcl.WithStateMask` with `mask.EncryptAES256GCM`, key handling, the reversible-only rule, the wrong-key error, the plaintext warning and all four built-in maskers. "State masking" appears under Guides in the navigation and opens the page; the Sensitive values page links to it.
- **Passing**: every item is present, reads correctly, and the page is reachable from the navigation.

## 4. Review: library documentation

- **Where**: `README.md` (section "Sensitive values", subsections "Encrypting sensitive values in state", "The plaintext state warning", "Masking sensitive values in events", "Built-in maskers", "Writing your own masker"; and "Resource data on events"), `docs/state.md` ("Reading a saved record back as configuration", "Sensitive values in state"), `CHANGELOG.md` (top entry).
- **What to look for**: key handling is described (32 byte key from a secret store, never committed, no rotation); nothing suggests a one-way masker for state, and the reversible-only rule is stated with `xcl.ErrMaskNotReversible`; the wording matches the site pages; the CHANGELOG Breaking list names the key requirement and `xcl.ErrUnrecoverable`.
- **Passing**: all of the above hold, with no contradiction between README, state guide and site.
