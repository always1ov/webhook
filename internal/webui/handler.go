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

type server struct {
	basePath         string
	notifyConfigFile string
	customHooksFile  string
	hooksDir         string
	scriptsDir       string
	addr             string
}

func newServer(basePath, notifyConfigFile, customHooksFile, hooksDir, addr string) *server {
	return &server{
		basePath:         basePath,
		notifyConfigFile: notifyConfigFile,
		customHooksFile:  customHooksFile,
		hooksDir:         hooksDir,
		scriptsDir:       filepath.Join(filepath.Dir(customHooksFile), "scripts"),
		addr:             addr,
	}
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(s.basePath+"/", s.serveIndex)
	mux.HandleFunc(s.basePath+"/api/hooks", s.listHooks)
	mux.HandleFunc(s.basePath+"/api/hooks/", s.testHook)
	mux.HandleFunc(s.basePath+"/api/logs", s.listLogs)
	mux.HandleFunc(s.basePath+"/api/config", s.handleConfig)
	mux.HandleFunc(s.basePath+"/api/custom-hooks", s.handleCustomHooks)
	mux.HandleFunc(s.basePath+"/api/custom-hooks/", s.handleCustomHookByID)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func okData(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Data: data})
}

func okMsg(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: msg})
}

func okMsgData(w http.ResponseWriter, msg string, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{OK: true, Msg: msg, Data: data})
}

func fail(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Msg: msg})
}

func (s *server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, s.basePath+"/api/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, _ := staticFiles.ReadFile("static/index.html")
	_, _ = w.Write(data)
}

func (s *server) listHooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	all := rules.GetAllHooks()
	summaries := make([]HookSummary, 0, len(all))
	for _, h := range all {
		summaries = append(summaries, HookSummary{
			ID:             h.ID,
			ExecuteCommand: h.ExecuteCommand,
			HTTPMethods:    h.HTTPMethods,
			HasTriggerRule: h.TriggerRule != nil,
		})
	}
	okData(w, summaries)
}

func (s *server) testHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	suffix := strings.TrimPrefix(r.URL.Path, s.basePath+"/api/hooks/")
	parts := strings.SplitN(suffix, "/", 2)
	if len(parts) != 2 || parts[1] != "test" {
		http.NotFound(w, r)
		return
	}
	hookID := parts[0]
	if hookID == "" {
		fail(w, http.StatusBadRequest, "hook id is required")
		return
	}
	if rules.MatchLoadedHook(hookID) == nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("hook %q not found", hookID))
		return
	}
	testURL := fmt.Sprintf("http://%s/hooks/%s?msg=测试消息&title=WebUI+Test", s.addr, hookID)
	resp, err := http.Get(testURL) // #nosec G107 -- internal URL, hookID validated
	if err != nil {
		fail(w, http.StatusInternalServerError, fmt.Sprintf("test request failed: %v", err))
		return
	}
	_ = resp.Body.Close()
	okMsg(w, fmt.Sprintf("test request sent to hook %q (status %d)", hookID, resp.StatusCode))
}

func (s *server) listLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	okData(w, ListLogs())
}

func (s *server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := LoadNotifyConfig(s.notifyConfigFile)
		if err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("load config failed: %v", err))
			return
		}
		okData(w, cfg)
	case http.MethodPost:
		var cfg NotifyConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			fail(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		if err := SaveNotifyConfig(s.notifyConfigFile, &cfg); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("save config failed: %v", err))
			return
		}
		okMsg(w, "配置已保存")
	default:
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleCustomHooks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		hooks, err := LoadCustomHooks(s.customHooksFile)
		if err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
			return
		}
		okData(w, hooks)

	case http.MethodPost:
		var h CustomHook
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			fail(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		if err := ValidateCustomHookID(h.ID); err != nil {
			fail(w, http.StatusBadRequest, err.Error())
			return
		}
		if h.TargetURL == "" {
			fail(w, http.StatusBadRequest, "targetURL 不能为空")
			return
		}
		hooks, err := LoadCustomHooks(s.customHooksFile)
		if err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
			return
		}
		for _, existing := range hooks {
			if existing.ID == h.ID {
				fail(w, http.StatusConflict, fmt.Sprintf("ID %q 已存在", h.ID))
				return
			}
		}
		h.CreatedAt = time.Now()
		h.UpdatedAt = h.CreatedAt
		if err := WriteCustomHookFiles(s.hooksDir, s.scriptsDir, h); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("write files failed: %v", err))
			return
		}
		hooks = append(hooks, h)
		if err := SaveCustomHooks(s.customHooksFile, hooks); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
			return
		}
		okMsgData(w, fmt.Sprintf("推送 %q 已创建", h.ID), h)

	default:
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleCustomHookByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, s.basePath+"/api/custom-hooks/"), "/")
	if id == "" {
		fail(w, http.StatusBadRequest, "id is required")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var h CustomHook
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			fail(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}
		h.ID = id
		if h.TargetURL == "" {
			fail(w, http.StatusBadRequest, "targetURL 不能为空")
			return
		}
		hooks, err := LoadCustomHooks(s.customHooksFile)
		if err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
			return
		}
		idx := -1
		for i, existing := range hooks {
			if existing.ID == id {
				h.CreatedAt = existing.CreatedAt
				idx = i
				break
			}
		}
		if idx < 0 {
			fail(w, http.StatusNotFound, fmt.Sprintf("hook %q not found", id))
			return
		}
		h.UpdatedAt = time.Now()
		hooks[idx] = h
		if err := WriteCustomHookFiles(s.hooksDir, s.scriptsDir, h); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("write files failed: %v", err))
			return
		}
		if err := SaveCustomHooks(s.customHooksFile, hooks); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
			return
		}
		okMsgData(w, fmt.Sprintf("推送 %q 已更新", id), h)

	case http.MethodDelete:
		hooks, err := LoadCustomHooks(s.customHooksFile)
		if err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
			return
		}
		idx := -1
		for i, h := range hooks {
			if h.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			fail(w, http.StatusNotFound, fmt.Sprintf("hook %q not found", id))
			return
		}
		hooks = append(hooks[:idx], hooks[idx+1:]...)
		DeleteCustomHookFiles(s.hooksDir, s.scriptsDir, id)
		if err := SaveCustomHooks(s.customHooksFile, hooks); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
			return
		}
		okMsg(w, fmt.Sprintf("推送 %q 已删除", id))

	default:
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
