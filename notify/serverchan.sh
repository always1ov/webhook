#!/bin/sh
# Server酱（方糖）通知脚本
# 可推送消息到微信，注册地址：https://sct.ftqq.com
#
# 必需环境变量:
#   SERVERCHAN_SENDKEY  Server酱 SendKey（登录后在 Key & API 页面获取）
#
# 消息内容（二选一或同时存在，payload 优先）:
#   MSG        消息正文（支持 Markdown）
#   MSG_QUERY  消息正文（来自 URL 参数 ?msg=...）
#   TITLE        消息标题（必填，默认为「通知」）
#   TITLE_QUERY  消息标题（来自 URL 参数 ?title=...）
#
# 使用示例:
#   curl "http://localhost:9000/hooks/serverchan?msg=Hello&title=测试"
#   curl -X POST http://localhost:9000/hooks/serverchan \
#     -H "Content-Type: application/json" \
#     -d '{"msg":"服务恢复正常","title":"恢复通知"}'

set -e

MSG="${MSG:-$MSG_QUERY}"
TITLE="${TITLE:-$TITLE_QUERY}"
TITLE="${TITLE:-通知}"

if [ -z "$SERVERCHAN_SENDKEY" ]; then
    echo "[ERROR] 未设置 SERVERCHAN_SENDKEY，请在 .env 文件中配置 Server酱 SendKey"
    exit 1
fi

if [ -z "$MSG" ]; then
    echo "[ERROR] 消息内容为空，请通过 ?msg=... 或 POST body {\"msg\":\"...\"} 传入消息"
    exit 1
fi

API_URL="https://sctapi.ftqq.com/${SERVERCHAN_SENDKEY}.send"

HTTP_CODE=$(curl -s -o /tmp/.serverchan_resp -w "%{http_code}" \
    -X POST \
    -H "Content-Type: application/x-www-form-urlencoded" \
    --data-urlencode "title=${TITLE}" \
    --data-urlencode "desp=${MSG}" \
    "$API_URL")

RESP=$(cat /tmp/.serverchan_resp)

if [ "$HTTP_CODE" != "200" ]; then
    echo "[ERROR] Server酱通知失败 (HTTP $HTTP_CODE): $RESP"
    exit 1
fi

CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" != "0" ]; then
    MESSAGE=$(echo "$RESP" | jq -r '.message // "unknown error"')
    echo "[ERROR] Server酱接口返回错误 (code=$CODE): $MESSAGE"
    exit 1
fi

echo "[OK] Server酱通知发送成功，消息将推送到微信"
