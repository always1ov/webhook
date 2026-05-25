package webui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	method := strings.ToUpper(h.Method)
	if method == "" {
		method = "POST"
	}
	tpl := h.BodyTemplate
	if tpl == "" {
		tpl = `{"msg":$msg,"title":$title}`
	}

	var hdrLines strings.Builder
	for _, line := range strings.Split(h.Headers, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, ":") {
			fmt.Fprintf(&hdrLines, "  -H %q \\\n", line)
		}
	}

	return fmt.Sprintf(`#!/bin/sh
# 自定义推送脚本 - %s
# 由 Web UI 自动生成，请勿手动编辑

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"
TITLE="${TITLE:-通知}"

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入"
    exit 1
fi

BODY=$(jq -n --arg msg "$MSG" --arg title "$TITLE" '%s')

HTTP_CODE=$(curl -s -o /tmp/.custom_%s_resp -w "%%{http_code}" \\
  -X %s \\
  -H "Content-Type: application/json" \\
%s  -d "$BODY" \\
  "%s")

RESP=$(cat /tmp/.custom_%s_resp)

if [ "$HTTP_CODE" -ge 200 ] && [ "$HTTP_CODE" -lt 300 ]; then
    echo "[OK] 推送成功 (HTTP $HTTP_CODE)"
else
    echo "[ERROR] 推送失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi
`,
		h.Name,
		tpl,
		h.ID,
		method,
		hdrLines.String(),
		h.TargetURL,
		h.ID,
	)
}

func generateYAML(h CustomHook, scriptPath string) string {
	return fmt.Sprintf(`# 自定义推送 Hook - %s
# 由 Web UI 自动生成，请勿手动编辑

- id: %s
  execute-command: %s
  command-working-directory: /data/scripts
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
      envname: MSG_QUERY
    - source: url
      name: title
      envname: TITLE_QUERY
`,
		h.Name,
		h.ID,
		scriptPath,
	)
}
