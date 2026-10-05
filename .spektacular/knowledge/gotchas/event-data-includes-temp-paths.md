---
tags: [testing, sensitive, events]
---

# Leak checks for short secrets must target the field

Event data includes `meta.file`, which in tests is a random temp path. A leak assertion that searches the whole payload for a short or numeric secret (such as `1234`) can match digits in that path and fail at random.

Assert on the field that would carry the secret (for example the `pin` attribute), or use secrets long and distinctive enough not to appear in paths or ordinary output words.
