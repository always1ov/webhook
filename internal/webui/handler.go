package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

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
func newMux(basePath, notifyConfigFile, customHooksFile, hooksDir string) *http.ServeMux {
	mux := http.NewServeMux()
	scriptsDir := filepath.Join(filepath.Dir(customHooksFile), "scripts")

	// 静态页面
	mux.HandleFunc(basePath+"/", func(w http.ResponseWriter, r *http.Request) {
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

	// GET  /api/custom-hooks        — 列出自定义推送 Hook
	// POST /api/custom-hooks        — 新增自定义推送 Hook
	mux.HandleFunc(basePath+"/api/custom-hooks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			hooks, err := LoadCustomHooks(customHooksFile)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("load failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: hooks})

		case http.MethodPost:
			var h CustomHook
			if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: fmt.Sprintf("invalid JSON: %v", err)})
				return
			}
			if err := ValidateCustomHookID(h.ID); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: err.Error()})
				return
			}
			if h.TargetURL == "" {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: "targetURL 不能为空"})
				return
			}
			hooks, err := LoadCustomHooks(customHooksFile)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("load failed: %v", err)})
				return
			}
			for _, existing := range hooks {
				if existing.ID == h.ID {
					writeJSON(w, http.StatusConflict, apiResponse{Msg: fmt.Sprintf("ID %q 已存在", h.ID)})
					return
				}
			}
			h.CreatedAt = time.Now()
			h.UpdatedAt = h.CreatedAt
			if err := WriteCustomHookFiles(hooksDir, scriptsDir, h); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("write files failed: %v", err)})
				return
			}
			hooks = append(hooks, h)
			if err := SaveCustomHooks(customHooksFile, hooks); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("save failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: fmt.Sprintf("推送 %q 已创建", h.ID), Data: h})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
		}
	})

	// PUT    /api/custom-hooks/{id} — 更新自定义推送 Hook
	// DELETE /api/custom-hooks/{id} — 删除自定义推送 Hook
	mux.HandleFunc(basePath+"/api/custom-hooks/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, basePath+"/api/custom-hooks/")
		id = strings.TrimSuffix(id, "/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{Msg: "id is required"})
			return
		}

		switch r.Method {
		case http.MethodPut:
			var h CustomHook
			if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: fmt.Sprintf("invalid JSON: %v", err)})
				return
			}
			h.ID = id
			if h.TargetURL == "" {
				writeJSON(w, http.StatusBadRequest, apiResponse{Msg: "targetURL 不能为空"})
				return
			}
			hooks, err := LoadCustomHooks(customHooksFile)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("load failed: %v", err)})
				return
			}
			found := false
			for i, existing := range hooks {
				if existing.ID == id {
					h.CreatedAt = existing.CreatedAt
					h.UpdatedAt = time.Now()
					hooks[i] = h
					found = true
					break
				}
			}
			if !found {
				writeJSON(w, http.StatusNotFound, apiResponse{Msg: fmt.Sprintf("hook %q not found", id)})
				return
			}
			if err := WriteCustomHookFiles(hooksDir, scriptsDir, h); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("write files failed: %v", err)})
				return
			}
			if err := SaveCustomHooks(customHooksFile, hooks); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("save failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: fmt.Sprintf("推送 %q 已更新", id), Data: h})

		case http.MethodDelete:
			hooks, err := LoadCustomHooks(customHooksFile)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("load failed: %v", err)})
				return
			}
			newHooks := hooks[:0]
			found := false
			for _, h := range hooks {
				if h.ID == id {
					found = true
				} else {
					newHooks = append(newHooks, h)
				}
			}
			if !found {
				writeJSON(w, http.StatusNotFound, apiResponse{Msg: fmt.Sprintf("hook %q not found", id)})
				return
			}
			DeleteCustomHookFiles(hooksDir, scriptsDir, id)
			if err := SaveCustomHooks(customHooksFile, newHooks); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Msg: fmt.Sprintf("save failed: %v", err)})
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: fmt.Sprintf("推送 %q 已删除", id)})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, apiResponse{Msg: "method not allowed"})
		}
	})

	return mux
}

// serverAddr 由 Handler() 初始化时注入，用于构造自测试请求 URL
var serverAddr string
