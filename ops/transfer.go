package ops

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"mixer/chain"
	"mixer/wallet"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ERC20 transfer(address,uint256) selector
var transferSelector = common.Hex2Bytes("a9059cbb")

// TxResult holds the result of a transaction attempt.
type TxResult struct {
	FromIndex int
	From      string
	To        string
	TxHash    string
	Error     string
}

// Distribute sends native tokens from one wallet to multiple wallets.
// Supports random delay between transactions for anti-detection.
func Distribute(provider *chain.Provider, ch chain.Chain, from wallet.Wallet, to []wallet.Wallet, amountWei *big.Int, delay DelayConfig, logger *TxLogger) ([]TxResult, error) {
	key, err := from.ToECDSA()
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	results := make([]TxResult, len(to))
	chainID := big.NewInt(ch.ChainID)

	ctx := context.Background()
	gasPrice, err := provider.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}

	nonce, err := provider.PendingNonceAt(ctx, ch, from.CommonAddress())
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	for i, w := range to {
		toAddr := w.CommonAddress()
		tx := types.NewTransaction(nonce, toAddr, amountWei, 21000, gasPrice, nil)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
		if err != nil {
			results[i] = TxResult{FromIndex: 0, From: from.Address, To: w.Address, Error: err.Error()}
			continue
		}

		err = provider.SendTransaction(ctx, ch, signedTx)
		if err != nil {
			results[i] = TxResult{FromIndex: 0, From: from.Address, To: w.Address, Error: err.Error()}
			continue
		}

		results[i] = TxResult{
			FromIndex: 0,
			From:      from.Address,
			To:        w.Address,
			TxHash:    signedTx.Hash().Hex(),
		}
		nonce++

		// Random delay between transactions
		delay.Wait()
	}

	// Log all results
	if logger != nil {
		logger.LogTxResults(results, ch.Name, "distribute")
	}

	return results, nil
}

// Collect sends native tokens from multiple wallets to one destination.
// Supports random delay and transaction logging.
func Collect(provider *chain.Provider, ch chain.Chain, from []wallet.Wallet, to common.Address, leaveGasWei *big.Int, delay DelayConfig, logger *TxLogger) ([]TxResult, error) {
	results := make([]TxResult, len(from))
	chainID := big.NewInt(ch.ChainID)

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 15)
	defer rateLimiter.Stop()

	for i, w := range from {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			// Random jitter per goroutine
			delay.Jitter()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result := TxResult{FromIndex: idx, From: wl.Address, To: to.Hex()}

			key, err := wl.ToECDSA()
			if err != nil {
				result.Error = fmt.Sprintf("parse key: %s", err)
				results[idx] = result
				return
			}

			balance, err := provider.BalanceAt(ctx, ch, wl.CommonAddress())
			if err != nil {
				result.Error = fmt.Sprintf("get balance: %s", err)
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
			totalDeduct := new(big.Int).Add(gasCost, leaveGasWei)

			if balance.Cmp(totalDeduct) <= 0 {
				result.Error = "insufficient balance"
				results[idx] = result
				return
			}

			amount := new(big.Int).Sub(balance, totalDeduct)

			nonce, err := provider.PendingNonceAt(ctx, ch, wl.CommonAddress())
			if err != nil {
				result.Error = fmt.Sprintf("nonce: %s", err)
				results[idx] = result
				return
			}

			tx := types.NewTransaction(nonce, to, amount, 21000, gasPrice, nil)
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

			result.TxHash = signedTx.Hash().Hex()
			results[idx] = result
		}(i, w)
	}

	wg.Wait()

	if logger != nil {
		logger.LogTxResults(results, ch.Name, "collect")
	}

	return results, nil
}

