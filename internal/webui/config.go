package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type NotifyConfig struct {
	FeishuWebhookURL   string `json:"feishu_webhook_url"`
	DingtalkWebhookURL string `json:"dingtalk_webhook_url"`
	DingtalkSecret     string `json:"dingtalk_secret"`
	WecomWebhookURL    string `json:"wecom_webhook_url"`
	TelegramBotToken   string `json:"telegram_bot_token"`
	TelegramChatID     string `json:"telegram_chat_id"`
	SlackWebhookURL    string `json:"slack_webhook_url"`
	DiscordWebhookURL  string `json:"discord_webhook_url"`
	ServerChanSendKey  string `json:"serverchan_sendkey"`
}

func LoadNotifyConfig(filePath string) (*NotifyConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &NotifyConfig{}, nil
		}
		return nil, err
	}
	var cfg NotifyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveNotifyConfig(filePath string, cfg *NotifyConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// temp file must be on the same device as target to avoid cross-device rename
	tmp, err := os.CreateTemp(dir, ".notify-config-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, filePath)
}
