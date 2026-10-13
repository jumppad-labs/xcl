package main

import (
	"os/exec"
	"syscall"
	"testing"
)

// newProcessGroup starts the program in a process group of its own, so a
// console control event can be sent to it alone and not to the test
func newProcessGroup(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// sendCtrlBreak sends CTRL_BREAK_EVENT to the process group pid leads
func sendCtrlBreak(t *testing.T, pid int) {
	t.Helper()

	generate := syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

	sent, _, err := generate.Call(uintptr(syscall.CTRL_BREAK_EVENT), uintptr(pid))
	if sent == 0 {
		t.Fatalf("GenerateConsoleCtrlEvent failed: %v", err)
	}
}
