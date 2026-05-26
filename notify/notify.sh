#!/bin/bash
# Universal notification script
# Reads /data/notify-targets.json and sends to all configured targets
# Env vars: MSG (message body), TITLE (optional title)

TARGETS_FILE="${NOTIFY_TARGETS_FILE:-/data/notify-targets.json}"
MSG="${MSG:-$(cat /dev/stdin 2>/dev/null)}"
TITLE="${TITLE:-Webhook 通知}"

if [ -z "$MSG" ]; then
  echo "ERROR: MSG is empty" >&2
  exit 1
fi

if [ ! -f "$TARGETS_FILE" ]; then
  echo "ERROR: targets file not found: $TARGETS_FILE" >&2
  exit 1
fi

count=$(jq 'length' "$TARGETS_FILE" 2>/dev/null)
if [ -z "$count" ] || [ "$count" -eq 0 ]; then
  echo "WARN: no targets configured in $TARGETS_FILE"
  exit 0
fi

success=0
failure=0

send_feishu() {
  local url="$1"
  local body
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msg_type":"post","content":{"post":{"zh_cn":{"title":$title,"content":[[{"tag":"text","text":$text}]]}}}}')
  resp=$(curl -s -o /tmp/feishu_resp.json -w "%{http_code}" \
    -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    code=$(jq -r '.code // 0' /tmp/feishu_resp.json 2>/dev/null)
    if [ "$code" = "0" ]; then
      return 0
    fi
    echo "WARN: feishu API error code=$code msg=$(jq -r '.msg // ""' /tmp/feishu_resp.json)"
    return 1
  fi
  echo "WARN: feishu HTTP $resp"
  return 1
}

send_dingtalk() {
  local url="$1"
  local secret="$2"
  if [ -n "$secret" ]; then
    ts=$(date +%s%3N)
    sign=$(printf "%s\n%s" "$ts" "$secret" | openssl dgst -sha256 -hmac "$secret" -binary | base64 | tr -d '\n')
    sign_enc=$(python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1]))" "$sign" 2>/dev/null || \
               node -e "process.stdout.write(encodeURIComponent(process.argv[1]))" "$sign" 2>/dev/null || \
               echo "$sign")
    url="${url}&timestamp=${ts}&sign=${sign_enc}"
  fi
  local body
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msgtype":"markdown","markdown":{"title":$title,"text":("## "+$title+"\n"+$text)}}')
  resp=$(curl -s -o /tmp/dingtalk_resp.json -w "%{http_code}" \
    -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    errcode=$(jq -r '.errcode // 0' /tmp/dingtalk_resp.json 2>/dev/null)
    if [ "$errcode" = "0" ]; then
      return 0
    fi
    echo "WARN: dingtalk API error=$errcode msg=$(jq -r '.errmsg // ""' /tmp/dingtalk_resp.json)"
    return 1
  fi
  echo "WARN: dingtalk HTTP $resp"
  return 1
}

send_wecom() {
  local url="$1"
  local body
  body=$(jq -n --arg title "$TITLE" --arg text "$MSG" \
    '{"msgtype":"markdown","markdown":{"content":("## "+$title+"\n"+$text)}}')
  resp=$(curl -s -o /tmp/wecom_resp.json -w "%{http_code}" \
    -H "Content-Type: application/json" -d "$body" "$url")
  if [ "$resp" = "200" ]; then
    errcode=$(jq -r '.errcode // 0' /tmp/wecom_resp.json 2>/dev/null)
    if [ "$errcode" = "0" ]; then
      return 0
    fi
    echo "WARN: wecom API error=$errcode msg=$(jq -r '.errmsg // ""' /tmp/wecom_resp.json)"
    return 1
  fi
  echo "WARN: wecom HTTP $resp"
  return 1
}

while IFS= read -r target; do
  type=$(echo "$target" | jq -r '.type // ""')
  name=$(echo "$target" | jq -r '.name // ""')
  url=$(echo "$target" | jq -r '.url // ""')
  secret=$(echo "$target" | jq -r '.secret // ""')

  if [ -z "$url" ]; then
    echo "SKIP: target \"$name\" has no url"
    continue
  fi

  case "$type" in
    feishu)
      if send_feishu "$url"; then
        echo "OK: feishu \"$name\""
        success=$((success+1))
      else
        echo "FAIL: feishu \"$name\""
        failure=$((failure+1))
      fi
      ;;
    dingtalk)
      if send_dingtalk "$url" "$secret"; then
        echo "OK: dingtalk \"$name\""
        success=$((success+1))
      else
        echo "FAIL: dingtalk \"$name\""
        failure=$((failure+1))
      fi
      ;;
    wecom)
      if send_wecom "$url"; then
        echo "OK: wecom \"$name\""
        success=$((success+1))
      else
        echo "FAIL: wecom \"$name\""
        failure=$((failure+1))
      fi
      ;;
    *)
      echo "SKIP: unknown type \"$type\" for \"$name\""
      ;;
  esac
done < <(jq -c '.[]' "$TARGETS_FILE")

echo "DONE: success=$success failure=$failure"
[ "$failure" -eq 0 ]
