package ethereum

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"OpenMCP-Chain/internal/web3"

	gethcore "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	coretypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
)

// Config describes how to construct an EVM compatible client.
type Config struct {
	Name        string
	RPCURL      string
	WSURL       string
	BatchRPCURL string
	Notes       string
}

// Client implements the web3.Client interface for EVM compatible chains.
type Client struct {
	name        string
	notes       string
	rpcClient   *gethrpc.Client
	batchClient *gethrpc.Client
	eth         *ethclient.Client
	eventClient logSubscriber
	backend     bind.ContractBackend
	chainID     *big.Int
	mu          sync.Mutex
}

// logSubscriber mirrors the subset of methods required for log subscriptions.
type logSubscriber interface {
	SubscribeFilterLogs(ctx context.Context, q gethcore.FilterQuery, ch chan<- coretypes.Log) (gethcore.Subscription, error)
}

// NewClient dials the configured RPC endpoints and returns a ready-to-use client.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	rpcURL := strings.TrimSpace(cfg.RPCURL)
	if rpcURL == "" {
		return nil, errors.New("未配置以太坊 RPC 地址")
	}

	rpcClient, err := gethrpc.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("连接以太坊节点失败: %w", err)
	}

	eth := ethclient.NewClient(rpcClient)

	batchClient := rpcClient
	if batchURL := strings.TrimSpace(cfg.BatchRPCURL); batchURL != "" && batchURL != rpcURL {
		batchClient, err = gethrpc.DialContext(ctx, batchURL)
		if err != nil {
			return nil, fmt.Errorf("连接批量交易节点失败: %w", err)
		}
	}

	eventClient := logSubscriber(eth)
	if wsURL := strings.TrimSpace(cfg.WSURL); wsURL != "" {
		if wsRPC, wsErr := gethrpc.DialContext(ctx, wsURL); wsErr == nil {
			eventClient = ethclient.NewClient(wsRPC)
		}
	}

	return &Client{
		name:        cfg.Name,
		notes:       cfg.Notes,
		rpcClient:   rpcClient,
		batchClient: batchClient,
		eth:         eth,
		eventClient: eventClient,
		backend:     eth,
	}, nil
}

// NewSimulatedClient wraps a go-ethereum simulated backend for testing purposes.
func NewSimulatedClient(name string, chainID *big.Int, backend *backends.SimulatedBackend) *Client {
	return &Client{
		name:        name,
		backend:     backend,
		eventClient: backend,
		chainID:     new(big.Int).Set(chainID),
		notes:       "simulated backend",
	}
}

// Close releases network connections held by the client.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.eth != nil {
		c.eth.Close()
		c.eth = nil
	}
	if c.eventClient != nil {
		if ec, ok := c.eventClient.(*ethclient.Client); ok {
			ec.Close()
		}
		c.eventClient = nil
	}
	if c.batchClient != nil && c.batchClient != c.rpcClient {
		c.batchClient.Close()
	}
	if c.rpcClient != nil {
		c.rpcClient.Close()
	}
	c.rpcClient = nil
	c.batchClient = nil
}

// FetchChainSnapshot gathers lightweight metadata from the chain.
func (c *Client) FetchChainSnapshot(ctx context.Context) (web3.ChainSnapshot, error) {
	if c == nil {
		return web3.ChainSnapshot{}, errors.New("未初始化的以太坊客户端")
	}

	if c.eth != nil {
		chainID, err := c.eth.ChainID(ctx)
		if err != nil {
			return web3.ChainSnapshot{}, fmt.Errorf("获取链 ID 失败: %w", err)
		}
		blockNumber, err := c.eth.BlockNumber(ctx)
		if err != nil {
			return web3.ChainSnapshot{}, fmt.Errorf("获取最新区块高度失败: %w", err)
		}
		return web3.ChainSnapshot{
			ChainID:     toHexBig(chainID),
			BlockNumber: fmt.Sprintf("0x%x", blockNumber),
			Notes:       c.notes,
		}, nil
	}

	backend := c.backend
	if backend == nil {
		return web3.ChainSnapshot{}, errors.New("客户端缺少链访问后端")
	}

	id := c.chainID
	if id == nil {
		return web3.ChainSnapshot{}, errors.New("未配置链 ID")
	}

	blockReader, ok := backend.(interface {
		BlockByNumber(context.Context, *big.Int) (*coretypes.Block, error)
	})
	if !ok {
		return web3.ChainSnapshot{}, errors.New("后端不支持区块查询")
	}
	block, err := blockReader.BlockByNumber(ctx, nil)
	if err != nil {
		return web3.ChainSnapshot{}, fmt.Errorf("获取区块信息失败: %w", err)
	}

	return web3.ChainSnapshot{
		ChainID:     toHexBig(id),
		BlockNumber: fmt.Sprintf("0x%x", block.NumberU64()),
		Notes:       c.notes,
	}, nil
}

