package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"OpenMCP-Chain/internal/config"
	"OpenMCP-Chain/internal/web3"
	"OpenMCP-Chain/internal/web3/ethereum"
)

// Registry manages a set of chain clients keyed by human readable names.
type Registry struct {
	defaultChain string
	clients      map[string]web3.Client
}

// NewRegistry loads chain definitions and instantiates concrete clients.
func NewRegistry(ctx context.Context, cfg config.Web3Config) (*Registry, error) {
	defs, err := web3.LoadChainDefinitions(cfg.ChainConfig)
	if err != nil {
		return nil, err
	}

	clients := make(map[string]web3.Client)
	for name, chain := range defs.Chains {
		chainType := strings.ToLower(strings.TrimSpace(chain.Type))
		if chainType == "" {
			chainType = "evm"
		}
		switch chainType {
		case "evm":
			client, err := ethereum.NewClient(ctx, ethereum.Config{
				Name:        name,
				RPCURL:      chain.RPCURL,
				WSURL:       chain.WSURL,
				BatchRPCURL: chain.BatchRPCURL,
				Notes:       chain.Description,
			})
			if err != nil {
				return nil, fmt.Errorf("初始化链 %s 失败: %w", name, err)
			}
			clients[name] = client
		default:
			return nil, fmt.Errorf("链 %s 使用了不支持的类型 %s", name, chain.Type)
		}
	}

	if len(clients) == 0 && strings.TrimSpace(cfg.RPCURL) != "" {
		client, err := ethereum.NewClient(ctx, ethereum.Config{RPCURL: cfg.RPCURL})
		if err != nil {
			return nil, err
		}
		clients["default"] = client
		if cfg.DefaultChain == "" {
			cfg.DefaultChain = "default"
		}
	}

	if len(clients) == 0 {
		return nil, errors.New("未配置任何链的 RPC 端点")
	}

	defaultChain := cfg.DefaultChain
	if defaultChain == "" {
		names := make([]string, 0, len(clients))
		for name := range clients {
			names = append(names, name)
		}
		sort.Strings(names)
		defaultChain = names[0]
	}
	if _, ok := clients[defaultChain]; !ok {
		return nil, fmt.Errorf("默认链 %s 未在配置中找到", defaultChain)
	}

	return &Registry{defaultChain: defaultChain, clients: clients}, nil
}

// DefaultClient returns the client configured as default chain.
func (r *Registry) DefaultClient() (web3.Client, error) {
	if r == nil {
		return nil, errors.New("未初始化的链客户端注册表")
	}
	client, ok := r.clients[r.defaultChain]
	if !ok {
		return nil, fmt.Errorf("默认链 %s 未在注册表中", r.defaultChain)
	}
	return client, nil
}

// Client returns the chain client identified by name.
func (r *Registry) Client(name string) (web3.Client, bool) {
	if r == nil {
		return nil, false
	}
	client, ok := r.clients[name]
	return client, ok
}

// Close releases all clients managed by the registry.
func (r *Registry) Close() {
	if r == nil {
		return
	}
	for name, client := range r.clients {
		if client != nil {
			client.Close()
		}
		delete(r.clients, name)
	}
}

// Chains returns the list of registered chain names.
func (r *Registry) Chains() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
