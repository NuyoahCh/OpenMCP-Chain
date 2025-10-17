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
	// 监听系统中断信号以优雅关闭服务。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("openmcpd 运行失败: %v", err)
	}
}

// run 执行守护进程的主要逻辑。
func run(ctx context.Context) error {
	// 加载配置文件。
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

	// 准备数据存储目录。
	dataDir := cfg.Runtime.DataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// 初始化任务存储库。
	var taskRepo mysql.TaskRepository
	switch cfg.Storage.TaskStore.Driver {
	case "memory", "":
		repo, err := mysql.NewMemoryTaskRepository(dataDir)
		if err != nil {
			return err
		}
		taskRepo = repo
	default:
		return mysql.ErrUnsupportedDriver
	}

	// 初始化 Web3 客户端。
	web3Client := ethereum.NewClient(cfg.Web3.RPCURL)

	// 创建智能体与 API 服务器。
	ag := agent.New(llmClient, web3Client, taskRepo)
	server := api.NewServer(cfg.Server.Address, ag)

	// 启动 API 服务器。
	if err := server.Start(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
