package pythonbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"OpenMCP-Chain/internal/llm"
)

// Client 通过调用 Python 脚本实现大模型推理。
type Client struct {
	pythonExec string
	scriptPath string
	workingDir string
}

// NewClient 创建 Python Bridge 客户端。
func NewClient(pythonExec, scriptPath, workingDir string) (*Client, error) {
	if scriptPath == "" {
		return nil, fmt.Errorf("未指定 Python 脚本路径")
	}
	if pythonExec == "" {
		pythonExec = "python3"
	}
	return &Client{
		pythonExec: pythonExec,
		scriptPath: scriptPath,
		workingDir: workingDir,
	}, nil
}

// Generate 调用外部脚本，并解析输出。
func (c *Client) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	payload := map[string]any{
		"goal":         req.Goal,
		"chain_action": req.ChainAction,
		"address":      req.Address,
		"timestamp":    time.Now().Unix(),
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	command := exec.CommandContext(ctx, c.pythonExec, c.scriptPath)
	if c.workingDir != "" {
		command.Dir = c.workingDir
	}
	command.Stdin = bytes.NewReader(encoded)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("执行 Python 脚本失败: %v, stderr=%s", err, strings.TrimSpace(stderr.String()))
	}

	var resp struct {
		Thought string `json:"thought"`
		Reply   string `json:"reply"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("解析 Python 输出失败: %w", err)
	}

	return &llm.Response{
		Thought: resp.Thought,
		Reply:   resp.Reply,
	}, nil
}

// ResolveScriptPath 根据工作目录推导脚本绝对路径。
func ResolveScriptPath(baseDir, script string) string {
	if script == "" {
		return ""
	}
	if filepath.IsAbs(script) {
		return script
	}
	if baseDir == "" {
		return script
	}
	return filepath.Join(baseDir, script)
}
