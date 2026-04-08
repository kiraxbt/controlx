package chain

import (
	"bufio"
	"context"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

const maxRetries = 3

// Provider manages multiple Ankr RPC keys with round-robin load balancing
// and automatic retry/failover across keys.
type Provider struct {
	keys    []string
	counter uint64
	mu      sync.Mutex
	clients map[string]*ethclient.Client // keyed by rpcURL
}

// NewProvider loads Ankr keys from the given file and creates a provider.
func NewProvider(keysFile string) (*Provider, error) {
	f, err := os.Open(keysFile)
	if err != nil {
		return nil, fmt.Errorf("open keys file: %w", err)
	}
	defer f.Close()

	var keys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		key := strings.TrimSpace(scanner.Text())
		if key != "" {
			keys = append(keys, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read keys file: %w", err)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys found in %s", keysFile)
	}

	return &Provider{
		keys:    keys,
		clients: make(map[string]*ethclient.Client),
	}, nil
}

// KeyCount returns the number of loaded RPC keys.
func (p *Provider) KeyCount() int {
	return len(p.keys)
}

// nextKey returns the next key using round-robin.
func (p *Provider) nextKey() string {
	idx := atomic.AddUint64(&p.counter, 1)
	return p.keys[idx%uint64(len(p.keys))]
}

// rpcURL builds the full Ankr RPC URL for a chain and key.
func rpcURL(ch Chain, key string) string {
	return fmt.Sprintf("https://rpc.ankr.com/%s/%s", ch.RPCPath, key)
}

// getClient returns a cached or new ethclient for a specific chain+key combo.
func (p *Provider) getClient(ch Chain, key string) (*ethclient.Client, error) {
	url := rpcURL(ch, key)

	p.mu.Lock()
	if c, ok := p.clients[url]; ok {
		p.mu.Unlock()
		return c, nil
	}
	p.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c, err := ethclient.DialContext(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", ch.Name, err)
	}

	p.mu.Lock()
	// check again in case another goroutine created it
	if existing, ok := p.clients[url]; ok {
		p.mu.Unlock()
		c.Close()
		return existing, nil
	}
	p.clients[url] = c
	p.mu.Unlock()

	return c, nil
}

// removeClient removes a failed client from cache so it gets recreated.
func (p *Provider) removeClient(ch Chain, key string) {
	url := rpcURL(ch, key)
	p.mu.Lock()
	if c, ok := p.clients[url]; ok {
		c.Close()
		delete(p.clients, url)
	}
	p.mu.Unlock()
}

// withRetry executes fn with auto-retry failover across multiple RPC keys.
// On failure, it rotates to the next key and retries up to maxRetries * len(keys) times.
func (p *Provider) withRetry(ch Chain, fn func(client *ethclient.Client) error) error {
	totalAttempts := maxRetries * len(p.keys)
	var lastErr error
	for attempt := 0; attempt < totalAttempts; attempt++ {
		key := p.nextKey()
		client, err := p.getClient(ch, key)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}
		err = fn(client)
		if err == nil {
			return nil
		}
		lastErr = err
		// Remove failed client so it gets recreated on next attempt
		p.removeClient(ch, key)
		time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
	}
	return fmt.Errorf("all retries failed for %s: %w", ch.Name, lastErr)
}

// GetClient returns an ethclient for the given chain (simple round-robin, no retry).
func (p *Provider) GetClient(ch Chain) (*ethclient.Client, error) {
	key := p.nextKey()
	return p.getClient(ch, key)
}

// GetClientWithRetry tries multiple keys until a connection succeeds.
func (p *Provider) GetClientWithRetry(ch Chain) (*ethclient.Client, error) {
	var lastErr error
	for i := 0; i < len(p.keys); i++ {
		key := p.nextKey()
		client, err := p.getClient(ch, key)
		if err != nil {
			lastErr = err
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err = client.ChainID(ctx)
		cancel()
		if err != nil {
			lastErr = err
			p.removeClient(ch, key)
			continue
		}
		return client, nil
	}
	return nil, fmt.Errorf("all keys failed for %s: %w", ch.Name, lastErr)
}

// BalanceAt returns the native token balance with auto-retry failover.
func (p *Provider) BalanceAt(ctx context.Context, ch Chain, addr common.Address) (*big.Int, error) {
	var result *big.Int
	err := p.withRetry(ch, func(client *ethclient.Client) error {
		var err error
		result, err = client.BalanceAt(ctx, addr, nil)
		return err
	})
	return result, err
}

// SuggestGasPrice returns the suggested gas price with auto-retry failover.
func (p *Provider) SuggestGasPrice(ctx context.Context, ch Chain) (*big.Int, error) {
	var result *big.Int
	err := p.withRetry(ch, func(client *ethclient.Client) error {
		var err error
		result, err = client.SuggestGasPrice(ctx)
		return err
	})
	return result, err
}

// EstimateGas estimates gas with auto-retry failover.
func (p *Provider) EstimateGas(ctx context.Context, ch Chain, msg ethereum.CallMsg) (uint64, error) {
	var result uint64
	err := p.withRetry(ch, func(client *ethclient.Client) error {
		var err error
		result, err = client.EstimateGas(ctx, msg)
		return err
	})
	return result, err
}

// PendingNonceAt returns the next nonce with auto-retry failover.
func (p *Provider) PendingNonceAt(ctx context.Context, ch Chain, addr common.Address) (uint64, error) {
	var result uint64
	err := p.withRetry(ch, func(client *ethclient.Client) error {
		var err error
		result, err = client.PendingNonceAt(ctx, addr)
		return err
	})
	return result, err
}

// SendTransaction sends a signed transaction with auto-retry failover.
func (p *Provider) SendTransaction(ctx context.Context, ch Chain, tx *types.Transaction) error {
	return p.withRetry(ch, func(client *ethclient.Client) error {
		return client.SendTransaction(ctx, tx)
	})
}

// WaitForReceipt polls for a transaction receipt until it's mined or context expires.
// Returns the receipt status (1 = success, 0 = revert).
func (p *Provider) WaitForReceipt(ctx context.Context, ch Chain, txHash common.Hash) (*types.Receipt, error) {
	var receipt *types.Receipt
	for {
		err := p.withRetry(ch, func(client *ethclient.Client) error {
			var err error
			receipt, err = client.TransactionReceipt(ctx, txHash)
			return err
		})
		if err == nil && receipt != nil {
			return receipt, nil
		}
		// "not found" means not mined yet — keep polling
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		time.Sleep(3 * time.Second)
	}
}

// CallContract executes a contract call with auto-retry failover.
func (p *Provider) CallContract(ctx context.Context, ch Chain, msg ethereum.CallMsg) ([]byte, error) {
	var result []byte
	err := p.withRetry(ch, func(client *ethclient.Client) error {
		var err error
		result, err = client.CallContract(ctx, msg, nil)
		return err
	})
	return result, err
}

// Close closes all cached clients.
func (p *Provider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.clients {
		c.Close()
	}
	p.clients = make(map[string]*ethclient.Client)
}
