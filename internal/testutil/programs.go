package testutil

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ProgramRun is what one run of a built program produced
type ProgramRun struct {
	Stdout string
	Stderr string
	Err    error
}

// BuildProgram compiles the Go package in directory into a temporary
// directory and returns the path of the binary, so a test exercises main
// exactly as a user running the program would
func BuildProgram(t testing.TB, directory string) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "program")

	build := exec.Command("go", "build", "-o", binary, directory)
	output, err := build.CombinedOutput()
	require.NoError(t, err, "unable to build %s: %s", directory, output)

	return binary
}

// RunProgram runs binary with args from the test's working directory and
// returns what it wrote and how it exited
func RunProgram(t testing.TB, binary string, args ...string) ProgramRun {
	t.Helper()

	var stdout, stderr bytes.Buffer

	command := exec.Command(binary, args...)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()

	return ProgramRun{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
}
