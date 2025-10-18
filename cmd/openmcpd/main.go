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
	"unicode"

	"OpenMCP-Chain/internal/agent"
	"OpenMCP-Chain/internal/api"
	"OpenMCP-Chain/internal/auth"
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
	// 监听中断信号以优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 运行主逻辑。
	if err := run(ctx); err != nil {
		logger.L().Error("openmcpd 运行失败", slog.Any("error", err))
		os.Exit(1)
	}
}

// run 包含 openmcpd 的主逻辑。
func run(ctx context.Context) error {
	// 加载配置。
	configPath := os.Getenv("OPENMCP_CONFIG")
	if configPath == "" {
		configPath = filepath.Join("configs", "openmcp.json")
	}

	// 载入配置文件。
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 初始化日志系统。
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

	// 记录启动信息。
	log := logger.L()
	log.Info("启动 openmcpd", slog.String("config", configPath))

	// 初始化大模型客户端。
	llmClient, err := createLLMClient(cfg)
	if err != nil {
		return err
	}

	// 确保数据目录存在。
	dataDir := cfg.Runtime.DataDir
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// 初始化任务存储库。
	var taskRepo mysql.TaskRepository
	switch cfg.Storage.TaskStore.Driver {
	// 默认使用内存存储库。
	case "memory", "":
		repo, err := mysql.NewMemoryTaskRepository(dataDir)
		if err != nil {
			return err
		}
		taskRepo = repo
	// 使用 MySQL 存储库。
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
	// 不支持的存储驱动。
	default:
		return mysql.ErrUnsupportedDriver
	}

	// 确保任务存储库在函数退出时关闭。
	if closer, ok := taskRepo.(interface{ Close() error }); ok {
		defer closer.Close()
	}

	// 初始化任务存储和队列。
	var taskStore task.Store
	// 选择任务存储驱动。
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

	// 选择任务队列驱动。
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

	// 初始化区块链提供者和客户端。
	chainRegistry, err := provider.NewRegistry(ctx, cfg.Web3)
	if err != nil {
		return err
	}
	defer chainRegistry.Close()

	// 获取默认的区块链客户端。
	web3Client, err := chainRegistry.DefaultClient()
	if err != nil {
		return err
	}

	// 初始化知识库提供者（如果配置了的话）。
	var knowledgeProvider knowledge.Provider
	if cfg.Knowledge.Source != "" {
		provider, err := knowledge.LoadStaticProvider(cfg.Knowledge.Source, cfg.Knowledge.MaxResults)
		if err != nil {
			return err
		}
		knowledgeProvider = provider
	}

	// 初始化认证服务。
	authService, authCleanup, err := initAuth(ctx, cfg)
	if err != nil {
		return fmt.Errorf("初始化认证失败: %w", err)
	}
	if authCleanup != nil {
		defer authCleanup()
	}
	if authService != nil && authService.Mode() != auth.ModeDisabled {
		log.Info("已启用身份认证", slog.String("mode", string(authService.Mode())))
	} else {
		log.Info("身份认证未启用")
	}

	// 初始化智能体。
	opts := []agent.Option{
		agent.WithMemoryDepth(cfg.Agent.MemoryDepth),
		agent.WithKnowledgeProvider(knowledgeProvider),
	}
	if cfg.LLM.Provider == "openai" {
		opts = append(opts, agent.WithLLMTimeout(cfg.LLM.OpenAI.Timeout()))
	}

	// 创建智能体实例。
	ag := agent.New(
		llmClient,
		web3Client,
		taskRepo,
		opts...,
	)

	// 初始化任务服务和处理器。
	taskService := task.NewService(taskStore, taskQueue, cfg.Storage.TaskStore.Retries)
	processor := task.NewProcessor(ag, taskStore, taskQueue, taskQueue,
		task.WithWorkerCount(cfg.TaskQueue.Worker),
		task.WithProcessorLogger(logger.Named("processor")),
	)

	// 启动任务处理器。
	processorCtx, processorCancel := context.WithCancel(ctx)
	defer processorCancel()

	// 启动任务处理器协程。
	go func() {
		if err := processor.Start(processorCtx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("任务处理器异常退出", slog.Any("error", err))
		}
	}()

	// 启动任务服务协程。
	metricsAddr := strings.TrimSpace(cfg.Observability.Metrics.Address)
	serveMetricsInAPIServer := cfg.Observability.Metrics.Enabled && metricsAddr == ""

	// 启动独立的指标服务（如果配置了的话）。
	if cfg.Observability.Metrics.Enabled && metricsAddr != "" {
		go func() {
			if err := metrics.StartServer(ctx, metricsAddr); err != nil && !errors.Is(err, context.Canceled) {
				logger.L().Error("指标服务异常退出", slog.Any("error", err), slog.String("address", metricsAddr))
			}
		}()
		log.Info("已开启独立指标端点", slog.String("address", metricsAddr))
	}

	// 初始化并启动 API 服务器。
	server := api.NewServer(cfg.Server.Address, taskService,
		api.WithMetrics(serveMetricsInAPIServer),
		api.WithAuthService(authService),
	)

	if cfg.Observability.Metrics.Enabled && serveMetricsInAPIServer {
		log.Info("在 API 服务上暴露 /metrics", slog.String("address", cfg.Server.Address))
	}

	if err := server.Start(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}

// createLLMClient 根据配置创建大模型客户端。
func createLLMClient(cfg *config.Config) (llm.Client, error) {
	// 根据配置选择大模型提供者。
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

// initAuth 初始化认证服务。
func initAuth(ctx context.Context, cfg *config.Config) (*auth.Service, func() error, error) {
	// 检查认证模式。
	mode := strings.ToLower(strings.TrimSpace(cfg.Auth.Mode))
	if mode == "" || mode == string(auth.ModeDisabled) {
		return nil, nil, nil
	}

	// 收集初始用户凭据。
	seeds := make([]auth.Seed, 0, len(cfg.Auth.Seeds))
	for _, seed := range cfg.Auth.Seeds {
		seeds = append(seeds, auth.Seed{
			Username:    seed.Username,
			Password:    seed.Password,
			Roles:       append([]string(nil), seed.Roles...),
			Permissions: append([]string(nil), seed.Permissions...),
			Disabled:    seed.Disabled,
		})
	}

	// 初始化认证存储。
	var (
		store   auth.Store
		cleanup func() error
	)

	// 选择认证存储驱动。
	switch strings.ToLower(strings.TrimSpace(cfg.Storage.AuthStore.Driver)) {
	case "", "memory":
		memStore, err := auth.NewMemoryStore(nil)
		if err != nil {
			return nil, nil, err
		}
		store = memStore
	case "mysql":
		dbCfg := mysql.Config{
			DSN:             cfg.Storage.AuthStore.DSN,
			MaxOpenConns:    cfg.Storage.AuthStore.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.AuthStore.MaxIdleConns,
			ConnMaxLifetime: time.Duration(cfg.Storage.AuthStore.ConnMaxLifetimeSeconds) * time.Second,
			ConnMaxIdleTime: time.Duration(cfg.Storage.AuthStore.ConnMaxIdleTimeSeconds) * time.Second,
		}
		sqlStore, err := mysql.NewSQLAuthStore(ctx, dbCfg)
		if err != nil {
			return nil, nil, err
		}
		store = sqlStore
		cleanup = sqlStore.Close
	default:
		return nil, nil, fmt.Errorf("未知的认证存储驱动: %s", cfg.Storage.AuthStore.Driver)
	}

	jwtSecret := strings.TrimSpace(cfg.Auth.JWT.Secret)
	if jwtSecret == "" && cfg.Auth.JWT.SecretEnv != "" {
		jwtSecret = strings.TrimSpace(os.Getenv(cfg.Auth.JWT.SecretEnv))
	}

	// 配置认证服务。
	authCfg := auth.Config{
		Mode: auth.Mode(mode),
		JWT: auth.JWTOptions{
			Secret:     jwtSecret,
			Issuer:     cfg.Auth.JWT.Issuer,
			Audience:   parseAudience(cfg.Auth.JWT.Audience),
			AccessTTL:  int64(cfg.Auth.JWT.AccessTokenTTLSeconds),
			RefreshTTL: int64(cfg.Auth.JWT.RefreshTokenTTLSeconds),
		},
		OAuth: auth.OAuthOptions{
			TokenURL:         cfg.Auth.OAuth.TokenURL,
			IntrospectionURL: cfg.Auth.OAuth.IntrospectionURL,
			ClientID:         cfg.Auth.OAuth.ClientID,
			ClientSecret:     cfg.Auth.OAuth.ClientSecret,
			Scopes:           append([]string(nil), cfg.Auth.OAuth.Scopes...),
			TimeoutSeconds:   cfg.Auth.OAuth.TimeoutSeconds,
			UsernameClaim:    cfg.Auth.OAuth.UsernameClaim,
		},
		Seeds: seeds,
	}

	service, err := auth.NewService(ctx, authCfg, store)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, err
	}
	return service, cleanup, nil
}

// parseAudience 解析 JWT 受众字符串为字符串切片。
func parseAudience(raw string) []string {
	// 按逗号、分号或空白字符分割字符串。
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	// 使用 FieldsFunc 进行分割。
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		if r == ',' || r == ';' {
			return true
		}
		return unicode.IsSpace(r)
	})
	// 清理并返回结果。
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
