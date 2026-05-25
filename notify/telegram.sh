#!/bin/sh
# Telegram Bot 通知脚本
#
# 必需环境变量:
#   TELEGRAM_BOT_TOKEN  Telegram Bot Token（通过 @BotFather 获取）
#   TELEGRAM_CHAT_ID    目标 Chat ID（用户/群组/频道 ID）
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题（会加粗显示）
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 获取 Chat ID 方法:
#   向 @userinfobot 发消息，它会回复你的 Chat ID
#
# 使用示例:
#   curl "http://localhost:9000/hooks/telegram?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/telegram \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"部署完成！","title":"CI 通知"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "[ERROR] 未设置 TELEGRAM_BOT_TOKEN，请在 .env 文件中配置 Telegram Bot Token"
    exit 1
fi

if [ -z "$TELEGRAM_CHAT_ID" ]; then
    echo "[ERROR] 未设置 TELEGRAM_CHAT_ID，请在 .env 文件中配置目标 Chat ID"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

# 构造消息文本（HTML 格式，标题加粗）
if [ -n "$TITLE" ]; then
    TEXT="<b>${TITLE}</b>\n${MSG}"
else
    TEXT="$MSG"
fi

API_URL="https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage"

PAYLOAD=$(jq -n \
    --arg chat_id "$TELEGRAM_CHAT_ID" \
    --arg text "$TEXT" \
    '{
        chat_id: $chat_id,
        text: $text,
        parse_mode: "HTML"
    }')

HTTP_CODE=$(curl -s -o /tmp/.telegram_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$API_URL")

RESP=$(cat /tmp/.telegram_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] Telegram 通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

OK=$(echo "$RESP" | jq -r '.ok // false')
if [ "$OK" != "true" ]; then
    DESCRIPTION=$(echo "$RESP" | jq -r '.description // "unknown error"')
    echo "[ERROR] Telegram API 返回错误: $DESCRIPTION"
    exit 1
fi

echo "[OK] Telegram 通知发送成功"
