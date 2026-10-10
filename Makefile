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

.PHONY: protos install-mockery mocks mocks-clean setup-mocks
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
