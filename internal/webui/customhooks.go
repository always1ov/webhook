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
# 修改 HOOK_ID 下方的逻辑可自定义发送行为

HOOK_ID="%s"
TARGETS_FILE="${NOTIFY_TARGETS_FILE:-/data/notify-targets.json}"
MSG="${MSG:-}"
TITLE="${TITLE:-Webhook 通知}"

if [ -z "$MSG" ]; then
  echo "ERROR: MSG is empty" >&2
  exit 1
fi

if [ ! -f "$TARGETS_FILE" ]; then
  echo "WARN: targets file not found: $TARGETS_FILE"
  exit 0
fi

targets_json=$(jq -e --arg id "$HOOK_ID" '.[$id] // []' "$TARGETS_FILE" 2>/dev/null || echo "[]")
count=$(echo "$targets_json" | jq 'length' 2>/dev/null || echo 0)

if [ "$count" -eq 0 ]; then
  echo "WARN: no targets configured for hook \"$HOOK_ID\""
  exit 0
fi

TMPDIR_SELF=$(mktemp -d)
trap 'rm -rf "$TMPDIR_SELF"' EXIT

success=0
failure=0

send_feishu() {
  local url="$1" body tmp
  tmp="$TMPDIR_SELF/resp.json"
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msg_type":"post","content":{"post":{"zh_cn":{"title":$title,"content":[[{"tag":"text","text":$text}]]}}}}')
  resp=$(curl -s --max-time 10 --connect-timeout 5 \
    -o "$tmp" -w "%%{http_code}" -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    code=$(jq -r '.code // 0' "$tmp" 2>/dev/null)
    [ "$code" = "0" ] && return 0
    echo "WARN: feishu code=$code msg=$(jq -r '.msg // ""' "$tmp")"
    return 1
  fi
  echo "WARN: feishu HTTP $resp"; return 1
}

send_dingtalk() {
  local url="$1" secret="$2" body tmp
  tmp="$TMPDIR_SELF/resp.json"
  if [ -n "$secret" ]; then
    ts=$(date +%%s%%3N)
    sign=$(printf "%%s\n%%s" "$ts" "$secret" | openssl dgst -sha256 -hmac "$secret" -binary | base64 | tr -d '\n')
    sign_enc=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$sign" 2>/dev/null || echo "$sign")
    url="${url}&timestamp=${ts}&sign=${sign_enc}"
  fi
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msgtype":"markdown","markdown":{"title":$title,"text":("## "+$title+"\n"+$text)}}')
  resp=$(curl -s --max-time 10 --connect-timeout 5 \
    -o "$tmp" -w "%%{http_code}" -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    errcode=$(jq -r '.errcode // 0' "$tmp" 2>/dev/null)
    [ "$errcode" = "0" ] && return 0
    echo "WARN: dingtalk error=$errcode msg=$(jq -r '.errmsg // ""' "$tmp")"
    return 1
  fi
  echo "WARN: dingtalk HTTP $resp"; return 1
}

send_wecom() {
  local url="$1" body tmp
  tmp="$TMPDIR_SELF/resp.json"
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msgtype":"markdown","markdown":{"content":("## "+$title+"\n"+$text)}}')
  resp=$(curl -s --max-time 10 --connect-timeout 5 \
    -o "$tmp" -w "%%{http_code}" -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    errcode=$(jq -r '.errcode // 0' "$tmp" 2>/dev/null)
    [ "$errcode" = "0" ] && return 0
    echo "WARN: wecom error=$errcode msg=$(jq -r '.errmsg // ""' "$tmp")"
    return 1
  fi
  echo "WARN: wecom HTTP $resp"; return 1
}

while IFS= read -r target; do
  type=$(echo "$target" | jq -r '.type // ""')
  name=$(echo "$target" | jq -r '.name // ""')
  url=$(echo "$target"  | jq -r '.url // ""')
  secret=$(echo "$target" | jq -r '.secret // ""')
  [ -z "$url" ] && { echo "SKIP: \"$name\" has no url"; continue; }
  case "$type" in
    feishu)   if send_feishu "$url";          then echo "OK: feishu \"$name\"";   success=$((success+1)); else echo "FAIL: feishu \"$name\"";   failure=$((failure+1)); fi ;;
    dingtalk) if send_dingtalk "$url" "$secret"; then echo "OK: dingtalk \"$name\""; success=$((success+1)); else echo "FAIL: dingtalk \"$name\""; failure=$((failure+1)); fi ;;
    wecom)    if send_wecom "$url";           then echo "OK: wecom \"$name\"";    success=$((success+1)); else echo "FAIL: wecom \"$name\"";    failure=$((failure+1)); fi ;;
    *) echo "SKIP: unknown type \"$type\" for \"$name\"" ;;
  esac
done < <(echo "$targets_json" | jq -c '.[]')

echo "DONE: success=$success failure=$failure"
[ "$failure" -eq 0 ]
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
