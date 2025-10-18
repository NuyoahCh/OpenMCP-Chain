package web3

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ChainDefinitions models the structure of configs/chain.yaml.
type ChainDefinitions struct {
	Chains map[string]ChainDefinition `yaml:"chains"`
}

// ChainDefinition describes a single chain endpoint definition.
type ChainDefinition struct {
	Type        string `yaml:"type"`
	RPCURL      string `yaml:"rpc_url"`
	WSURL       string `yaml:"ws_url"`
	BatchRPCURL string `yaml:"batch_rpc_url"`
	Description string `yaml:"description"`
}

// LoadChainDefinitions parses the YAML file containing chain metadata.
func LoadChainDefinitions(path string) (ChainDefinitions, error) {
	if strings.TrimSpace(path) == "" {
		return ChainDefinitions{Chains: map[string]ChainDefinition{}}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ChainDefinitions{}, fmt.Errorf("读取链配置失败: %w", err)
	}

	var defs ChainDefinitions
	if err := yaml.Unmarshal(content, &defs); err != nil {
		return ChainDefinitions{}, fmt.Errorf("解析链配置失败: %w", err)
	}
	if defs.Chains == nil {
		defs.Chains = map[string]ChainDefinition{}
	}
	return defs, nil
}
