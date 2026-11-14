---
tags: [plugins, testing, go-plugin, sockets]
---

# A long TMPDIR stops external plugins from starting

go-plugin creates each external plugin's unix socket under `TMPDIR`. A unix socket path is limited to about 108 bytes, so when `TMPDIR` is long, such as a session scratch directory or a deep worktree path, every plugin fails to start even though the engine and the plugin binary are fine.

Run real-engine suites of any SDK with a short `TMPDIR` (a directory directly under `/tmp` is enough).

go-plugin leaves those `plugin*` socket files behind when the engine is stopped or killed, so a shared `TMPDIR` slowly collects them. They are harmless, and a test should not treat them as a sign of a leaked process: check for the process.
