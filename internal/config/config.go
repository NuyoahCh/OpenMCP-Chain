package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config 描述了 OpenMCP 在启动阶段需要加载的核心配置。
type Config struct {
	Server        ServerConfig        `json:"server"`
	Storage       StorageConfig       `json:"storage"`
	LLM           LLMConfig           `json:"llm"`
	Web3          Web3Config          `json:"web3"`
	Agent         AgentConfig         `json:"agent"`
	Runtime       RuntimeConfig       `json:"runtime"`
	Knowledge     KnowledgeConfig     `json:"knowledge"`
	TaskQueue     TaskQueueConfig     `json:"task_queue"`
	Observability ObservabilityConfig `json:"observability"`
	Auth          AuthConfig          `json:"auth"`
}

// ServerConfig 控制 API 服务的监听地址等参数。
type ServerConfig struct {
	Address string `json:"address"`
}

// StorageConfig 统一描述 MySQL、Redis 等后端的连接信息。
type StorageConfig struct {
	TaskStore TaskStoreConfig `json:"task_store"`
	AuthStore AuthStoreConfig `json:"auth_store"`
}

// TaskStoreConfig 目前提供内存实现，后续可以切换到真正的 MySQL。
type TaskStoreConfig struct {
	Driver                 string `json:"driver"`
	DSN                    string `json:"dsn"`
	Retries                int    `json:"retries"`
	MaxOpenConns           int    `json:"max_open_conns"`
	MaxIdleConns           int    `json:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `json:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int    `json:"conn_max_idle_time_seconds"`
}

// AuthStoreConfig 描述用户、角色与权限存储的参数。
type AuthStoreConfig struct {
	Driver                 string `json:"driver"`
	DSN                    string `json:"dsn"`
	MaxOpenConns           int    `json:"max_open_conns"`
	MaxIdleConns           int    `json:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `json:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int    `json:"conn_max_idle_time_seconds"`
}

// TaskQueueConfig 描述异步任务队列。
type TaskQueueConfig struct {
	Driver   string            `json:"driver"`
	Worker   int               `json:"worker"`
	Redis    RedisQueueConfig  `json:"redis"`
	RabbitMQ RabbitQueueConfig `json:"rabbitmq"`
}

// RedisQueueConfig 对应 Redis 队列。
type RedisQueueConfig struct {
	Address   string `json:"address"`
	Password  string `json:"password"`
	DB        int    `json:"db"`
	Queue     string `json:"queue"`
	BlockWait int    `json:"block_wait_seconds"`
}

// RabbitQueueConfig 描述 RabbitMQ 参数。
type RabbitQueueConfig struct {
	URL        string `json:"url"`
	Queue      string `json:"queue"`
	Prefetch   int    `json:"prefetch"`
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
}

// ObservabilityConfig groups logging, metrics and audit settings.
type ObservabilityConfig struct {
	Logging LoggingConfig `json:"logging"`
	Metrics MetricsConfig `json:"metrics"`
	Audit   AuditConfig   `json:"audit"`
}

// AuthConfig 控制身份认证和授权的工作模式。
type AuthConfig struct {
	Mode  string      `json:"mode"`
	JWT   JWTConfig   `json:"jwt"`
	OAuth OAuthConfig `json:"oauth"`
	Seeds []UserSeed  `json:"seeds"`
}

// JWTConfig 描述本地 JWT 签发与校验所需的参数。
type JWTConfig struct {
	Secret                 string `json:"secret"`
	SecretEnv              string `json:"secret_env"`
	Issuer                 string `json:"issuer"`
	Audience               string `json:"audience"`
	AccessTokenTTLSeconds  int    `json:"access_token_ttl_seconds"`
	RefreshTokenTTLSeconds int    `json:"refresh_token_ttl_seconds"`
}

// OAuthConfig 描述与外部 OAuth2/OIDC 服务的集成方式。
type OAuthConfig struct {
	TokenURL         string   `json:"token_url"`
	IntrospectionURL string   `json:"introspection_url"`
	ClientID         string   `json:"client_id"`
	ClientSecret     string   `json:"client_secret"`
	Scopes           []string `json:"scopes"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
	UsernameClaim    string   `json:"username_claim"`
}

// UserSeed 用于在非生产环境快速初始化内存/数据库的默认账号。
type UserSeed struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	Disabled    bool     `json:"disabled"`
}

// LoggingConfig controls structured logging behaviour.
type LoggingConfig struct {
	Level   string   `json:"level"`
	Format  string   `json:"format"`
	Outputs []string `json:"outputs"`
}

// MetricsConfig enables Prometheus-compatible metrics export.
type MetricsConfig struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address"`
}

