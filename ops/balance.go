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
)

// ERC20 balanceOf(address) selector
var balanceOfSelector = common.Hex2Bytes("70a08231")

// WalletBalance holds balance info for a single wallet.
type WalletBalance struct {
	Index         int
	Address       string
	NativeBalance *big.Int
	TokenBalance  *big.Int
	Error         string
}

// CheckBalances checks native token balance for all wallets concurrently.
func CheckBalances(provider *chain.Provider, ch chain.Chain, wallets []wallet.Wallet, tokenAddr string) ([]WalletBalance, error) {
	results := make([]WalletBalance, len(wallets))
	var hasToken bool
	var tokenAddress common.Address
	if tokenAddr != "" {
		hasToken = true
		tokenAddress = common.HexToAddress(tokenAddr)
	}

	sem := make(chan struct{}, 50)
	var wg sync.WaitGroup
	rateLimiter := time.NewTicker(time.Millisecond * 10) // ~100 req/s
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
			result := WalletBalance{
				Index:   idx,
				Address: wl.Address,
			}

			balance, err := provider.BalanceAt(ctx, ch, addr)
			if err != nil {
				result.Error = err.Error()
				results[idx] = result
				return
			}
			result.NativeBalance = balance

			if hasToken {
				tokenBal, err := getERC20Balance(ctx, provider, ch, tokenAddress, addr)
				if err != nil {
					result.Error = fmt.Sprintf("token: %s", err.Error())
				} else {
					result.TokenBalance = tokenBal
				}
			}

			results[idx] = result
		}(i, w)
	}

	wg.Wait()
	return results, nil
}

// getERC20Balance calls balanceOf on an ERC-20 contract.
func getERC20Balance(ctx context.Context, provider *chain.Provider, ch chain.Chain, token, holder common.Address) (*big.Int, error) {
	// Build calldata: balanceOf(address)
	data := make([]byte, 36)
	copy(data[:4], balanceOfSelector)
	copy(data[4:], common.LeftPadBytes(holder.Bytes(), 32))

	msg := ethereum.CallMsg{
		To:   &token,
		Data: data,
	}

	result, err := provider.CallContract(ctx, ch, msg)
	if err != nil {
		return nil, err
	}
	if len(result) < 32 {
		return big.NewInt(0), nil
	}

	return new(big.Int).SetBytes(result[:32]), nil
}

// FormatBalance formats a wei balance to a human-readable string with 4 decimal places.
func FormatBalance(wei *big.Int, decimals int) string {
	if wei == nil {
		return "0.0000"
	}
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole := new(big.Int).Div(wei, divisor)
	remainder := new(big.Int).Mod(wei, divisor)

	// Get fractional part as string, padded
	fracStr := fmt.Sprintf("%0*s", decimals, remainder.String())
	if len(fracStr) > 4 {
		fracStr = fracStr[:4]
	}
	return fmt.Sprintf("%s.%s", whole.String(), fracStr)
}
