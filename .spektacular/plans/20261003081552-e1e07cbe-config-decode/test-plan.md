---
created_date: "2026-10-03"
document_status: final
closed_date: "2026-10-03"
---

# Test Plan: 20261003081552-e1e07cbe-config-decode

All success metrics are covered by automated behavioural tests; no manual test plan is required.

- Metric 1 (one call fills a struct however many registered types it holds): `TestDecodeFillsEveryRegisteredTypeInOneCall` in `decode_test.go`.
- Metric 2 (a new block type needs only a new field): `TestDecodeFillsAFieldForANewlyDeclaredTypeWithNoOtherCode` in `decode_test.go`.
- Metric 3 (shipped examples that assemble several block types use Decode): `TestConfigOnlyExampleAssemblesItsConfigurationWithDecode` and `TestConfigOnlyExampleMakesNoPerTypeLookups` in `example/configonly/main_test.go`.
