package entities

import "github.com/jumppad-labs/xcl/types"

// Container defines the block type docker "container", a Docker container
// named after the block
type Container struct {
	types.ResourceBase `xcl:",remain"`

	// Image is the image the container runs, it is pulled when Docker does
	// not have it
	Image string `xcl:"image" json:"image"`

	// Command overrides the image's command
	Command []string `xcl:"command,optional" json:"command,omitempty"`

	// Environment holds the container's environment variables
	Environment map[string]string `xcl:"environment,optional" json:"environment,omitempty"`

	// InitScript is the path of a script the container runs when it starts,
	// mounted read-only into the nginx image's entrypoint directory. It is
	// usually a template's destination, so replacing that template replaces
	// the container.
	InitScript string `xcl:"init_script,optional" json:"init_script,omitempty"`

	// Networks are the networks the container joins, one nested network block
	// each. The container is created on the first and connected to the rest.
	Networks []NetworkAttachment `xcl:"network,block" json:"network,omitempty"`

	// DockerID is computed, the ID Docker gives the container
	DockerID string `xcl:"docker_id,optional,computed" json:"docker_id,omitempty"`

	// IPAddress is computed, the container's address on its first network
	IPAddress string `xcl:"ip_address,optional,computed" json:"ip_address,omitempty"`
}

// NetworkAttachment is a nested network block of a docker "container" block
type NetworkAttachment struct {
	// Name is the Docker name of the network, i.e. docker.network.app.meta.name
	Name string `xcl:"name" json:"name"`

	// Aliases are extra names the container is known by on the network
	Aliases []string `xcl:"aliases,optional" json:"aliases,omitempty"`
}
