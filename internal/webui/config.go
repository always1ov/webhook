package webui

import (
	"encoding/json"
	"os"
)

// NotifyConfig 各通知平台的凭证配置
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

// LoadNotifyConfig 从 JSON 文件读取通知配置，文件不存在时返回空配置
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

// SaveNotifyConfig 原子写入通知配置到 JSON 文件
func SaveNotifyConfig(filePath string, cfg *NotifyConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// 写临时文件再 rename，保证原子性
	tmp, err := os.CreateTemp("", "notify-config-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, filePath)
}
