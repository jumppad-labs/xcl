---
tags: [proto, buf, coreservice]
---

# rpc comments in core.proto never reach the generated Go code

`buf.gen.yaml` runs only the `go` plugin, so `make core-protos` generates the core service's messages and nothing for the service itself: the service is dispatched by name in `coreservice` and held to the descriptor by a test. A comment on an `rpc` line in `proto/xcl/core/v1/core.proto` therefore appears nowhere in `coreservice/proto/core.pb.go`, and regenerating after a comment-only rpc change produces no diff. Document an rpc's contract in the proto (and in the handler's Go comment). Do not plan or check for the comment in the generated code.
