package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"OpenMCP-Chain/internal/llm"
)

const (
	defaultBaseURL   = "https://api.openai.com/v1"
	defaultModelName = "gpt-4o-mini"
	defaultTimeout   = 60 * time.Second
)

// Config 描述了调用 OpenAI Chat Completions API 所需的信息。
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// Client 通过 HTTP 调用 OpenAI 提供的大模型能力。
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClient 根据配置创建 OpenAI 客户端。
func NewClient(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("未提供 OpenAI API Key")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModelName
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &Client{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Generate 调用 OpenAI 生成结构化回复。
func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	payload, err := c.buildPayload(req)
	if err != nil {
		return nil, err
	}

	endpoint := c.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构建 OpenAI 请求失败: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 OpenAI 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("OpenAI 返回错误状态 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("解析 OpenAI 响应失败: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return nil, errors.New("OpenAI 响应中没有有效的 choices")
	}

	content := strings.TrimSpace(decoded.Choices[0].Message.Content)
	if content == "" {
		return nil, errors.New("OpenAI 响应内容为空")
	}

	var structured struct {
		Thought string `json:"thought"`
		Reply   string `json:"reply"`
	}
	if err := json.Unmarshal([]byte(content), &structured); err != nil {
		structured.Reply = content
		structured.Thought = ""
	}
	if strings.TrimSpace(structured.Reply) == "" {
		structured.Reply = content
	}

	return &llm.Response{
		Thought: structured.Thought,
		Reply:   structured.Reply,
	}, nil
}

func (c *Client) buildPayload(req llm.Request) ([]byte, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	messages := []message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: buildUserPrompt(req),
		},
	}

	body := map[string]any{
		"model":       c.model,
		"messages":    messages,
		"temperature": 0.2,
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化 OpenAI 请求失败: %w", err)
	}
	return encoded, nil
}

const systemPrompt = "" +
	"You are OpenMCP's reasoning engine. " +
	"Always respond with a compact JSON object: {\"thought\": string, \"reply\": string}. " +
	"Use Chinese for the reply and summarise the reasoning in \"thought\"."

func buildUserPrompt(req llm.Request) string {
	var builder strings.Builder
	builder.WriteString("## 当前任务\n")
	builder.WriteString(fmt.Sprintf("目标: %s\n", strings.TrimSpace(req.Goal)))
	if action := strings.TrimSpace(req.ChainAction); action != "" {
		builder.WriteString(fmt.Sprintf("链上操作: %s\n", action))
	}
	if address := strings.TrimSpace(req.Address); address != "" {
		builder.WriteString(fmt.Sprintf("相关地址: %s\n", address))
	}

	if len(req.History) > 0 {
		builder.WriteString("\n## 历史上下文\n")
		for idx, entry := range req.History {
			builder.WriteString(fmt.Sprintf("[%d] 目标:%s | 反馈:%s | 观察:%s\n",
				idx+1,
				strings.TrimSpace(entry.Goal),
				truncate(entry.Reply),
				truncate(entry.Observations),
			))
			if idx >= 4 {
				break
			}
		}
	}

	if len(req.Knowledge) > 0 {
		builder.WriteString("\n## 知识库\n")
		for idx, card := range req.Knowledge {
			builder.WriteString(fmt.Sprintf("[%d] %s: %s\n",
				idx+1,
				strings.TrimSpace(card.Title),
				truncate(card.Content),
			))
			if idx >= 4 {
				break
			}
		}
	}

	builder.WriteString("\n请依据上述信息给出最合理的推理 thought，以及供用户执行的 reply。")
	return builder.String()
}

func truncate(text string) string {
	text = strings.TrimSpace(text)
	if len([]rune(text)) > 80 {
		return string([]rune(text)[:80]) + "..."
	}
	return text
}
