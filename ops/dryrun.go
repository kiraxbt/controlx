package ops

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"controlx/chain"
	"controlx/wallet"

	"github.com/ethereum/go-ethereum"
)

// DryRunResult holds the simulation result for a batch operation.
type DryRunResult struct {
	Operation    string
	Chain        chain.Chain
	TxCount      int
	GasPrice     *big.Int
	GasPerTx     uint64
	TotalGasWei  *big.Int
	TotalGasETH  string
	TotalUSD     float64
	PriceUSD     float64
	SourceBal    *big.Int
	SourceAddr   string
	Affordable   bool
	Errors       []string
}

// DryRunDistribute simulates a distribute operation.
func DryRunDistribute(prov *chain.Provider, ch chain.Chain, from wallet.Wallet, toCount int, amountWei *big.Int, tokenAddr string, priceCache *PriceCache) (*DryRunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := &DryRunResult{
		Operation:  "distribute",
		Chain:      ch,
		TxCount:    toCount,
		SourceAddr: from.Address,
	}

	// Get gas price
	gasPrice, err := prov.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}
	result.GasPrice = gasPrice

	// Determine gas limit
	if tokenAddr == "" {
		result.GasPerTx = 21000
	} else {
		result.GasPerTx = 65000 // ERC-20 transfer estimate
	}

	// Calculate total gas
	totalGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(result.GasPerTx)))
	totalGas.Mul(totalGas, big.NewInt(int64(toCount)))
	result.TotalGasWei = totalGas
	result.TotalGasETH = FormatBalance(totalGas, 18)

	// Get source balance
	sourceBal, err := prov.BalanceAt(ctx, ch, from.CommonAddress())
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("source balance: %s", err))
	} else {
		result.SourceBal = sourceBal
	}

	// Check affordability
	if sourceBal != nil {
		totalNeeded := new(big.Int).Add(totalGas, new(big.Int).Mul(amountWei, big.NewInt(int64(toCount))))
		result.Affordable = sourceBal.Cmp(totalNeeded) >= 0
		if !result.Affordable {
			deficit := new(big.Int).Sub(totalNeeded, sourceBal)
			result.Errors = append(result.Errors,
				fmt.Sprintf("insufficient: need %s, have %s (deficit: %s %s)",
					FormatBalance(totalNeeded, 18), FormatBalance(sourceBal, 18),
					FormatBalance(deficit, 18), ch.Symbol))
		}
	}

	// USD price
	if priceCache != nil {
		priceCache.FetchPrices()
		result.PriceUSD = priceCache.GetPrice(ch.Name)
		if result.PriceUSD > 0 {
			result.TotalUSD = weiToFloat(totalGas) * result.PriceUSD
		}
	}

	return result, nil
}

// DryRunSweep simulates a sweep operation.
func DryRunSweep(prov *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, tokenAddr string, priceCache *PriceCache) (*DryRunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := &DryRunResult{
		Operation: "sweep",
		Chain:     ch,
	}

	gasPrice, err := prov.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}
	result.GasPrice = gasPrice

	if tokenAddr == "" {
		result.GasPerTx = 21000
	} else {
		result.GasPerTx = 65000
	}

	// Count wallets that would actually sweep (non-zero balance)
	sweepable := 0
	for _, w := range wallets {
		bCtx, bCancel := context.WithTimeout(context.Background(), 5*time.Second)
		bal, bErr := prov.BalanceAt(bCtx, ch, w.CommonAddress())
		bCancel()
		if bErr == nil && bal.Sign() > 0 {
			gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(result.GasPerTx)))
			if bal.Cmp(gasCost) > 0 {
				sweepable++
			}
		}
	}
	result.TxCount = sweepable

	totalGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(result.GasPerTx)))
	totalGas.Mul(totalGas, big.NewInt(int64(sweepable)))
	result.TotalGasWei = totalGas
	result.TotalGasETH = FormatBalance(totalGas, 18)
	result.Affordable = true

	if priceCache != nil {
		priceCache.FetchPrices()
		result.PriceUSD = priceCache.GetPrice(ch.Name)
		if result.PriceUSD > 0 {
			result.TotalUSD = weiToFloat(totalGas) * result.PriceUSD
		}
	}

	return result, nil
}

// DryRunSwap simulates gas cost for a DEX mix operation.
func DryRunSwap(prov *chain.Provider, ch chain.Chain, numHops int, priceCache *PriceCache) (*DryRunResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result := &DryRunResult{
		Operation: "dexmix",
		Chain:     ch,
		TxCount:   numHops,
	}

	gasPrice, err := prov.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("gas price: %w", err)
	}
	result.GasPrice = gasPrice

	// Average: even hops = 1 tx (swap), odd hops = 2 tx (approve+swap)
	// Approximate: 1.5 tx per hop * 300k gas each
	avgGasPerHop := uint64(450000)
	result.GasPerTx = avgGasPerHop

	totalGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(avgGasPerHop)))
	totalGas.Mul(totalGas, big.NewInt(int64(numHops)))
	result.TotalGasWei = totalGas
	result.TotalGasETH = FormatBalance(totalGas, 18)
	result.Affordable = true

	if priceCache != nil {
		priceCache.FetchPrices()
		result.PriceUSD = priceCache.GetPrice(ch.Name)
		if result.PriceUSD > 0 {
			result.TotalUSD = weiToFloat(totalGas) * result.PriceUSD
		}
	}

	return result, nil
}

// EstimateGasLimit estimates gas for a specific call.
func EstimateGasLimit(prov *chain.Provider, ch chain.Chain, msg ethereum.CallMsg) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return prov.EstimateGas(ctx, ch, msg)
}
