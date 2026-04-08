package ops

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"controlx/chain"
	"controlx/wallet"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// FundResult holds the result of auto-funding a single wallet.
type FundResult struct {
	Index      int
	Address    string
	NeedGas    bool
	HasToken   bool
	GasNeeded  *big.Int
	GasFunded  *big.Int
	TxHash     string
	Error      string
}

// ScanGasNeeds scans wallets to find which ones need gas for ERC-20 transfers.
// Returns only wallets that have token balance but insufficient native balance for gas.
func ScanGasNeeds(provider *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, tokenAddr common.Address, gasLimit uint64) ([]FundResult, error) {
	results := make([]FundResult, len(wallets))

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 10)
	defer rateLimiter.Stop()

	for i, w := range wallets {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			addr := wl.CommonAddress()
			result := FundResult{Index: idx, Address: wl.Address}

			// Check token balance
			tokenBal, err := getERC20Balance(ctx, provider, ch, tokenAddr, addr)
			if err != nil {
				result.Error = fmt.Sprintf("token balance: %s", err)
				results[idx] = result
				return
			}

			if tokenBal.Sign() == 0 {
				// No token, no need to fund
				results[idx] = result
				return
			}
			result.HasToken = true

			// Check native balance
			nativeBal, err := provider.BalanceAt(ctx, ch, addr)
			if err != nil {
				result.Error = fmt.Sprintf("native balance: %s", err)
				results[idx] = result
				return
			}

			// Estimate gas cost for ERC-20 transfer
			gasPrice, err := provider.SuggestGasPrice(ctx, ch)
			if err != nil {
				result.Error = fmt.Sprintf("gas price: %s", err)
				results[idx] = result
				return
			}

			// Use provided gasLimit or estimate
			gl := gasLimit
			if gl == 0 {
				dummyData := buildTransferData(common.HexToAddress("0x0000000000000000000000000000000000000001"), tokenBal)
				estimated, err := provider.EstimateGas(ctx, ch, ethereum.CallMsg{
					From: addr,
					To:   &tokenAddr,
					Data: dummyData,
				})
				if err != nil {
					gl = 100000 // fallback
				} else {
					gl = estimated + estimated/5 // +20% buffer
				}
			}

			gasCost := new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gl))
			// Add 30% buffer for gas price fluctuation
			buffer := new(big.Int).Div(gasCost, big.NewInt(3))
			totalNeeded := new(big.Int).Add(gasCost, buffer)

			if nativeBal.Cmp(totalNeeded) < 0 {
				result.NeedGas = true
				result.GasNeeded = new(big.Int).Sub(totalNeeded, nativeBal)
			}

			results[idx] = result
		}(i, w)
	}

	wg.Wait()
	return results, nil
}

// ScanLowGas scans wallets and marks those with native balance below minBalance.
// Unlike ScanGasNeeds, this does not require an ERC-20 token address.
func ScanLowGas(provider *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, minBalance *big.Int) ([]FundResult, error) {
	results := make([]FundResult, len(wallets))

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 10)
	defer rateLimiter.Stop()

	for i, w := range wallets {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			addr := wl.CommonAddress()
			result := FundResult{Index: idx, Address: wl.Address}

			nativeBal, err := provider.BalanceAt(ctx, ch, addr)
			if err != nil {
				result.Error = fmt.Sprintf("balance: %s", err)
				results[idx] = result
				return
			}

			if nativeBal.Cmp(minBalance) < 0 {
				result.NeedGas = true
				result.GasNeeded = new(big.Int).Sub(minBalance, nativeBal)
			}

			results[idx] = result
		}(i, w)
	}

	wg.Wait()
	return results, nil
}

// AutoFundGas distributes gas from a funder wallet to all wallets that need it.
// Only funds wallets that have ERC-20 token balance but not enough native token for gas.
func AutoFundGas(provider *chain.Provider, ch chain.Chain, funder wallet.Wallet, targets []wallet.Wallet, scanResults []FundResult, logger *TxLogger) ([]FundResult, error) {
	key, err := funder.ToECDSA()
	if err != nil {
		return nil, fmt.Errorf("parse funder key: %w", err)
	}

	chainID := big.NewInt(ch.ChainID)
	ctx := context.Background()

	gasPrice, err := provider.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}

	nonce, err := provider.PendingNonceAt(ctx, ch, funder.CommonAddress())
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	// Build list of wallets that need funding
	type fundTarget struct {
		idx    int
		addr   common.Address
		amount *big.Int
	}
	var needsFunding []fundTarget
	for i, r := range scanResults {
		if r.NeedGas && r.GasNeeded != nil && r.GasNeeded.Sign() > 0 {
			needsFunding = append(needsFunding, fundTarget{
				idx:    i,
				addr:   targets[i].CommonAddress(),
				amount: r.GasNeeded,
			})
		}
	}

	if len(needsFunding) == 0 {
		return scanResults, nil
	}

	// Check funder balance
	funderBal, err := provider.BalanceAt(ctx, ch, funder.CommonAddress())
	if err != nil {
		return nil, fmt.Errorf("funder balance: %w", err)
	}

	totalNeeded := new(big.Int)
	for _, t := range needsFunding {
		totalNeeded.Add(totalNeeded, t.amount)
	}
	// Add gas cost for funding txs themselves
	fundingGas := new(big.Int).Mul(gasPrice, big.NewInt(21000*int64(len(needsFunding))))
	totalNeeded.Add(totalNeeded, fundingGas)

	if funderBal.Cmp(totalNeeded) < 0 {
		return nil, fmt.Errorf("funder insufficient: need %s, have %s %s",
			FormatBalance(totalNeeded, 18), FormatBalance(funderBal, 18), ch.Symbol)
	}

	// Send gas to each wallet that needs it
	for _, t := range needsFunding {
		tx := types.NewTransaction(nonce, t.addr, t.amount, 21000, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
		if err != nil {
			scanResults[t.idx].Error = fmt.Sprintf("sign: %s", err)
			continue
		}

		err = provider.SendTransaction(ctx, ch, signedTx)
		if err != nil {
			scanResults[t.idx].Error = fmt.Sprintf("send: %s", err)
			continue
		}

		scanResults[t.idx].GasFunded = t.amount
		scanResults[t.idx].TxHash = signedTx.Hash().Hex()

		// Log the funding transaction
		if logger != nil {
			logger.Log(TxLogEntry{
				Timestamp: time.Now(),
				Chain:     ch.Name,
				Type:      "auto-fund-gas",
				From:      funder.Address,
				To:        targets[t.idx].Address,
				Amount:    FormatBalance(t.amount, 18) + " " + ch.Symbol,
				TxHash:    signedTx.Hash().Hex(),
				Status:    "sent",
			})
		}

		nonce++
		time.Sleep(50 * time.Millisecond)
	}

	return scanResults, nil
}
