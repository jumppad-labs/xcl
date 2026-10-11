protos:
	mkdir -p plugins/proto
	protoc --go_out=. --go_opt=module=github.com/jumppad-labs/xcl --go-grpc_out=. --go-grpc_opt=module=github.com/jumppad-labs/xcl --proto_path=. plugins/plugin.proto

# Install mockery for generating mocks
install-mockery:
	go install github.com/vektra/mockery/v3@v3.8.0

# Generate mocks using mockery configuration
mocks:
	mockery

# Generate mocks with clean slate (removes existing mocks first)
mocks-clean:
	rm -rf plugins/mocks
	mockery

# Install tools and generate mocks
setup-mocks: install-mockery mocks

# Generate the Go message types of the CoreService protocol, proto/xcl/core/v1,
# into coreservice/proto with buf. buf-lint and buf-breaking keep the
# protocol's names consistent and its fields and methods stable.
core-protos:
	buf generate

buf-lint:
	buf lint

buf-breaking:
	buf breaking --against '.git#branch=main'

.PHONY: protos install-mockery mocks mocks-clean setup-mocks core-protos buf-lint buf-breaking

# libxcl is the engine built as a C shared library from the nested module
# bindings/libxcl, for a language binding to load. LIBXCL_VERSION is stamped
# into the library as the version GetVersion reports, a release sets it to the
# tag. LIBXCL_OUT is the library's path; the default extension follows GOOS.
# CC, GOOS and GOARCH pass through for cross builds, e.g.
# make libxcl CC="zig cc -target x86_64-linux-gnu.2.28" LIBXCL_VERSION=v1.0.0
LIBXCL_VERSION ?= dev
LIBXCL_GOOS ?= $(shell go env GOOS)
ifeq ($(LIBXCL_GOOS),windows)
LIBXCL_EXT := dll
else ifeq ($(LIBXCL_GOOS),darwin)
LIBXCL_EXT := dylib
else
LIBXCL_EXT := so
endif
LIBXCL_OUT ?= $(CURDIR)/bindings/libxcl/build/libxcl.$(LIBXCL_EXT)
LIBXCL_LDFLAGS := -s -w -X github.com/jumppad-labs/xcl/coreservice.Version=$(LIBXCL_VERSION)

libxcl:
	cd bindings/libxcl && CGO_ENABLED=1 go build -buildmode=c-shared -trimpath -ldflags="$(LIBXCL_LDFLAGS)" -o "$(LIBXCL_OUT)" .

# test-libxcl runs the nested module's smoke test through the C ABI, with
# the library's version stamped exactly as the libxcl target stamps it
test-libxcl:
	cd bindings/libxcl && CGO_ENABLED=1 go test -count=1 -ldflags="-X github.com/jumppad-labs/xcl/coreservice.Version=$(LIBXCL_VERSION)" ./...

.PHONY: libxcl test-libxcl
# Install and start a real plugin release from github.com. Set
# XCL_GITHUB_LIVE_PLUGIN=owner/repo@vX.Y.Z, optionally XCL_GITHUB_LIVE_KEY to
# the path of an armoured public key to trust, and GITHUB_TOKEN or GH_TOKEN
# for a private repository. The default test run skips this test.
test-github-live:
	go test -count=1 -run TestGitHubRegistryInstallsARealRelease -v ./registry/

.PHONY: test-github-live

# EXAMPLES are the example modules, each a Go module of its own. The Examples
# workflow tests each one as its own job, with the same list.
EXAMPLES := example/configonly example/plugin example/plugin/plugins/docker example/prettylog example/remoteplugin

# Build, vet and test every example. These check the examples work, not the
# library, and the plugin example and its Docker plugin need a Docker engine.
test-examples:
	@set -e; for example in $(EXAMPLES); do \
		$(MAKE) --no-print-directory test-example EXAMPLE=$$example; \
	done

# Build, vet and test one example, for example
# make test-example EXAMPLE=example/configonly
test-example:
	@test -n "$(EXAMPLE)" || { echo "usage: make test-example EXAMPLE=<one of: $(EXAMPLES)>"; exit 1; }
	@echo "== $(EXAMPLE)"
	cd $(EXAMPLE) && go build ./... && go vet ./... && go test ./...

.PHONY: test-examples test-example
