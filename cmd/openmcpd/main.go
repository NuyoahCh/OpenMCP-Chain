package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/internal/api"
	"OpenMCP-Chain/internal/config"
	"OpenMCP-Chain/internal/knowledge"
	"OpenMCP-Chain/internal/llm"
	"OpenMCP-Chain/internal/llm/openai"
	"OpenMCP-Chain/internal/llm/pythonbridge"
	"OpenMCP-Chain/internal/storage/mysql"
	"OpenMCP-Chain/internal/web3/ethereum"
)

// main 是 OpenMCP 守护进程的入口。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("openmcpd 运行失败: %v", err)
	}
}

func run(ctx context.Context) error {
	configPath := os.Getenv("OPENMCP_CONFIG")
	if configPath == "" {
		configPath = filepath.Join("configs", "openmcp.json")
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 初始化大模型客户端。
	llmClient, err := createLLMClient(cfg)
	if err != nil {
		return err
	}

	dataDir := cfg.Runtime.DataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	var taskRepo mysql.TaskRepository
	switch cfg.Storage.TaskStore.Driver {
	case "memory", "":
		repo, err := mysql.NewMemoryTaskRepository(dataDir)
		if err != nil {
			return err
		}
		taskRepo = repo
	case "mysql":
		repo, err := mysql.NewSQLTaskRepository(cfg.Storage.TaskStore.DSN)
		if err != nil {
			return err
		}
		taskRepo = repo
	default:
		return mysql.ErrUnsupportedDriver
	}

	if closer, ok := taskRepo.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	web3Client := ethereum.NewClient(cfg.Web3.RPCURL)

	var knowledgeProvider knowledge.Provider
	if cfg.Knowledge.Source != "" {
		provider, err := knowledge.LoadStaticProvider(cfg.Knowledge.Source, cfg.Knowledge.MaxResults)
		if err != nil {
			return err
		}
		knowledgeProvider = provider
	}

	opts := []agent.Option{
		agent.WithMemoryDepth(cfg.Agent.MemoryDepth),
		agent.WithKnowledgeProvider(knowledgeProvider),
	}
	if cfg.LLM.Provider == "openai" {
		opts = append(opts, agent.WithLLMTimeout(cfg.LLM.OpenAI.Timeout()))
	}

	ag := agent.New(
		llmClient,
		web3Client,
		taskRepo,
		opts...,
	)
	server := api.NewServer(cfg.Server.Address, ag)

	if err := server.Start(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

func createLLMClient(cfg *config.Config) (llm.Client, error) {
	switch cfg.LLM.Provider {
	case "", "python_bridge":
		scriptPath := pythonbridge.ResolveScriptPath(cfg.LLM.Python.WorkingDir, cfg.LLM.Python.ScriptPath)
		return pythonbridge.NewClient(cfg.LLM.Python.PythonExecutable, scriptPath, cfg.LLM.Python.WorkingDir)
	case "openai":
		apiKey := strings.TrimSpace(cfg.LLM.OpenAI.APIKey)
		if apiKey == "" && cfg.LLM.OpenAI.APIKeyEnv != "" {
			apiKey = strings.TrimSpace(os.Getenv(cfg.LLM.OpenAI.APIKeyEnv))
		}
		if apiKey == "" {
			return nil, errors.New("OpenAI provider 需要配置 api_key 或 api_key_env")
		}
		return openai.NewClient(openai.Config{
			APIKey:  apiKey,
			BaseURL: cfg.LLM.OpenAI.BaseURL,
			Model:   cfg.LLM.OpenAI.Model,
			Timeout: cfg.LLM.OpenAI.Timeout(),
		})
	default:
		return nil, fmt.Errorf("未知的大模型 provider: %s", cfg.LLM.Provider)
	}
}
