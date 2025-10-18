package task

import (
	"context"
	"errors"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig 描述 RabbitMQ 队列的连接参数。
type RabbitMQConfig struct {
	URL        string
	Queue      string
	Prefetch   int
	Durable    bool
	AutoDelete bool
}

// RabbitMQQueue 使用 RabbitMQ 实现任务队列。
type RabbitMQQueue struct {
	conn  *amqp.Connection
	ch    *amqp.Channel
	queue string
}

// NewRabbitMQQueue 创建 RabbitMQ 队列实例。
func NewRabbitMQQueue(cfg RabbitMQConfig) (*RabbitMQQueue, error) {
	if cfg.URL == "" {
		return nil, errors.New("RabbitMQ URL 不能为空")
	}
	queue := cfg.Queue
	if queue == "" {
		queue = "openmcp.tasks"
	}
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建 RabbitMQ channel 失败: %w", err)
	}
	if cfg.Prefetch > 0 {
		if err := ch.Qos(cfg.Prefetch, 0, false); err != nil {
			ch.Close()
			conn.Close()
			return nil, fmt.Errorf("设置 RabbitMQ QOS 失败: %w", err)
		}
	}
	_, err = ch.QueueDeclare(queue, cfg.Durable, cfg.AutoDelete, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("声明 RabbitMQ 队列失败: %w", err)
	}
	return &RabbitMQQueue{conn: conn, ch: ch, queue: queue}, nil
}

// Publish 将任务投递到 RabbitMQ。
func (q *RabbitMQQueue) Publish(ctx context.Context, taskID string) error {
	if q == nil || q.ch == nil {
		return errors.New("RabbitMQ 队列未初始化")
	}
	return q.ch.PublishWithContext(ctx, "", q.queue, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(taskID),
	})
}

// Consume 使用手动确认模式消费 RabbitMQ 队列。
func (q *RabbitMQQueue) Consume(ctx context.Context, workerCount int, handler Handler) error {
	if q == nil || q.ch == nil {
		return errors.New("RabbitMQ 队列未初始化")
	}
	if workerCount <= 0 {
		workerCount = 1
	}
	msgs, err := q.ch.Consume(q.queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("订阅 RabbitMQ 队列失败: %w", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-msgs:
					if !ok {
						return
					}
					if err := handler(ctx, string(msg.Body)); err != nil {
						_ = msg.Ack(false)
						continue
					}
					_ = msg.Ack(false)
				}
			}
		}()
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

// Close 关闭 RabbitMQ 连接。
func (q *RabbitMQQueue) Close() error {
	if q == nil {
		return nil
	}
	if q.ch != nil {
		_ = q.ch.Close()
	}
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
