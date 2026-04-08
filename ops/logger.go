package ops

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// TxLogEntry represents a single transaction log entry.
type TxLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Chain     string    `json:"chain"`
	Type      string    `json:"type"` // distribute, collect, sweep, auto-fund-gas
	From      string    `json:"from"`
	To        string    `json:"to"`
	Amount    string    `json:"amount"`
	TxHash    string    `json:"tx_hash"`
	Status    string    `json:"status"` // sent, failed
	Error     string    `json:"error,omitempty"`
}

// TxLogger handles logging transactions to CSV and JSON files.
type TxLogger struct {
	csvFile  string
	jsonFile string
	mu       sync.Mutex
	entries  []TxLogEntry
}

// NewTxLogger creates a new transaction logger.
func NewTxLogger(csvFile, jsonFile string) *TxLogger {
	return &TxLogger{
		csvFile:  csvFile,
		jsonFile: jsonFile,
	}
}

// Log adds a transaction entry to the log.
func (l *TxLogger) Log(entry TxLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

// LogTxResults logs TxResult slice from distribute/collect operations.
func (l *TxLogger) LogTxResults(results []TxResult, ch, txType string) {
	for _, r := range results {
		status := "sent"
		errMsg := ""
		if r.TxHash == "" {
			status = "failed"
			errMsg = r.Error
		}
		l.Log(TxLogEntry{
			Timestamp: time.Now(),
			Chain:     ch,
			Type:      txType,
			From:      r.From,
			To:        r.To,
			TxHash:    r.TxHash,
			Status:    status,
			Error:     errMsg,
		})
	}
}

// LogSweepResults logs SweepResult slice from sweep operations.
func (l *TxLogger) LogSweepResults(results []SweepResult, ch, txType, dest string) {
	for _, r := range results {
		if r.TxHash == "" && (r.Error == "zero balance" || r.Error == "zero token balance" ||
			r.Error == "balance <= gas cost" || r.Error == "skip: destination") {
			continue // skip non-actionable entries
		}
		status := "sent"
		errMsg := ""
		amount := ""
		if r.TxHash == "" {
			status = "failed"
			errMsg = r.Error
		}
		if r.Amount != nil {
			amount = FormatBalance(r.Amount, 18)
		}
		l.Log(TxLogEntry{
			Timestamp: time.Now(),
			Chain:     ch,
			Type:      txType,
			From:      r.Address,
			To:        dest,
			Amount:    amount,
			TxHash:    r.TxHash,
			Status:    status,
			Error:     errMsg,
		})
	}
}

// Count returns the total number of logged entries.
func (l *TxLogger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// Flush writes all entries to CSV and JSON files.
func (l *TxLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.entries) == 0 {
		return nil
	}

	if err := l.flushCSV(); err != nil {
		return fmt.Errorf("csv: %w", err)
	}
	if err := l.flushJSON(); err != nil {
		return fmt.Errorf("json: %w", err)
	}
	return nil
}

func (l *TxLogger) flushCSV() error {
	// Check if file exists to determine if we need headers
	needHeader := true
	if _, err := os.Stat(l.csvFile); err == nil {
		needHeader = false
	}

	f, err := os.OpenFile(l.csvFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if needHeader {
		w.Write([]string{"timestamp", "chain", "type", "from", "to", "amount", "tx_hash", "status", "error"})
	}

	for _, e := range l.entries {
		w.Write([]string{
			e.Timestamp.Format(time.RFC3339),
			e.Chain,
			e.Type,
			e.From,
			e.To,
			e.Amount,
			e.TxHash,
			e.Status,
			e.Error,
		})
	}

	return nil
}

func (l *TxLogger) flushJSON() error {
	// Load existing entries if file exists
	var existing []TxLogEntry
	if data, err := os.ReadFile(l.jsonFile); err == nil {
		json.Unmarshal(data, &existing)
	}

	all := append(existing, l.entries...)

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(l.jsonFile, data, 0644)
}

// Summary returns a summary of logged transactions.
type LogSummary struct {
	Total     int
	Sent      int
	Failed    int
	ByChain   map[string]int
	ByType    map[string]int
}

func (l *TxLogger) Summary() LogSummary {
	l.mu.Lock()
	defer l.mu.Unlock()

	s := LogSummary{
		Total:   len(l.entries),
		ByChain: make(map[string]int),
		ByType:  make(map[string]int),
	}

	for _, e := range l.entries {
		if e.Status == "sent" {
			s.Sent++
		} else {
			s.Failed++
		}
		s.ByChain[e.Chain]++
		s.ByType[e.Type]++
	}

	return s
}
