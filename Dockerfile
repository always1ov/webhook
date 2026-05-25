FROM golang:1.26-alpine3.23 AS builder
RUN apk add --no-cache ca-certificates upx
ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-w -s" -trimpath -o webhook . \
    && upx --best --lzma webhook

FROM alpine:3.23
RUN apk add --no-cache bash curl jq ca-certificates
COPY --from=builder /app/webhook /usr/bin/webhook
COPY notify/ /notify/
RUN chmod +x /notify/*.sh
EXPOSE 9000/tcp
ENTRYPOINT ["/usr/bin/webhook"]
