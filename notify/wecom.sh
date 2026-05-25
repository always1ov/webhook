#!/bin/sh
# 企业微信群机器人通知脚本
#
# 必需环境变量:
#   WECOM_WEBHOOK_URL  企业微信群机器人 Webhook 地址
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题（Markdown 模式）
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/wecom?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/wecom \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"服务器磁盘告警","title":"系统告警"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$WECOM_WEBHOOK_URL" ]; then
    echo "[ERROR] 未设置 WECOM_WEBHOOK_URL，请在 .env 文件中配置企业微信机器人 Webhook 地址"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

# 构造 Markdown 消息（带标题时更美观）
if [ -n "$TITLE" ]; then
    CONTENT="## ${TITLE}\n${MSG}"
else
    CONTENT="$MSG"
fi

PAYLOAD=$(jq -n \
    --arg content "$CONTENT" \
    '{msgtype: "markdown", markdown: {content: $content}}')

HTTP_CODE=$(curl -s -o /tmp/.wecom_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$WECOM_WEBHOOK_URL")

RESP=$(cat /tmp/.wecom_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 企业微信通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

ERRCODE=$(echo "$RESP" | jq -r '.errcode // 0')
if [ "$ERRCODE" != "0" ]; then
    ERRMSG=$(echo "$RESP" | jq -r '.errmsg // "unknown error"')
    echo "[ERROR] 企业微信接口返回错误 (errcode=$ERRCODE): $ERRMSG"
    exit 1
fi

echo "[OK] 企业微信通知发送成功"
