package wallet

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Wallet represents a single EVM wallet with its private key and address.
type Wallet struct {
	PrivateKey string `json:"private_key"`
	Address    string `json:"address"`
}

// GenerateWallets creates n new EVM wallets.
func GenerateWallets(n int) ([]Wallet, error) {
	wallets := make([]Wallet, 0, n)
	for i := 0; i < n; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("generate key %d: %w", i, err)
		}
		wallets = append(wallets, walletFromKey(key))
	}
	return wallets, nil
}

// walletFromKey converts a private key to a Wallet struct.
func walletFromKey(key *ecdsa.PrivateKey) Wallet {
	privBytes := crypto.FromECDSA(key)
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return Wallet{
		PrivateKey: fmt.Sprintf("%x", privBytes),
		Address:    addr.Hex(),
	}
}

// ToECDSA converts the hex private key string back to an ECDSA private key.
func (w *Wallet) ToECDSA() (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(w.PrivateKey)
}

// CommonAddress returns the wallet address as a common.Address.
func (w *Wallet) CommonAddress() common.Address {
	return common.HexToAddress(w.Address)
}

// ShortAddress returns a shortened version of the address (0x1234...abcd).
func (w *Wallet) ShortAddress() string {
	if len(w.Address) < 12 {
		return w.Address
	}
	return w.Address[:6] + "..." + w.Address[len(w.Address)-4:]
}
