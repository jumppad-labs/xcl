//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// run starts program as a child with the launcher's standard streams and
// environment, and exits with its status, since Windows cannot replace a
// running process
func run(program string, args []string) {
	command := exec.Command(program, args[1:]...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Env = os.Environ()

	err := command.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
