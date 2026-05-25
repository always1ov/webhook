#!/bin/sh
# 飞书自定义机器人通知脚本
#
# 必需环境变量:
#   FEISHU_WEBHOOK_URL  飞书机器人 Webhook 地址
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文（来自 POST body 的 msg 字段）
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#
# 可选环境变量:
#   TITLE        消息标题（来自 POST body 的 title 字段）
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/feishu?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/feishu \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"服务器异常！","title":"告警"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"

if [ -z "$FEISHU_WEBHOOK_URL" ]; then
    echo "[ERROR] 未设置 FEISHU_WEBHOOK_URL，请在 .env 文件中配置飞书机器人 Webhook 地址"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

# 构造飞书消息 payload（使用 jq 保证 JSON 转义正确）
if [ -n "$TITLE" ]; then
    PAYLOAD=$(jq -n \
        --arg title "$TITLE" \
        --arg msg "$MSG" \
        '{
            msg_type: "post",
            content: {
                post: {
                    "zh_cn": {
                        title: $title,
                        content: [[{tag: "text", text: $msg}]]
                    }
                }
            }
        }')
else
    PAYLOAD=$(jq -n \
        --arg msg "$MSG" \
        '{msg_type: "text", content: {text: $msg}}')
fi

HTTP_CODE=$(curl -s -o /tmp/.feishu_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$FEISHU_WEBHOOK_URL")

RESP=$(cat /tmp/.feishu_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 飞书通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" != "0" ]; then
    MSG_FROM_RESP=$(echo "$RESP" | jq -r '.msg // "unknown error"')
    echo "[ERROR] 飞书接口返回错误 (code=$CODE): $MSG_FROM_RESP"
    exit 1
fi

echo "[OK] 飞书通知发送成功"
