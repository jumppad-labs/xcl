---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Test Plan: module boundary and output entities

## Success metrics

The spec defines no success metrics, so none need manual verification.

## Manual reviews

### Documentation site read-through

- **What**: Check that the site describes the output-only module boundary with a re-export example, and shows an output read as an entity.
- **Where**: In `xcl-website`, run `npm ci` and then `npm run dev`. Open:
  - `/`, the home page. Read the **Modules** feature card, which says outputs are the only way into a module, anything else is rejected by validation, and a nested value is exposed by re-exporting it. Read the **Variables and outputs** card, which says `Find[types.Output]` returns an output and `.Value` holds its value.
  - `/examples/plugins/`. Under **The program**, the snippet reads `output.web_database` and `module.analytics.output.location` with `xcl.Find[types.Output]` and prints `.Value`. Under **What to notice**, the "A module is reached through its outputs" bullet carries the re-export snippet `output "replica_location" { value = module.replica.output.location }`.
- **Pass**: Both pages render without layout problems in light and dark themes. The wording is consistent with the README's "Reading values a configuration publishes" and "The module boundary" sections. The code in the example snippet matches `xclconfig: example/plugin/main.go`. The "## Published" lines in the program output listing are unchanged.
- **Who / when**: A maintainer, before the site is deployed.
