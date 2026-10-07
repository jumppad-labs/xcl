// Package resources holds the Docker plugin's block types, docker "network"
// and docker "container", and the providers that create them as real Docker
// networks and containers. A program using the plugin imports it to read the
// blocks it applied, i.e. xcl.FindByType[resources.Container].
package resources

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
