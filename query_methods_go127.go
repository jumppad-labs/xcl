//go:build go1.27

package xcl

// The lookup surface as methods on the configuration.
//
// This is the form the documentation presents as the destination, and it is
// what a developer on a current toolchain will reach for. Generic methods
// arrived in Go 1.27, so this file is excluded below that version and the
// package level functions in query.go remain available to everyone: a
// consumer on the project's minimum supported version loses no capability,
// only a spelling.
//
// Neither spelling carries any logic. Both delegate to the same unexported
// implementation, so they cannot drift in behaviour, and the tests assert that
// they return equal values and equal errors for identical inputs.
//
// As has no method form, because it takes no configuration for one to hang off.

// Find returns the entity declared at address, as T.
//
// See the package level Find for the full contract.
func (c *Config) Find[T any](address string) (*T, error) {
	return find[T](c, address)
}

// FindByType returns every entity whose address begins with the segments
// given, as T.
//
// See the package level FindByType for the full contract.
func (c *Config) FindByType[T any](path ...string) ([]*T, error) {
	return findByType[T](c, path...)
}

// FindOne returns the single entity matching the segments given.
//
// See the package level FindOne for the full contract.
func (c *Config) FindOne[T any](path ...string) (*T, error) {
	return findOne[T](c, path...)
}

// All returns every entity of the Go type T, without naming it as a string.
//
// See the package level All for the full contract.
func (c *Config) All[T any]() ([]*T, error) {
	return all[T](c)
}
