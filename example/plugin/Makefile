# Builds and runs xcl-docker, the plugin example. The in-process template
# plugin (plugins/template/) is compiled into xcl-docker. The external Docker
# plugin (plugins/docker/) is a standalone program, compiled to its own binary
# next to xcl-docker, which starts it as a separate process. Applying needs a
# Docker engine.

BUILD_DIR := build
XCL_DOCKER := $(BUILD_DIR)/xcl-docker
DOCKER_PLUGIN := $(BUILD_DIR)/docker-plugin
CONFIG_DIR := ./config
SUBNET_DIR := ./config-subnet
SWAP_DIR := ./config-swap
INIT_DIR := ./config-init
REMOVE_DIR := ./config-remove

# MOCKERY is the pinned Mockery release that generates the Docker client mock,
# run through go run so it needs no install
MOCKERY := go run github.com/vektra/mockery/v3@v3.8.0

.PHONY: build run replace swap rebuild-init remove-network test generate clean

# Build xcl-docker and the Docker plugin side by side, where xcl-docker
# finds the plugin
build:
	go build -o $(XCL_DOCKER) .
	go build -o $(DOCKER_PLUGIN) ./plugins/docker

# Build, then apply the example configuration, print the saved state and
# destroy it again, each a separate run of xcl-docker
run: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Build, apply the example configuration, then show and apply the address
# range change in config-subnet, which replaces the network and updates the
# container attached to it in place, reattaching it to the new network, and
# updates the template, then destroy everything again
replace: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) plan $(SUBNET_DIR)
	$(XCL_DOCKER) apply $(SUBNET_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Build, apply the example configuration, then show and apply config-swap,
# which adds a backend network and moves the running container to it in
# place, then destroy everything again
swap: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) plan $(SWAP_DIR)
	$(XCL_DOCKER) apply $(SWAP_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Build, apply the example configuration, then show and apply config-init,
# which moves the init script so its template is replaced, and the container
# that mounts it with it, then destroy everything again
rebuild-init: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) plan $(INIT_DIR)
	$(XCL_DOCKER) apply $(INIT_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Build, apply the example configuration, then show and apply config-remove,
# which removes the network and every reference to it, leaving the container
# running with no network, then destroy everything again
remove-network: build
	$(XCL_DOCKER) apply $(CONFIG_DIR)
	$(XCL_DOCKER) plan $(REMOVE_DIR)
	$(XCL_DOCKER) apply $(REMOVE_DIR)
	$(XCL_DOCKER) status
	$(XCL_DOCKER) destroy

# Run the example tests, they build the binaries themselves. The tests
# that need a Docker engine skip when none is reachable.
test:
	go test -v ./...

# Regenerate the mock of the Docker client interface, plugins/docker/client/mocks
generate:
	$(MOCKERY)

# Remove the built binaries, the rendered template and the saved state
clean:
	rm -rf $(BUILD_DIR) .xcl-docker
