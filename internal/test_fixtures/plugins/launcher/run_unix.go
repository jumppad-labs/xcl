//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

// run replaces the launcher with program, so the plugin host's process is
// the plugin itself
func run(program string, args []string) {
	err := syscall.Exec(program, args, os.Environ())

	fmt.Fprintln(os.Stderr, err)
	os.Exit(2)
}
