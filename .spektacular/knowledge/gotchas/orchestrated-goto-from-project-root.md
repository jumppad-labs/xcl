---
tags: [spektacular, workflow, worktrees]
---

# Run spektacular implement goto from the project root worktree

In an orchestrated multi-repo implement run, each spec has a worktree per repo. Every `spektacular implement goto` must run from the spec's project worktree (the xclconfig one). A goto run from the xcl-website worktree does not reach the spec's lane, and the workflow appears not to advance.

Do work in the other repo's worktree by absolute path, but drive the workflow from the project root.
