package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrPluginNotFound means a remote registry could not find a plugin to
// install: the release does not exist, it has no build for the current
// platform, or the repository is private and no token was given. The GitHub
// registry returns it, inside a PluginInstallError, when a plugin it was asked
// to install cannot be found. Check for it with errors.Is.
var ErrPluginNotFound = errors.New("plugin release not found")

// ErrPluginVerification means a plugin a remote registry downloaded, or one it
// kept in its cache, failed verification: the release has no checksums file,
// a file does not match its checksum, or, when trusted keys were given, the
// release is not signed or its signature does not verify against any of them.
// The GitHub registry returns it, inside a PluginInstallError, and never
// starts a plugin that fails verification. Check for it with errors.Is.
var ErrPluginVerification = errors.New("plugin failed verification")

// PluginInstallError reports the plugin a remote registry could not install,
// or could not verify, and why. Repository is the repository the plugin was
// declared from (owner/repo), Version the exact release it was pinned to and
// Platform the build it looked for (linux/arm64). Err wraps ErrPluginNotFound
// or ErrPluginVerification with the detail.
type PluginInstallError struct {
	Repository string
	Version    string
	Platform   string
	Err        error
}

func (e *PluginInstallError) Error() string {
	parts := []string{"plugin"}
	if e.Repository != "" {
		parts = append(parts, e.Repository)
	}
	if e.Version != "" {
		parts = append(parts, e.Version)
	}
	if e.Platform != "" {
		parts = append(parts, "for", e.Platform)
	}

	return fmt.Sprintf("%s: %s", strings.Join(parts, " "), e.Err)
}

// Unwrap returns the reason the plugin could not be installed, so the
// sentinel it wraps answers errors.Is
func (e *PluginInstallError) Unwrap() error { return e.Err }
