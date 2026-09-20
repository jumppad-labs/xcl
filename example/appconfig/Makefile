# Builds and runs the application configuration example. It uses no plugins,
# the block type is a plain Go type registered with the plugin registry.

CONFIG_DIR := ./config

.PHONY: run test

# Run the example against its configuration
run:
	go run . $(CONFIG_DIR)

# Run the example tests
test:
	go test -v ./...
