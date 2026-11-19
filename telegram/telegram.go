// Package telegram sends notifications to the operator's phone.
// If Bot or Chat are empty, messages are only logged.
package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Config struct {
	BaseURL string `json:"base_url" usage:"Telegram API base URL (default https://api.telegram.org)"`
	Bot     string `json:"bot" usage:"Telegram bot path segment, e.g. bot123456:ABC..."`
	Chat    string `json:"chat" usage:"Telegram chat ID"`
}

type Telegram struct {
	config Config
	client *http.Client
}

func New(c Config) *Telegram {
	if c.BaseURL == "" {
		c.BaseURL = "https://api.telegram.org"
	}
	return &Telegram{config: c, client: &http.Client{Timeout: 10 * time.Second}}
}

// Send is fire and forget.
func (t *Telegram) Send(text string) {
	go func() {
		if err := t.SendSync(text); err != nil {
			log.Println("TELEGRAM ERROR:", err)
		}
	}()
}

func (t *Telegram) SendSync(text string) error {
	log.Println("TELEGRAM:", text)
	if t == nil || t.config.Bot == "" || t.config.Chat == "" {
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"chat_id": t.config.Chat,
		"text":    text,
	})
	if err != nil {
		return err
	}

	u := fmt.Sprintf("%s/%s/sendMessage", t.config.BaseURL, t.config.Bot)
	resp, err := t.client.Post(u, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, b)
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}
