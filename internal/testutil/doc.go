// Package testutil holds the test helpers shared by more than one package.
//
// Go only compiles a _test.go file into its own package's tests, so a helper
// two packages need has to live in an ordinary file. Only _test.go files may
// import this package, which keeps it out of every library and binary. It
// must not import the root xcl package, the root package's own tests import
// it and that would be an import cycle
package testutil
