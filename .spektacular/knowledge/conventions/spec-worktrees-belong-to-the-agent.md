---
tags: [worktrees, git, workflow]
---

# Spec worktrees belong to the agent; the user works on the branch

The implement workflow builds each spec in worktrees under `.spektacular/worktrees/<spec>/`. Those are the agent's working copies. The user works only on their own branch in the main checkout (for xcl, the feature branch such as `f-python-plugin`; elsewhere usually `main` or a PR branch). They don't want to look in a worktree unless it's absolutely necessary.

**Why:** the user said so after being asked to look at changes that only existed in a worktree: "you own the worktree, it is for you. I don't ever want to see it unless absolutely necessary. I only work on the branch".

**How to apply:**
- When work is ready for the user to see, review or change, get it onto their branch: commit it on the spec branch and merge it with `spektacular implement merge`, or fast-forward their branch to the spec branch. Then tell them it's on the branch.
- Never point the user at a worktree path, or report changes as "in the worktree" as if they could see them.
- Don't hand-copy worktree changes into their checkout as uncommitted edits either: that makes the later `implement merge` collide with them.
- Work that ships through another repository's PR (for example an engine-bump branch in xcl-python) is delivered by pushing to that PR's branch.
