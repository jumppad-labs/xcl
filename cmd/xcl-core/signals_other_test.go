//go:build !windows

package main

import (
	"os/exec"
	"testing"
)

// newProcessGroup does nothing outside Windows, where a signal is sent to the
// program's process alone
func newProcessGroup(_ *exec.Cmd) {}

// sendCtrlBreak exists only on Windows; a test that reaches it elsewhere is
// wrong
func sendCtrlBreak(t *testing.T, _ int) {
	t.Helper()

	t.Fatal("console control events exist only on Windows")
}
