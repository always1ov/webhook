#!/bin/sh
# ──────────────────────────────────────────
# 企业微信群机器人通知脚本
# 在下方填入你的企业微信群机器人 Webhook 地址
WECOM_WEBHOOK_URL=""
# ──────────────────────────────────────────

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$WECOM_WEBHOOK_URL" ]; then
    echo "[ERROR] 请编辑此脚本，在顶部填入 WECOM_WEBHOOK_URL"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入"
    exit 1
fi

if [ -n "$TITLE" ]; then
    CONTENT="## ${TITLE}\n${MSG}"
else
    CONTENT="$MSG"
fi

PAYLOAD=$(jq -n --arg content "$CONTENT" '{msgtype:"markdown",markdown:{content:$content}}')

HTTP_CODE=$(curl -s -o /tmp/.wecom_resp -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" \
    -d "$PAYLOAD" "$WECOM_WEBHOOK_URL")

RESP=$(cat /tmp/.wecom_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 企业微信通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

ERRCODE=$(echo "$RESP" | jq -r '.errcode // 0')
if [ "$ERRCODE" != "0" ]; then
    echo "[ERROR] 企业微信接口错误 (errcode=$ERRCODE): $(echo "$RESP" | jq -r '.errmsg // ""')"
    exit 1
fi

echo "[OK] 企业微信通知发送成功"
