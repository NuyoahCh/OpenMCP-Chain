package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Config 描述了 OpenMCP 在启动阶段需要加载的核心配置。
type Config struct {
	Server  ServerConfig  `json:"server"`
	Storage StorageConfig `json:"storage"`
	LLM     LLMConfig     `json:"llm"`
	Web3    Web3Config    `json:"web3"`
	Runtime RuntimeConfig `json:"runtime"`
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
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// LLMConfig 用于配置大模型推理的调用方式。
type LLMConfig struct {
	Provider string             `json:"provider"`
	Python   PythonBridgeConfig `json:"python_bridge"`
}

// PythonBridgeConfig 描述通过 Python 脚本完成推理时所需的信息。
type PythonBridgeConfig struct {
	Enabled          bool   `json:"enabled"`
	PythonExecutable string `json:"python_executable"`
	ScriptPath       string `json:"script_path"`
	WorkingDir       string `json:"working_dir"`
}

// Web3Config 包含访问区块链节点所需的 RPC 地址。
type Web3Config struct {
	RPCURL string `json:"rpc_url"`
}

// RuntimeConfig 用于放置运行时的通用参数。
type RuntimeConfig struct {
	DataDir string `json:"data_dir"`
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

	if c.Runtime.DataDir == "" {
		c.Runtime.DataDir = filepath.Join(baseDir, "data")
	} else if !filepath.IsAbs(c.Runtime.DataDir) {
		c.Runtime.DataDir = filepath.Join(baseDir, c.Runtime.DataDir)
	}
}
