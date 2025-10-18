package plugin

import "context"

// Plugin defines the lifecycle hooks that each plugin implementation must satisfy.
type Plugin interface {
	// Info returns the static metadata for the plugin.
	Info() Info
	// Configure allows the plugin to inspect its configuration block prior to initialisation.
	// Implementations may mutate the configuration map to inject defaults.
	Configure(cfg map[string]any) error
	// Init prepares the plugin for use.
	Init(ctx *ExecutionContext) error
	// Start activates the plugin and should spawn long running routines if required.
	Start(ctx *ExecutionContext) error
	// Stop gracefully halts the plugin and releases any resources.
	Stop(ctx *ExecutionContext) error
}

// ExecutionContext is passed to plugins for every lifecycle stage.
type ExecutionContext struct {
	// C is the underlying context for cancellation and deadlines.
	C context.Context
	// Config is the plugin specific configuration block merged with manager overrides.
	Config map[string]any
	// Resources exposes shared services supplied by the host application.
	Resources map[string]any
}

// Clone returns a shallow copy of the execution context so plugins can safely mutate maps.
func (c *ExecutionContext) Clone() *ExecutionContext {
	if c == nil {
		return nil
	}
	dup := *c
	if c.Config != nil {
		dup.Config = make(map[string]any, len(c.Config))
		for k, v := range c.Config {
			dup.Config[k] = v
		}
	}
	if c.Resources != nil {
		dup.Resources = make(map[string]any, len(c.Resources))
		for k, v := range c.Resources {
			dup.Resources[k] = v
		}
	}
	return &dup
}

// Option modifies the behaviour of a plugin manager instance.
type Option func(*Manager)

// WithLoader overrides the default binary loader implementation.
func WithLoader(loader Loader) Option {
	return func(m *Manager) {
		if loader != nil {
			m.loader = loader
		}
	}
}

// WithIsolationStrategy sets a custom isolation policy enforcement strategy.
func WithIsolationStrategy(strategy IsolationStrategy) Option {
	return func(m *Manager) {
		if strategy != nil {
			m.isolation = strategy
		}
	}
}

// WithResource registers a shared resource that will be exposed to all plugins.
func WithResource(key string, value any) Option {
	return func(m *Manager) {
		if key == "" || value == nil {
			return
		}
		if m.resources == nil {
			m.resources = make(map[string]any)
		}
		m.resources[key] = value
	}
}
