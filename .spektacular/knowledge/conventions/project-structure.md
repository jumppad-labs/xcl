# Project Structure

**Tier:** always-applied

- `/cmd` — main applications.
- `/internal` — private application code.
- Public library packages live at the module's top level (e.g. `mask/`, `events/`,
  `state/`, `types/`, `highlight/`), not under `/pkg`. Users import them as
  `github.com/jumppad-labs/xcl/<name>`.
- `/api` — API definitions (OpenAPI, protobuf).
- `/configs` — configuration files.
