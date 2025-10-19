package task

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"time"

	"OpenMCP-Chain/internal/agent"
	xerrors "OpenMCP-Chain/internal/errors"
	"OpenMCP-Chain/internal/observability/alerting"
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
	recovery    RecoveryHandler
	alerter     alerting.Dispatcher
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

// WithRecoveryHandler 配置失败补偿策略。
func WithRecoveryHandler(handler RecoveryHandler) ProcessorOption {
	return func(p *Processor) {
		p.recovery = handler
	}
}

// WithAlertDispatcher 配置告警派发器。
func WithAlertDispatcher(dispatcher alerting.Dispatcher) ProcessorOption {
	return func(p *Processor) {
		p.alerter = dispatcher
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
		return xerrors.New(xerrors.CodeInitializationFailure, "未配置任务消费者")
	}
	return p.consumer.Consume(ctx, p.workerCount, p.handle)
}

func (p *Processor) handle(ctx context.Context, taskID string) error {
	if p.store == nil || p.executor == nil {
		return xerrors.New(xerrors.CodeInitializationFailure, "处理器未初始化")
	}
	task, err := p.store.Claim(ctx, taskID)
	if err != nil {
		if stdErrors.Is(err, ErrTaskNotFound) || stdErrors.Is(err, ErrTaskCompleted) || stdErrors.Is(err, ErrTaskExhausted) {
			p.logDebug("跳过任务", slog.String("task_id", taskID), slog.String("reason", err.Error()))
			return nil
		}
		logger.L().Error("领取任务失败", slog.Any("error", err), slog.String("task_id", taskID))
		p.emitAlert(ctx, &Task{ID: taskID}, CodeTaskProcessing, err, "claim")
		return err
	}

	result, execErr := p.executor.Execute(ctx, agent.TaskRequest{
		Goal:        task.Goal,
		ChainAction: task.ChainAction,
		Address:     task.Address,
		Metadata:    cloneMetadata(task.Metadata),
	})
	if execErr != nil {
		return p.handleExecutionFailure(ctx, task, execErr)
	}

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
		if storeErr := p.store.MarkFailed(ctx, task.ID, CodeTaskProcessing, err.Error(), false); storeErr != nil {
			logger.L().Error("回写失败状态出错", slog.Any("error", storeErr), slog.String("task_id", task.ID))
			return storeErr
		}
		if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
			return xerrors.Wrap(CodeTaskPublish, pubErr, fmt.Sprintf("任务 %s 在标记成功失败后重投失败", task.ID))
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

func (p *Processor) handleExecutionFailure(ctx context.Context, task *Task, execErr error) error {
	code := xerrors.CodeOf(execErr)
	if code == xerrors.CodeUnknown {
		code = CodeTaskProcessing
	}
	retryable := xerrors.RetryableError(execErr)
	terminal := task.Attempts >= task.MaxRetries || !retryable

	if !retryable && p.recovery != nil {
		if fallback, recErr := p.recovery.Recover(ctx, task, execErr); recErr != nil {
			wrapped := xerrors.Wrap(CodeTaskCompensate, recErr, "任务补偿失败")
			logger.L().Error("执行补偿逻辑失败",
				slog.Any("error", wrapped),
				slog.String("task_id", task.ID))
			p.emitAlert(ctx, task, CodeTaskCompensate, wrapped, "compensate")
		} else if fallback != nil {
			if fallback.Observations == "" {
				fallback.Observations = fmt.Sprintf("降级处理: %v", execErr)
			}
			if err := p.store.MarkSucceeded(ctx, task.ID, *fallback); err != nil {
				logger.L().Error("记录降级结果失败", slog.Any("error", err), slog.String("task_id", task.ID))
				if storeErr := p.store.MarkFailed(ctx, task.ID, code, err.Error(), false); storeErr != nil {
					logger.L().Error("降级失败后的回写失败状态出错", slog.Any("error", storeErr), slog.String("task_id", task.ID))
					return storeErr
				}
				if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
					return xerrors.Wrap(CodeTaskPublish, pubErr, fmt.Sprintf("任务 %s 在降级失败后重投失败", task.ID))
				}
				return nil
			}
			logger.Audit().Warn("任务降级完成",
				slog.String("task_id", task.ID),
				slog.String("goal", task.Goal),
				slog.String("observations", fallback.Observations),
			)
			p.emitAlert(ctx, task, code, execErr, "degraded")
			return nil
		}
	}

	if storeErr := p.store.MarkFailed(ctx, task.ID, code, execErr.Error(), terminal); storeErr != nil {
		logger.L().Error("标记任务失败状态出错", slog.Any("error", storeErr), slog.String("task_id", task.ID))
		return storeErr
	}
	logger.Audit().Warn("任务执行失败",
		slog.String("task_id", task.ID),
		slog.String("goal", task.Goal),
		slog.Bool("terminal", terminal),
		slog.String("error", execErr.Error()),
		slog.String("error_code", string(code)),
		slog.Int("attempts", task.Attempts),
		slog.Int("max_retries", task.MaxRetries),
	)

	stage := "retry"
	if terminal {
		stage = "terminal"
	} else if !retryable {
		stage = "non_retryable"
	}
	p.emitAlert(ctx, task, code, execErr, stage)

	if retryable && !terminal {
		if pubErr := p.producer.Publish(ctx, task.ID); pubErr != nil {
			return xerrors.Wrap(CodeTaskPublish, pubErr, fmt.Sprintf("任务 %s 重投失败", task.ID))
		}
		p.logDebug("任务已重新排队", slog.String("task_id", task.ID), slog.Int("attempts", task.Attempts))
	}
	return nil
}

func (p *Processor) logDebug(msg string, attrs ...slog.Attr) {
	if p.logger != nil {
		// 将slog.Attr转换为[]any
		args := make([]any, len(attrs))
		for i, attr := range attrs {
			args[i] = attr
		}
		p.logger.Debug(msg, args...)
	}
}

func (p *Processor) emitAlert(ctx context.Context, task *Task, code xerrors.Code, cause error, stage string) {
	if p == nil || p.alerter == nil || task == nil {
		return
	}
	attrs := xerrors.AttributesOf(code)
	message := attrs.Message
	if cause != nil {
		message = cause.Error()
	}
	metadata := map[string]string{
		"stage": stage,
	}
	if cause != nil {
		metadata["cause"] = cause.Error()
	}
	event := alerting.Event{
		Code:       code,
		Message:    message,
		Severity:   attrs.Severity,
		TaskID:     task.ID,
		Attempts:   task.Attempts,
		MaxRetries: task.MaxRetries,
		Metadata:   metadata,
		OccurredAt: time.Now(),
	}
	if err := p.alerter.Notify(ctx, event); err != nil {
		logger.L().Error("告警通知失败",
			slog.Any("error", err),
			slog.String("task_id", task.ID),
			slog.String("stage", stage),
		)
	}
}
