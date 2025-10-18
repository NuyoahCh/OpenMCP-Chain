package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/pkg/logger"
)

// Service 负责任务的创建与查询。
type Service struct {
	store      Store
	producer   Producer
	maxRetries int
}

// NewService 构造任务服务。
func NewService(store Store, producer Producer, maxRetries int) *Service {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &Service{store: store, producer: producer, maxRetries: maxRetries}
}

// Submit 创建一个新的任务并推送到队列。
func (s *Service) Submit(ctx context.Context, req agent.TaskRequest) (*Task, error) {
	if strings.TrimSpace(req.Goal) == "" {
		return nil, errors.New("任务目标不能为空")
	}
	if s.store == nil || s.producer == nil {
		return nil, errors.New("任务服务未初始化")
	}

	taskID := strings.TrimSpace(req.ID)
	if taskID != "" {
		task, err := s.store.Get(ctx, taskID)
		if err == nil {
			return task, nil
		}
		if !errors.Is(err, ErrTaskNotFound) {
			return nil, err
		}
	} else {
		taskID = uuid.NewString()
	}

	task := &Task{
		ID:          taskID,
		Goal:        req.Goal,
		ChainAction: req.ChainAction,
		Address:     req.Address,
		Status:      StatusPending,
		Attempts:    0,
		MaxRetries:  s.maxRetries,
	}
	if err := s.store.Create(ctx, task); err != nil {
		if errors.Is(err, ErrTaskConflict) {
			existing, getErr := s.store.Get(ctx, taskID)
			if getErr == nil {
				return existing, nil
			}
			if !errors.Is(getErr, ErrTaskNotFound) {
				return nil, getErr
			}
		}
		return nil, err
	}
	if err := s.producer.Publish(ctx, taskID); err != nil {
		logger.L().Error("任务入队失败", slog.Any("error", err), slog.String("task_id", taskID))
		_ = s.store.MarkFailed(ctx, taskID, fmt.Sprintf("发布任务到队列失败: %v", err), true)
		return nil, err
	}
	logger.Audit().Info("任务入队成功",
		slog.String("task_id", taskID),
		slog.String("goal", task.Goal),
		slog.String("address", task.Address),
		slog.Int("max_retries", task.MaxRetries),
	)
	return task, nil
}

// Get 返回指定任务的状态。
func (s *Service) Get(ctx context.Context, id string) (*Task, error) {
	if s.store == nil {
		return nil, errors.New("任务存储未初始化")
	}
	return s.store.Get(ctx, id)
}

// List 返回最近的任务列表。
func (s *Service) List(ctx context.Context, limit int) ([]*Task, error) {
	if s.store == nil {
		return nil, errors.New("任务存储未初始化")
	}
	return s.store.List(ctx, limit)
}

// Close 释放资源。
func (s *Service) Close() error {
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return err
		}
	}
	if s.producer != nil {
		return s.producer.Close()
	}
	return nil
}

// WaitUntilCompleted 在指定超时时间内轮询任务状态。
func (s *Service) WaitUntilCompleted(ctx context.Context, id string, interval time.Duration) (*Task, error) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		task, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		if task.Status == StatusSucceeded || task.Status == StatusFailed {
			return task, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
