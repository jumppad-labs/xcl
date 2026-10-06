# Builds and runs the plugin example. The in-process template plugin
# (template/) is compiled into the example. The external Docker plugin
# (docker/) is compiled to its own binary from cmd/docker-plugin, which the
# example starts as a separate process. Running it needs a Docker engine.

BUILD_DIR := build
DOCKER_PLUGIN := $(BUILD_DIR)/docker-plugin
CONFIG_DIR := ./config

# MOCKERY is the pinned Mockery release that generates the Docker client mock,
# run through go run so it needs no install
MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0

.PHONY: build run test generate clean

# Build the Docker plugin binary
build:
	go build -o $(DOCKER_PLUGIN) ./cmd/docker-plugin

# Build the Docker plugin, then run the example against its configuration
run: build
	go run . $(CONFIG_DIR) $(DOCKER_PLUGIN)

# Run the example tests, they build the Docker plugin themselves. The tests
# that need a Docker engine skip when none is reachable.
test:
	go test -v ./...

# Regenerate the mock of the Docker client interface, docker/mocks
generate:
	$(MOCKERY)

# Remove the built Docker plugin and the rendered template
clean:
	rm -rf $(BUILD_DIR)
