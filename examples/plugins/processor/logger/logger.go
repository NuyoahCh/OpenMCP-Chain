package main

import (
	"errors"
	"fmt"

	"OpenMCP-Chain/pkg/logger"
	"OpenMCP-Chain/pkg/plugin"
)

// loggingProcessor writes incoming records to the shared application logger.
type loggingProcessor struct{}

// Plugin 是日志处理器插件的实例。
var Plugin plugin.Plugin = &loggingProcessor{}

// Info 返回插件的元信息。
func (l *loggingProcessor) Info() plugin.Info {
	return plugin.Info{
		ID:           "logging-processor",
		Name:         "Structured logging processor",
		Description:  "Logs received payloads using the host slog logger.",
		Author:       "OpenMCP",
		Version:      "1.0.0",
		Category:     plugin.TypeProcessor,
		Capabilities: []plugin.Capability{plugin.CapabilityExecution},
	}
}

// Configure 配置日志处理器插件。
func (l *loggingProcessor) Configure(map[string]any) error { return nil }

// Init 初始化日志处理器插件。
func (l *loggingProcessor) Init(*plugin.ExecutionContext) error { return nil }

// Start 启动日志处理器插件，开始记录传入的负载。
func (l *loggingProcessor) Start(ctx *plugin.ExecutionContext) error {
	// 获取传入数据的通道。
	sink, ok := ctx.Resources["processor:input"].(<-chan map[string]any)
	if !ok {
		return errors.New("processor input channel not provided")
	}
	// 启动协程处理传入的负载。
	go func() {
		for payload := range sink {
			logger.L().Info("processor received payload", "payload", payload)
		}
	}()
	return nil
}

// Stop 停止日志处理器插件。
func (l *loggingProcessor) Stop(ctx *plugin.ExecutionContext) error {
	closer, ok := ctx.Resources["processor:onStop"].(func() error)
	if ok {
		if err := closer(); err != nil {
			return fmt.Errorf("stop callback: %w", err)
		}
	}
	return nil
}
