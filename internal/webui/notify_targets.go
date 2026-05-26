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

// HookTargetsMap maps hookID → list of notification targets for that hook.
type HookTargetsMap map[string][]NotifyTarget

func LoadHookTargets(file string) (HookTargetsMap, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return HookTargetsMap{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m HookTargetsMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = HookTargetsMap{}
	}
	return m, nil
}

func GetHookTargets(file, hookID string) ([]NotifyTarget, error) {
	m, err := LoadHookTargets(file)
	if err != nil {
		return nil, err
	}
	t := m[hookID]
	if t == nil {
		t = []NotifyTarget{}
	}
	return t, nil
}

func SetHookTargets(file, hookID string, targets []NotifyTarget) error {
	m, err := LoadHookTargets(file)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		delete(m, hookID)
	} else {
		m[hookID] = targets
	}
	return saveHookTargetsFile(file, m)
}

func saveHookTargetsFile(file string, m HookTargetsMap) error {
	data, err := json.MarshalIndent(m, "", "  ")
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
