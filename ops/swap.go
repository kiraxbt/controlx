package ops

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"controlx/chain"
	"controlx/wallet"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// SwapHopResult holds the result of a single DEX swap hop.
type SwapHopResult struct {
	HopIndex   int
	FromWallet string
	ToWallet   string
	TokenIn    string // "native" or token symbol
	TokenOut   string // "native" or token symbol
	AmountIn   *big.Int
	AmountOut  *big.Int
	ApproveTx  string
	SwapTx     string
	Error      string
}

// Gas limits for DEX operations
const (
	gasLimitSwap    = uint64(300000)
	gasLimitApprove = uint64(60000)
)

// ABI selectors
var (
	selGetAmountsOut         = common.Hex2Bytes("d06ca61f")
	selApprove               = common.Hex2Bytes("095ea7b3")
	selSwapExactETHForTokens = common.Hex2Bytes("7ff36ab5")
	selSwapExactTokensForETH = common.Hex2Bytes("18cbafe5")
	selBalanceOf             = common.Hex2Bytes("70a08231")
)

// ── ABI Encoding Helpers ─────────────────────────────────────────────

func encUint256(v *big.Int) []byte {
	if v == nil {
		return make([]byte, 32)
	}
	return common.LeftPadBytes(v.Bytes(), 32)
}

func encAddress(addr common.Address) []byte {
	return common.LeftPadBytes(addr.Bytes(), 32)
}

func encPath(addrs []common.Address) []byte {
	data := encUint256(big.NewInt(int64(len(addrs))))
	for _, a := range addrs {
		data = append(data, encAddress(a)...)
	}
	return data
}

// ── Contract View Calls ──────────────────────────────────────────────

// callGetAmountsOut calls router.getAmountsOut (view, no gas).
func callGetAmountsOut(ctx context.Context, prov *chain.Provider, ch chain.Chain, router common.Address, amountIn *big.Int, path []common.Address) (*big.Int, error) {
	data := append([]byte(nil), selGetAmountsOut...)
	data = append(data, encUint256(amountIn)...)
	data = append(data, encUint256(big.NewInt(64))...) // offset to path
	data = append(data, encPath(path)...)

	msg := ethereum.CallMsg{To: &router, Data: data}
	result, err := prov.CallContract(ctx, ch, msg)
	if err != nil {
		return nil, err
	}
	// Response: offset(32) + at offset: length(32) + amounts[0..n]
	// Last 32 bytes = output amount
	if len(result) < 96 {
		return nil, fmt.Errorf("response too short (%d bytes)", len(result))
	}
	return new(big.Int).SetBytes(result[len(result)-32:]), nil
}

// callBalanceOf calls token.balanceOf(addr) (view, no gas).
func callBalanceOf(ctx context.Context, prov *chain.Provider, ch chain.Chain, token, addr common.Address) (*big.Int, error) {
	data := append([]byte(nil), selBalanceOf...)
	data = append(data, encAddress(addr)...)

	msg := ethereum.CallMsg{To: &token, Data: data}
	result, err := prov.CallContract(ctx, ch, msg)
	if err != nil {
		return nil, err
	}
	if len(result) < 32 {
		return big.NewInt(0), nil
	}
	return new(big.Int).SetBytes(result[:32]), nil
}

// ── Transaction Senders ──────────────────────────────────────────────

// signAndSend signs and sends a transaction.
func signAndSend(ctx context.Context, prov *chain.Provider, ch chain.Chain, w wallet.Wallet, to common.Address, value *big.Int, gasLimit uint64, txData []byte) (string, error) {
	gasPrice, err := prov.SuggestGasPrice(ctx, ch)
	if err != nil {
		return "", fmt.Errorf("gas price: %w", err)
	}
	nonce, err := prov.PendingNonceAt(ctx, ch, w.CommonAddress())
	if err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	key, err := w.ToECDSA()
	if err != nil {
		return "", fmt.Errorf("key: %w", err)
	}

	tx := types.NewTransaction(nonce, to, value, gasLimit, gasPrice, txData)
	signed, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(ch.ChainID)), key)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	err = prov.SendTransaction(ctx, ch, signed)
	if err != nil {
		return "", fmt.Errorf("send: %w", err)
	}
	return signed.Hash().Hex(), nil
}

