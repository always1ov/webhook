package webui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type NotifyTarget struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}

func LoadNotifyTargets(file string) ([]NotifyTarget, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return []NotifyTarget{}, nil
	}
	if err != nil {
		return nil, err
	}
	var targets []NotifyTarget
	if err := json.Unmarshal(data, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func SaveNotifyTargets(file string, targets []NotifyTarget) error {
	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".notify-targets-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	_ = tmp.Close()
	return os.Rename(name, file)
}
