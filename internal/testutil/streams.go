package testutil

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// CapturedOutput is what was written to the process's standard output and
// standard error while a function ran
type CapturedOutput struct {
	Stdout string
	Stderr string
}

// CaptureStandardStreams runs fn with os.Stdout and os.Stderr redirected to
// pipes, and returns what was written to each. The streams are restored when
// fn returns, and again in cleanup should fn fail the test
func CaptureStandardStreams(t testing.TB, fn func()) CapturedOutput {
	t.Helper()

	originalStdout := os.Stdout
	originalStderr := os.Stderr

	restore := func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}
	t.Cleanup(restore)

	stdoutReader, stdoutWriter, err := os.Pipe()
	require.NoError(t, err)

	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	var drained sync.WaitGroup
	drained.Go(func() { _, _ = io.Copy(stdout, stdoutReader) })
	drained.Go(func() { _, _ = io.Copy(stderr, stderrReader) })

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	fn()

	restore()

	require.NoError(t, stdoutWriter.Close())
	require.NoError(t, stderrWriter.Close())
	drained.Wait()

	require.NoError(t, stdoutReader.Close())
	require.NoError(t, stderrReader.Close())

	return CapturedOutput{Stdout: stdout.String(), Stderr: stderr.String()}
}
