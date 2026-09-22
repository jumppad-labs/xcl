package xcl

import (
	"bytes"
	"log"
	"path/filepath"
	"testing"

	"github.com/jumppad-labs/xcl/logger"
	"github.com/stretchr/testify/require"
)

// captureStandardLogger points the standard library logger at a buffer with
// no flags, so a message written through it can be found verbatim, and puts
// back the original output and flags when the test ends
func captureStandardLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	originalOutput := log.Writer()
	originalFlags := log.Flags()

	t.Cleanup(func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	})

	buffer := &bytes.Buffer{}
	log.SetOutput(buffer)
	log.SetFlags(0)

	return buffer
}

// TestStandardLoggerOutputIsUntouchedAfterApply asserts Apply leaves the
// application's standard library logger writing where the application
// pointed it
func TestStandardLoggerOutputIsUntouchedAfterApply(t *testing.T) {
	buffer := captureStandardLogger(t)

	applyQueryFixtureWithEvents(t)

	log.Print("written after apply")

	require.Contains(t, buffer.String(), "written after apply")
}

// TestStandardLoggerOutputIsUntouchedAfterValidate asserts Validate leaves
// the application's standard library logger writing where the application
// pointed it
func TestStandardLoggerOutputIsUntouchedAfterValidate(t *testing.T) {
	buffer := captureStandardLogger(t)

	path, err := filepath.Abs("./internal/test_fixtures/config/simple/container.xcl")
	require.NoError(t, err)

	c, _, _ := setupConfig(t)

	err = c.Validate(path)
	require.NoError(t, err)

	log.Print("written after validate")

	require.Contains(t, buffer.String(), "written after validate")
}

// TestStandardLoggerOutputIsUntouchedAfterDestroy asserts Destroy leaves the
// application's standard library logger writing where the application
// pointed it. The prior state comes from a real apply, and the logger is only
// redirected once that apply is done so the test sees what the destroy does.
func TestStandardLoggerOutputIsUntouchedAfterDestroy(t *testing.T) {
	f := setupDestroyConfig(t, logger.Nop())
	applyDestroyFixture(t, f)

	buffer := captureStandardLogger(t)

	err := f.config.Destroy()
	require.NoError(t, err)
	require.Len(t, f.plugin.GetDestroyedResources(), 4)

	log.Print("written after destroy")

	require.Contains(t, buffer.String(), "written after destroy")
}
