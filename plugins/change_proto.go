package plugins

import (
	"encoding/json"
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

// toProtoStepKind converts a path step kind to its protocol form
func toProtoStepKind(kind entity.StepKind) (proto.StepKind, error) {
	switch kind {
	case entity.StepAttribute:
		return proto.StepKind_STEP_KIND_ATTRIBUTE, nil
	case entity.StepIndex:
		return proto.StepKind_STEP_KIND_INDEX, nil
	case entity.StepKey:
		return proto.StepKind_STEP_KIND_KEY, nil
	default:
		return proto.StepKind_STEP_KIND_ATTRIBUTE, fmt.Errorf("unknown step kind %d", int(kind))
	}
}

// fromProtoStepKind converts a path step kind from its protocol form. A value
// this version does not know is an error, so a mismatched plugin cannot
// misread a path.
func fromProtoStepKind(kind proto.StepKind) (entity.StepKind, error) {
	switch kind {
	case proto.StepKind_STEP_KIND_ATTRIBUTE:
		return entity.StepAttribute, nil
	case proto.StepKind_STEP_KIND_INDEX:
		return entity.StepIndex, nil
	case proto.StepKind_STEP_KIND_KEY:
		return entity.StepKey, nil
	default:
		return entity.StepAttribute, fmt.Errorf("unknown step kind %d", int32(kind))
	}
}

// toProtoPath converts a path to its protocol form
func toProtoPath(path entity.Path) ([]*proto.PathStep, error) {
	if len(path) == 0 {
		return nil, nil
	}

	converted := make([]*proto.PathStep, 0, len(path))
	for _, step := range path {
		kind, err := toProtoStepKind(step.Kind)
		if err != nil {
			return nil, err
		}

		converted = append(converted, &proto.PathStep{
			Kind:      kind,
			Attribute: step.Attribute,
			Index:     int64(step.Index),
			Key:       step.Key,
		})
	}

	return converted, nil
}

// fromProtoPath converts a path from its protocol form
func fromProtoPath(path []*proto.PathStep) (entity.Path, error) {
	if len(path) == 0 {
		return nil, nil
	}

	converted := make(entity.Path, 0, len(path))
	for _, step := range path {
		kind, err := fromProtoStepKind(step.GetKind())
		if err != nil {
			return nil, err
		}

		converted = append(converted, entity.Step{
			Kind:      kind,
			Attribute: step.GetAttribute(),
			Index:     int(step.GetIndex()),
			Key:       step.GetKey(),
		})
	}

	return converted, nil
}

// toProtoValue encodes a change value as JSON; nil is absent and encodes to
// empty bytes
func toProtoValue(value any) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	return json.Marshal(value)
}

// fromProtoValue decodes a change value from JSON; empty bytes are absent and
// decode to nil
func fromProtoValue(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

// toProtoPropertyChanges converts a property change list to its protocol
// form. Each value is encoded on its own: the PropertyChange itself is never
// marshalled, as its JSON form masks sensitive values.
func toProtoPropertyChanges(changes []entity.PropertyChange) ([]*proto.PropertyChange, error) {
	if len(changes) == 0 {
		return nil, nil
	}

	converted := make([]*proto.PropertyChange, 0, len(changes))
	for _, change := range changes {
		path, err := toProtoPath(change.Path)
		if err != nil {
			return nil, fmt.Errorf("change %s: %w", change.Path, err)
		}

		before, err := toProtoValue(change.Before)
		if err != nil {
			return nil, fmt.Errorf("change %s: before: %w", change.Path, err)
		}

		after, err := toProtoValue(change.After)
		if err != nil {
			return nil, fmt.Errorf("change %s: after: %w", change.Path, err)
		}

		converted = append(converted, &proto.PropertyChange{
			Path:      path,
			Before:    before,
			After:     after,
			Unknown:   change.Unknown,
			Sensitive: change.Sensitive,
		})
	}

	return converted, nil
}

// fromProtoPropertyChanges converts a property change list from its protocol
// form
func fromProtoPropertyChanges(changes []*proto.PropertyChange) ([]entity.PropertyChange, error) {
	if len(changes) == 0 {
		return nil, nil
	}

	converted := make([]entity.PropertyChange, 0, len(changes))
	for _, change := range changes {
		path, err := fromProtoPath(change.GetPath())
		if err != nil {
			return nil, fmt.Errorf("change: %w", err)
		}

		before, err := fromProtoValue(change.GetBefore())
		if err != nil {
			return nil, fmt.Errorf("change %s: before: %w", path, err)
		}

		after, err := fromProtoValue(change.GetAfter())
		if err != nil {
			return nil, fmt.Errorf("change %s: after: %w", path, err)
		}

		converted = append(converted, entity.PropertyChange{
			Path:      path,
			Before:    before,
			After:     after,
			Unknown:   change.GetUnknown(),
			Sensitive: change.GetSensitive(),
		})
	}

	return converted, nil
}
