package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/pkg/logger"
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
	logger      *slog.Logger
}

// ProcessorOption 定义可选配置。
type ProcessorOption func(*Processor)

// WithProcessorLogger 指定日志输出。
func WithProcessorLogger(logger *slog.Logger) ProcessorOption {
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
			p.logDebug("跳过任务", slog.String("task_id", taskID), slog.String("reason", err.Error()))
			return nil
		}
		logger.L().Error("领取任务失败", slog.Any("error", err), slog.String("task_id", taskID))
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
			logger.L().Error("标记任务失败状态出错", slog.Any("error", storeErr), slog.String("task_id", task.ID))
			return storeErr
		}
		logger.Audit().Warn("任务执行失败",
			slog.String("task_id", task.ID),
			slog.String("goal", task.Goal),
			slog.Bool("terminal", terminal),
			slog.String("error", execErr.Error()),
			slog.Int("attempts", task.Attempts),
			slog.Int("max_retries", task.MaxRetries),
		)
		if !terminal {
			if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
				return fmt.Errorf("任务 %s 重投失败: %v", task.ID, pubErr)
			}
			p.logDebug("任务已重新排队", slog.String("task_id", task.ID), slog.Int("attempts", task.Attempts))
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
		logger.L().Error("标记任务成功状态失败", slog.Any("error", err), slog.String("task_id", task.ID))
		if storeErr := p.store.MarkFailed(ctx, task.ID, err.Error(), false); storeErr != nil {
			logger.L().Error("回写失败状态出错", slog.Any("error", storeErr), slog.String("task_id", task.ID))
			return storeErr
		}
		if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
			return fmt.Errorf("任务 %s 在标记成功失败后重投失败: %v", task.ID, pubErr)
		}
		logger.Audit().Warn("任务标记成功失败后重试",
			slog.String("task_id", task.ID),
			slog.String("goal", task.Goal),
			slog.String("error", err.Error()),
		)
		return nil
	}
	logger.Audit().Info("任务执行成功",
		slog.String("task_id", task.ID),
		slog.String("goal", task.Goal),
		slog.String("chain_id", record.ChainID),
	)
	return nil
}

func (p *Processor) logDebug(msg string, attrs ...slog.Attr) {
	if p.logger != nil {
		p.logger.Debug(msg, attrs...)
	}
}
