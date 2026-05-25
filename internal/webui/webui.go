package webui

import (
	"embed"
	"net/http"
)

//go:embed static/index.html
var staticFiles embed.FS

// Handler 返回 WebUI 的 http.Handler，注册所有 /ui/ 路由
func Handler(basePath, notifyConfigFile, customHooksFile, hooksDir, addr string) (http.Handler, error) {
	serverAddr = addr
	return newMux(basePath, notifyConfigFile, customHooksFile, hooksDir), nil
}
