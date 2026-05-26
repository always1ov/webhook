package webui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// CustomHook 用户在 UI 上定义的自定义推送 Hook
type CustomHook struct {
	ID           string    `json:"id"`           // URL 路径段，如 my-notify → /hooks/my-notify
	Name         string    `json:"name"`         // 显示名称
	TargetURL    string    `json:"targetURL"`    // 推送目标 URL
	Method       string    `json:"method"`       // HTTP 方法：POST / GET / PUT
	BodyTemplate string    `json:"bodyTemplate"` // jq 表达式，默认 {"msg":$msg,"title":$title}
	Headers      string    `json:"headers"`      // 附加请求头，每行一个：Key: Value
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{0,62}$`)

func ValidateCustomHookID(id string) error {
	if !validID.MatchString(id) {
		return fmt.Errorf("id 只允许小写字母、数字和连字符，且必须以字母或数字开头，长度 1~63")
	}
	return nil
}

// LoadCustomHooks 从 JSON 文件加载自定义 Hook 列表
func LoadCustomHooks(file string) ([]CustomHook, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return []CustomHook{}, nil
	}
	if err != nil {
		return nil, err
	}
	var hooks []CustomHook
	if err := json.Unmarshal(data, &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

// SaveCustomHooks 原子写入 JSON 文件
func SaveCustomHooks(file string, hooks []CustomHook) error {
	data, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".custom-hooks-*.tmp")
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
	return os.Rename(name, file)
}

// WriteCustomHookFiles 生成 YAML + Shell 脚本文件
func WriteCustomHookFiles(hooksDir, scriptsDir string, h CustomHook) error {
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return err
	}
	scriptPath := filepath.Join(scriptsDir, "custom_"+h.ID+".sh")
	if err := os.WriteFile(scriptPath, []byte(generateScript(h)), 0755); err != nil {
		return err
	}
	return os.WriteFile(
		filepath.Join(hooksDir, "custom_"+h.ID+".yaml"),
		[]byte(generateYAML(h, scriptPath)),
		0644,
	)
}

// DeleteCustomHookFiles 删除 YAML + Shell 脚本文件
func DeleteCustomHookFiles(hooksDir, scriptsDir, id string) {
	_ = os.Remove(filepath.Join(hooksDir, "custom_"+id+".yaml"))
	_ = os.Remove(filepath.Join(scriptsDir, "custom_"+id+".sh"))
}

func generateScript(h CustomHook) string {
	return fmt.Sprintf(`#!/bin/bash
# 自定义推送脚本 - %s
# 由 Web UI 自动生成；在「通知配置」中添加目标地址后即可触发推送

export HOOK_ID="%s"
export MSG="${MSG:-}"
export TITLE="${TITLE:-通知}"
exec /notify/notify.sh
`,
		h.Name,
		h.ID,
	)
}

func generateYAML(h CustomHook, scriptPath string) string {
	return fmt.Sprintf(`# 自定义推送 Hook - %s
# 由 Web UI 自动生成，请勿手动编辑

- id: %s
  execute-command: %s
  command-working-directory: /
  include-command-output-in-response: true
  include-command-out-in-response-on-error: true
  pass-environment-to-command:
    - source: payload
      name: msg
      envname: MSG
    - source: payload
      name: title
      envname: TITLE
    - source: url
      name: msg
      envname: MSG
    - source: url
      name: title
      envname: TITLE
`,
		h.Name,
		h.ID,
		scriptPath,
	)
}