// ExecuteAction runs small helper RPC calls for the agent layer.
func (c *Client) ExecuteAction(ctx context.Context, action, address string) (string, error) {
	if c == nil {
		return "", errors.New("未初始化的以太坊客户端")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return "", errors.New("链上操作不能为空")
	}

	switch action {
	case "eth_getBalance":
		addr := strings.TrimSpace(address)
		if addr == "" {
			return "", errors.New("eth_getBalance 需要提供地址")
		}
		backend := c.balanceBackend()
		if backend == nil {
			return "", errors.New("当前客户端不支持余额查询")
		}
		balance, err := backend.BalanceAt(ctx, common.HexToAddress(addr), nil)
		if err != nil {
			return "", fmt.Errorf("查询余额失败: %w", err)
		}
		return toHexBig(balance), nil
	case "eth_getTransactionCount":
		addr := strings.TrimSpace(address)
		if addr == "" {
			return "", errors.New("eth_getTransactionCount 需要提供地址")
		}
		backend := c.nonceBackend()
		if backend == nil {
			return "", errors.New("当前客户端不支持交易计数查询")
		}
		nonce, err := backend.PendingNonceAt(ctx, common.HexToAddress(addr))
		if err != nil {
			return "", fmt.Errorf("查询交易计数失败: %w", err)
		}
		return fmt.Sprintf("0x%x", nonce), nil
	default:
		return "", fmt.Errorf("暂不支持的链上操作: %s", action)
	}
}

// DeployContract sends the contract creation transaction using the provided
// transact opts and bytecode.
func (c *Client) DeployContract(ctx context.Context, auth *bind.TransactOpts, abiJSON string, bytecode []byte, params ...any) (web3.DeploymentResult, error) {
	if auth == nil {
		return web3.DeploymentResult{}, errors.New("未提供交易签名器")
	}
	backend := c.contractBackend()
	if backend == nil {
		return web3.DeploymentResult{}, errors.New("当前客户端不支持合约部署")
	}
	if len(bytecode) == 0 {
		return web3.DeploymentResult{}, errors.New("合约字节码不能为空")
	}

	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return web3.DeploymentResult{}, fmt.Errorf("解析 ABI 失败: %w", err)
	}

	originalCtx := auth.Context
	auth.Context = ctx
	defer func() { auth.Context = originalCtx }()

	address, tx, _, err := bind.DeployContract(auth, parsedABI, bytecode, backend, params...)
	if err != nil {
		return web3.DeploymentResult{}, fmt.Errorf("部署合约失败: %w", err)
	}

	if sim, ok := backend.(*backends.SimulatedBackend); ok {
		sim.Commit()
	}

	return web3.DeploymentResult{ContractAddress: address, Transaction: tx}, nil
}

// SubscribeEvents attaches a log subscription to the chain.
func (c *Client) SubscribeEvents(ctx context.Context, query gethcore.FilterQuery) (*web3.EventSubscription, error) {
	if c == nil {
		return nil, errors.New("未初始化的以太坊客户端")
	}
	subscriber := c.eventBackend()
	if subscriber == nil {
		return nil, errors.New("当前客户端不支持事件订阅")
	}

	logs := make(chan coretypes.Log, 64)
	sub, err := subscriber.SubscribeFilterLogs(ctx, query, logs)
	if err != nil {
		return nil, fmt.Errorf("订阅事件失败: %w", err)
	}
	return web3.NewEventSubscription(logs, sub), nil
}

// SendBatchTransactions broadcasts multiple signed transactions in a single
// RPC batch call when possible.
func (c *Client) SendBatchTransactions(ctx context.Context, txs []*coretypes.Transaction) ([]common.Hash, error) {
	if len(txs) == 0 {
		return nil, errors.New("没有可发送的交易")
	}

	if backend, ok := c.backend.(*backends.SimulatedBackend); ok && c.rpcClient == nil {
		hashes := make([]common.Hash, 0, len(txs))
		for _, tx := range txs {
			if err := backend.SendTransaction(ctx, tx); err != nil {
				return nil, fmt.Errorf("发送交易失败: %w", err)
			}
			backend.Commit()
			hashes = append(hashes, tx.Hash())
		}
		return hashes, nil
	}

	if c.batchClient == nil {
		return nil, errors.New("当前客户端未配置批量 RPC")
	}

	hashes := make([]common.Hash, len(txs))
	elems := make([]gethrpc.BatchElem, len(txs))
	for i, tx := range txs {
		raw, err := tx.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("序列化交易失败: %w", err)
		}
		hexPayload := "0x" + hex.EncodeToString(raw)
		elems[i] = gethrpc.BatchElem{
			Method: "eth_sendRawTransaction",
			Args:   []any{hexPayload},
			Result: &hashes[i],
		}
	}

	if err := c.batchClient.BatchCallContext(ctx, elems); err != nil {
		return nil, fmt.Errorf("批量发送交易失败: %w", err)
	}
	for i := range elems {
		if elems[i].Error != nil {
			return nil, fmt.Errorf("交易 %d 发送失败: %w", i, elems[i].Error)
		}
	}
	return hashes, nil
}

func (c *Client) contractBackend() bind.ContractBackend {
	if c.backend != nil {
		return c.backend
	}
	if c.eth != nil {
		return c.eth
	}
	return nil
}

func (c *Client) eventBackend() logSubscriber {
	if c.eventClient != nil {
		return c.eventClient
	}
	if subscriber, ok := c.backend.(logSubscriber); ok {
		return subscriber
	}
	return nil
}

func (c *Client) balanceBackend() interface {
	BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
} {
	if backend, ok := c.backend.(interface {
		BalanceAt(context.Context, common.Address, *big.Int) (*big.Int, error)
	}); ok {
		return backend
	}
	if c.eth != nil {
		return c.eth
	}
	return nil
}

func (c *Client) nonceBackend() interface {
	PendingNonceAt(context.Context, common.Address) (uint64, error)
} {
	if backend, ok := c.backend.(interface {
		PendingNonceAt(context.Context, common.Address) (uint64, error)
	}); ok {
		return backend
	}
	if c.eth != nil {
		return c.eth
	}
	return nil
}

func toHexBig(n *big.Int) string {
	if n == nil {
		return "0x0"
	}
	return "0x" + n.Text(16)
}
