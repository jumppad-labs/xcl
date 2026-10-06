// Package failingexample is a module whose only test fails, the e2e suite
// runs it to show the example runner reports a failing example by name
package failingexample

import "testing"

func TestFails(t *testing.T) {
	t.Fatal("this example's tests always fail")
}