// sendApproveTx sends approve(spender, amount) on token contract.
func sendApproveTx(ctx context.Context, prov *chain.Provider, ch chain.Chain, w wallet.Wallet, token, spender common.Address, amount *big.Int) (string, error) {
	data := append([]byte(nil), selApprove...)
	data = append(data, encAddress(spender)...)
	data = append(data, encUint256(amount)...)
	return signAndSend(ctx, prov, ch, w, token, big.NewInt(0), gasLimitApprove, data)
}

// sendSwapETHForTokens sends swapExactETHForTokens tx.
// value = native ETH to swap, output tokens go to 'to' address.
func sendSwapETHForTokens(ctx context.Context, prov *chain.Provider, ch chain.Chain, w wallet.Wallet, router common.Address, value, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (string, error) {
	// ABI: amountOutMin, offset(0x80=128), to, deadline, path[]
	data := append([]byte(nil), selSwapExactETHForTokens...)
	data = append(data, encUint256(amountOutMin)...)
	data = append(data, encUint256(big.NewInt(128))...) // offset: 4 head slots × 32
	data = append(data, encAddress(to)...)
	data = append(data, encUint256(deadline)...)
	data = append(data, encPath(path)...)

	return signAndSend(ctx, prov, ch, w, router, value, gasLimitSwap, data)
}

// sendSwapTokensForETH sends swapExactTokensForETH tx.
// Token input, native ETH output goes to 'to' address.
func sendSwapTokensForETH(ctx context.Context, prov *chain.Provider, ch chain.Chain, w wallet.Wallet, router common.Address, amountIn, amountOutMin *big.Int, path []common.Address, to common.Address, deadline *big.Int) (string, error) {
	// ABI: amountIn, amountOutMin, offset(0xa0=160), to, deadline, path[]
	data := append([]byte(nil), selSwapExactTokensForETH...)
	data = append(data, encUint256(amountIn)...)
	data = append(data, encUint256(amountOutMin)...)
	data = append(data, encUint256(big.NewInt(160))...) // offset: 5 head slots × 32
	data = append(data, encAddress(to)...)
	data = append(data, encUint256(deadline)...)
	data = append(data, encPath(path)...)

	return signAndSend(ctx, prov, ch, w, router, big.NewInt(0), gasLimitSwap, data)
}

// ── Main DEX Mix Operation ───────────────────────────────────────────

// SwapProgressFn is called at key moments during DEX swap execution.
type SwapProgressFn func(msg string)

// DexMix performs DEX chain-hop swaps through multiple wallets.
//
// Flow: Wallet1 → DEX(native→token, to=Wallet2) → Wallet2 → DEX(token→native, to=Wallet3) → ...
//
//	Even hops: native → token via swapExactETHForTokens (no approve needed)
//	Odd hops:  token → native via approve + swapExactTokensForETH
//
// The DEX router's 'to' parameter sends output directly to the next wallet.
// Each intermediate wallet (odd hops) needs native gas for approve+swap tx fees.
func DexMix(prov *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, tokenAddr string, slippageBps int, delay DelayConfig, logger *TxLogger, onProgress SwapProgressFn) ([]SwapHopResult, error) {
	if len(wallets) < 2 {
		return nil, fmt.Errorf("need at least 2 wallets")
	}

	router, ok := chain.DexRouters[ch.Name]
	if !ok {
		return nil, fmt.Errorf("no DEX router for %s (supported: Ethereum, BSC, Polygon, Arbitrum, Avalanche, Fantom)", ch.Name)
	}
	wnative, ok := chain.WrappedNative[ch.Name]
	if !ok {
		return nil, fmt.Errorf("no wrapped native for %s", ch.Name)
	}

	routerAddr := common.HexToAddress(router.Address)
	wnativeAddr := common.HexToAddress(wnative)
	tokenAddress := common.HexToAddress(tokenAddr)

	emit := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		MixLog("%s", msg)
		if onProgress != nil {
			onProgress(msg)
		}
	}

	numHops := len(wallets) - 1
	results := make([]SwapHopResult, 0, numHops)

	emit("=== DexMix START ===")
	emit("chain=%s router=%s(%s) token=%s wallets=%d hops=%d slippage=%dbps",
		ch.Name, router.Name, router.Address, tokenAddr, len(wallets), numHops, slippageBps)

	for i := 0; i < numHops; i++ {
		from := wallets[i]
		to := wallets[i+1]

		isNativeIn := (i % 2 == 0) // even=native→token, odd=token→native

		result := SwapHopResult{
			HopIndex:   i,
			FromWallet: from.Address,
			ToWallet:   to.Address,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		if isNativeIn {
			// ── Native → Token (swapExactETHForTokens) ──
			result.TokenIn = ch.Symbol
			result.TokenOut = "TOKEN"
			path := []common.Address{wnativeAddr, tokenAddress}

			// Get native balance
			balance, err := prov.BalanceAt(ctx, ch, from.CommonAddress())
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("balance: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			emit("hop %d: native→token from=%s bal=%s %s", i, from.ShortAddress(), FormatBalance(balance, 18), ch.Symbol)

			// Reserve gas for swap tx
			gasPrice, err := prov.SuggestGasPrice(ctx, ch)
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("gas price: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimitSwap)))
			if balance.Cmp(gasCost) <= 0 {
				cancel()
				result.Error = fmt.Sprintf("insufficient: %s %s < gas %s",
					FormatBalance(balance, 18), ch.Symbol, FormatBalance(gasCost, 18))
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			swapValue := new(big.Int).Sub(balance, gasCost)
			result.AmountIn = swapValue

			// Get quote
			amountOut, err := callGetAmountsOut(ctx, prov, ch, routerAddr, swapValue, path)
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("getAmountsOut: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			// Apply slippage
			amountOutMin := new(big.Int).Mul(amountOut, big.NewInt(int64(10000-slippageBps)))
			amountOutMin.Div(amountOutMin, big.NewInt(10000))
			result.AmountOut = amountOut
			emit("hop %d: quote amountOut=%s minOut=%s", i, amountOut.String(), amountOutMin.String())

			deadline := big.NewInt(time.Now().Unix() + 300)

			// Execute swap → output tokens go to next wallet
			txHash, err := sendSwapETHForTokens(ctx, prov, ch, from, routerAddr, swapValue, amountOutMin, path, to.CommonAddress(), deadline)
			cancel()
			if err != nil {
				result.Error = fmt.Sprintf("swap: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			result.SwapTx = txHash
			emit("hop %d: swap tx sent %s, waiting for receipt...", i, txHash)

			// Wait for swap receipt
			receiptCtx, receiptCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			receipt, receiptErr := prov.WaitForReceipt(receiptCtx, ch, common.HexToHash(txHash))
			receiptCancel()
			if receiptErr != nil {
				result.Error = fmt.Sprintf("swap receipt: %s", receiptErr)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			if receipt.Status == 0 {
				result.Error = fmt.Sprintf("swap REVERTED (gas used: %d)", receipt.GasUsed)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			emit("hop %d OK: swapTx=%s (gas: %d)", i, txHash, receipt.GasUsed)

		} else {
			// ── Token → Native (approve + swapExactTokensForETH) ──
			result.TokenIn = "TOKEN"
			result.TokenOut = ch.Symbol
			path := []common.Address{tokenAddress, wnativeAddr}

			// Get token balance at this wallet
			tokenBal, err := callBalanceOf(ctx, prov, ch, tokenAddress, from.CommonAddress())
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("token balance: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			if tokenBal.Sign() == 0 {
				cancel()
				result.Error = "zero token balance"
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			result.AmountIn = tokenBal
			emit("hop %d: token→native from=%s tokenBal=%s", i, from.ShortAddress(), tokenBal.String())

			// Check native balance for gas (approve + swap = 2 txs)
			nativeBal, err := prov.BalanceAt(ctx, ch, from.CommonAddress())
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("native balance: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			gasPrice, err := prov.SuggestGasPrice(ctx, ch)
			if err != nil {
				cancel()
				result.Error = fmt.Sprintf("gas price: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			needGas := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimitApprove+gasLimitSwap)))
			if nativeBal.Cmp(needGas) < 0 {
				cancel()
				result.Error = fmt.Sprintf("need gas: have %s, need %s %s (run auto-fund first)",
					FormatBalance(nativeBal, 18), FormatBalance(needGas, 18), ch.Symbol)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			// 1) Approve router to spend tokens
			emit("hop %d: approving router for %s tokens...", i, tokenBal.String())
			approveTx, err := sendApproveTx(ctx, prov, ch, from, tokenAddress, routerAddr, tokenBal)
			cancel()
			if err != nil {
				result.Error = fmt.Sprintf("approve: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			result.ApproveTx = approveTx
			emit("hop %d: approveTx=%s, waiting for receipt...", i, approveTx)

			// Wait for approve receipt instead of hardcoded sleep
			approveCtx, approveCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			approveReceipt, approveErr := prov.WaitForReceipt(approveCtx, ch, common.HexToHash(approveTx))
			approveCancel()
			if approveErr != nil {
				result.Error = fmt.Sprintf("approve receipt: %s", approveErr)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			if approveReceipt.Status == 0 {
				result.Error = fmt.Sprintf("approve REVERTED (gas used: %d)", approveReceipt.GasUsed)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			emit("hop %d: approve confirmed (gas: %d)", i, approveReceipt.GasUsed)

			// 2) Get quote
			quoteCtx, quoteCancel := context.WithTimeout(context.Background(), 60*time.Second)
			amountOut, err := callGetAmountsOut(quoteCtx, prov, ch, routerAddr, tokenBal, path)
			if err != nil {
				quoteCancel()
				result.Error = fmt.Sprintf("getAmountsOut: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			amountOutMin := new(big.Int).Mul(amountOut, big.NewInt(int64(10000-slippageBps)))
			amountOutMin.Div(amountOutMin, big.NewInt(10000))
			result.AmountOut = amountOut
			emit("hop %d: quote amountOut=%s minOut=%s", i, amountOut.String(), amountOutMin.String())

			deadline := big.NewInt(time.Now().Unix() + 300)

			// 3) Execute swap → output native goes to next wallet
			txHash, err := sendSwapTokensForETH(quoteCtx, prov, ch, from, routerAddr, tokenBal, amountOutMin, path, to.CommonAddress(), deadline)
			quoteCancel()
			if err != nil {
				result.Error = fmt.Sprintf("swap: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			result.SwapTx = txHash
			emit("hop %d: swap tx sent %s, waiting for receipt...", i, txHash)

			// Wait for swap receipt
			swapCtx, swapCancel := context.WithTimeout(context.Background(), 3*time.Minute)
			swapReceipt, swapErr := prov.WaitForReceipt(swapCtx, ch, common.HexToHash(txHash))
			swapCancel()
			if swapErr != nil {
				result.Error = fmt.Sprintf("swap receipt: %s", swapErr)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			if swapReceipt.Status == 0 {
				result.Error = fmt.Sprintf("swap REVERTED (gas used: %d)", swapReceipt.GasUsed)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			emit("hop %d OK: swapTx=%s (gas: %d)", i, txHash, swapReceipt.GasUsed)
		}

		results = append(results, result)

		// Log to tx logger
		if logger != nil {
			logger.Log(TxLogEntry{
				Timestamp: time.Now(),
				Chain:     ch.Name,
				Type:      "dexmix",
				From:      from.Address,
				To:        to.Address,
				Amount:    result.AmountIn.String(),
				TxHash:    result.SwapTx,
				Status:    "sent",
			})
		}

		// Delay between hops
		if i < numHops-1 {
			emit("hop %d: waiting before next hop...", i)
			delay.Wait()
		}
	}

	if logger != nil {
		logger.Flush()
	}

	emit("=== DexMix COMPLETE: %d hops ===", len(results))
	return results, nil
}