// DistributeERC20 sends ERC-20 tokens from one wallet to multiple wallets.
func DistributeERC20(provider *chain.Provider, ch chain.Chain, from wallet.Wallet, to []wallet.Wallet, tokenAddr common.Address, amountWei *big.Int, delay DelayConfig, logger *TxLogger) ([]TxResult, error) {
	key, err := from.ToECDSA()
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	results := make([]TxResult, len(to))
	chainID := big.NewInt(ch.ChainID)
	ctx := context.Background()

	gasPrice, err := provider.SuggestGasPrice(ctx, ch)
	if err != nil {
		return nil, fmt.Errorf("get gas price: %w", err)
	}

	nonce, err := provider.PendingNonceAt(ctx, ch, from.CommonAddress())
	if err != nil {
		return nil, fmt.Errorf("get nonce: %w", err)
	}

	for i, w := range to {
		toAddr := w.CommonAddress()
		data := buildTransferData(toAddr, amountWei)

		gasLimit, err := provider.EstimateGas(ctx, ch, ethereum.CallMsg{
			From: from.CommonAddress(),
			To:   &tokenAddr,
			Data: data,
		})
		if err != nil {
			gasLimit = 100000 // fallback
		}

		tx := types.NewTransaction(nonce, tokenAddr, big.NewInt(0), gasLimit, gasPrice, data)
		signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), key)
		if err != nil {
			results[i] = TxResult{FromIndex: 0, From: from.Address, To: w.Address, Error: err.Error()}
			continue
		}

		err = provider.SendTransaction(ctx, ch, signedTx)
		if err != nil {
			results[i] = TxResult{FromIndex: 0, From: from.Address, To: w.Address, Error: err.Error()}
			continue
		}

		results[i] = TxResult{
			FromIndex: 0,
			From:      from.Address,
			To:        w.Address,
			TxHash:    signedTx.Hash().Hex(),
		}
		nonce++
		delay.Wait()
	}

	if logger != nil {
		logger.LogTxResults(results, ch.Name, "distribute-erc20")
	}

	return results, nil
}

// CollectERC20 sends ERC-20 tokens from multiple wallets to one destination.
func CollectERC20(provider *chain.Provider, ch chain.Chain, from []wallet.Wallet, to common.Address, tokenAddr common.Address, amountWei *big.Int, delay DelayConfig, logger *TxLogger) ([]TxResult, error) {
	results := make([]TxResult, len(from))
	chainID := big.NewInt(ch.ChainID)

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 15)
	defer rateLimiter.Stop()

	for i, w := range from {
		wg.Add(1)
		go func(idx int, wl wallet.Wallet) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			<-rateLimiter.C

			delay.Jitter()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result := TxResult{FromIndex: idx, From: wl.Address, To: to.Hex()}

			key, err := wl.ToECDSA()
			if err != nil {
				result.Error = fmt.Sprintf("parse key: %s", err)
				results[idx] = result
				return
			}

			data := buildTransferData(to, amountWei)

			gasPrice, err := provider.SuggestGasPrice(ctx, ch)
			if err != nil {
				result.Error = fmt.Sprintf("gas price: %s", err)
				results[idx] = result
				return
			}

			gasLimit, err := provider.EstimateGas(ctx, ch, ethereum.CallMsg{
				From: wl.CommonAddress(),
				To:   &tokenAddr,
				Data: data,
			})
			if err != nil {
				gasLimit = 100000
			}

			nonce, err := provider.PendingNonceAt(ctx, ch, wl.CommonAddress())
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

			result.TxHash = signedTx.Hash().Hex()
			results[idx] = result
		}(i, w)
	}

	wg.Wait()

	if logger != nil {
		logger.LogTxResults(results, ch.Name, "collect-erc20")
	}

	return results, nil
}

// buildTransferData builds the calldata for ERC-20 transfer(address,uint256).
func buildTransferData(to common.Address, amount *big.Int) []byte {
	data := make([]byte, 68)
	copy(data[:4], transferSelector)
	copy(data[4:36], common.LeftPadBytes(to.Bytes(), 32))
	copy(data[36:68], common.LeftPadBytes(amount.Bytes(), 32))
	return data
}
