package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"mixer/chain"
	"mixer/wallet"
)

// PriceCache caches token prices from CoinGecko.
type PriceCache struct {
	mu     sync.RWMutex
	prices map[string]float64 // coinGeckoID → USD price
	expiry time.Time
	ttl    time.Duration
}

// NewPriceCache creates a price cache with the given TTL.
func NewPriceCache(ttl time.Duration) *PriceCache {
	return &PriceCache{
		prices: make(map[string]float64),
		ttl:    ttl,
	}
}

// FetchPrices retrieves current prices from CoinGecko for all supported chains.
func (pc *PriceCache) FetchPrices() error {
	pc.mu.RLock()
	if time.Now().Before(pc.expiry) {
		pc.mu.RUnlock()
		return nil
	}
	pc.mu.RUnlock()

	// Collect unique CoinGecko IDs
	idSet := make(map[string]bool)
	for _, id := range chain.CoinGeckoIDs {
		idSet[id] = true
	}
	var ids []string
	for id := range idSet {
		ids = append(ids, id)
	}

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd",
		strings.Join(ids, ","))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("price fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("coingecko: %s", resp.Status)
	}

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse prices: %w", err)
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()
	for id, data := range result {
		if usd, ok := data["usd"]; ok {
			pc.prices[id] = usd
		}
	}
	pc.expiry = time.Now().Add(pc.ttl)
	return nil
}

// GetPrice returns the USD price for a chain's native token.
func (pc *PriceCache) GetPrice(chainName string) float64 {
	cgID, ok := chain.CoinGeckoIDs[chainName]
	if !ok {
		return 0
	}
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.prices[cgID]
}

// ChainPortfolio holds portfolio data for a single chain.
type ChainPortfolio struct {
	Chain        chain.Chain
	TotalNative  *big.Int
	NonZero      int
	Errors       int
	PriceUSD     float64
	TotalUSD     float64
}

// PortfolioResult holds the full portfolio scan result.
type PortfolioResult struct {
	Chains    []ChainPortfolio
	TotalUSD  float64
	ScanTime  time.Duration
}

// ScanPortfolio scans native balance across all chains for all wallets.
func ScanPortfolio(prov *chain.Provider, wallets []wallet.Wallet, chains []chain.Chain, priceCache *PriceCache) (*PortfolioResult, error) {
	start := time.Now()

	// Fetch latest prices
	priceCache.FetchPrices() // best-effort, don't fail

	result := &PortfolioResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ch := range chains {
		wg.Add(1)
		go func(c chain.Chain) {
			defer wg.Done()

			cp := ChainPortfolio{
				Chain:       c,
				TotalNative: new(big.Int),
				PriceUSD:    priceCache.GetPrice(c.Name),
			}

			sem := make(chan struct{}, 30)
			var innerWg sync.WaitGroup
			var innerMu sync.Mutex
			rateLimiter := time.NewTicker(15 * time.Millisecond)
			defer rateLimiter.Stop()

			for _, w := range wallets {
				innerWg.Add(1)
				go func(wl wallet.Wallet) {
					defer innerWg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					<-rateLimiter.C

					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()

					bal, err := prov.BalanceAt(ctx, c, wl.CommonAddress())
					innerMu.Lock()
					defer innerMu.Unlock()
					if err != nil {
						cp.Errors++
						return
					}
					if bal.Sign() > 0 {
						cp.TotalNative.Add(cp.TotalNative, bal)
						cp.NonZero++
					}
				}(w)
			}
			innerWg.Wait()

			// Calculate USD value
			if cp.PriceUSD > 0 && cp.TotalNative.Sign() > 0 {
				ethFloat := weiToFloat(cp.TotalNative)
				cp.TotalUSD = ethFloat * cp.PriceUSD
			}

			mu.Lock()
			result.Chains = append(result.Chains, cp)
			result.TotalUSD += cp.TotalUSD
			mu.Unlock()
		}(ch)
	}

	wg.Wait()
	result.ScanTime = time.Since(start)
	return result, nil
}

// weiToFloat converts wei to ether as float64.
func weiToFloat(wei *big.Int) float64 {
	f := new(big.Float).SetInt(wei)
	divisor := new(big.Float).SetFloat64(1e18)
	f.Quo(f, divisor)
	result, _ := f.Float64()
	return result
}

// GasEstimate holds gas estimation for a batch operation.
type GasEstimate struct {
	Chain       chain.Chain
	GasPrice    *big.Int
	GasLimit    uint64
	TxCount     int
	TotalGasWei *big.Int
	TotalUSD    float64
	PriceUSD    float64
}

// EstimateBatchGas estimates total gas cost for a batch of transactions.
func EstimateBatchGas(prov *chain.Provider, ch chain.Chain, txCount int, gasLimit uint64, priceCache *PriceCache) (*GasEstimate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gasPrice, err := prov.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}

	totalGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	totalGas.Mul(totalGas, big.NewInt(int64(txCount)))

	priceCache.FetchPrices()
	priceUSD := priceCache.GetPrice(ch.Name)
	totalUSD := 0.0
	if priceUSD > 0 {
		totalUSD = weiToFloat(totalGas) * priceUSD
	}

	return &GasEstimate{
		Chain:       ch,
		GasPrice:    gasPrice,
		GasLimit:    gasLimit,
		TxCount:     txCount,
		TotalGasWei: totalGas,
		TotalUSD:    totalUSD,
		PriceUSD:    priceUSD,
	}, nil
}
