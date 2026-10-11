---
tags: [spektacular, workflow, repos]
---

# Register a new repo before planning tasks in it

`spektacular plan file write` refuses a plan whose task has a `**Repo:**` that isn't registered (`spektacular repo list`). A plan can't attribute work to a repository that doesn't exist yet unless it's registered first.

**How to apply:** when a spec creates a new repository (for example xcl-python), register it with `spektacular repo add` (name, location, description, role, tags) before the plan is written. Creating the remote repository stays a task for a person.
