package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AlertType defines the notification channel.
type AlertType int

const (
	AlertNone     AlertType = iota
	AlertTelegram
	AlertDiscord
)

// AlertConfig holds webhook/notification configuration.
type AlertConfig struct {
	Enabled   bool
	Type      AlertType
	Webhook   string // Discord webhook URL
	BotToken  string // Telegram bot token
	ChatID    string // Telegram chat ID
}

// NoAlert returns a disabled alert config.
func NoAlert() AlertConfig {
	return AlertConfig{Enabled: false}
}

// Send sends a notification message via the configured channel.
func (ac AlertConfig) Send(title, msg string) error {
	if !ac.Enabled {
		return nil
	}
	switch ac.Type {
	case AlertTelegram:
		return ac.sendTelegram(title, msg)
	case AlertDiscord:
		return ac.sendDiscord(title, msg)
	default:
		return nil
	}
}

// SendTxAlert sends a transaction completion alert.
func (ac AlertConfig) SendTxAlert(opType, chain string, success, failed int, elapsed time.Duration) {
	if !ac.Enabled {
		return
	}
	status := "✅"
	if failed > 0 {
		status = "⚠️"
	}
	title := fmt.Sprintf("%s %s Complete", status, opType)
	msg := fmt.Sprintf("Chain: %s\nSuccess: %d\nFailed: %d\nDuration: %s",
		chain, success, failed, elapsed.Round(time.Second))
	ac.Send(title, msg)
}

// SendBalanceAlert sends a balance drop alert.
func (ac AlertConfig) SendBalanceAlert(wallet, chain, balance string) {
	if !ac.Enabled {
		return
	}
	ac.Send("🔴 Low Balance Alert",
		fmt.Sprintf("Wallet: %s\nChain: %s\nBalance: %s", wallet, chain, balance))
}

func (ac AlertConfig) sendTelegram(title, msg string) error {
	text := fmt.Sprintf("*%s*\n%s", title, msg)
	payload := map[string]interface{}{
		"chat_id":    ac.ChatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", ac.BotToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram: %s", resp.Status)
	}
	return nil
}

func (ac AlertConfig) sendDiscord(title, msg string) error {
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": msg,
				"color":       0x00FF41,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	data, _ := json.Marshal(payload)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(ac.Webhook, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord: %s", resp.Status)
	}
	return nil
}

// TypeName returns a display string for the alert type.
func (ac AlertConfig) TypeName() string {
	switch ac.Type {
	case AlertTelegram:
		return "Telegram"
	case AlertDiscord:
		return "Discord"
	default:
		return "None"
	}
}
