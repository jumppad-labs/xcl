---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Test plan: 20261003153421-bf87d907-references-as-written

## Success metrics

The spec defines no success metrics, so none need manual measurement. Every behaviour in the acceptance criteria is covered by automated tests in `xclconfig` (`internal/parser/references_written_test.go`, `encode_references_test.go`, `config_test.go`, `readme_test.go`).

## Manual review: configuration-text guide on the documentation site

- **What**: The new configuration-text guide in `xcl-website` is reachable from the navigation, and it describes requesting references with an example of a reference written as the user wrote it, next to the default resolved form.
- **How**:
  1. In the `xcl-website` repo, run `npm ci`, then `npm run dev`.
  2. Open the local URL Astro prints (usually `http://localhost:4321/`) in a browser.
  3. Open the **Guides** menu in the top navigation and choose **Configuration text**. It should open `/configuration-text/`.
  4. Read the **Showing references as written** section.
- **Expected result**:
  - The Guides menu lists "Configuration text", and it links to `/configuration-text/`.
  - The section shows the `xcl.ShowReferences()` call and output with `db_location = resource.postgres.main.location` and the template `url = "https://${resource.postgres.main.location}/app"`, followed by the default resolved form (`db_location = "localhost"`).
  - The sensitive-field rule and the note on state saved by earlier versions appear, and their wording matches the README's "Showing references as written" paragraph.
  - Every code block renders, and the page has no layout breakage at desktop or phone width.
- **Who / when**: The documentation owner, before publishing the site release that includes this change.
