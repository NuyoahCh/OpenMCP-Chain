package ethereum

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client 通过 JSON-RPC 与以太坊兼容链交互。
type Client struct {
	rpcURL     string
	httpClient *http.Client
}

// ChainSnapshot 表示一次对链上状态的快速查询结果。
type ChainSnapshot struct {
	ChainID     string
	BlockNumber string
	Notes       string
}

// NewClient 创建一个新的以太坊 RPC 客户端。
func NewClient(rpcURL string) *Client {
	return &Client{
		rpcURL: rpcURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchChainSnapshot 获取链 ID 和最新区块高度。
func (c *Client) FetchChainSnapshot(ctx context.Context) (ChainSnapshot, error) {
	if c == nil {
		return ChainSnapshot{}, errors.New("未初始化的以太坊客户端")
	}
	if strings.TrimSpace(c.rpcURL) == "" {
		return ChainSnapshot{}, errors.New("未配置以太坊 RPC 地址")
	}

	chainID, err := c.callStringResult(ctx, "eth_chainId", nil)
	if err != nil {
		return ChainSnapshot{}, err
	}

	blockNumber, err := c.callStringResult(ctx, "eth_blockNumber", nil)
	if err != nil {
		return ChainSnapshot{}, err
	}

	return ChainSnapshot{
		ChainID:     chainID,
		BlockNumber: blockNumber,
		Notes:       "",
	}, nil
}

func (c *Client) callStringResult(ctx context.Context, method string, params any) (string, error) {
	var paramSlice []any
	if params != nil {
		if arr, ok := params.([]any); ok {
			paramSlice = arr
		} else {
			paramSlice = []any{params}
		}
	}

	payload := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramSlice,
		ID:      1,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化 JSON-RPC 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("构造 HTTP 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用链上节点失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("节点返回异常状态码: %s", resp.Status)
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("解析 JSON-RPC 响应失败: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("链上返回错误: %s", rpcResp.Error.Message)
	}

	var result string
	if err := json.Unmarshal(rpcResp.Result, &result); err != nil {
		return "", fmt.Errorf("解析 JSON-RPC 结果失败: %w", err)
	}

	return result, nil
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
	ID      int             `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
