package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisQueueConfig 描述 Redis 队列的连接参数。
type RedisQueueConfig struct {
	Address   string
	Password  string
	DB        int
	Queue     string
	BlockWait time.Duration
}

// RedisQueue 使用 Redis list 实现简单的任务队列。
type RedisQueue struct {
	client *redis.Client
	queue  string
	wait   time.Duration
}

// NewRedisQueue 创建 Redis 队列实例。
func NewRedisQueue(cfg RedisQueueConfig) (*RedisQueue, error) {
	if cfg.Address == "" {
		return nil, errors.New("Redis address 不能为空")
	}
	queue := cfg.Queue
	if queue == "" {
		queue = "openmcp:tasks"
	}
	wait := cfg.BlockWait
	if wait <= 0 {
		wait = 5 * time.Second
	}
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}
	return &RedisQueue{client: client, queue: queue, wait: wait}, nil
}

// Publish 将任务投递到 Redis。
func (q *RedisQueue) Publish(ctx context.Context, taskID string) error {
	if err := q.client.LPush(ctx, q.queue, taskID).Err(); err != nil {
		return fmt.Errorf("Redis 发布任务失败: %w", err)
	}
	return nil
}

// Consume 通过 BRPOP 从 Redis 获取任务。
func (q *RedisQueue) Consume(ctx context.Context, workerCount int, handler Handler) error {
	if workerCount <= 0 {
		workerCount = 1
	}
	errCh := make(chan error, workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				default:
				}
				values, err := q.client.BRPop(ctx, q.wait, q.queue).Result()
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, redis.ErrClosed) {
						errCh <- err
						return
					}
					if err == redis.Nil {
						continue
					}
					errCh <- fmt.Errorf("Redis 取任务失败: %w", err)
					return
				}
				if len(values) != 2 {
					continue
				}
				taskID := values[1]
				if handlerErr := handler(ctx, taskID); handlerErr != nil {
					// 处理失败时重新投递任务。
					_ = q.client.RPush(ctx, q.queue, taskID).Err()
				}
			}
		}()
	}
	// 等待第一个错误或取消信号。
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// Close 关闭 Redis 连接。
func (q *RedisQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}