// AuditConfig defines audit log rotation settings.
type AuditConfig struct {
	Enabled    bool   `json:"enabled"`
	File       string `json:"file"`
	MaxSizeMB  int    `json:"max_size_mb"`
	MaxBackups int    `json:"max_backups"`
	MaxAgeDays int    `json:"max_age_days"`
}

// LLMConfig 用于配置大模型推理的调用方式。
type LLMConfig struct {
	Provider string             `json:"provider"`
	Python   PythonBridgeConfig `json:"python_bridge"`
	OpenAI   OpenAIConfig       `json:"openai"`
}

// PythonBridgeConfig 描述通过 Python 脚本完成推理时所需的信息。
type PythonBridgeConfig struct {
	Enabled          bool   `json:"enabled"`
	PythonExecutable string `json:"python_executable"`
	ScriptPath       string `json:"script_path"`
	WorkingDir       string `json:"working_dir"`
}

// OpenAIConfig 描述访问 OpenAI 兼容 API 所需的配置。
type OpenAIConfig struct {
	Enabled        bool   `json:"enabled"`
	APIKey         string `json:"api_key"`
	APIKeyEnv      string `json:"api_key_env"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// Timeout 返回配置的超时时间，默认 60 秒。
func (c OpenAIConfig) Timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 60 * time.Second
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// Web3Config 包含访问区块链节点所需的 RPC 地址。
type Web3Config struct {
	RPCURL       string `json:"rpc_url"`
	ChainConfig  string `json:"chain_config"`
	DefaultChain string `json:"default_chain"`
}

// RuntimeConfig 用于放置运行时的通用参数。
type RuntimeConfig struct {
	DataDir string `json:"data_dir"`
}

// AgentConfig 控制智能体的工作方式。
type AgentConfig struct {
	MemoryDepth int `json:"memory_depth"`
}

// KnowledgeConfig 描述知识库的加载方式。
type KnowledgeConfig struct {
	Source     string `json:"source"`
	MaxResults int    `json:"max_results"`
}

// Load 负责解析指定路径的 JSON 配置文件。
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("配置文件路径为空")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开配置文件失败: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	cfg.applyDefaults(filepath.Dir(path))

	return &cfg, nil
}

// applyDefaults 在用户未填写部分字段时设置合理的默认值。
func (c *Config) applyDefaults(baseDir string) {
	if c.Server.Address == "" {
		c.Server.Address = ":8080"
	}

	if c.Storage.TaskStore.Driver == "" {
		c.Storage.TaskStore.Driver = "memory"
	}
	if c.Storage.TaskStore.Retries <= 0 {
		c.Storage.TaskStore.Retries = 3
	}
	if c.Storage.TaskStore.MaxOpenConns <= 0 {
		c.Storage.TaskStore.MaxOpenConns = 20
	}
	if c.Storage.TaskStore.MaxIdleConns <= 0 {
		c.Storage.TaskStore.MaxIdleConns = 10
	}
	if c.Storage.TaskStore.ConnMaxLifetimeSeconds <= 0 {
		c.Storage.TaskStore.ConnMaxLifetimeSeconds = 1800
	}
	if c.Storage.TaskStore.ConnMaxIdleTimeSeconds < 0 {
		c.Storage.TaskStore.ConnMaxIdleTimeSeconds = 0
	}

	if c.Storage.AuthStore.Driver == "" {
		c.Storage.AuthStore.Driver = c.Storage.TaskStore.Driver
	}
	if c.Storage.AuthStore.MaxOpenConns <= 0 {
		c.Storage.AuthStore.MaxOpenConns = 10
	}
	if c.Storage.AuthStore.MaxIdleConns <= 0 {
		c.Storage.AuthStore.MaxIdleConns = 5
	}
	if c.Storage.AuthStore.ConnMaxLifetimeSeconds <= 0 {
		c.Storage.AuthStore.ConnMaxLifetimeSeconds = 1800
	}
	if c.Storage.AuthStore.ConnMaxIdleTimeSeconds < 0 {
		c.Storage.AuthStore.ConnMaxIdleTimeSeconds = 0
	}

	if c.LLM.Provider == "" {
		c.LLM.Provider = "python_bridge"
	}

	if c.LLM.Python.PythonExecutable == "" {
		c.LLM.Python.PythonExecutable = "python3"
	}

	if c.LLM.Python.WorkingDir == "" {
		c.LLM.Python.WorkingDir = baseDir
	} else if !filepath.IsAbs(c.LLM.Python.WorkingDir) {
		c.LLM.Python.WorkingDir = filepath.Join(baseDir, c.LLM.Python.WorkingDir)
	}

	if c.LLM.OpenAI.BaseURL == "" {
		c.LLM.OpenAI.BaseURL = "https://api.openai.com/v1"
	}
	if c.LLM.OpenAI.Model == "" {
		c.LLM.OpenAI.Model = "gpt-4o-mini"
	}
	if c.LLM.OpenAI.TimeoutSeconds <= 0 {
		c.LLM.OpenAI.TimeoutSeconds = 60
	}

	if c.Runtime.DataDir == "" {
		c.Runtime.DataDir = filepath.Join(baseDir, "data")
	} else if !filepath.IsAbs(c.Runtime.DataDir) {
		c.Runtime.DataDir = filepath.Join(baseDir, c.Runtime.DataDir)
	}

	if len(c.Observability.Logging.Outputs) == 0 {
		c.Observability.Logging.Outputs = []string{"stdout"}
	}
	if c.Observability.Logging.Level == "" {
		c.Observability.Logging.Level = "info"
	}
	if c.Observability.Logging.Format == "" {
		c.Observability.Logging.Format = "json"
	}

	if !c.Observability.Audit.Enabled {
		c.Observability.Audit.Enabled = true
	}
	if c.Observability.Audit.File == "" {
		c.Observability.Audit.File = filepath.Join(c.Runtime.DataDir, "audit.log")
	} else if !filepath.IsAbs(c.Observability.Audit.File) {
		c.Observability.Audit.File = filepath.Join(baseDir, c.Observability.Audit.File)
	}
	if c.Observability.Audit.MaxSizeMB <= 0 {
		c.Observability.Audit.MaxSizeMB = 100
	}
	if c.Observability.Audit.MaxBackups <= 0 {
		c.Observability.Audit.MaxBackups = 7
	}
	if c.Observability.Audit.MaxAgeDays <= 0 {
		c.Observability.Audit.MaxAgeDays = 30
	}

	if !c.Observability.Metrics.Enabled {
		c.Observability.Metrics.Enabled = true
	}

	if c.TaskQueue.Driver == "" {
		c.TaskQueue.Driver = "memory"
	}
	if c.TaskQueue.Worker <= 0 {
		c.TaskQueue.Worker = 1
	}
	if c.TaskQueue.Redis.BlockWait <= 0 {
		c.TaskQueue.Redis.BlockWait = 5
	}
	if c.TaskQueue.Redis.Queue == "" {
		c.TaskQueue.Redis.Queue = "openmcp:tasks"
	}
	if c.TaskQueue.RabbitMQ.Queue == "" {
		c.TaskQueue.RabbitMQ.Queue = "openmcp.tasks"
	}
	if c.TaskQueue.RabbitMQ.Prefetch <= 0 {
		c.TaskQueue.RabbitMQ.Prefetch = c.TaskQueue.Worker
	}

	if c.Agent.MemoryDepth <= 0 {
		c.Agent.MemoryDepth = 5
	}

	if c.Knowledge.MaxResults <= 0 {
		c.Knowledge.MaxResults = 3
	}
	if c.Knowledge.Source != "" && !filepath.IsAbs(c.Knowledge.Source) {
		c.Knowledge.Source = filepath.Join(baseDir, c.Knowledge.Source)
	}

	if c.Web3.ChainConfig != "" && !filepath.IsAbs(c.Web3.ChainConfig) {
		c.Web3.ChainConfig = filepath.Join(baseDir, c.Web3.ChainConfig)
	}

	if strings.TrimSpace(c.Auth.Mode) == "" {
		c.Auth.Mode = "disabled"
	}
	if c.Auth.JWT.AccessTokenTTLSeconds <= 0 {
		c.Auth.JWT.AccessTokenTTLSeconds = 3600
	}
	if c.Auth.JWT.RefreshTokenTTLSeconds <= 0 {
		c.Auth.JWT.RefreshTokenTTLSeconds = 86400
	}
	if c.Auth.OAuth.TimeoutSeconds <= 0 {
		c.Auth.OAuth.TimeoutSeconds = 15
	}
	if c.Auth.OAuth.UsernameClaim == "" {
		c.Auth.OAuth.UsernameClaim = "username"
	}
}
