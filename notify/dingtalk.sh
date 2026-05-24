#!/bin/sh
# 钉钉自定义机器人通知脚本
#
# 必需环境变量:
#   DINGTALK_WEBHOOK_URL  钉钉机器人 Webhook 地址
#
# 可选环境变量（若机器人开启了签名验证则必填）:
#   DINGTALK_SECRET  机器人加签密钥（以 SEC 开头的字符串）
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/dingtalk?msg=Hello"
#   curl -X POST http://localhost:9000/hooks/dingtalk \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"部署成功！","title":"CI/CD 通知"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"
TITLE="${TITLE:-通知}"

if [ -z "$DINGTALK_WEBHOOK_URL" ]; then
    echo "[ERROR] 未设置 DINGTALK_WEBHOOK_URL，请在 .env 文件中配置钉钉机器人 Webhook 地址"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

FINAL_URL="$DINGTALK_WEBHOOK_URL"

# 如果配置了加签密钥，生成签名
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
    '{
        msgtype: "markdown",
        markdown: {
            title: $title,
            text: ("### " + $title + "\n\n" + $msg)
        }
    }')

HTTP_CODE=$(curl -s -o /tmp/.dingtalk_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD" \
    "$FINAL_URL")

RESP=$(cat /tmp/.dingtalk_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] 钉钉通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

ERRCODE=$(echo "$RESP" | jq -r '.errcode // 0')
if [ "$ERRCODE" != "0" ]; then
    ERRMSG=$(echo "$RESP" | jq -r '.errmsg // "unknown error"')
    echo "[ERROR] 钉钉接口返回错误 (errcode=$ERRCODE): $ERRMSG"
    exit 1
fi

echo "[OK] 钉钉通知发送成功"
