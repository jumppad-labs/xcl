package catalog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jumppad-labs/xcl/events"
	"github.com/jumppad-labs/xcl/plugins"
	"github.com/jumppad-labs/xcl/registry"
)

// localWith returns a local registry holding the in-process plugins ps, in
// the order given
func localWith(ps ...plugins.Plugin) *registry.Local {
	local := registry.NewLocal()
	for _, p := range ps {
		local.RegisterPlugin(p)
	}

	return local
}

// namedRegistry is a local registry known by a name of its own, so tests can
// tell two registries apart in events and errors
type namedRegistry struct {
	*registry.Local
	name string
}

// Name returns the name the registry was given
func (r namedRegistry) Name() string {
	return r.name
}

// failingRegistry is a registry that cannot provide its plugins
type failingRegistry struct {
	name string
}

// Name returns the name the registry was given
func (r failingRegistry) Name() string {
	return r.name
}

// Plugins always fails
func (r failingRegistry) Plugins(ctx context.Context, emit events.Emit) ([]registry.Plugin, error) {
	return nil, fmt.Errorf("registry %s is unreachable", r.name)
}

// recoverPanic runs f and returns the value it panicked with, nil when it did
// not panic
func recoverPanic(f func()) (value any) {
	defer func() {
		value = recover()
	}()

	f()

	return nil
}

// testPluginSetup builds plugin binaries into a temporary directory
type testPluginSetup struct {
	t       *testing.T
	testDir string
}

// newTestPluginSetup creates a new test setup with its own directory
func newTestPluginSetup(t *testing.T) *testPluginSetup {
	return &testPluginSetup{
		t:       t,
		testDir: t.TempDir(),
	}
}

// buildExamplePlugin builds the example plugin binary and returns its path
func (s *testPluginSetup) buildExamplePlugin(outputName string) string {
	outputPath := filepath.Join(s.testDir, outputName)
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", outputPath, "./plugins/example")
	cmd.Dir = getRootDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("Failed to build example plugin: %v\nOutput: %s", err, output)
	}

	return outputPath
}

// createBrokenPlugin creates an executable in dir, named name, that exits
// at once with an error rather than serving a plugin
func (s *testPluginSetup) createBrokenPlugin(dir, name string) string {
	if runtime.GOOS == "windows" {
		s.t.Skip("the broken plugin is a shell script")
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		s.t.Fatalf("Failed to create broken plugin %s: %v", path, err)
	}

	return path
}

// getRootDir finds the project root directory
func getRootDir() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			panic("Could not find project root")
		}

		dir = parent
	}
}

// defaultWait and pollInterval bound how long a test waits for a plugin
// process to exit
const (
	defaultWait  = 5 * time.Second
	pollInterval = 50 * time.Millisecond
)
