#!/bin/sh
# ──────────────────────────────────────────
# 钉钉机器人通知脚本
# 在下方填入你的钉钉群机器人 Webhook 地址
DINGTALK_WEBHOOK_URL=""
# 如果机器人开启了加签验证，填入密钥（SEC 开头），否则留空
DINGTALK_SECRET=""
# ──────────────────────────────────────────

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"
TITLE="${TITLE:-通知}"

if [ -z "$DINGTALK_WEBHOOK_URL" ]; then
    echo "[ERROR] 请编辑此脚本，在顶部填入 DINGTALK_WEBHOOK_URL"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入"
    exit 1
fi

FINAL_URL="$DINGTALK_WEBHOOK_URL"

if [ -n "$DINGTALK_SECRET" ]; then
    TIMESTAMP=$(date +%s%3N)
    STRING_TO_SIGN="${TIMESTAMP}\n${DINGTALK_SECRET}"
    SIGN=$(printf "%b" "$STRING_TO_SIGN" | openssl dgst -sha256 -hmac "$DINGTALK_SECRET" -binary | base64 | tr -d '\n')
    ENCODED_SIGN=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$SIGN'))" 2>/dev/null || \
                   printf '%s' "$SIGN" | sed 's/+/%2B/g;s/=/%3D/g;s|/|%2F|g')
    FINAL_URL="${DINGTALK_WEBHOOK_URL}&timestamp=${TIMESTAMP}&sign=${ENCODED_SIGN}"
fi

PAYLOAD=$(jq -n \
    --arg title "$TITLE" \
    --arg msg "$MSG" \
    '{msgtype:"markdown",markdown:{title:$title,text:("### " + $title + "\n\n" + $msg)}}')

HTTP_CODE=$(curl -s -o /tmp/.dingtalk_resp -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" \
    -d "$PAYLOAD" "$FINAL_URL")

RESP=$(cat /tmp/.dingtalk_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 钉钉通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

ERRCODE=$(echo "$RESP" | jq -r '.errcode // 0')
if [ "$ERRCODE" != "0" ]; then
    echo "[ERROR] 钉钉接口错误 (errcode=$ERRCODE): $(echo "$RESP" | jq -r '.errmsg // ""')"
    exit 1
fi

echo "[OK] 钉钉通知发送成功"
