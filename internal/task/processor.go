package task

import (
	"context"
	"errors"
	"fmt"
	"log"

	"OpenMCP-Chain/internal/agent"
)

// Executor 定义了处理器所需的 Agent 能力。
type Executor interface {
	Execute(ctx context.Context, req agent.TaskRequest) (*agent.TaskResult, error)
}

// Processor 负责从队列消费任务并交给 Agent 执行。
type Processor struct {
	executor    Executor
	store       Store
	consumer    Consumer
	producer    Producer
	workerCount int
	logger      *log.Logger
}

// ProcessorOption 定义可选配置。
type ProcessorOption func(*Processor)

// WithProcessorLogger 指定日志输出。
func WithProcessorLogger(logger *log.Logger) ProcessorOption {
	return func(p *Processor) {
		p.logger = logger
	}
}

// WithWorkerCount 设置消费协程数量。
func WithWorkerCount(workers int) ProcessorOption {
	return func(p *Processor) {
		if workers > 0 {
			p.workerCount = workers
		}
	}
}

// NewProcessor 构造 Processor。
func NewProcessor(executor Executor, store Store, consumer Consumer, producer Producer, opts ...ProcessorOption) *Processor {
	p := &Processor{
		executor:    executor,
		store:       store,
		consumer:    consumer,
		producer:    producer,
		workerCount: 1,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	if p.workerCount <= 0 {
		p.workerCount = 1
	}
	return p
}

// Start 启动任务处理循环。
func (p *Processor) Start(ctx context.Context) error {
	if p.consumer == nil {
		return errors.New("未配置任务消费者")
	}
	return p.consumer.Consume(ctx, p.workerCount, p.handle)
}

func (p *Processor) handle(ctx context.Context, taskID string) error {
	if p.store == nil || p.executor == nil {
		return errors.New("处理器未初始化")
	}
	task, err := p.store.Claim(ctx, taskID)
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrTaskCompleted) || errors.Is(err, ErrTaskExhausted) {
			p.debugf("跳过任务 %s: %v", taskID, err)
			return nil
		}
		p.debugf("领取任务 %s 失败: %v", taskID, err)
		return err
	}

	result, execErr := p.executor.Execute(ctx, agent.TaskRequest{
		Goal:        task.Goal,
		ChainAction: task.ChainAction,
		Address:     task.Address,
	})
	if execErr != nil {
		terminal := task.Attempts >= task.MaxRetries
		if storeErr := p.store.MarkFailed(ctx, task.ID, execErr.Error(), terminal); storeErr != nil {
			p.debugf("标记任务 %s 失败状态出错: %v", task.ID, storeErr)
			return storeErr
		}
		if !terminal {
			if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
				return fmt.Errorf("任务 %s 重投失败: %v", task.ID, pubErr)
			}
		}
		return nil
	}

	// 记录成功结果。
	var record ExecutionResult
	if result != nil {
		record = ExecutionResult{
			Thought:      result.Thought,
			Reply:        result.Reply,
			ChainID:      result.ChainID,
			BlockNumber:  result.BlockNumber,
			Observations: result.Observations,
		}
	}
	if err := p.store.MarkSucceeded(ctx, task.ID, record); err != nil {
		p.debugf("标记任务 %s 成功失败: %v", task.ID, err)
		if storeErr := p.store.MarkFailed(ctx, task.ID, err.Error(), false); storeErr != nil {
			p.debugf("在写入失败状态时再次出错: %v", storeErr)
			return storeErr
		}
		if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
			return fmt.Errorf("任务 %s 在标记成功失败后重投失败: %v", task.ID, pubErr)
		}
	}
	return nil
}

func (p *Processor) debugf(format string, args ...interface{}) {
	if p.logger != nil {
		p.logger.Printf(format, args...)
	}
}
