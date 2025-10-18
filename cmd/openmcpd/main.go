package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/internal/api"
	"OpenMCP-Chain/internal/config"
	"OpenMCP-Chain/internal/knowledge"
	"OpenMCP-Chain/internal/llm"
	"OpenMCP-Chain/internal/llm/openai"
	"OpenMCP-Chain/internal/llm/pythonbridge"
	"OpenMCP-Chain/internal/observability/metrics"
	"OpenMCP-Chain/internal/storage/mysql"
	"OpenMCP-Chain/internal/task"
	"OpenMCP-Chain/internal/web3/provider"
	"OpenMCP-Chain/pkg/logger"
)

// main 是 OpenMCP 守护进程的入口。
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		logger.L().Error("openmcpd 运行失败", slog.Any("error", err))
		os.Exit(1)
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

	if err := logger.Init(logger.Config{
		Level:       cfg.Observability.Logging.Level,
		Format:      cfg.Observability.Logging.Format,
		OutputPaths: append([]string(nil), cfg.Observability.Logging.Outputs...),
		Audit: logger.AuditConfig{
			Enabled:    cfg.Observability.Audit.Enabled,
			Path:       cfg.Observability.Audit.File,
			MaxSizeMB:  cfg.Observability.Audit.MaxSizeMB,
			MaxBackups: cfg.Observability.Audit.MaxBackups,
			MaxAgeDays: cfg.Observability.Audit.MaxAgeDays,
		},
	}); err != nil {
		return fmt.Errorf("初始化日志失败: %w", err)
	}
	defer logger.Sync()

	log := logger.L()
	log.Info("启动 openmcpd", slog.String("config", configPath))

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
		repo, err := mysql.NewSQLTaskRepository(ctx, mysql.Config{
			DSN:             cfg.Storage.TaskStore.DSN,
			MaxOpenConns:    cfg.Storage.TaskStore.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.TaskStore.MaxIdleConns,
			ConnMaxLifetime: time.Duration(cfg.Storage.TaskStore.ConnMaxLifetimeSeconds) * time.Second,
			ConnMaxIdleTime: time.Duration(cfg.Storage.TaskStore.ConnMaxIdleTimeSeconds) * time.Second,
		})
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

	var taskStore task.Store
	switch cfg.Storage.TaskStore.Driver {
	case "memory", "":
		taskStore = task.NewMemoryStore()
	case "mysql":
		store, err := task.NewMySQLStore(cfg.Storage.TaskStore.DSN)
		if err != nil {
			return err
		}
		taskStore = store
	default:
		return mysql.ErrUnsupportedDriver
	}
	defer func() {
		if taskStore != nil {
			_ = taskStore.Close()
		}
	}()

	var taskQueue task.Queue
	switch cfg.TaskQueue.Driver {
	case "", "memory":
		taskQueue = task.NewMemoryQueue(1024)
	case "redis":
		queue, err := task.NewRedisQueue(task.RedisQueueConfig{
			Address:   cfg.TaskQueue.Redis.Address,
			Password:  cfg.TaskQueue.Redis.Password,
			DB:        cfg.TaskQueue.Redis.DB,
			Queue:     cfg.TaskQueue.Redis.Queue,
			BlockWait: time.Duration(cfg.TaskQueue.Redis.BlockWait) * time.Second,
		})
		if err != nil {
			return err
		}
		taskQueue = queue
	case "rabbitmq":
		queue, err := task.NewRabbitMQQueue(task.RabbitMQConfig{
			URL:        cfg.TaskQueue.RabbitMQ.URL,
			Queue:      cfg.TaskQueue.RabbitMQ.Queue,
			Prefetch:   cfg.TaskQueue.RabbitMQ.Prefetch,
			Durable:    cfg.TaskQueue.RabbitMQ.Durable,
			AutoDelete: cfg.TaskQueue.RabbitMQ.AutoDelete,
		})
		if err != nil {
			return err
		}
		taskQueue = queue
	default:
		return fmt.Errorf("未知的队列驱动: %s", cfg.TaskQueue.Driver)
	}
	defer func() {
		if taskQueue != nil {
			if err := taskQueue.Close(); err != nil {
				log.Warn("关闭任务队列失败", slog.Any("error", err))
			}
		}
	}()

	chainRegistry, err := provider.NewRegistry(ctx, cfg.Web3)
	if err != nil {
		return err
	}
	defer chainRegistry.Close()

	web3Client, err := chainRegistry.DefaultClient()
	if err != nil {
		return err
	}

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

	taskService := task.NewService(taskStore, taskQueue, cfg.Storage.TaskStore.Retries)
	processor := task.NewProcessor(ag, taskStore, taskQueue, taskQueue,
		task.WithWorkerCount(cfg.TaskQueue.Worker),
		task.WithProcessorLogger(logger.Named("processor")),
	)

	processorCtx, processorCancel := context.WithCancel(ctx)
	defer processorCancel()

	go func() {
		if err := processor.Start(processorCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("任务处理器异常退出", slog.Any("error", err))
		}
	}()

	metricsAddr := strings.TrimSpace(cfg.Observability.Metrics.Address)
	serveMetricsInAPIServer := cfg.Observability.Metrics.Enabled && metricsAddr == ""

	if cfg.Observability.Metrics.Enabled && metricsAddr != "" {
		go func() {
			if err := metrics.StartServer(ctx, metricsAddr); err != nil && !errors.Is(err, context.Canceled) {
				logger.L().Error("指标服务异常退出", slog.Any("error", err), slog.String("address", metricsAddr))
			}
		}()
		log.Info("已开启独立指标端点", slog.String("address", metricsAddr))
	}

	server := api.NewServer(cfg.Server.Address, taskService,
		api.WithMetrics(serveMetricsInAPIServer),
	)

	if cfg.Observability.Metrics.Enabled && serveMetricsInAPIServer {
		log.Info("在 API 服务上暴露 /metrics", slog.String("address", cfg.Server.Address))
	}

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
