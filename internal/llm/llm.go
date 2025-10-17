package llm

import "context"

// Request 描述发送给大模型的任务上下文。
type Request struct {
	Goal        string
	ChainAction string
	Address     string
	History     []HistoryEntry
	Knowledge   []KnowledgeCard
}

// Response 是大模型推理得到的结构化输出。
type Response struct {
	Thought string
	Reply   string
}

// KnowledgeCard 表示提供给大模型的知识切片，帮助生成更加准确的回复。
type KnowledgeCard struct {
	Title   string
	Content string
}

// Client 定义了调用大模型的统一接口。
type Client interface {
	Generate(ctx context.Context, req Request) (*Response, error)
}

// HistoryEntry 描述了一段历史任务，用于为大模型提供上下文记忆。
type HistoryEntry struct {
	Goal         string
	ChainAction  string
	Address      string
	Reply        string
	Observations string
	CreatedAt    int64
}
