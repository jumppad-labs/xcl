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

// PluginLoadError reports the plugin that failed to load, the registry it
// came from, and why. Plugin is the plugin's name, the Go type name of an
// in-process plugin or the file name of a plugin binary. Plugin is empty when
// the registry itself failed to provide its plugins, i.e. when a plugin
// directory could not be read.
type PluginLoadError struct {
	Plugin   string
	Registry string
	Err      error
}

func (e *PluginLoadError) Error() string {
	switch {
	case e.Plugin == "":
		return fmt.Sprintf("registry %s failed to load its plugins: %s", e.Registry, e.Err)
	case e.Registry == "":
		return fmt.Sprintf("plugin %s failed to load: %s", e.Plugin, e.Err)
	default:
		return fmt.Sprintf("plugin %s from registry %s failed to load: %s", e.Plugin, e.Registry, e.Err)
	}
}

// Unwrap returns ErrPluginLoad and the reason the plugin failed, so both
// answer errors.Is
func (e *PluginLoadError) Unwrap() []error { return []error{ErrPluginLoad, e.Err} }
