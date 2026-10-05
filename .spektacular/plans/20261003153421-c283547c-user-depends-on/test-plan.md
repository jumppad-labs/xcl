---
created_date: "2026-10-05"
document_status: final
closed_date: "2026-10-05"
---

# Test plan: 20261003153421-c283547c-user-depends-on

The spec defines no success metrics, so there are none to verify by hand. Every acceptance criterion is covered by the automated suite (`go test ./... -count=1` in the `xclconfig` repo). One manual review is listed in the plan's Testing Approach.

## Manual review: the site's configuration-text guide

- **What**: the new "Dependency lists" section of the configuration-text guide in `xcl-website` (`src/pages/configuration-text.mdx`).
- **How**: in the `xcl-website` repo run `npm ci` and `npm run dev`, then open `http://localhost:4321/configuration-text/` in a browser and scroll to "Dependency lists", which sits just before "Showing references as written".
- **Look for**:
  - It says a written `depends_on` list is written exactly as written, and left out when none was written.
  - It says dependencies xcl works out from references are never added to the list, but still order creation and destruction.
  - The example is correct: the configuration's `resource "app" "a"` has `depends_on = ["resource.app.b"]` and `x = resource.app.c.y`, with `resource "app" "c"` setting `y = "blue"`; the text shown for `a` has `depends_on = ["resource.app.b"]` (no `resource.app.c` in it) and `x = "blue"`.
  - The wording agrees with the README's configuration-text "What is left out" paragraph, and the page renders cleanly in light and dark themes.
- **Pass**: all of the above hold, with no rendering or formatting problems.
- **Who / when**: the documentation owner, before the site is published with this change.
