// Command launcher runs the program named by its first argument with the
// rest of its arguments and its own environment, as an interpreter runs a
// script. Tests register a plugin as the command launcher plus a plugin
// binary, to start a plugin through a program that is not the plugin itself.
package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: launcher <program> [args...]")
		os.Exit(2)
	}

	program, err := exec.LookPath(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	run(program, os.Args[1:])
}
