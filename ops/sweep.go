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

// SweepResult holds the result of sweeping a single wallet.
type SweepResult struct {
	Index   int
	Address string
	Amount  *big.Int
	TxHash  string
	Error   string
}

// SweepNative scans all wallets and sends any native balance to the destination.
func SweepNative(provider *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, dest common.Address, delay DelayConfig, logger *TxLogger) ([]SweepResult, error) {
	results := make([]SweepResult, len(wallets))
	chainID := big.NewInt(ch.ChainID)

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 15)
	defer rateLimiter.Stop()

	for i, w := range wallets {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			delay.Jitter()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result := SweepResult{Index: idx, Address: wl.Address}
			addr := wl.CommonAddress()

			if addr == dest {
				result.Error = "skip: destination"
				results[idx] = result
				return
			}

			balance, err := provider.BalanceAt(ctx, ch, addr)
			if err != nil {
				result.Error = fmt.Sprintf("balance: %s", err)
				results[idx] = result
				return
			}

			if balance.Sign() == 0 {
				result.Error = "zero balance"
				results[idx] = result
				return
			}

			gasPrice, err := provider.SuggestGasPrice(ctx, ch)
			if err != nil {
				result.Error = fmt.Sprintf("gas price: %s", err)
				results[idx] = result
				return
			}

			gasCost := new(big.Int).Mul(gasPrice, big.NewInt(21000))
			if balance.Cmp(gasCost) <= 0 {
				result.Error = "balance <= gas cost"
				results[idx] = result
				return
			}

			amount := new(big.Int).Sub(balance, gasCost)

			key, err := wl.ToECDSA()
			if err != nil {
				result.Error = fmt.Sprintf("key: %s", err)
				results[idx] = result
				return
			}

			nonce, err := provider.PendingNonceAt(ctx, ch, addr)
			if err != nil {
				result.Error = fmt.Sprintf("nonce: %s", err)
				results[idx] = result
				return
			}

			tx := types.NewTransaction(nonce, dest, amount, 21000, gasPrice, nil)
			signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
			if err != nil {
				result.Error = fmt.Sprintf("sign: %s", err)
				results[idx] = result
				return
			}

			err = provider.SendTransaction(ctx, ch, signedTx)
			if err != nil {
				result.Error = fmt.Sprintf("send: %s", err)
				results[idx] = result
				return
			}

			result.Amount = amount
			result.TxHash = signedTx.Hash().Hex()
			results[idx] = result
		}(i, w)
	}

	wg.Wait()

	if logger != nil {
		logger.LogSweepResults(results, ch.Name, "sweep-native", dest.Hex())
	}

	return results, nil
}

// SweepERC20 scans all wallets and sends any ERC-20 balance to the destination.
func SweepERC20(provider *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, dest common.Address, tokenAddr common.Address, delay DelayConfig, logger *TxLogger) ([]SweepResult, error) {
	results := make([]SweepResult, len(wallets))
	chainID := big.NewInt(ch.ChainID)

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 15)
	defer rateLimiter.Stop()

	for i, w := range wallets {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			delay.Jitter()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result := SweepResult{Index: idx, Address: wl.Address}
			addr := wl.CommonAddress()

			if addr == dest {
				result.Error = "skip: destination"
				results[idx] = result
				return
			}

			tokenBal, err := getERC20Balance(ctx, provider, ch, tokenAddr, addr)
			if err != nil {
				result.Error = fmt.Sprintf("token balance: %s", err)
				results[idx] = result
				return
			}

			if tokenBal.Sign() == 0 {
				result.Error = "zero token balance"
				results[idx] = result
				return
			}

			key, err := wl.ToECDSA()
			if err != nil {
				result.Error = fmt.Sprintf("key: %s", err)
				results[idx] = result
				return
			}

			data := buildTransferData(dest, tokenBal)

			gasPrice, err := provider.SuggestGasPrice(ctx, ch)
			if err != nil {
				result.Error = fmt.Sprintf("gas price: %s", err)
				results[idx] = result
				return
			}

			gasLimit, err := provider.EstimateGas(ctx, ch, ethereum.CallMsg{
				From: addr,
				To:   &tokenAddr,
				Data: data,
			})
			if err != nil {
				gasLimit = 100000
			}

			nonce, err := provider.PendingNonceAt(ctx, ch, addr)
			if err != nil {
				result.Error = fmt.Sprintf("nonce: %s", err)
				results[idx] = result
				return
			}

			tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, data)
			signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
			if err != nil {
				result.Error = fmt.Sprintf("sign: %s", err)
				results[idx] = result
				return
			}

			err = provider.SendTransaction(ctx, ch, signedTx)
			if err != nil {
				result.Error = fmt.Sprintf("send: %s", err)
				results[idx] = result
				return
			}

			result.Amount = tokenBal
			result.TxHash = signedTx.Hash().Hex()
			results[idx] = result
		}(i, w)
	}

	wg.Wait()

	if logger != nil {
		logger.LogSweepResults(results, ch.Name, "sweep-erc20", dest.Hex())
	}

	return results, nil
}
