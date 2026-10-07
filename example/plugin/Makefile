# Builds and runs xcl-docker, the plugin example. The in-process template
# plugin (template/) is compiled into xcl-docker. The external Docker plugin
# (docker/) is a standalone program, compiled to its own binary next to
# xcl-docker, which starts it as a separate process. Applying needs a Docker
# engine.

BUILD_DIR := build
XCL_DOCKER := $(BUILD_DIR)/xcl-docker
DOCKER_PLUGIN := $(BUILD_DIR)/docker-plugin
CONFIG_DIR := ./config

# MOCKERY is the pinned Mockery release that generates the Docker client mock,
# run through go run so it needs no install
MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0

.PHONY: build run test generate clean

# Build xcl-docker and the Docker plugin side by side, where xcl-docker
# finds the plugin
build:
	go build -o $(XCL_DOCKER) .
	go build -o $(DOCKER_PLUGIN) ./docker

# Build, then apply the example configuration, print the saved state and
# destroy it again, each a separate run of xcl-docker
run: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Run the example tests, they build the binaries themselves. The tests
# that need a Docker engine skip when none is reachable.
test:
	go test -v ./...

# Regenerate the mock of the Docker client interface, docker/client/mocks
generate:
	$(MOCKERY)

# Remove the built binaries, the rendered template and the saved state
clean:
	rm -rf $(BUILD_DIR) .xcl-docker
