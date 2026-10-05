---
tags: [testing, dag, dependencies, concurrency]
---

# Assert ordering on graph parents, not provider call order

The DAG walker visits vertices with no edge between them concurrently. A test that applies or destroys, records the order of provider calls, and asserts that order can pass by luck when the resources it compares are not actually linked.

To test ordering, assert on the graph: build it (for saved state, the `savedGraphParents` test helper builds the destroy graph) and check each entity's parents. Keep call-order assertions only for resources that are genuinely linked.
