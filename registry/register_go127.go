//go:build go1.27

package registry

// Declaring a block type by its Go type, as methods on the registries.
//
// Generic methods arrived in Go 1.27, so this file is excluded below that
// version and the package level Register remains available to everyone. A
// generic method cannot be part of an interface, so these are on the
// registries types are declared on rather than on Registry.

// Register declares the Go type T as a block type, as the package level
// Register does.
func (l *Local) Register[T any](name ...string) {
	register[T](l, name...)
}

// Register declares the Go type T as a block type, as the package level
// Register does.
func (g *GitHub) Register[T any](name ...string) {
	register[T](g, name...)
}
