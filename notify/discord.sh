#!/bin/sh
# Discord Webhook 通知脚本
#
# 必需环境变量:
#   DISCORD_WEBHOOK_URL  Discord Webhook URL
#   (在频道设置 → 整合 → Webhooks 中创建)
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题（Embed 标题）
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/discord?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/discord \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"Deployment done!","title":"Deploy"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$DISCORD_WEBHOOK_URL" ]; then
    echo "[ERROR] 未设置 DISCORD_WEBHOOK_URL，请在 .env 文件中配置 Discord Webhook URL"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

# 使用 Discord Embed 格式（带标题时显示更规范）
if [ -n "$TITLE" ]; then
    PAYLOAD=$(jq -n \
        --arg title "$TITLE" \
        --arg msg "$MSG" \
        '{embeds: [{title: $title, description: $msg, color: 3447003}]}')
else
    PAYLOAD=$(jq -n --arg msg "$MSG" '{content: $msg}')
fi

HTTP_CODE=$(curl -s -o /tmp/.discord_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$DISCORD_WEBHOOK_URL")

RESP=$(cat /tmp/.discord_resp)

# Discord 成功返回 204 No Content 或 200（带 wait=true）
if [ "$HTTP_CODE" != "204" ] && [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] Discord 通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

echo "[OK] Discord 通知发送成功"
