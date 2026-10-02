package errors

import (
	"errors"
	"fmt"
)

// ErrPluginLoad is returned by the first Validate, Apply or Destroy of a
// configuration when a registered plugin cannot be loaded. Plugins are loaded
// when they are first needed rather than when they are registered, so this is
// where a missing or broken plugin is reported. Check for it with errors.Is,
// and recover the plugin's name with errors.As and PluginLoadError.
//
// It is declared here, rather than in the registry that returns it, so the
// public package can re-export it alongside the other shared errors.
var ErrPluginLoad = errors.New("plugin failed to load")

// PluginLoadError reports the plugin that failed to load and why. Plugin is
// the plugin's name for an in-process plugin, or its path for an external one.
type PluginLoadError struct {
	Plugin string
	Err    error
}

func (e *PluginLoadError) Error() string {
	return fmt.Sprintf("plugin %s failed to load: %s", e.Plugin, e.Err)
}

// Unwrap returns ErrPluginLoad and the reason the plugin failed, so both
// answer errors.Is
func (e *PluginLoadError) Unwrap() []error { return []error{ErrPluginLoad, e.Err} }
