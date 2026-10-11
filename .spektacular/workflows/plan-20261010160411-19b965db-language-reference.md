# Plan working context: language reference (orchestrated)

- Spec read. Docs-only work: pages on xcl-website, link in xcl README.
- Terminology rules: entities not resources; no "superset", "interpolation", "Terraform".
- Sibling spec 20261011084948-c995ed25-using-xcl-with-go planned in parallel: keep page boundaries clear.
- Discovery: no design refs; no interview notes. Latest tag v0.2.1 (HEAD only docs/test commits after). Home page badge says v0.2.2.
- Website: Astro MDX pages in src/pages/*.mdx, nav in src/components/Nav.astro (Features, Examples, Guides dropdowns), README pages table in xcl-website README.md. No redirects (knowledge gotcha).
- Changelog convention: one entry per spec in xcl CHANGELOG.md ("## <spec name>" + "**Breaking:** None.").
- Module boundary: config cannot reference entities inside a module; only Go Find by full address. Spec AC "reference to an entity inside it" reconciled via that.
- Env var prefix for variables is HCL_VAR_ (to verify).
- Research saved to work/research.md. Five AC-vs-code gaps found (locals, env override prefix bug, no public custom-function API, no map of blocks, no config refs into modules). Handed back as QUESTION at discovery step; resume at discovery end -> goto architecture once answered.
- Draft page split idea: new top-level nav "Language" dropdown, pages under /language/ (overview+files/blocks, entities & Go types, references & meta, variables & outputs, modules, functions, expressions). Go guide (sibling) goes under Guides.
