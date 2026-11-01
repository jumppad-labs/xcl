// Command recordargs writes the arguments it was started with, one per line,
// to the file named by XCL_RECORD_ARGS, then exits with status 1. It is not
// a plugin: a test starts it as one to see which arguments the host passes to
// a plugin process, and expects the start itself to fail.
package main

import (
	"os"
	"strings"
)

func main() {
	if file := os.Getenv("XCL_RECORD_ARGS"); file != "" {
		os.WriteFile(file, []byte(strings.Join(os.Args[1:], "\n")), 0644)
	}

	os.Exit(1)
}
