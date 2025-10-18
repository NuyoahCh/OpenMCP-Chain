package web3

import (
	"context"

	gethcore "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethevent "github.com/ethereum/go-ethereum/event"
)

// ChainSnapshot represents summarized network metadata for UI/reporting.
type ChainSnapshot struct {
	ChainID     string
	BlockNumber string
	Notes       string
}

// DeploymentResult captures the outcome of a contract deployment request.
type DeploymentResult struct {
	ContractAddress common.Address
	Transaction     *types.Transaction
}

// EventSubscription wraps a log subscription so callers can manage lifecycle
// without depending on the go-ethereum event package.
type EventSubscription struct {
	logs <-chan types.Log
	sub  gethevent.Subscription
}

// NewEventSubscription constructs a managed subscription wrapper.
func NewEventSubscription(logs <-chan types.Log, sub gethevent.Subscription) *EventSubscription {
	return &EventSubscription{logs: logs, sub: sub}
}

// Logs returns the channel that receives blockchain logs.
func (e *EventSubscription) Logs() <-chan types.Log {
	return e.logs
}

// Err forwards the subscription error channel.
func (e *EventSubscription) Err() <-chan error {
	if e == nil || e.sub == nil {
		return nil
	}
	return e.sub.Err()
}

// Close terminates the subscription.
func (e *EventSubscription) Close() {
	if e == nil || e.sub == nil {
		return
	}
	e.sub.Unsubscribe()
}

// Client defines the common interface that any chain implementation must
// provide so higher layers can interact with different networks uniformly.
type Client interface {
	FetchChainSnapshot(ctx context.Context) (ChainSnapshot, error)
	ExecuteAction(ctx context.Context, action, address string) (string, error)
	DeployContract(ctx context.Context, auth *bind.TransactOpts, abiJSON string, bytecode []byte, params ...any) (DeploymentResult, error)
	SubscribeEvents(ctx context.Context, query gethcore.FilterQuery) (*EventSubscription, error)
	SendBatchTransactions(ctx context.Context, txs []*types.Transaction) ([]common.Hash, error)
	Close()
}
