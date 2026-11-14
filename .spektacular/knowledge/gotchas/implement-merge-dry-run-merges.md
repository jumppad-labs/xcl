---
tags: [spektacular, worktrees, git, workflow]
---

# `spektacular implement merge --dry-run` really merges

`implement merge` lists the global `-n, --dry-run` flag in its help, but doesn't honour it. Run with `--dry-run`, it merges `spek/<spec>` into every touched repo's current branch, removes the worktrees and branches, and returns `"merged": true, "removed": true`. It happened on 2026-11-02 with the plugin-release-contract-v2 spec.

**How to apply:** treat `implement merge`, with or without `--dry-run`, as the real, irreversible merge. Before running it, preview each repo by hand: its current branch, and whether `spek/<spec>` is already contained in it, fast-forwards it, or needs a merge commit. Check the result is wanted for every repo, including checkouts left on unrelated branches. The command merges into whatever branch each main checkout is on.
