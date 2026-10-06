package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// externalPlugin is the path of the suite's external plugin binary, built once
// for the whole suite by TestMain
var externalPlugin string

// TestMain builds the external plugin into a temporary directory before the
// tests run and removes it afterwards, so the suite never depends on a binary
// built by hand
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "xcl-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create the build directory: %s\n", err)
		os.Exit(1)
	}

	externalPlugin = filepath.Join(dir, "externalplugin")

	build := exec.Command("go", "build", "-o", externalPlugin, "./fixtures/externalplugin")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "unable to build the external plugin: %s\n%s", err, output)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(dir)
	os.Exit(code)
}
