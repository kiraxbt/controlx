package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// BackupData holds all wallet groups and metadata for encrypted backup.
type BackupData struct {
	Version   int           `json:"version"`
	CreatedAt time.Time     `json:"created_at"`
	Groups    []WalletGroup `json:"groups"`
	Files     []BackupFile  `json:"files"`
	Labels    WalletLabels  `json:"labels,omitempty"`
}

// BackupFile holds the encrypted content of a single wallet file.
type BackupFile struct {
	Filename string `json:"filename"`
	Content  []byte `json:"content"`
}

// CreateBackup creates an encrypted backup of all wallet groups.
func CreateBackup(indexFile, password string, groups []WalletGroup) ([]byte, error) {
	backup := BackupData{
		Version:   1,
		CreatedAt: time.Now(),
		Groups:    groups,
	}

	// Read each wallet file
	for _, g := range groups {
		data, err := os.ReadFile(g.File)
		if err != nil {
			continue // skip missing files
		}
		backup.Files = append(backup.Files, BackupFile{
			Filename: g.File,
			Content:  data,
		})
	}

	// Load and include labels
	labels, err := LoadLabels("wallet_labels.json")
	if err == nil {
		backup.Labels = *labels
	}

	// Serialize
	plaintext, err := json.Marshal(backup)
	if err != nil {
		return nil, fmt.Errorf("marshal backup: %w", err)
	}

	// Encrypt
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key, err := deriveKey(password, salt)
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

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	store := EncryptedStore{
		Salt:       salt,
		Nonce:      nonce,
		Ciphertext: ciphertext,
	}

	return json.MarshalIndent(store, "", "  ")
}

// RestoreBackup decrypts a backup and restores all wallet files.
func RestoreBackup(backupFile, password string) (*BackupData, error) {
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return nil, fmt.Errorf("read backup: %w", err)
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

	var backup BackupData
	if err := json.Unmarshal(plaintext, &backup); err != nil {
		return nil, fmt.Errorf("unmarshal backup: %w", err)
	}

	return &backup, nil
}

// WriteRestored writes all restored files to disk.
func WriteRestored(backup *BackupData, indexFile string) error {
	// Restore wallet files
	for _, f := range backup.Files {
		if err := os.WriteFile(f.Filename, f.Content, 0600); err != nil {
			return fmt.Errorf("restore %s: %w", f.Filename, err)
		}
	}

	// Restore group index
	idx := &GroupIndex{Groups: backup.Groups}
	if err := SaveGroupIndex(indexFile, idx); err != nil {
		return fmt.Errorf("restore index: %w", err)
	}

	// Restore labels
	if len(backup.Labels.Labels) > 0 {
		if err := SaveLabels("wallet_labels.json", &backup.Labels); err != nil {
			return fmt.Errorf("restore labels: %w", err)
		}
	}

	return nil
}

// WalletLabels stores labels/tags for wallets.
type WalletLabels struct {
	Labels map[string]string `json:"labels"` // address → label
}

// LoadLabels loads wallet labels from disk.
func LoadLabels(filename string) (*WalletLabels, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &WalletLabels{Labels: make(map[string]string)}, nil
		}
		return nil, err
	}
	var labels WalletLabels
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, err
	}
	if labels.Labels == nil {
		labels.Labels = make(map[string]string)
	}
	return &labels, nil
}

// SaveLabels saves wallet labels to disk.
func SaveLabels(filename string, labels *WalletLabels) error {
	data, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0600)
}

// SetLabel sets a label for a wallet address.
func (wl *WalletLabels) SetLabel(address, label string) {
	wl.Labels[address] = label
}

// GetLabel returns the label for a wallet address.
func (wl *WalletLabels) GetLabel(address string) string {
	return wl.Labels[address]
}

// RemoveLabel removes a wallet's label.
func (wl *WalletLabels) RemoveLabel(address string) {
	delete(wl.Labels, address)
}
