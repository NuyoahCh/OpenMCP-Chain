package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"OpenMCP-Chain/internal/llm"
	"OpenMCP-Chain/internal/storage/mysql"
	"OpenMCP-Chain/internal/web3/ethereum"
)

// TaskRequest 描述了一个简单的智能体任务。
type TaskRequest struct {
	Goal        string `json:"goal"`
	ChainAction string `json:"chain_action"`
	Address     string `json:"address"`
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
	web3Client  *ethereum.Client
	taskStorage mysql.TaskRepository
	memoryDepth int
}

// Option 定义可选的 Agent 配置。
type Option func(*Agent)

const defaultMemoryDepth = 5

// WithMemoryDepth 设置大模型调用时可参考的历史任务数量。
func WithMemoryDepth(depth int) Option {
	return func(a *Agent) {
		a.memoryDepth = depth
	}
}

// New 创建一个 Agent。
func New(llmClient llm.Client, web3Client *ethereum.Client, repo mysql.TaskRepository, opts ...Option) *Agent {
	ag := &Agent{
		llmClient:   llmClient,
		web3Client:  web3Client,
		taskStorage: repo,
		memoryDepth: defaultMemoryDepth,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(ag)
		}
	}
	if ag.memoryDepth <= 0 {
		ag.memoryDepth = defaultMemoryDepth
	}
	return ag
}

// New 创建一个 Agent。
func New(llmClient llm.Client, web3Client *ethereum.Client, repo mysql.TaskRepository) *Agent {
	return &Agent{
		llmClient:   llmClient,
		web3Client:  web3Client,
		taskStorage: repo,
	}
}

// Execute 根据任务目标调用大模型，并尝试从链上获取实时信息。
func (a *Agent) Execute(ctx context.Context, req TaskRequest) (*TaskResult, error) {
	if a.llmClient == nil {
		return nil, errors.New("未配置大模型客户端")
	}

	if req.Goal == "" {
		return nil, errors.New("任务目标不能为空")
	}

	historyEntries, historyObservation := a.loadHistory(ctx)

	llmOutput, err := a.llmClient.Generate(ctx, llm.Request{
		Goal:        req.Goal,
		ChainAction: req.ChainAction,
		Address:     req.Address,
		History:     historyEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("大模型推理失败: %w", err)
	}

	chainInfo := ethereum.ChainSnapshot{}
	observations := historyObservation
	var observations string
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

	if req.ChainAction != "" && a.web3Client != nil {
		actionResult, actionErr := a.web3Client.ExecuteAction(ctx, req.ChainAction, req.Address)
		if actionErr != nil {
			observations = appendObservation(observations, fmt.Sprintf("执行链上操作失败: %v", actionErr))
		} else {
			observations = appendObservation(observations, fmt.Sprintf("%s 返回: %s", req.ChainAction, actionResult))
		}
	}
	if strings.TrimSpace(observations) == "" {
		observations = "未执行任何链上操作"
	}

	result := &TaskResult{
		Goal:         req.Goal,
		ChainAction:  req.ChainAction,
		Address:      req.Address,
		Thought:      llmOutput.Thought,
		Reply:        llmOutput.Reply,
		ChainID:      chainInfo.ChainID,
		BlockNumber:  chainInfo.BlockNumber,
		Observations: observations,
		CreatedAt:    time.Now().Unix(),
	}

	if a.taskStorage != nil {
		if err := a.taskStorage.Save(ctx, mysql.TaskRecord{
			Goal:        req.Goal,
			ChainAction: req.ChainAction,
			Address:     req.Address,
			Thought:     result.Thought,
			Reply:       result.Reply,
			ChainID:     result.ChainID,
			BlockNumber: result.BlockNumber,
			Observes:    result.Observations,
			CreatedAt:   result.CreatedAt,
		}); err != nil {
			return nil, fmt.Errorf("保存任务记录失败: %w", err)
		}
	}

	return result, nil
}

// ListHistory 获取最近的任务执行记录。
func (a *Agent) ListHistory(ctx context.Context, limit int) ([]TaskResult, error) {
	if a.taskStorage == nil {
		return nil, errors.New("未配置任务仓库")
	}

	records, err := a.taskStorage.ListLatest(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("查询任务记录失败: %w", err)
	}

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

func (a *Agent) loadHistory(ctx context.Context) ([]llm.HistoryEntry, string) {
	if a.taskStorage == nil || a.memoryDepth <= 0 {
		return nil, ""
	}

	records, err := a.taskStorage.ListLatest(ctx, a.memoryDepth)
	if err != nil {
		return nil, appendObservation("", fmt.Sprintf("加载历史任务失败: %v", err))
	}

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
