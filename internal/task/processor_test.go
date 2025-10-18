package task

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"OpenMCP-Chain/internal/agent"
)

type fakeAgent struct {
	processed atomic.Int32
	latency   time.Duration
}

func (f *fakeAgent) Execute(ctx context.Context, req agent.TaskRequest) (*agent.TaskResult, error) {
	if f.latency > 0 {
		select {
		case <-time.After(f.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f.processed.Add(1)
	return &agent.TaskResult{Goal: req.Goal, Reply: "ok", Thought: "done"}, nil
}

func (f *fakeAgent) ListHistory(context.Context, int) ([]agent.TaskResult, error) {
	return nil, nil
}

func TestProcessorHandlesConcurrentTasks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store := NewMemoryStore()
	queue := NewMemoryQueue(1024)
	agent := &fakeAgent{latency: 10 * time.Millisecond}

	service := NewService(store, queue, 3)
	processor := NewProcessor(agent, store, queue, queue, WithWorkerCount(8))

	go func() {
		if err := processor.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("processor exited: %v", err)
		}
	}()

	total := 200
	for i := 0; i < total; i++ {
		goal := fmt.Sprintf("goal-%d", i)
		if _, err := service.Submit(ctx, agent.TaskRequest{Goal: goal}); err != nil {
			t.Fatalf("提交任务失败: %v", err)
		}
	}

	deadline := time.After(5 * time.Second)
	for {
		if int(agent.processed.Load()) >= total {
			cancel()
			break
		}
		select {
		case <-deadline:
			t.Fatalf("任务未能及时处理，已完成 %d", agent.processed.Load())
		case <-time.After(50 * time.Millisecond):
		}
	}
}
