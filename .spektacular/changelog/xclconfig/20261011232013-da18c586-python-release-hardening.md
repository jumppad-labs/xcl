---
created_date: "2026-10-13"
document_status: draft
project: xclconfig
spec: 20261011232013-da18c586-python-release-hardening
plan: 20261011232013-da18c586-python-release-hardening
---

# The untrusted-key refusal names the signing key

When a plugin installed from GitHub releases is signed by a key you have not trusted, xcl still refuses it with the same error, and the message now also says which key signed it, by its 16-hex-digit key ID. You can see straight away which key you would need to trust, and every language binding shows the same message. Tampered, unsigned and correctly signed releases behave exactly as before.

> Derived from project xcl, spec/plan 20261011232013-da18c586-python-release-hardening. See the project-level record for the full feature.

## What changed in this repo

- `registry/github_verify.go`: when the detached signature fails because its issuer is not among the trusted keys (`ErrUnknownIssuer`), the error keeps `ErrPluginVerification` and the text "signature `<file>` does not verify against any trusted key", and adds ": signed by key `<%016X>`, which is not trusted". The key ID comes from the new `signatureIssuer`, which reads the signature packet's issuer key ID, or the low 64 bits of its issuer fingerprint. Every other failure keeps the previous message.
- `registry/github_verify_test.go`, `registry/github_signature_test.go`: tests that the issuer is read from a real signature and that garbage reports none, and that a release signed by another key is refused naming that key. The existing refusal test is unchanged.
- `internal/testutil/github_release.go`: `FakeGitHubRelease.OtherKeyID()`, the long ID of the key `SignedByOtherKey` signs with.
- `CHANGELOG.md`: this spec's entry, with no breaking change.

## Why

The Python release-hardening work requires that a release signed by an untrusted key is "refused naming the key". The old message named only the signature file. Adding the key ID to the engine's error makes that true for every binding at once, and keeps the existing wording, which the tamper tests match on.
