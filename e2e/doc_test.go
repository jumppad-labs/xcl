// Package e2e_test holds black-box tests of the whole library. They use xcl
// only through its public packages and the suite's own fixtures, the block
// types, configurations and plugins under fixtures and testdata, exactly as an
// application or plugin author would. The only internal package they may
// import is internal/testutil, which holds test helpers and no library code.
//
// The suite does not run the examples. Each example is a module of its own,
// tested on its own by the Examples workflow, which checks the examples work,
// not the library.
package e2e_test
