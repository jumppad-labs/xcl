// Package diff holds the result of comparing an xcl configuration with the
// state its last apply saved: what an apply of that configuration would do,
// without doing any of it.
//
// A Diff lists every provider-backed resource an apply would create, update,
// replace or delete, sorted by address, and counts the resources that would
// stay unchanged. Each listed Resource carries the Changes to its configured
// values, one per changed field, with the field's Path and its value before
// and after. A value that is only known once an apply has run is marked
// Unknown, and a sensitive value is marked Sensitive and carries neither
// value unless the diff was asked to reveal them with RevealSensitive.
//
// The types are plain data: code reads them directly, and json.Marshal of a
// Diff is its machine-readable form.
package diff

// Action is what an apply would do with a resource.
type Action string

const (
	// ActionCreate is a resource declared in the configuration and not in the
	// saved state, or one whose provider reports its real counterpart is gone.
	ActionCreate Action = "create"

	// ActionUpdate is a resource whose provider reports a change, or one that
	// depends on a value only known once an apply has run.
	ActionUpdate Action = "update"

	// ActionReplace is a resource an apply destroys and creates again: one
	// whose last apply failed, one its provider cannot update in place, or
	// one whose provider answers replace because a resource it depends on is
	// replaced. Resource.Reason says which.
	ActionReplace Action = "replace"

	// ActionDelete is a resource in the saved state that is no longer in the
	// configuration.
	ActionDelete Action = "delete"
)

// ReplaceReason says why a resource will be replaced.
type ReplaceReason string

const (
	// ReplaceFailed is a resource whose last apply failed.
	ReplaceFailed ReplaceReason = "failed"

	// ReplaceProvider is a resource its provider cannot update in place.
	ReplaceProvider ReplaceReason = "provider"

	// ReplaceDependency is a resource that depends on a resource that is
	// replaced.
	ReplaceDependency ReplaceReason = "dependency"
)

// Diff is what an apply of a configuration would do.
type Diff struct {
	// Summary counts the resources for each action and the unchanged ones.
	Summary Summary `json:"summary"`

	// Resources lists every resource an apply would change, sorted by
	// address. Unchanged resources are counted in Summary but not listed.
	Resources []Resource `json:"resources"`
}

// Changed returns the number of resources an apply would change: the sum of
// the create, update, replace and delete counts. A nil Diff changes nothing.
func (d *Diff) Changed() int {
	if d == nil {
		return 0
	}

	return d.Summary.Create + d.Summary.Update + d.Summary.Replace + d.Summary.Delete
}

// Summary counts the resources a diff found for each action, and those that
// would not change.
type Summary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Replace   int `json:"replace"`
	Delete    int `json:"delete"`
	Unchanged int `json:"unchanged"`
}

// Resource is one resource an apply would change.
type Resource struct {
	// Address is the resource's address, such as resource.container.api.
	Address string `json:"address"`

	// Action is what an apply would do with the resource.
	Action Action `json:"action"`

	// Reason says why a resource with ActionReplace is replaced. It is
	// empty for every other action.
	Reason ReplaceReason `json:"reason,omitempty"`

	// ReplacedDeps are the addresses of the replaced resources behind a
	// replacement with ReplaceDependency, sorted. It is empty for every
	// other reason and action.
	ReplacedDeps []string `json:"replaced_dependencies,omitempty"`

	// Changes lists the configured values that would change, one per
	// changed field in field declaration order. An update may have none,
	// when its real counterpart changed outside xcl, and a delete never has
	// any.
	Changes []Change `json:"changes,omitempty"`
}

// Change is one configured value that would change.
type Change struct {
	// Path locates the value within the resource.
	Path Path `json:"path"`

	// Before is the saved value. It is absent for an added value and for a
	// sensitive one that was not revealed.
	Before any `json:"before,omitempty"`

	// After is the configured value. It is absent for a removed value, an
	// unknown one and a sensitive one that was not revealed.
	After any `json:"after,omitempty"`

	// Unknown is set when the value is only known once an apply has run.
	Unknown bool `json:"unknown,omitempty"`

	// Sensitive is set when the value is sensitive, whether or not it was
	// revealed.
	Sensitive bool `json:"sensitive,omitempty"`
}
