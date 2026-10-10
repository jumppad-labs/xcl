# Builds, tests and regenerates the doubles of the Docker plugin, a module of
# its own. Run every target from this directory.

BUILD_DIR := build

# MOCKERY is the pinned Mockery release that generates the client mocks, run
# through go run so it needs no install
MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0

.PHONY: build test generate clean

# Build the plugin binary, build/docker-plugin
build:
	go build -o $(BUILD_DIR)/docker-plugin ./cmd/docker

# Run the plugin's tests. The tests that need a Docker engine skip when none
# is reachable.
test:
	go test -v ./...

# Regenerate the mock of each client interface, client/*/mocks
generate:
	$(MOCKERY)

# Remove the built plugin binary
clean:
	rm -rf $(BUILD_DIR)
