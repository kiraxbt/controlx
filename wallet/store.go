package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/scrypt"
)

// WalletGroup represents metadata for a named group of wallets.
type WalletGroup struct {
	Name      string    `json:"name"`
	File      string    `json:"file"`
	Count     int       `json:"count"`
	CreatedAt time.Time `json:"created"`
}

// GroupIndex stores the list of all wallet groups.
type GroupIndex struct {
	Groups []WalletGroup `json:"groups"`
}

// LoadGroupIndex loads the group index from disk, or returns an empty index.
func LoadGroupIndex(indexFile string) (*GroupIndex, error) {
	data, err := os.ReadFile(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &GroupIndex{}, nil
		}
		return nil, fmt.Errorf("read group index: %w", err)
	}
	var idx GroupIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("unmarshal group index: %w", err)
	}
	return &idx, nil
}

// SaveGroupIndex writes the group index to disk.
func SaveGroupIndex(indexFile string, idx *GroupIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal group index: %w", err)
	}
	return os.WriteFile(indexFile, data, 0600)
}

// Add appends a new group to the index and returns it.
func (idx *GroupIndex) Add(name string, count int) WalletGroup {
	g := WalletGroup{
		Name:      name,
		File:      "wallets_" + name + ".json",
		Count:     count,
		CreatedAt: time.Now(),
	}
	idx.Groups = append(idx.Groups, g)
	return g
}

// NextName returns an auto-generated group name like "group-001", "group-002", etc.
func (idx *GroupIndex) NextName() string {
	max := 0
	for _, g := range idx.Groups {
		var n int
		if _, err := fmt.Sscanf(g.Name, "group-%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("group-%03d", max+1)
}

// EncryptedStore represents the on-disk format of encrypted wallets.
type EncryptedStore struct {
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

// deriveKey derives a 32-byte AES key from password and salt using scrypt.
func deriveKey(password string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(password), salt, 1<<15, 8, 1, 32)
}

// SaveWallets encrypts and saves wallets to a JSON file.
func SaveWallets(wallets []Wallet, filename, password string) error {
	plaintext, err := json.Marshal(wallets)
	if err != nil {
		return fmt.Errorf("marshal wallets: %w", err)
	}

	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generate salt: %w", err)
	}

	key, err := deriveKey(password, salt)
	if err != nil {
		return fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	store := EncryptedStore{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}

	return os.WriteFile(filename, data, 0600)
}

// LoadWallets decrypts and loads wallets from a JSON file.
func LoadWallets(filename, password string) ([]Wallet, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var store EncryptedStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("unmarshal store: %w", err)
	}

	key, err := deriveKey(password, store.Salt)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, store.Nonce, store.Ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt (wrong password?): %w", err)
	}

	var wallets []Wallet
	if err := json.Unmarshal(plaintext, &wallets); err != nil {
		return nil, fmt.Errorf("unmarshal wallets: %w", err)
	}

	return wallets, nil
}

// WalletFileExists checks if the wallet file exists.
func WalletFileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// Fingerprint returns a short hash of the wallet file for identification.
func Fingerprint(filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "unknown"
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:4])
}
