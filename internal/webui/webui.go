package webui

import (
	"embed"
	"net/http"
)

//go:embed static/index.html
var staticFiles embed.FS

func Handler(basePath, customHooksFile, hooksDir, notifyTargetsFile, cronJobsFile, addr string) (http.Handler, error) {
	StartCronScheduler(cronJobsFile, addr)
	s := newServer(basePath, customHooksFile, hooksDir, notifyTargetsFile, cronJobsFile, addr)
	return s.routes(), nil
}
