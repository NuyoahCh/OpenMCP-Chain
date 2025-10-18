package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"OpenMCP-Chain/internal/llm"
)

type stubLLM struct {
	resp *llm.Response
	err  error
	wait time.Duration
}

func (s *stubLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	if s.wait > 0 {
		select {
		case <-time.After(s.wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestAgentExecuteSuccess(t *testing.T) {
	llmClient := &stubLLM{resp: &llm.Response{Thought: "分析", Reply: "结果"}}
	ag := New(llmClient, nil, nil)

	result, err := ag.Execute(context.Background(), TaskRequest{Goal: "测试"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Reply != "结果" || result.Thought != "分析" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAgentExecuteTimeout(t *testing.T) {
	llmClient := &stubLLM{wait: 50 * time.Millisecond}
	ag := New(llmClient, nil, nil, WithLLMTimeout(10*time.Millisecond))

	_, err := ag.Execute(context.Background(), TaskRequest{Goal: "测试"})
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}
