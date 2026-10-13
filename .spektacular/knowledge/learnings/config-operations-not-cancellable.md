---
tags: [config, context, cancellation]
---

# Config operations could not be cancelled from outside

As of v0.2.1, `Config.run` derives its context from `context.Background()`. Nothing outside the library can cancel a running Validate, Apply, Diff, Destroy or Load. The only thing that stops one early is a panicking event handler.

The python-support plan adds context-taking operations (`ApplyContext` and friends) so callers, including the Python asyncio API, can cancel.

**How to apply:** new long-running operations take a `context.Context` from the caller and pass it through. Don't start from `context.Background()` inside `Config`.
