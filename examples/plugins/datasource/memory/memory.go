package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"OpenMCP-Chain/pkg/plugin"
)

// memorySource 是一个内存数据源插件，实现了 plugin.DataSource 接口。
type memorySource struct {
	mu      sync.RWMutex
	records []map[string]any
}

// Plugin 是内存数据源插件的实例。
var Plugin plugin.Plugin = &memorySource{}

// Info 返回插件的元信息。
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

// Configure 配置内存数据源插件。
func (m *memorySource) Configure(cfg map[string]any) error {
	// 解析配置中的记录。
	raw, ok := cfg["records"]
	if !ok {
		cfg["records"] = []map[string]any{}
		return nil
	}
	// 根据类型处理不同格式的记录。
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

// Init 初始化内存数据源插件。
func (m *memorySource) Init(*plugin.ExecutionContext) error { return nil }

// Start 启动内存数据源插件，向下游发送配置的记录。
func (m *memorySource) Start(ctx *plugin.ExecutionContext) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// 获取下游数据接收函数。
	sink, ok := ctx.Resources["datasource:sink"].(func(context.Context, map[string]any) error)
	if !ok {
		return errors.New("datasource sink resource not provided")
	}
	// 逐条发送记录。
	for _, record := range m.records {
		if err := sink(ctx.C, record); err != nil {
			return err
		}
	}
	return nil
}

// Stop 停止内存数据源插件。
func (m *memorySource) Stop(*plugin.ExecutionContext) error { return nil }
