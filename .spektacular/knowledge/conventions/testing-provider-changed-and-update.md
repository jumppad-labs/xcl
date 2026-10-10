---
tags: [testing, plugins, providers, changed, update]
---

# Testing provider Changed and Update

Provider `Changed` and `Update` tests check the action taken for given inputs.
A test builds the `[]entity.PropertyChange` and `[]entity.DependencyChange` that
xcl would really pass, calls the method, and asserts the outcome:

- for `Changed`, the replace or update answer;
- for `Update`, that the expected call was made on the mocked client.

Most tests cover what happens when a dependency changed. Passing `nil` changes
doesn't test a `Changed` that decides with `change.Within`.
