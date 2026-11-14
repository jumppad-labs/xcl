---
tags: [cancellation, plugins, engine]
---

# Cancelling an operation waits for the provider call in flight

The engine calls providers with a context that is never cancelled, so `Cancel` stops the operation from starting anything further but lets the current provider call finish. An SDK's cancelled operation therefore rejects only once that call returns: a 30-second provider call delays the rejection by about 30 seconds, whatever the SDK does.

This also means an engine outlives an interrupted host by the call in flight, and a test asserting a bound on cancellation must hold its provider call for less than that bound.
