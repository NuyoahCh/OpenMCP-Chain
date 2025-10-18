package task

import (
	"context"
	stdErrors "errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"OpenMCP-Chain/internal/agent"
	xerrors "OpenMCP-Chain/internal/errors"
	"OpenMCP-Chain/internal/observability/alerting"
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
		if err := processor.Start(ctx); err != nil && !stdErrors.Is(err, context.Canceled) {
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

type retryExecutor struct {
	failCount int32
	calls     atomic.Int32
}

func (r *retryExecutor) Execute(ctx context.Context, req agent.TaskRequest) (*agent.TaskResult, error) {
	if r.calls.Add(1) <= r.failCount {
		return nil, xerrors.New(xerrors.CodeExecutorFailure, "temporary failure")
	}
	return &agent.TaskResult{Goal: req.Goal, Reply: "ok"}, nil
}

type recoveryExecutor struct {
	err error
}

func (r *recoveryExecutor) Execute(context.Context, agent.TaskRequest) (*agent.TaskResult, error) {
	return nil, r.err
}

type captureRecovery struct {
	result *ExecutionResult
	err    error
}

func (c *captureRecovery) Recover(ctx context.Context, task *Task, cause error) (*ExecutionResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.result, nil
}

type captureDispatcher struct {
	mu     sync.Mutex
	events []alerting.Event
}

func (c *captureDispatcher) Notify(ctx context.Context, event alerting.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return nil
}

func (c *captureDispatcher) lastEvent() (alerting.Event, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		return alerting.Event{}, false
	}
	return c.events[len(c.events)-1], true
}

func TestProcessorRetriesOnRetryableError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := NewMemoryStore()
	queue := NewMemoryQueue(16)
	exec := &retryExecutor{failCount: 1}

	processor := NewProcessor(exec, store, queue, queue, WithWorkerCount(1))
	go func() {
		if err := processor.Start(ctx); err != nil && !stdErrors.Is(err, context.Canceled) {
			t.Errorf("processor exited: %v", err)
		}
	}()

	task := &Task{ID: "retry-1", Goal: "goal", Status: StatusPending, MaxRetries: 3}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if err := queue.Publish(ctx, task.ID); err != nil {
		t.Fatalf("publish task failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not complete")
		default:
			stored, err := store.Get(ctx, task.ID)
			if err != nil {
				continue
			}
			if stored.Status == StatusSucceeded {
				if stored.Attempts != 2 {
					t.Fatalf("expected 2 attempts, got %d", stored.Attempts)
				}
				if stored.ErrorCode != "" {
					t.Fatalf("expected empty error code, got %s", stored.ErrorCode)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestProcessorCompensatesOnPermanentFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := NewMemoryStore()
	queue := NewMemoryQueue(8)
	dispatcher := &captureDispatcher{}
	recovery := &captureRecovery{result: &ExecutionResult{Reply: "fallback", Observations: "补偿完成"}}
	exec := &recoveryExecutor{err: xerrors.New(CodeTaskProcessing, "permanent", xerrors.WithRetryable(false))}

	processor := NewProcessor(exec, store, queue, queue, WithWorkerCount(1), WithRecoveryHandler(recovery), WithAlertDispatcher(dispatcher))
	go func() {
		if err := processor.Start(ctx); err != nil && !stdErrors.Is(err, context.Canceled) {
			t.Errorf("processor exited: %v", err)
		}
	}()

	task := &Task{ID: "compensate", Goal: "goal", Status: StatusPending, MaxRetries: 1}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if err := queue.Publish(ctx, task.ID); err != nil {
		t.Fatalf("publish task failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not finish")
		default:
			stored, err := store.Get(ctx, task.ID)
			if err != nil {
				continue
			}
			if stored.Status == StatusSucceeded {
				if stored.Result == nil || stored.Result.Reply != "fallback" {
					t.Fatalf("expected fallback result, got %#v", stored.Result)
				}
				if stored.LastError != "" {
					t.Fatalf("expected cleared last error, got %s", stored.LastError)
				}
				if stored.ErrorCode != "" {
					t.Fatalf("expected cleared error code, got %s", stored.ErrorCode)
				}
				if event, ok := dispatcher.lastEvent(); !ok || event.Metadata["stage"] != "degraded" {
					t.Fatalf("expected degraded alert, got %#v", event)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestProcessorAlertsOnTerminalFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store := NewMemoryStore()
	queue := NewMemoryQueue(8)
	dispatcher := &captureDispatcher{}
	execErr := xerrors.New(CodeTaskProcessing, "fatal", xerrors.WithRetryable(false))
	exec := &recoveryExecutor{err: execErr}

	processor := NewProcessor(exec, store, queue, queue, WithWorkerCount(1), WithAlertDispatcher(dispatcher))
	go func() {
		if err := processor.Start(ctx); err != nil && !stdErrors.Is(err, context.Canceled) {
			t.Errorf("processor exited: %v", err)
		}
	}()

	task := &Task{ID: "terminal", Goal: "goal", Status: StatusPending, MaxRetries: 1}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if err := queue.Publish(ctx, task.ID); err != nil {
		t.Fatalf("publish task failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("task did not reach terminal state")
		default:
			stored, err := store.Get(ctx, task.ID)
			if err != nil {
				continue
			}
			if stored.Status == StatusFailed {
				if stored.ErrorCode != string(CodeTaskProcessing) {
					t.Fatalf("expected error code %s, got %s", CodeTaskProcessing, stored.ErrorCode)
				}
				if stored.LastError == "" {
					t.Fatalf("expected last error to be recorded")
				}
				event, ok := dispatcher.lastEvent()
				if !ok {
					t.Fatal("expected alert event")
				}
				if event.Metadata["stage"] != "terminal" {
					t.Fatalf("expected terminal stage, got %#v", event.Metadata)
				}
				if event.Code != CodeTaskProcessing {
					t.Fatalf("unexpected alert code %s", event.Code)
				}
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
