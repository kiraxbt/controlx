package ops

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"mixer/chain"
	"mixer/wallet"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// BridgeHopResult holds the result of a single cross-chain bridge hop.
type BridgeHopResult struct {
	HopIndex   int
	FromWallet string
	ToWallet   string
	FromIndex  int    // wallet index in group (0-based)
	ToIndex    int    // wallet index in group (0-based)
	FromChain  string
	ToChain    string
	AmountIn   *big.Int
	AmountOut  string // estimated output from bridge
	TxHash     string
	Bridge     string // bridge protocol used (e.g., "stargate", "across")
	ETASeconds int    // estimated bridge time from Li.Fi
	Error      string
}

// ── Li.Fi API Types ──────────────────────────────────────────────────

type lifiQuote struct {
	TransactionRequest struct {
		To       string `json:"to"`
		Data     string `json:"data"`
		Value    string `json:"value"`
		GasLimit string `json:"gasLimit"`
		GasPrice string `json:"gasPrice"`
		ChainID  int64  `json:"chainId"`
	} `json:"transactionRequest"`
	Estimate struct {
		ToAmount         string `json:"toAmount"`
		ExecutionDuration float64 `json:"executionDuration"` // seconds
	} `json:"estimate"`
	Tool    string `json:"tool"`
	ToolDetails struct {
		Name string `json:"name"`
	} `json:"toolDetails"`
}

type lifiError struct {
	Message string `json:"message"`
}

const nativeTokenAddr = "0x0000000000000000000000000000000000000000"

// ── Li.Fi Quote ──────────────────────────────────────────────────────

// getLiFiQuote fetches a bridge quote from Li.Fi with order=FASTEST.
// This prioritizes bridge speed (target <2min) over output amount.
func getLiFiQuote(fromChainID, toChainID int64, fromAddr, toAddr string, amount *big.Int) (*lifiQuote, error) {
	reqURL := fmt.Sprintf(
		"https://li.quest/v1/quote?fromChain=%d&toChain=%d&fromToken=%s&toToken=%s&fromAmount=%s&fromAddress=%s&toAddress=%s&slippage=0.03&order=FASTEST",
		fromChainID, toChainID, nativeTokenAddr, nativeTokenAddr, amount.String(), fromAddr, toAddr,
	)

	MixLog("lifi request: %s", reqURL)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errBody lifiError
		json.NewDecoder(resp.Body).Decode(&errBody)
		if errBody.Message != "" {
			return nil, fmt.Errorf("Li.Fi %d: %s", resp.StatusCode, errBody.Message)
		}
		return nil, fmt.Errorf("Li.Fi: %s", resp.Status)
	}

	var quote lifiQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	etaStr := "unknown"
	if quote.Estimate.ExecutionDuration > 0 {
		etaStr = fmt.Sprintf("~%ds", int(quote.Estimate.ExecutionDuration))
	}

	MixLog("lifi quote: bridge=%s estOut=%s ETA=%s to=%s gasLimit=%s",
		quote.Tool, quote.Estimate.ToAmount, etaStr,
		quote.TransactionRequest.To, quote.TransactionRequest.GasLimit)

	return &quote, nil
}

// ── Helpers ──────────────────────────────────────────────────────────

// parseBigHex parses hex (0x...) or decimal string to *big.Int
func parseBigHex(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" || s == "0x0" || s == "0x" {
		return big.NewInt(0)
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		v, ok := new(big.Int).SetString(s[2:], 16)
		if !ok {
			return big.NewInt(0)
		}
		return v
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return v
}

func chainNameList(chains []chain.Chain) string {
	names := make([]string, len(chains))
	for i, c := range chains {
		names[i] = c.Name
	}
	return strings.Join(names, " → ")
}

// reduceAmount reduces amount by pct percent (e.g. 5 = reduce 5%).
func reduceAmount(amount *big.Int, pct int64) *big.Int {
	reduced := new(big.Int).Mul(amount, big.NewInt(100-pct))
	reduced.Div(reduced, big.NewInt(100))
	return reduced
}

