package plugin

import (
	"errors"
	goplugin "plugin"
)

// Loader resolves plugin binaries into Plugin implementations.
type Loader interface {
	Load(path string) (Plugin, error)
}

// GoPluginLoader uses the Go standard library plugin mechanism to dynamically load modules.
type GoPluginLoader struct{}

// Load opens the shared object and searches for a `Plugin` symbol implementing the Plugin interface.
func (GoPluginLoader) Load(path string) (Plugin, error) {
	if path == "" {
		return nil, errors.New("plugin path cannot be empty")
	}
	so, err := goplugin.Open(path)
	if err != nil {
		return nil, err
	}
	symbol, err := so.Lookup("Plugin")
	if err != nil {
		return nil, err
	}
	switch p := symbol.(type) {
	case Plugin:
		return p, nil
	case *Plugin:
		if p == nil {
			return nil, errors.New("plugin symbol is nil")
		}
		return *p, nil
	case func() Plugin:
		return p(), nil
	default:
		return nil, errors.New("plugin symbol must implement plugin.Plugin")
	}
}
