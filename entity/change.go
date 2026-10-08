package entity

// Change is what applying a new configuration needs for one resource.
type Change int

const (
	// NoChange leaves the resource as it is.
	NoChange Change = iota
	// Update calls the provider's Update to change the resource in place.
	Update
	// Replace destroys the resource, then creates it again.
	Replace
)

// String returns the change in words: "no change", "update" or "replace".
func (c Change) String() string {
	switch c {
	case NoChange:
		return "no change"
	case Update:
		return "update"
	case Replace:
		return "replace"
	default:
		return "unknown"
	}
}

// DependencyChange is a resource this one depends on that the same apply will
// update or replace.
type DependencyChange struct {
	// Address is the dependency's address, for example "docker.network.app".
	Address string
	// Change is Update or Replace, never NoChange.
	Change Change
}
