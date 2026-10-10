// Package providers holds the Docker plugin's resource providers, which create
// docker "network" and docker "container" blocks as real Docker networks and
// containers. Each provider is built from the container task layer in
// client/containers, so no provider imports a Docker library: the real task
// layer when the plugin runs and a mock in the unit tests.
package providers

import "github.com/jumppad-labs/xcl/types"

// The labels every Docker object the plugin creates carries, so the objects
// the example made can be found with
//
//	docker network ls --filter label=created_by=xcl-example-plugin
//	docker ps --filter label=created_by=xcl-example-plugin
const (
	LabelCreatedBy = "created_by"
	CreatedByValue = "xcl-example-plugin"
	LabelXCLID     = "xcl_id"
)

// labels returns the labels for the Docker object created for the block meta
// describes
func labels(meta types.Meta) map[string]string {
	return map[string]string{
		LabelCreatedBy: CreatedByValue,
		LabelXCLID:     meta.ID,
	}
}
