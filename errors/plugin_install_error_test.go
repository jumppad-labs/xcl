package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// PluginInstallError. A plugin a remote registry could not install has to say
// which repository, release and platform it looked for, and still answer
// errors.Is for the sentinel that says why.

func TestPluginInstallErrorMessageNamesTheRepositoryVersionAndPlatform(t *testing.T) {
	err := &PluginInstallError{
		Repository: "jumppad-labs/xcl-plugin-docker",
		Version:    "v1.2.3",
		Platform:   "linux/arm64",
		Err:        fmt.Errorf("no build: %w", ErrPluginNotFound),
	}

	require.Equal(t, "plugin jumppad-labs/xcl-plugin-docker v1.2.3 for linux/arm64: no build: plugin release not found", err.Error())
}

func TestPluginInstallErrorMessageWithoutAPlatformOmitsIt(t *testing.T) {
	err := &PluginInstallError{
		Repository: "jumppad-labs/xcl-plugin-docker",
		Version:    "v1.2.3",
		Err:        errors.New("boom"),
	}

	require.Equal(t, "plugin jumppad-labs/xcl-plugin-docker v1.2.3: boom", err.Error())
}

func TestPluginInstallErrorMatchesErrPluginNotFoundWhenItWrapsIt(t *testing.T) {
	err := &PluginInstallError{Repository: "o/r", Version: "v1", Platform: "linux/amd64", Err: fmt.Errorf("no build: %w", ErrPluginNotFound)}

	require.ErrorIs(t, err, ErrPluginNotFound)
}

func TestPluginInstallErrorMatchesErrPluginVerificationWhenItWrapsIt(t *testing.T) {
	err := &PluginInstallError{Repository: "o/r", Version: "v1", Platform: "linux/amd64", Err: fmt.Errorf("checksum mismatch: %w", ErrPluginVerification)}

	require.ErrorIs(t, err, ErrPluginVerification)
}

func TestPluginInstallErrorWrappingNotFoundDoesNotMatchErrPluginVerification(t *testing.T) {
	err := &PluginInstallError{Repository: "o/r", Version: "v1", Platform: "linux/amd64", Err: fmt.Errorf("no build: %w", ErrPluginNotFound)}

	require.NotErrorIs(t, err, ErrPluginVerification)
}

func TestPluginInstallErrorWrappingVerificationDoesNotMatchErrPluginNotFound(t *testing.T) {
	err := &PluginInstallError{Repository: "o/r", Version: "v1", Platform: "linux/amd64", Err: fmt.Errorf("bad signature: %w", ErrPluginVerification)}

	require.NotErrorIs(t, err, ErrPluginNotFound)
}

func TestPluginInstallErrorDetailIsRecoverableThroughAWrap(t *testing.T) {
	wrapped := fmt.Errorf("installing plugins: %w", &PluginInstallError{
		Repository: "o/r",
		Version:    "v1.0.0",
		Platform:   "linux/amd64",
		Err:        ErrPluginNotFound,
	})

	var detail *PluginInstallError
	require.True(t, errors.As(wrapped, &detail))
	require.Equal(t, "o/r", detail.Repository)
	require.Equal(t, "v1.0.0", detail.Version)
	require.Equal(t, "linux/amd64", detail.Platform)
}
