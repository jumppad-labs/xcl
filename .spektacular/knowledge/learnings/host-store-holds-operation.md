---
tags: [coreservice, testing, events, concurrency]
---

# A host state store holds an operation open

The core service posts events to the host fire-and-forget: there is no completion, and the engine never waits for the host to handle one. State and function tasks are different: the engine waits for the host's `Complete`. Validate, Apply, Diff, Destroy and Load all call the state store's `Exists`, so a host-provided state store that does not answer holds any operation at that point. This is the deterministic way to test "while an operation is in progress".

Corollary: something done from a create-success event handler is only provably mid-operation when a host store makes the engine wait for the SAVE that follows; with the built-in file store the operation may already have finished. A registry that blocks during plugin load is not a substitute: calls that need the catalog lock wait on it and the test hangs.
