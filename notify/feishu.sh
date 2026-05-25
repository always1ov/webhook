#!/bin/sh
# ──────────────────────────────────────────
# 飞书机器人通知脚本
# 在下方填入你的飞书群机器人 Webhook 地址
FEISHU_WEBHOOK_URL=""
# ──────────────────────────────────────────

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$FEISHU_WEBHOOK_URL" ]; then
    echo "[ERROR] 请编辑此脚本，在顶部填入 FEISHU_WEBHOOK_URL"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入"
    exit 1
fi

if [ -n "$TITLE" ]; then
    PAYLOAD=$(jq -n \
        --arg title "$TITLE" \
        --arg msg "$MSG" \
        '{msg_type:"post",content:{post:{"zh_cn":{title:$title,content:[[{tag:"text",text:$msg}]]}}}}')
else
    PAYLOAD=$(jq -n --arg msg "$MSG" '{msg_type:"text",content:{text:$msg}}')
fi

HTTP_CODE=$(curl -s -o /tmp/.feishu_resp -w "%{http_code}" \
    -X POST -H "Content-Type: application/json" \
    -d "$PAYLOAD" "$FEISHU_WEBHOOK_URL")

RESP=$(cat /tmp/.feishu_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 飞书通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" != "0" ]; then
    echo "[ERROR] 飞书接口错误 (code=$CODE): $(echo "$RESP" | jq -r '.msg // ""')"
    exit 1
fi

echo "[OK] 飞书通知发送成功"
