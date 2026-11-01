---
tags: [examples, testing, git]
---

# Building example/remoteplugin overwrites a tracked binary

`example/remoteplugin/remoteplugin` is committed to the repository. `go build ./...`
inside `example/remoteplugin`, which `make test-examples` and `make test-example
EXAMPLE=example/remoteplugin` both run, writes the freshly built program over it.
The working tree then shows the binary as modified, and it lands in a commit or a
goal-check diff unless it is put back.

After running the example builds, restore it with
`git checkout -- example/remoteplugin/remoteplugin` before staging or committing.
