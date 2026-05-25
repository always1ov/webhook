package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/soulteary/webhook/internal/rules"
)

// HookSummary hook 摘要信息，供前端展示
type HookSummary struct {
	ID             string   `json:"id"`
	ExecuteCommand string   `json:"executeCommand"`
	HTTPMethods    []string `json:"httpMethods"`
	HasTriggerRule bool     `json:"hasTriggerRule"`
}

type apiResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
	Msg  string      `json:"msg,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// newMux 创建 WebUI 的 HTTP 路由
func newMux(basePath, notifyConfigFile string) *http.ServeMux {
	mux := http.NewServeMux()

	// 静态页面
	mux.HandleFunc(basePath+"/", func(w http.ResponseWriter, r *http.Request) {
		// 只服务根路径，非 API 的请求都返回 SPA
		if strings.HasPrefix(r.URL.Path, basePath+"/api/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data, _ := staticFiles.ReadFile("static/index.html")
		_, _ = w.Write(data)
	})

	// GET /api/hooks — 列出所有已加载的 hook
	mux.HandleFunc(basePath+"/api/hooks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
			return
		}
		allHooks := rules.GetAllHooks()
		summaries := make([]HookSummary, 0, len(allHooks))
		for _, h := range allHooks {
			summaries = append(summaries, HookSummary{
				ID:             h.ID,
				ExecuteCommand: h.ExecuteCommand,
				HTTPMethods:    h.HTTPMethods,
				HasTriggerRule: h.TriggerRule != nil,
			})
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: summaries})
	})

	// POST /api/hooks/{id}/test — 发送测试请求触发 hook
	mux.HandleFunc(basePath+"/api/hooks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
			return
		}
		// 解析 /api/hooks/{id}/test
		suffix := strings.TrimPrefix(r.URL.Path, basePath+"/api/hooks/")
		parts := strings.SplitN(suffix, "/", 2)
		if len(parts) != 2 || parts[1] != "test" {
			http.NotFound(w, r)
			return
		}
		hookID := parts[0]
		if hookID == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{Msg: "hook id is required"})
			return
		}
		if rules.MatchLoadedHook(hookID) == nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Msg: fmt.Sprintf("hook %q not found", hookID)})
			return
		}
		// 用 HTTP client 向本服务自身发送测试请求
		// serverAddr 在 Handler 初始化时注入
		testURL := fmt.Sprintf("http://%s/hooks/%s?msg=测试消息&title=WebUI+Test", serverAddr, hookID)
		resp, err := http.Get(testURL) // #nosec G107 -- URL 由内部构造，hookID 已校验存在
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("test request failed: %v", err)})
			return
		}
		_ = resp.Body.Close()
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: fmt.Sprintf("test request sent to hook %q (status %d)", hookID, resp.StatusCode)})
	})

	// GET /api/logs — 返回执行日志
	mux.HandleFunc(basePath+"/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: ListLogs()})
	})

	// GET /api/config — 读取通知配置
	// POST /api/config — 保存通知配置
	mux.HandleFunc(basePath+"/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			cfg, err := LoadNotifyConfig(notifyConfigFile)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("load config failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: cfg})

		case http.MethodPost:
			var cfg NotifyConfig
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: fmt.Sprintf("invalid JSON: %v", err)})
				return
			}
			if err := SaveNotifyConfig(notifyConfigFile, &cfg); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("save config failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: "配置已保存"})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
		}
	})

	return mux
}

// serverAddr 由 Handler() 初始化时注入，用于构造自测试请求 URL
var serverAddr string
