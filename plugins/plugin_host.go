package plugins

import (
	"context"

	"github.com/jumppad-labs/xcl/entity"
)

// PluginHost is the unified interface for both in-process and external plugins.
// This interface abstracts the communication mechanism, allowing the rest of the
// codebase to work with plugins regardless of whether they are in-process or external.
type PluginHost interface {
	// GetTypes returns the types handled by the plugin
	GetTypes() []RegisteredType

	// Validate validates the given entity data
	Validate(ctx context.Context, entityType, entitySubType string, entityData []byte) error

	// Create creates a new entity
	Create(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)

	// Destroy deletes an existing entity
	Destroy(ctx context.Context, entityType, entitySubType string, entityData []byte) error

	// Read reports the real entity, given its saved and configured copies
	Read(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte) ([]byte, error)

	// Update updates an existing entity
	Update(ctx context.Context, entityType, entitySubType string, entityData []byte) ([]byte, error)

	// Changed decides what applying new needs for the entity saved as old,
	// given the dependencies the same apply will update or replace
	Changed(ctx context.Context, entityType, entitySubType string, oldEntityData []byte, newEntityData []byte, dependencies []entity.DependencyChange) (entity.Change, error)

	// Stop shuts down the plugin host and cleans up resources
	Stop()
}
