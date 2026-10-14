---
tags: [ci, github-actions, reusable-workflows, permissions, gotchas]
---

# A reusable workflow can't use more permissions than its caller grants

A job in a reusable workflow (`on: workflow_call`) gets at most the
`GITHUB_TOKEN` permissions its caller grants. jumppad-labs repositories default
to a read-only token (`default_workflow_permissions: read`), which can read
contents and nothing else; there is no `pull-requests` access.

When a called job asks for a permission the caller doesn't grant, GitHub
refuses to start the run. It ends in `startup_failure` with no job log, only
"This run likely failed because of a workflow file issue".

**Seen:** xcl's first `Tag` run (`tag.yml` calling `auto-tag.yml`, run
38531529299) failed this way, because `auto-tag.yml` asks for
`pull-requests: read` to find the merged PR. Fixed in jumppad-labs/xcl#24.

**How to apply:**
- Every caller of a shared workflow declares, at workflow or job level, at
  least the permissions the called jobs ask for. Put the required block in the
  shared workflow's usage comment so callers copy it.
- Testing a called workflow's API calls with your own `gh` token doesn't
  prove they work in Actions: your token has scopes the repository's default
  token lacks.
