package plugins

import (
	"fmt"

	"github.com/jumppad-labs/xcl/entity"
	"github.com/jumppad-labs/xcl/plugins/proto"
)

// toProtoChange converts a Change to its protocol form
func toProtoChange(change entity.Change) (proto.Change, error) {
	switch change {
	case entity.NoChange:
		return proto.Change_CHANGE_NO_CHANGE, nil
	case entity.Update:
		return proto.Change_CHANGE_UPDATE, nil
	case entity.Replace:
		return proto.Change_CHANGE_REPLACE, nil
	default:
		return proto.Change_CHANGE_NO_CHANGE, fmt.Errorf("unknown change %d", int(change))
	}
}

// fromProtoChange converts a Change from its protocol form. A value this
// version does not know is an error rather than NoChange, so a mismatched
// plugin cannot silently skip an update.
func fromProtoChange(change proto.Change) (entity.Change, error) {
	switch change {
	case proto.Change_CHANGE_NO_CHANGE:
		return entity.NoChange, nil
	case proto.Change_CHANGE_UPDATE:
		return entity.Update, nil
	case proto.Change_CHANGE_REPLACE:
		return entity.Replace, nil
	default:
		return entity.NoChange, fmt.Errorf("unknown change %d", int32(change))
	}
}

// toProtoDependencies converts a dependency list to its protocol form
func toProtoDependencies(dependencies []entity.DependencyChange) ([]*proto.DependencyChange, error) {
	if len(dependencies) == 0 {
		return nil, nil
	}

	converted := make([]*proto.DependencyChange, 0, len(dependencies))
	for _, dependency := range dependencies {
		change, err := toProtoChange(dependency.Change)
		if err != nil {
			return nil, fmt.Errorf("dependency %s: %w", dependency.Address, err)
		}

		converted = append(converted, &proto.DependencyChange{
			Address: dependency.Address,
			Change:  change,
		})
	}

	return converted, nil
}

// fromProtoDependencies converts a dependency list from its protocol form
func fromProtoDependencies(dependencies []*proto.DependencyChange) ([]entity.DependencyChange, error) {
	if len(dependencies) == 0 {
		return nil, nil
	}

	converted := make([]entity.DependencyChange, 0, len(dependencies))
	for _, dependency := range dependencies {
		change, err := fromProtoChange(dependency.GetChange())
		if err != nil {
			return nil, fmt.Errorf("dependency %s: %w", dependency.GetAddress(), err)
		}

		converted = append(converted, entity.DependencyChange{
			Address: dependency.GetAddress(),
			Change:  change,
		})
	}

	return converted, nil
}
