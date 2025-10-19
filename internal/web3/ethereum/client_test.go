package ethereum

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"OpenMCP-Chain/internal/web3"

	gethcore "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	coretypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	simpleContractABI        = "[]"
	simpleContractBin        = "0x6027600c60003960276000f37f0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2060006000a100"
	simpleContractEventTopic = "0x0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
)

func TestClientDeploySubscribeBatch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	chainID := big.NewInt(1337)
	auth, err := bind.NewKeyedTransactorWithChainID(key, chainID)
	if err != nil {
		t.Fatalf("new transactor: %v", err)
	}
	auth.GasLimit = 1_000_000

	alloc := core.GenesisAlloc{
		auth.From: {Balance: big.NewInt(1_000_000_000_000_000_000)},
	}
	backend := backends.NewSimulatedBackend(alloc, 8_000_000)
	client := NewSimulatedClient("simulated", chainID, backend)
	t.Cleanup(client.Close)

	bytecode := common.FromHex(simpleContractBin)
	deployResult, err := client.DeployContract(ctx, auth, simpleContractABI, bytecode)
	if err != nil {
		t.Fatalf("deploy contract: %v", err)
	}
	if deployResult.ContractAddress == (common.Address{}) {
		t.Fatal("expected contract address to be non-zero")
	}

	snapshot, err := client.FetchChainSnapshot(ctx)
	if err != nil {
		t.Fatalf("fetch snapshot: %v", err)
	}
	if snapshot.ChainID != "0x"+chainID.Text(16) {
		t.Fatalf("unexpected chain id %s", snapshot.ChainID)
	}
	if snapshot.BlockNumber == "0x0" {
		t.Fatal("expected block number to advance after deployment")
	}

	logQuery := gethcore.FilterQuery{Addresses: []common.Address{deployResult.ContractAddress}}
	sub, err := client.SubscribeEvents(ctx, logQuery)
	if err != nil {
		t.Fatalf("subscribe events: %v", err)
	}
	defer sub.Close()

	nonce, err := backend.PendingNonceAt(ctx, auth.From)
	if err != nil {
		t.Fatalf("pending nonce: %v", err)
	}
	head, err := backend.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatalf("latest header: %v", err)
	}
	gasTipCap := big.NewInt(1_000_000_000)
	gasFeeCap := new(big.Int).Set(gasTipCap)
	if head.BaseFee != nil {
		gasFeeCap = new(big.Int).Add(head.BaseFee, gasTipCap)
	}
	tx := coretypes.NewTx(&coretypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       120000,
		To:        &deployResult.ContractAddress,
	})
	signed, err := coretypes.SignTx(tx, coretypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatalf("sign tx: %v", err)
	}

	hashes, err := client.SendBatchTransactions(ctx, []*coretypes.Transaction{signed})
	if err != nil {
		t.Fatalf("send batch: %v", err)
	}
	if len(hashes) != 1 {
		t.Fatalf("expected 1 hash, got %d", len(hashes))
	}

	receipt, err := waitForReceipt(ctx, backend, hashes[0])
	if err != nil {
		t.Fatalf("wait mined: %v", err)
	}
	if len(receipt.Logs) == 0 {
		t.Fatal("expected receipt to contain logs")
	}

	expectedTopic := common.HexToHash(simpleContractEventTopic)
	logCh := sub.Logs()
	select {
	case log := <-logCh:
		if log.Address != deployResult.ContractAddress {
			t.Fatalf("unexpected log address %s", log.Address.Hex())
		}
		if len(log.Topics) == 0 || log.Topics[0] != expectedTopic {
			t.Fatalf("unexpected log topics %+v", log.Topics)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for event log")
	}

	balanceHex, err := client.ExecuteAction(ctx, "eth_getBalance", auth.From.Hex())
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	if balanceHex == "" {
		t.Fatal("expected balance result")
	}
}

func waitForReceipt(ctx context.Context, backend *backends.SimulatedBackend, hash common.Hash) (*coretypes.Receipt, error) {
	backend.Commit()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		receipt, err := backend.TransactionReceipt(ctx, hash)
		if err == nil && receipt != nil {
			return receipt, nil
		}
		if err != nil && !errors.Is(err, gethcore.NotFound) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			backend.Commit()
		}
	}
}

var _ web3.Client = (*Client)(nil)
