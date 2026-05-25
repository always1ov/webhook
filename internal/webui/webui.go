package webui

import (
	"embed"
	"net/http"
)

//go:embed static/index.html
var staticFiles embed.FS

func Handler(basePath, customHooksFile, hooksDir, addr string) (http.Handler, error) {
	s := newServer(basePath, customHooksFile, hooksDir, addr)
	return s.routes(), nil
}
