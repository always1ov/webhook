#!/bin/sh
# Slack Incoming Webhook 通知脚本
#
# 必需环境变量:
#   SLACK_WEBHOOK_URL  Slack Incoming Webhook URL
#   (在 https://api.slack.com/apps 创建 App 后获取)
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/slack?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/slack \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"Build failed!","title":"CI Alert"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$SLACK_WEBHOOK_URL" ]; then
    echo "[ERROR] 未设置 SLACK_WEBHOOK_URL，请在 .env 文件中配置 Slack Webhook URL"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

# 构造 Slack Block Kit 消息（带标题时显示为 header block）
if [ -n "$TITLE" ]; then
    PAYLOAD=$(jq -n \
        --arg title "$TITLE" \
        --arg msg "$MSG" \
        '{
            blocks: [
                {type: "header", text: {type: "plain_text", text: $title}},
                {type: "section", text: {type: "mrkdwn", text: $msg}}
            ]
        }')
else
    PAYLOAD=$(jq -n \
        --arg msg "$MSG" \
        '{text: $msg}')
fi

HTTP_CODE=$(curl -s -o /tmp/.slack_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$SLACK_WEBHOOK_URL")

RESP=$(cat /tmp/.slack_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] Slack 通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

if [ "$RESP" != "ok" ]; then
    echo "[ERROR] Slack API 返回错误: $RESP"
    exit 1
fi

echo "[OK] Slack 通知发送成功"
