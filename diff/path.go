package diff

import "github.com/jumppad-labs/xcl/entity"

// StepKind is the kind of a Path step, see entity.StepKind.
type StepKind = entity.StepKind

const (
	// StepAttribute names a field or nested block, such as image.
	StepAttribute = entity.StepAttribute

	// StepIndex is a position in a list, such as [0].
	StepIndex = entity.StepIndex

	// StepKey is a key in a map, such as ["LOG_LEVEL"].
	StepKey = entity.StepKey
)

// Step is one segment of a Path, see entity.Step.
type Step = entity.Step

// Path locates a value within a resource, see entity.Path. The plan and the
// changes told to plugins share the one type.
type Path = entity.Path
