package alerting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	xerrors "OpenMCP-Chain/internal/errors"
	"OpenMCP-Chain/pkg/logger"
)

// Channel 表示通知渠道。
type Channel string

// 支持的通知渠道
const (
	ChannelEmail    Channel = "email"
	ChannelDingTalk Channel = "dingtalk"
	ChannelSlack    Channel = "slack"
)

// Event 描述一次需要告警的事件。
type Event struct {
	Code       xerrors.Code
	Message    string
	Severity   xerrors.Severity
	Channel    Channel
	TaskID     string
	Attempts   int
	MaxRetries int
	Metadata   map[string]string
	OccurredAt time.Time
}

// Notifier 负责将事件发送到指定渠道。
type Notifier interface {
	Channel() Channel
	Notify(ctx context.Context, event Event) error
}

// Dispatcher 将事件广播给多个通知器。
type Dispatcher interface {
	Notify(ctx context.Context, event Event) error
}

// FanoutDispatcher 实现将事件投递到多个通知器的逻辑。
type FanoutDispatcher struct {
	notifiers map[Channel]Notifier
}

// NewFanout 创建一个新的 FanoutDispatcher。
func NewFanout(notifiers ...Notifier) *FanoutDispatcher {
	set := make(map[Channel]Notifier, len(notifiers))
	for _, n := range notifiers {
		if n == nil {
			continue
		}
		set[n.Channel()] = n
	}
	return &FanoutDispatcher{notifiers: set}
}

// Notify 将事件广播至所有注册渠道。
func (d *FanoutDispatcher) Notify(ctx context.Context, event Event) error {
	if d == nil {
		return nil
	}
	var errs []error
	for _, notifier := range d.notifiers {
		if err := notifier.Notify(ctx, event); err != nil {
			errs = append(errs, fmt.Errorf("channel %s: %w", notifier.Channel(), err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// EmailSender 定义发送邮件所需的能力。
type EmailSender interface {
	Send(ctx context.Context, subject, content string, to []string) error
}

// EmailNotifier 通过邮件发送告警。
type EmailNotifier struct {
	Sender        EmailSender
	To            []string
	SubjectPrefix string
}

// Channel 返回邮件渠道。
func (n *EmailNotifier) Channel() Channel { return ChannelEmail }

// Notify 发送邮件。
func (n *EmailNotifier) Notify(ctx context.Context, event Event) error {
	if n == nil || n.Sender == nil || len(n.To) == 0 {
		logger.L().Warn("EmailNotifier 未正确配置，跳过发送", slog.String("task_id", event.TaskID))
		return nil
	}
	subject := fmt.Sprintf("%s[%s] %s", n.SubjectPrefix, event.Severity, event.Code)
	content := fmt.Sprintf("告警时间: %s\n任务: %s\n重试: %d/%d\n错误码: %s\n描述: %s",
		event.OccurredAt.Format(time.RFC3339), event.TaskID, event.Attempts, event.MaxRetries, event.Code, event.Message)
	if len(event.Metadata) > 0 {
		content += "\n详情:\n"
		for k, v := range event.Metadata {
			content += fmt.Sprintf("- %s: %s\n", k, v)
		}
	}
	return n.Sender.Send(ctx, subject, content, n.To)
}

// DingTalkSender 负责向钉钉机器人发送消息。
type DingTalkSender interface {
	Send(ctx context.Context, content string) error
}

// DingTalkNotifier 通过钉钉机器人发送告警。
type DingTalkNotifier struct {
	Sender DingTalkSender
}

// Channel 返回钉钉渠道。
func (n *DingTalkNotifier) Channel() Channel { return ChannelDingTalk }

// Notify 发送钉钉消息。
func (n *DingTalkNotifier) Notify(ctx context.Context, event Event) error {
	if n == nil || n.Sender == nil {
		logger.L().Warn("DingTalkNotifier 未正确配置，跳过发送", slog.String("task_id", event.TaskID))
		return nil
	}
	payload := fmt.Sprintf("[%s] %s\n任务: %s\n重试: %d/%d\n%s",
		event.Severity, event.Code, event.TaskID, event.Attempts, event.MaxRetries, event.Message)
	return n.Sender.Send(ctx, payload)
}

// SlackSender 负责向 Slack 渠道发送消息。
type SlackSender interface {
	Send(ctx context.Context, channel, content string) error
}

// SlackNotifier 通过 Slack 发送告警。
type SlackNotifier struct {
	Sender    SlackSender
	ChannelID string
}

// Channel 返回 Slack 渠道。
func (n *SlackNotifier) Channel() Channel { return ChannelSlack }

// Notify 发送 Slack 消息。
func (n *SlackNotifier) Notify(ctx context.Context, event Event) error {
	if n == nil || n.Sender == nil || n.ChannelID == "" {
		logger.L().Warn("SlackNotifier 未正确配置，跳过发送", slog.String("task_id", event.TaskID))
		return nil
	}
	content := fmt.Sprintf("*[%s]* %s - %s (重试 %d/%d)", event.Severity, event.Code, event.Message, event.Attempts, event.MaxRetries)
	return n.Sender.Send(ctx, n.ChannelID, content)
}
