package main

import (
	"errors"
	"fmt"

	"OpenMCP-Chain/pkg/logger"
	"OpenMCP-Chain/pkg/plugin"
)

// loggingProcessor writes incoming records to the shared application logger.
type loggingProcessor struct{}

var Plugin plugin.Plugin = &loggingProcessor{}

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

func (l *loggingProcessor) Configure(map[string]any) error { return nil }

func (l *loggingProcessor) Init(*plugin.ExecutionContext) error { return nil }

func (l *loggingProcessor) Start(ctx *plugin.ExecutionContext) error {
	sink, ok := ctx.Resources["processor:input"].(<-chan map[string]any)
	if !ok {
		return errors.New("processor input channel not provided")
	}
	go func() {
		for payload := range sink {
			logger.L().Info("processor received payload", "payload", payload)
		}
	}()
	return nil
}

func (l *loggingProcessor) Stop(ctx *plugin.ExecutionContext) error {
	closer, ok := ctx.Resources["processor:onStop"].(func() error)
	if ok {
		if err := closer(); err != nil {
			return fmt.Errorf("stop callback: %w", err)
		}
	}
	return nil
}
