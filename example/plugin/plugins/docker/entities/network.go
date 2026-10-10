// Package entities holds the Docker plugin's block types, docker "network"
// and docker "container", and nothing else: no providers, no Docker client
// and no Docker libraries. A program using the plugin imports only this
// package to read the blocks it applied, i.e.
// xcl.FindByType[entities.Container].
package entities

import "github.com/jumppad-labs/xcl/types"

// Network defines the block type docker "network", a Docker bridge network
// named after the block
type Network struct {
	types.ResourceBase `xcl:",remain"`

	// Subnet is the network's address range, i.e. 10.42.0.0/24. When it is
	// not set Docker picks one.
	Subnet string `xcl:"subnet,optional" json:"subnet,omitempty"`

	// DockerID is computed, the provider sets it to the ID Docker gives the
	// network when it is created
	DockerID string `xcl:"docker_id,optional,computed" json:"docker_id,omitempty"`
}
