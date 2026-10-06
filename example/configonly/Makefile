# Builds and runs the configuration only example. It uses no plugins, the
# block types are plain Go types registered with the plugin registry.

CONFIG_DIR := ./config

# The database password the configuration reads with env("DB_PASSWORD"), a
# demo value unless one is already set
DB_PASSWORD ?= example-password
export DB_PASSWORD

.PHONY: run test

# Run the example against its configuration
run:
	go run . $(CONFIG_DIR)

# Run the example tests
test:
	go test -v ./...