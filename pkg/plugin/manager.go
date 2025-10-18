package plugin

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
)

// Manager keeps track of registered plugins and orchestrates their lifecycle.
type Manager struct {
	mu        sync.RWMutex
	registry  map[string]*instance
	loader    Loader
	isolation IsolationStrategy
	resources map[string]any
	defaults  IsolationPolicy
}

type instance struct {
	mu     sync.Mutex
	Plugin Plugin
	Info   Info
	State  State
	Config map[string]any
	Policy IsolationPolicy
	Source string
}

// NewManager constructs a manager using the supplied configuration and options.
func NewManager(cfg ManagerConfig, opts ...Option) (*Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	m := &Manager{
		registry:  make(map[string]*instance),
		loader:    GoPluginLoader{},
		isolation: NewIsolationStrategy(nil),
		resources: make(map[string]any),
		defaults:  cfg.Defaults,
	}
	for _, opt := range opts {
		opt(m)
	}
	m.isolation = NewIsolationStrategy(m.isolation)
	if err := m.loadConfigured(cfg); err != nil {
		return nil, err
	}
	return m, nil
}

// Register registers a plugin instance directly with the manager.
func (m *Manager) Register(id string, p Plugin, cfg map[string]any, policy IsolationPolicy) error {
	if id == "" {
		return errors.New("plugin id cannot be empty")
	}
	if p == nil {
		return errors.New("plugin implementation cannot be nil")
	}
	info := p.Info()
	if info.ID != "" && info.ID != id {
		return fmt.Errorf("plugin id mismatch: %s != %s", info.ID, id)
	}
	policy = MergePolicies(m.defaults, &policy)
	if err := EnsurePolicy(info, policy); err != nil {
		return err
	}
	if err := m.isolation.Validate(info, policy); err != nil {
		return err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	if err := p.Configure(cfg); err != nil {
		return fmt.Errorf("configure plugin %s: %w", id, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.registry[id]; exists {
		return fmt.Errorf("plugin %s already registered", id)
	}
	m.registry[id] = &instance{Plugin: p, Info: mergeInfo(info, id), State: StateRegistered, Config: cfg, Policy: policy, Source: "manual"}
	return nil
}

// Load loads a plugin implementation from disk and registers it with the manager.
func (m *Manager) Load(id string, path string, cfg map[string]any, policy IsolationPolicy) error {
	if path == "" {
		return errors.New("plugin path cannot be empty")
	}
	p, err := m.loader.Load(path)
	if err != nil {
		return fmt.Errorf("load plugin from %s: %w", path, err)
	}
	return m.Register(id, p, cfg, policy)
}

// Start initialises and starts a plugin by id.
func (m *Manager) Start(ctx context.Context, id string) error {
	inst, err := m.get(id)
	if err != nil {
		return err
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.State == StateStarted {
		return nil
	}
	execCtx := &ExecutionContext{C: ctx, Config: inst.Config, Resources: m.resources}
	if inst.State == StateRegistered {
		if err := inst.Plugin.Init(execCtx.Clone()); err != nil {
			return fmt.Errorf("initialise plugin %s: %w", id, err)
		}
		inst.State = StateInitialised
	}
	if err := m.isolation.Prepare(inst.Info); err != nil {
		return fmt.Errorf("prepare isolation for %s: %w", id, err)
	}
	if err := inst.Plugin.Start(execCtx.Clone()); err != nil {
		_ = m.isolation.Cleanup(inst.Info)
		return fmt.Errorf("start plugin %s: %w", id, err)
	}
	inst.State = StateStarted
	return nil
}

// Stop halts a plugin if it is running.
func (m *Manager) Stop(ctx context.Context, id string) error {
	inst, err := m.get(id)
	if err != nil {
		return err
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.State != StateStarted {
		return nil
	}
	execCtx := &ExecutionContext{C: ctx, Config: inst.Config, Resources: m.resources}
	if err := inst.Plugin.Stop(execCtx.Clone()); err != nil {
		return fmt.Errorf("stop plugin %s: %w", id, err)
	}
	if err := m.isolation.Cleanup(inst.Info); err != nil {
		return fmt.Errorf("cleanup isolation for %s: %w", id, err)
	}
	inst.State = StateStopped
	return nil
}

// StartAll starts all registered plugins.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.registry))
	for id := range m.registry {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if err := m.Start(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all active plugins.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.registry))
	for id := range m.registry {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if err := m.Stop(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// State returns the lifecycle state of a plugin.
func (m *Manager) State(id string) (State, error) {
	inst, err := m.get(id)
	if err != nil {
		return "", err
	}
	inst.mu.Lock()
	defer inst.mu.Unlock()
	return inst.State, nil
}

func (m *Manager) get(id string) (*instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.registry[id]
	if !ok {
		return nil, fmt.Errorf("plugin %s not registered", id)
	}
	return inst, nil
}

func (m *Manager) loadConfigured(cfg ManagerConfig) error {
	for id, pluginCfg := range cfg.Plugins {
		if !pluginCfg.Enabled {
			continue
		}
		path := pluginCfg.Path
		if !filepath.IsAbs(path) && cfg.PluginDir != "" {
			path = filepath.Join(cfg.PluginDir, path)
		}
		policy := MergePolicies(cfg.Defaults, pluginCfg.Policy)
		if err := m.Load(id, path, cloneConfig(pluginCfg.Config), policy); err != nil {
			return err
		}
	}
	return nil
}

func mergeInfo(info Info, id string) Info {
	if info.ID == "" {
		info.ID = id
	}
	return info
}

func cloneConfig(cfg map[string]any) map[string]any {
	if cfg == nil {
		return map[string]any{}
	}
	cp := make(map[string]any, len(cfg))
	for k, v := range cfg {
		cp[k] = v
	}
	return cp
}
