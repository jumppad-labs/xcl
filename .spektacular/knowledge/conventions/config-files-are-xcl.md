# Configuration and metadata files are written in xcl

Every configuration or metadata file the xcl project defines is written in xcl,
not JSON, YAML or TOML. The project uses its own language for its own files.
This covers files a person writes, such as a plugin's package manifest, and
files tooling generates, such as a plugin release's index.

- **New file formats:** when a design introduces a manifest, index, lock or
  settings file, specify it as xcl blocks and give it the `.xcl` extension.
  Don't reach for JSON because it is simpler for other tools. Readers get xcl
  through the Go library or, in other languages, through `xcl-core`.
- **Static documents:** a file read before anything runs, such as a manifest
  the registry reads to choose a package, holds literal values only: no
  variables, functions or references. It is parsed as a config-only document
  against a fixed set of types, and an expression is an error.
- **Exceptions:** files whose format some other standard tool owns keep that
  format, for example `sha256sum` checksum files and OpenPGP signatures. Wire
  payloads such as schema v1 over gRPC and entity JSON are protocol data, not
  configuration files, so this rule doesn't cover them.

Stated by the project lead on 2026-10-31 while designing the plugin package
format, after JSON had been proposed for the release index.
