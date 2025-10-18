package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Config 描述了 OpenMCP 在启动阶段需要加载的核心配置。
type Config struct {
	Server    ServerConfig    `json:"server"`
	Storage   StorageConfig   `json:"storage"`
	LLM       LLMConfig       `json:"llm"`
	Web3      Web3Config      `json:"web3"`
	Agent     AgentConfig     `json:"agent"`
	Runtime   RuntimeConfig   `json:"runtime"`
	Knowledge KnowledgeConfig `json:"knowledge"`
	TaskQueue TaskQueueConfig `json:"task_queue"`
}

// ServerConfig 控制 API 服务的监听地址等参数。
type ServerConfig struct {
	Address string `json:"address"`
}

// StorageConfig 统一描述 MySQL、Redis 等后端的连接信息。
type StorageConfig struct {
	TaskStore TaskStoreConfig `json:"task_store"`
}

// TaskStoreConfig 目前提供内存实现，后续可以切换到真正的 MySQL。
type TaskStoreConfig struct {
	Driver  string `json:"driver"`
	DSN     string `json:"dsn"`
	Retries int    `json:"retries"`
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
	Driver                 string `json:"driver"`
	DSN                    string `json:"dsn"`
	MaxOpenConns           int    `json:"max_open_conns"`
	MaxIdleConns           int    `json:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `json:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int    `json:"conn_max_idle_time_seconds"`
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
}
