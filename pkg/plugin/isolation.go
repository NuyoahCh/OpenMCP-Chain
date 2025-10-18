package plugin

import (
	"errors"
	"fmt"
	"slices"
)

// IsolationStrategy enforces security restrictions for plugins at runtime.
type IsolationStrategy interface {
	Validate(info Info, policy IsolationPolicy) error
	Prepare(info Info) error
	Cleanup(info Info) error
}

// NoopIsolationStrategy performs only capability validation.
type NoopIsolationStrategy struct{}

// Validate ensures the plugin requested capabilities are allowed.
func (NoopIsolationStrategy) Validate(info Info, policy IsolationPolicy) error {
	allowed := map[Capability]struct{}{}
	for _, cap := range policy.AllowedCapabilities {
		allowed[cap] = struct{}{}
	}
	for _, cap := range policy.DeniedCapabilities {
		if slices.Contains(info.Capabilities, cap) {
			return fmt.Errorf("capability %s is explicitly denied", cap)
		}
	}
	if len(allowed) == 0 {
		return nil
	}
	for _, cap := range info.Capabilities {
		if _, ok := allowed[cap]; !ok {
			return fmt.Errorf("capability %s not permitted", cap)
		}
	}
	return nil
}

// Prepare implements IsolationStrategy.
func (NoopIsolationStrategy) Prepare(Info) error { return nil }

// Cleanup implements IsolationStrategy.
func (NoopIsolationStrategy) Cleanup(Info) error { return nil }

// NewIsolationStrategy returns a default isolation strategy if none is supplied.
func NewIsolationStrategy(strategy IsolationStrategy) IsolationStrategy {
	if strategy == nil {
		return NoopIsolationStrategy{}
	}
	return strategy
}

// MergePolicies combines the default and plugin specific isolation policies.
func MergePolicies(defaults IsolationPolicy, plugin *IsolationPolicy) IsolationPolicy {
	if plugin == nil {
		return defaults
	}
	merged := plugin.Merge(defaults)
	if len(merged.AllowedCapabilities) == 0 && len(merged.DeniedCapabilities) == 0 {
		return defaults
	}
	return merged
}

// EnsurePolicy returns an error when the isolation policy is empty and the plugin requests capabilities.
func EnsurePolicy(info Info, policy IsolationPolicy) error {
	if len(info.Capabilities) == 0 {
		return nil
	}
	if len(policy.AllowedCapabilities) == 0 && len(policy.DeniedCapabilities) == 0 {
		return errors.New("plugins declaring capabilities require an isolation policy")
	}
	return nil
}
