package agent

import (
	"context"
	stdErrors "errors"
	"fmt"
	"strings"
	"time"

	xerrors "OpenMCP-Chain/internal/errors"
	"OpenMCP-Chain/internal/knowledge"
	"OpenMCP-Chain/internal/llm"
	"OpenMCP-Chain/internal/storage/mysql"
	"OpenMCP-Chain/internal/web3"
)

// TaskRequest 描述了一个简单的智能体任务。
type TaskRequest struct {
	ID          string         `json:"id,omitempty"`
	Goal        string         `json:"goal"`
	ChainAction string         `json:"chain_action"`
	Address     string         `json:"address"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// TaskResult 汇总大模型与链上交互得到的结果。
type TaskResult struct {
	Goal         string `json:"goal"`
	ChainAction  string `json:"chain_action"`
	Address      string `json:"address"`
	Thought      string `json:"thought"`
	Reply        string `json:"reply"`
	ChainID      string `json:"chain_id"`
	BlockNumber  string `json:"block_number"`
	Observations string `json:"observations"`
	CreatedAt    int64  `json:"created_at"`
}

// Agent 协调大模型与区块链交互，是系统的业务核心。
type Agent struct {
	llmClient   llm.Client
	web3Client  web3.Client
	taskStorage mysql.TaskRepository
	memoryDepth int
	knowledge   knowledge.Provider
	llmTimeout  time.Duration
}

// Option 定义可选的 Agent 配置。
type Option func(*Agent)

// defaultMemoryDepth 是大模型调用时可参考的历史任务数量的默认值。
const defaultMemoryDepth = 5

// WithMemoryDepth 设置大模型调用时可参考的历史任务数量。
func WithMemoryDepth(depth int) Option {
	return func(a *Agent) {
		a.memoryDepth = depth
	}
}

// WithKnowledgeProvider 配置知识库，用于在推理前补充上下文。
func WithKnowledgeProvider(provider knowledge.Provider) Option {
	return func(a *Agent) {
		a.knowledge = provider
	}
}

// WithLLMTimeout 设置调用大模型的超时时间。
func WithLLMTimeout(timeout time.Duration) Option {
	return func(a *Agent) {
		if timeout <= 0 {
			a.llmTimeout = 0
			return
		}
		a.llmTimeout = timeout
	}
}

// New 创建一个 Agent。
func New(llmClient llm.Client, web3Client web3.Client, repo mysql.TaskRepository, opts ...Option) *Agent {
	// 初始化 Agent 实例。
	ag := &Agent{
		llmClient:   llmClient,
		web3Client:  web3Client,
		taskStorage: repo,
		memoryDepth: defaultMemoryDepth,
		llmTimeout:  0,
	}
	// 应用可选配置。
	for _, opt := range opts {
		if opt != nil {
			opt(ag)
		}
	}
	// 设置默认的历史深度。
	if ag.memoryDepth <= 0 {
		ag.memoryDepth = defaultMemoryDepth
	}
	return ag
}

// Execute 根据任务目标调用大模型，并尝试从链上获取实时信息。
func (a *Agent) Execute(ctx context.Context, req TaskRequest) (*TaskResult, error) {
	// 验证必要的组件是否已配置。
	if a.llmClient == nil {
		return nil, xerrors.New(xerrors.CodeInitializationFailure, "未配置大模型客户端")
	}

	// 验证任务请求的合法性。
	if req.Goal == "" {
		return nil, xerrors.New(xerrors.CodeInvalidArgument, "任务目标不能为空")
	}

	// 加载历史记录与知识库内容。
	historyEntries, historyObservation := a.loadHistory(ctx)
	knowledgeEntries, knowledgeObservation := a.collectKnowledge(req.Goal, req.ChainAction)

	// 调用大模型生成响应。
	llmCtx := ctx
	if a.llmTimeout > 0 {
		var cancel context.CancelFunc
		llmCtx, cancel = context.WithTimeout(ctx, a.llmTimeout)
		defer cancel()
	}

	// 准备大模型请求。
	llmOutput, err := a.llmClient.Generate(llmCtx, llm.Request{
		Goal:        req.Goal,
		ChainAction: req.ChainAction,
		Address:     req.Address,
		History:     historyEntries,
		Knowledge:   knowledgeEntries,
	})

	// 处理大模型调用结果。
	if err != nil {
		if stdErrors.Is(err, context.DeadlineExceeded) {
			return nil, xerrors.Wrap(xerrors.CodeTimeout, err, "大模型推理超时")
		}
		return nil, xerrors.Wrap(xerrors.CodeExecutorFailure, err, "大模型推理失败")
	}

	// 获取链上最新信息与执行指定操作（如有）。
	chainInfo := web3.ChainSnapshot{}
	observations := appendObservation(historyObservation, knowledgeObservation)
	if a.web3Client == nil {
		observations = "未配置 Web3 客户端"
	} else {
		snapshot, err := a.web3Client.FetchChainSnapshot(ctx)
		if err != nil {
			observations = appendObservation(observations, fmt.Sprintf("获取链上信息失败: %v", err))
		} else {
			chainInfo = snapshot
		}
	}

	// 执行链上操作（如有）。
	if req.ChainAction != "" && a.web3Client != nil {
		actionResult, actionErr := a.web3Client.ExecuteAction(ctx, req.ChainAction, req.Address)
		if actionErr != nil {
			observations = appendObservation(observations, fmt.Sprintf("执行链上操作失败: %v", actionErr))
		} else {
			observations = appendObservation(observations, fmt.Sprintf("%s 返回: %s", req.ChainAction, actionResult))
		}
	}

	// 汇总结果。
	if strings.TrimSpace(observations) == "" {
		observations = "未执行任何链上操作"
	}

	// 记录任务结果。
	now := time.Now().Unix()

	// 构建任务结果。
	result := &TaskResult{
		Goal:         req.Goal,
		ChainAction:  req.ChainAction,
		Address:      req.Address,
		Thought:      llmOutput.Thought,
		Reply:        llmOutput.Reply,
		ChainID:      chainInfo.ChainID,
		BlockNumber:  chainInfo.BlockNumber,
		Observations: observations,
		CreatedAt:    now,
	}

	// 保存任务记录（如已配置存储）。
	if a.taskStorage != nil {
		record := &mysql.TaskRecord{
			Goal:        req.Goal,
			ChainAction: req.ChainAction,
			Address:     req.Address,
			Thought:     result.Thought,
			Reply:       result.Reply,
			ChainID:     result.ChainID,
			BlockNumber: result.BlockNumber,
			Observes:    result.Observations,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := a.taskStorage.Create(ctx, record); err != nil {
			return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "保存任务记录失败")
		}
	}

	return result, nil
}

// ListHistory 获取最近的任务执行记录。
func (a *Agent) ListHistory(ctx context.Context, limit int) ([]TaskResult, error) {
	if a.taskStorage == nil {
		return nil, xerrors.New(xerrors.CodeInitializationFailure, "未配置任务仓库")
	}

	// 查询最近的任务记录。
	records, err := a.taskStorage.ListLatest(ctx, limit)
	if err != nil {
		return nil, xerrors.Wrap(xerrors.CodeStorageFailure, err, "查询任务记录失败")
	}

	// 转换为 TaskResult 列表。
	results := make([]TaskResult, 0, len(records))
	for _, record := range records {
		results = append(results, TaskResult{
			Goal:         record.Goal,
			ChainAction:  record.ChainAction,
			Address:      record.Address,
			Thought:      record.Thought,
			Reply:        record.Reply,
			ChainID:      record.ChainID,
			BlockNumber:  record.BlockNumber,
			Observations: record.Observes,
			CreatedAt:    record.CreatedAt,
		})
	}
	return results, nil
}

// appendObservation 将新的观察结果追加到现有的观察字符串中。
func appendObservation(existing, next string) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return next
	}
	return existing + "\n" + next
}

// loadHistory 加载历史任务记录以供大模型参考。
func (a *Agent) loadHistory(ctx context.Context) ([]llm.HistoryEntry, string) {
	if a.taskStorage == nil || a.memoryDepth <= 0 {
		return nil, ""
	}

	// 查询最近的任务记录。
	records, err := a.taskStorage.ListLatest(ctx, a.memoryDepth)
	if err != nil {
		return nil, appendObservation("", fmt.Sprintf("加载历史任务失败: %v", err))
	}

	// 转换为 llm.HistoryEntry 列表。
	history := make([]llm.HistoryEntry, 0, len(records))
	for _, record := range records {
		history = append(history, llm.HistoryEntry{
			Goal:         record.Goal,
			ChainAction:  record.ChainAction,
			Address:      record.Address,
			Reply:        record.Reply,
			Observations: record.Observes,
			CreatedAt:    record.CreatedAt,
		})
	}
	return history, ""
}

// collectKnowledge 从知识库中检索相关内容以供大模型参考。
func (a *Agent) collectKnowledge(goal, chainAction string) ([]llm.KnowledgeCard, string) {
	if a.knowledge == nil {
		return nil, ""
	}

	// 查询知识库。
	snippets := a.knowledge.Query(goal, chainAction)
	if len(snippets) == 0 {
		return nil, ""
	}

	// 构建知识卡片列表。
	knowledgeCards := make([]llm.KnowledgeCard, 0, len(snippets))
	titles := make([]string, 0, len(snippets))

	// 转换知识片段为知识卡片。
	for _, snippet := range snippets {
		if strings.TrimSpace(snippet.Title) == "" && strings.TrimSpace(snippet.Content) == "" {
			continue
		}
		knowledgeCards = append(knowledgeCards, llm.KnowledgeCard{
			Title:   snippet.Title,
			Content: snippet.Content,
		})
		if snippet.Title != "" {
			titles = append(titles, snippet.Title)
		}
	}

	// 构建观察字符串。
	observation := ""
	if len(titles) > 0 {
		observation = fmt.Sprintf("知识库提示: %s", strings.Join(titles, "；"))
	}
	return knowledgeCards, observation
}
