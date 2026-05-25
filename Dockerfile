# 第一阶段：编译 webhook 二进制
FROM golang:1.26-alpine3.23 AS builder
RUN apk --update add ca-certificates
ENV GOPROXY=https://goproxy.cn
ENV CGO_ENABLED=0
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-w -s" -o webhook .

# 第二阶段：运行镜像（含通知脚本所需工具）
FROM alpine:3.23
RUN apk --no-cache add bash curl jq ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app/webhook /usr/bin/webhook
EXPOSE 9000/tcp
CMD ["/usr/bin/webhook"]
