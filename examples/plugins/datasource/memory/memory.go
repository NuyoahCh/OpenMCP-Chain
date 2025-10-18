package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"OpenMCP-Chain/pkg/plugin"
)

// memorySource emits a static list of records supplied via configuration.
type memorySource struct {
	mu      sync.RWMutex
	records []map[string]any
}

var Plugin plugin.Plugin = &memorySource{}

func (m *memorySource) Info() plugin.Info {
	return plugin.Info{
		ID:           "memory-datasource",
		Name:         "In-memory data source",
		Description:  "Emits configured JSON records without external dependencies.",
		Author:       "OpenMCP",
		Version:      "1.0.0",
		Category:     plugin.TypeDataSource,
		Capabilities: nil,
	}
}

func (m *memorySource) Configure(cfg map[string]any) error {
	raw, ok := cfg["records"]
	if !ok {
		cfg["records"] = []map[string]any{}
		return nil
	}
	switch value := raw.(type) {
	case []map[string]any:
		m.records = value
		return nil
	case []any:
		items := make([]map[string]any, 0, len(value))
		for _, item := range value {
			switch rec := item.(type) {
			case map[string]any:
				items = append(items, rec)
			case string:
				var parsed map[string]any
				if err := json.Unmarshal([]byte(rec), &parsed); err != nil {
					return fmt.Errorf("decode record: %w", err)
				}
				items = append(items, parsed)
			default:
				return fmt.Errorf("unsupported record type %T", item)
			}
		}
		m.records = items
		return nil
	default:
		return fmt.Errorf("records must be an array, got %T", raw)
	}
}

func (m *memorySource) Init(*plugin.ExecutionContext) error { return nil }

func (m *memorySource) Start(ctx *plugin.ExecutionContext) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sink, ok := ctx.Resources["datasource:sink"].(func(context.Context, map[string]any) error)
	if !ok {
		return errors.New("datasource sink resource not provided")
	}
	for _, record := range m.records {
		if err := sink(ctx.C, record); err != nil {
			return err
		}
	}
	return nil
}

func (m *memorySource) Stop(*plugin.ExecutionContext) error { return nil }
