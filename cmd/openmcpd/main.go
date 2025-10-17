package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/internal/api"
	"OpenMCP-Chain/internal/config"
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
	scriptPath := pythonbridge.ResolveScriptPath(cfg.LLM.Python.WorkingDir, cfg.LLM.Python.ScriptPath)
	llmClient, err := pythonbridge.NewClient(cfg.LLM.Python.PythonExecutable, scriptPath, cfg.LLM.Python.WorkingDir)
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

	ag := agent.New(llmClient, web3Client, taskRepo)
	server := api.NewServer(cfg.Server.Address, ag)

	if err := server.Start(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