// ── Main Bridge Mix Operation ────────────────────────────────────────

const bridgeGasReserve = uint64(500000)

// BridgeProgressFn is called at key moments during bridge execution.
// Use it to stream live updates to the UI.
type BridgeProgressFn func(msg string)

// BridgeMixOpts holds optional parameters for BridgeMix.
type BridgeMixOpts struct {
	StartIndex int    // starting wallet index in the group (for logging)
	GroupName  string // wallet group name (for logging)
}

// BridgeMix performs cross-chain native-to-native bridge hops through wallets.
//
// chains defines the rotation pattern (e.g., [BSC, Ethereum, Polygon]).
// Hop 0: chains[0] → chains[1], Hop 1: chains[1] → chains[2], etc. (wraps around)
//
// Each hop: wallet on source chain bridges native → wallet on dest chain receives native.
// Since output is always native = gas token, each wallet is self-sustaining.
// onProgress is optional; if non-nil it receives live status strings.
func BridgeMix(prov *chain.Provider, chains []chain.Chain, wallets []wallet.Wallet, delay DelayConfig, logger *TxLogger, onProgress BridgeProgressFn, opts *BridgeMixOpts) ([]BridgeHopResult, error) {
	if len(wallets) < 2 {
		return nil, fmt.Errorf("need at least 2 wallets")
	}
	if len(chains) < 2 {
		return nil, fmt.Errorf("need at least 2 chains for cross-chain bridge")
	}

	startIdx := 0
	groupName := ""
	if opts != nil {
		startIdx = opts.StartIndex
		groupName = opts.GroupName
	}

	numHops := len(wallets) - 1
	results := make([]BridgeHopResult, 0, numHops)

	emit := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		MixLog("%s", msg)
		if onProgress != nil {
			onProgress(msg)
		}
	}

	groupTag := ""
	if groupName != "" {
		groupTag = fmt.Sprintf(" group=%s", groupName)
	}

	emit("=== BridgeMix START ===")
	emit("rotation=%s wallets=%d hops=%d wallet#%d-#%d%s",
		chainNameList(chains), len(wallets), numHops,
		startIdx+1, startIdx+len(wallets), groupTag)

	for i := 0; i < numHops; i++ {
		from := wallets[i]
		to := wallets[i+1]
		fromIdx := startIdx + i
		toIdx := startIdx + i + 1
		fromChain := chains[i%len(chains)]
		toChain := chains[(i+1)%len(chains)]

		// Skip if same chain (shouldn't happen with proper rotation)
		if fromChain.ChainID == toChain.ChainID {
			toChain = chains[(i+2)%len(chains)]
		}

		result := BridgeHopResult{
			HopIndex:   i,
			FromWallet: from.Address,
			ToWallet:   to.Address,
			FromIndex:  fromIdx,
			ToIndex:    toIdx,
			FromChain:  fromChain.Name,
			ToChain:    toChain.Name,
		}

		emit("hop %d: wallet#%d %s on %s → wallet#%d %s on %s",
			i, fromIdx+1, from.ShortAddress(), fromChain.Name,
			toIdx+1, to.ShortAddress(), toChain.Name)

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

		// 1) Get balance on source chain
		balance, err := prov.BalanceAt(ctx, fromChain, from.CommonAddress())
		if err != nil {
			cancel()
			result.Error = fmt.Sprintf("balance on %s: %s", fromChain.Name, err)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}
		emit("hop %d: wallet#%d balance=%s %s on %s",
			i, fromIdx+1, FormatBalance(balance, 18), fromChain.Symbol, fromChain.Name)

		// 2) Get gas price
		gasPrice, err := prov.SuggestGasPrice(ctx, fromChain)
		if err != nil {
			cancel()
			result.Error = fmt.Sprintf("gas price: %s", err)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}
		emit("hop %d: gasPrice=%s Gwei on %s", i, FormatBalance(gasPrice, 9), fromChain.Name)

		// 3) Estimate gas cost with safety margin, compute bridge amount
		gasReserve := new(big.Int).Mul(gasPrice, big.NewInt(int64(bridgeGasReserve)))
		gasReserve.Mul(gasReserve, big.NewInt(130))
		gasReserve.Div(gasReserve, big.NewInt(100))
		if balance.Cmp(gasReserve) <= 0 {
			cancel()
			result.Error = fmt.Sprintf("insufficient: %s %s < gas reserve %s on %s",
				FormatBalance(balance, 18), fromChain.Symbol,
				FormatBalance(gasReserve, 18), fromChain.Name)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}

		bridgeAmount := new(big.Int).Sub(balance, gasReserve)
		emit("hop %d: bridging %s %s via Li.Fi (%s → %s)...",
			i, FormatBalance(bridgeAmount, 18), fromChain.Symbol,
			fromChain.Name, toChain.Name)

		// 4) Get bridge quote from Li.Fi
		quote, err := getLiFiQuote(fromChain.ChainID, toChain.ChainID, from.Address, to.Address, bridgeAmount)
		cancel()
		if err != nil {
			result.Error = fmt.Sprintf("quote: %s", err)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}

		result.Bridge = quote.Tool
		if quote.ToolDetails.Name != "" {
			result.Bridge = quote.ToolDetails.Name
		}
		result.AmountOut = quote.Estimate.ToAmount
		result.ETASeconds = int(quote.Estimate.ExecutionDuration)

		etaStr := "unknown"
		if result.ETASeconds > 0 {
			etaStr = fmt.Sprintf("~%ds", result.ETASeconds)
		}
		emit("hop %d: bridge=%s estOut=%s ETA=%s", i, result.Bridge, FormatBalance(parseBigHex(result.AmountOut), 18), etaStr)

		// 5) Parse tx data from Li.Fi quote
		txTo := common.HexToAddress(quote.TransactionRequest.To)
		txValue := parseBigHex(quote.TransactionRequest.Value)
		txData := common.FromHex(quote.TransactionRequest.Data)
		txGasLimit := parseBigHex(quote.TransactionRequest.GasLimit)
		if txGasLimit.Uint64() == 0 {
			txGasLimit = big.NewInt(int64(bridgeGasReserve))
		}

		// 6) Verify total tx cost fits in balance — adjust if needed
		actualGasCost := new(big.Int).Mul(gasPrice, txGasLimit)
		totalCost := new(big.Int).Add(txValue, actualGasCost)
		if totalCost.Cmp(balance) > 0 {
			overhead := new(big.Int).Sub(txValue, bridgeAmount)
			if overhead.Sign() < 0 {
				overhead = big.NewInt(0)
			}
			safeGas := new(big.Int).Mul(actualGasCost, big.NewInt(130))
			safeGas.Div(safeGas, big.NewInt(100))
			newAmount := new(big.Int).Sub(balance, safeGas)
			newAmount.Sub(newAmount, overhead)
			if newAmount.Sign() <= 0 {
				result.Error = fmt.Sprintf("insufficient after quote: totalCost=%s > balance=%s on %s",
					FormatBalance(totalCost, 18), FormatBalance(balance, 18), fromChain.Name)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			emit("hop %d: adjusting amount %s → %s (gas overhead from bridge contract)",
				i, FormatBalance(bridgeAmount, 18), FormatBalance(newAmount, 18))
			bridgeAmount = newAmount

			quote, err = getLiFiQuote(fromChain.ChainID, toChain.ChainID, from.Address, to.Address, bridgeAmount)
			if err != nil {
				result.Error = fmt.Sprintf("re-quote: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}
			txTo = common.HexToAddress(quote.TransactionRequest.To)
			txValue = parseBigHex(quote.TransactionRequest.Value)
			txData = common.FromHex(quote.TransactionRequest.Data)
			txGasLimit = parseBigHex(quote.TransactionRequest.GasLimit)
			if txGasLimit.Uint64() == 0 {
				txGasLimit = big.NewInt(int64(bridgeGasReserve))
			}
			result.AmountOut = quote.Estimate.ToAmount
			if quote.ToolDetails.Name != "" {
				result.Bridge = quote.ToolDetails.Name
			}
		}

		result.AmountIn = bridgeAmount

		// 7) Send with adaptive retry — on "insufficient funds", reduce amount
		//    by 5% each attempt, re-quote from Li.Fi, and retry.
		key, err := from.ToECDSA()
		if err != nil {
			result.Error = fmt.Sprintf("key: %s", err)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}

		chainID := big.NewInt(fromChain.ChainID)
		var signedTx *types.Transaction
		const maxSendRetries = 6
		var sendErr error

		for attempt := 0; attempt < maxSendRetries; attempt++ {
			if attempt > 0 {
				waitSec := 5 + attempt*3 // 8s, 11s, 14s, 17s, 20s
				emit("hop %d: retry %d/%d — waiting %ds before re-quote...",
					i, attempt, maxSendRetries-1, waitSec)
				time.Sleep(time.Duration(waitSec) * time.Second)

				// ── Adaptive: reduce bridge amount by 5% and re-quote ──
				bridgeAmount = reduceAmount(bridgeAmount, 5)
				emit("hop %d: retry %d — reduced amount to %s %s (-5%%)",
					i, attempt, FormatBalance(bridgeAmount, 18), fromChain.Symbol)

				if bridgeAmount.Sign() <= 0 {
					sendErr = fmt.Errorf("amount reduced to zero after %d retries", attempt)
					break
				}

				// Re-check balance
				rCtx, rCancel := context.WithTimeout(context.Background(), 10*time.Second)
				curBal, rErr := prov.BalanceAt(rCtx, fromChain, from.CommonAddress())
				rCancel()
				if rErr != nil {
					emit("hop %d: retry %d — balance check error: %s", i, attempt, rErr)
					continue
				}
				emit("hop %d: retry %d — current balance=%s %s",
					i, attempt, FormatBalance(curBal, 18), fromChain.Symbol)

				// Re-fetch gas price (it may have changed)
				gCtx, gCancel := context.WithTimeout(context.Background(), 10*time.Second)
				gasPrice, err = prov.SuggestGasPrice(gCtx, fromChain)
				gCancel()
				if err != nil {
					emit("hop %d: retry %d — gas price error: %s", i, attempt, err)
					continue
				}
				emit("hop %d: retry %d — gasPrice=%s Gwei", i, attempt, FormatBalance(gasPrice, 9))

				// Re-quote with reduced amount
				newQuote, qErr := getLiFiQuote(fromChain.ChainID, toChain.ChainID, from.Address, to.Address, bridgeAmount)
				if qErr != nil {
					emit("hop %d: retry %d — re-quote failed: %s", i, attempt, qErr)
					continue
				}

				txTo = common.HexToAddress(newQuote.TransactionRequest.To)
				txValue = parseBigHex(newQuote.TransactionRequest.Value)
				txData = common.FromHex(newQuote.TransactionRequest.Data)
				txGasLimit = parseBigHex(newQuote.TransactionRequest.GasLimit)
				if txGasLimit.Uint64() == 0 {
					txGasLimit = big.NewInt(int64(bridgeGasReserve))
				}
				result.AmountIn = bridgeAmount
				result.AmountOut = newQuote.Estimate.ToAmount
				if newQuote.ToolDetails.Name != "" {
					result.Bridge = newQuote.ToolDetails.Name
				}

				emit("hop %d: retry %d — new quote: bridge=%s value=%s gasLimit=%s estOut=%s",
					i, attempt, result.Bridge,
					FormatBalance(txValue, 18),
					txGasLimit.String(),
					FormatBalance(parseBigHex(result.AmountOut), 18))

				// Verify the new quote fits in balance
				newGasCost := new(big.Int).Mul(gasPrice, txGasLimit)
				newTotal := new(big.Int).Add(txValue, newGasCost)
				if newTotal.Cmp(curBal) > 0 {
					emit("hop %d: retry %d — still too expensive: cost=%s > bal=%s, reducing more...",
						i, attempt, FormatBalance(newTotal, 18), FormatBalance(curBal, 18))
					continue
				}
			}

			// Get fresh nonce
			nonce, nErr := prov.PendingNonceAt(context.Background(), fromChain, from.CommonAddress())
			if nErr != nil {
				emit("hop %d: nonce error: %s", i, nErr)
				continue
			}

			tx := types.NewTransaction(nonce, txTo, txValue, txGasLimit.Uint64(), gasPrice, txData)
			signedTx, err = types.SignTx(tx, types.NewEIP155Signer(chainID), key)
			if err != nil {
				result.Error = fmt.Sprintf("sign: %s", err)
				results = append(results, result)
				emit("hop %d FAIL: %s", i, result.Error)
				return results, nil
			}

			sendErr = prov.SendTransaction(context.Background(), fromChain, signedTx)
			if sendErr == nil {
				if attempt > 0 {
					emit("hop %d: SUCCESS on retry %d with amount=%s %s",
						i, attempt, FormatBalance(bridgeAmount, 18), fromChain.Symbol)
				}
				break
			}

			emit("hop %d: send attempt %d failed: %s", i, attempt, sendErr)

			// Only retry on insufficient funds — other errors are fatal
			if !strings.Contains(sendErr.Error(), "insufficient funds") {
				emit("hop %d: non-recoverable error, stopping retries", i)
				break
			}
		}
		if sendErr != nil {
			result.Error = fmt.Sprintf("send (after %d retries): %s", maxSendRetries, sendErr)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)

			// Log failure
			if logger != nil {
				logger.Log(TxLogEntry{
					Timestamp: time.Now(),
					Chain:     fromChain.Name,
					Type:      "bridge",
					From:      fmt.Sprintf("wallet#%d %s", fromIdx+1, from.Address),
					To:        fmt.Sprintf("wallet#%d %s", toIdx+1, to.Address),
					Amount:    FormatBalance(bridgeAmount, 18) + " " + fromChain.Symbol,
					TxHash:    "",
					Status:    "failed",
					Error:     result.Error,
				})
			}
			return results, nil
		}

		result.TxHash = signedTx.Hash().Hex()
		emit("hop %d: tx sent %s, waiting for confirmation on %s...",
			i, result.TxHash[:18]+"...", fromChain.Name)

		// 8) Wait for tx receipt (on-chain confirmation)
		receiptCtx, receiptCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		receipt, receiptErr := prov.WaitForReceipt(receiptCtx, fromChain, signedTx.Hash())
		receiptCancel()

		if receiptErr != nil {
			result.Error = fmt.Sprintf("receipt timeout on %s: %s", fromChain.Name, receiptErr)
			results = append(results, result)
			emit("hop %d FAIL: %s", i, result.Error)
			return results, nil
		}
		if receipt.Status == 0 {
			result.Error = fmt.Sprintf("tx REVERTED on %s (gas used: %d)", fromChain.Name, receipt.GasUsed)
			results = append(results, result)
			emit("hop %d FAIL: %s tx:%s", i, result.Error, result.TxHash[:18]+"...")
			return results, nil
		}

		results = append(results, result)
		emit("hop %d OK: wallet#%d→#%d %s→%s via %s amount=%s tx:%s gas:%d",
			i, fromIdx+1, toIdx+1,
			fromChain.Name, toChain.Name, result.Bridge,
			FormatBalance(bridgeAmount, 18),
			result.TxHash[:18]+"...", receipt.GasUsed)

		// Detailed log entry
		if logger != nil {
			logger.Log(TxLogEntry{
				Timestamp: time.Now(),
				Chain:     fromChain.Name,
				Type:      "bridge",
				From:      fmt.Sprintf("wallet#%d %s", fromIdx+1, from.Address),
				To:        fmt.Sprintf("wallet#%d %s", toIdx+1, to.Address),
				Amount:    fmt.Sprintf("%s %s → %s (est %s)", FormatBalance(bridgeAmount, 18), fromChain.Symbol, toChain.Name, FormatBalance(parseBigHex(result.AmountOut), 18)),
				TxHash:    result.TxHash,
				Status:    fmt.Sprintf("sent via %s gas:%d", result.Bridge, receipt.GasUsed),
			})
		}

		// 9) Wait for bridge to arrive on destination chain
		if i < numHops-1 {
			// Use ETA from Li.Fi to set smarter timeout: ETA*3 with min 3min, max 10min
			waitTimeout := 3 * time.Minute
			if result.ETASeconds > 0 {
				waitTimeout = time.Duration(result.ETASeconds*3) * time.Second
				if waitTimeout < 2*time.Minute {
					waitTimeout = 2 * time.Minute
				}
				if waitTimeout > 10*time.Minute {
					waitTimeout = 10 * time.Minute
				}
			}
			etaInfo := ""
			if result.ETASeconds > 0 {
				etaInfo = fmt.Sprintf(" (bridge ETA ~%ds, timeout %s)", result.ETASeconds, waitTimeout.Round(time.Second))
			}
			emit("hop %d: waiting for bridge arrival on %s (wallet#%d)%s",
				i, toChain.Name, toIdx+1, etaInfo)
			arrived := waitForBridgeArrival(prov, toChain, to.CommonAddress(), waitTimeout, emit)
			if !arrived {
				emit("hop %d: bridge timeout on %s, continuing anyway...", i, toChain.Name)
			} else {
				emit("hop %d: bridge confirmed on %s (wallet#%d)", i, toChain.Name, toIdx+1)
			}
			delay.Wait()
		}
	}

	if logger != nil {
		logger.Flush()
	}

	emit("=== BridgeMix COMPLETE: %d/%d hops%s ===", len(results), numHops, groupTag)
	return results, nil
}

// emitFn is the type for the progress callback used by waitForBridgeArrival.
type emitFn func(format string, args ...interface{})

// waitForBridgeArrival polls destination wallet balance until it increases
// and stays elevated for multiple consecutive checks (cross-node confirmation).
func waitForBridgeArrival(prov *chain.Provider, ch chain.Chain, addr common.Address, timeout time.Duration, emit emitFn) bool {
	const requiredConfirms = 3
	const pollInterval = 10 * time.Second

	ctx0, cancel0 := context.WithTimeout(context.Background(), 10*time.Second)
	initialBal, _ := prov.BalanceAt(ctx0, ch, addr)
	cancel0()
	if initialBal == nil {
		initialBal = big.NewInt(0)
	}

	emit("waiting: initial balance on %s = %s (need %d confirms)", ch.Name, FormatBalance(initialBal, 18), requiredConfirms)

	confirms := 0
	pollCount := 0
	var lastBal *big.Int
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		pollCount++

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		bal, err := prov.BalanceAt(ctx, ch, addr)
		cancel()

		if err != nil {
			emit("waiting: balance check error on %s: %s", ch.Name, err)
			confirms = 0
			continue
		}

		remaining := time.Until(deadline).Round(time.Second)

		if bal.Cmp(initialBal) > 0 {
			confirms++
			lastBal = bal
			emit("waiting: balance %s on %s (confirm %d/%d)", FormatBalance(bal, 18), ch.Name, confirms, requiredConfirms)
			if confirms >= requiredConfirms {
				emit("waiting: bridge confirmed! %s → %s on %s",
					FormatBalance(initialBal, 18), FormatBalance(lastBal, 18), ch.Name)
				return true
			}
		} else {
			if confirms > 0 {
				emit("waiting: balance dropped to %s on %s, resetting", FormatBalance(bal, 18), ch.Name)
			}
			confirms = 0
			// Heartbeat every 3rd poll so user knows we're alive
			if pollCount%3 == 0 {
				emit("waiting: still %s on %s, no change (%s left)", FormatBalance(bal, 18), ch.Name, remaining)
			}
		}
	}

	emit("waiting: timeout after %s on %s", timeout, ch.Name)
	return false
}
