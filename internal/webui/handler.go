package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	IsCustom       bool     `json:"isCustom"`
}

type apiResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data,omitempty"`
	Msg  string      `json:"msg,omitempty"`
}

type server struct {
	basePath           string
	customHooksFile    string
	hooksDir           string
	scriptsDir         string
	notifyTargetsFile  string
	addr               string
}

func newServer(basePath, customHooksFile, hooksDir, notifyTargetsFile, addr string) *server {
	return &server{
		basePath:          basePath,
		customHooksFile:   customHooksFile,
		hooksDir:          hooksDir,
		scriptsDir:        filepath.Join(filepath.Dir(customHooksFile), "scripts"),
		notifyTargetsFile: notifyTargetsFile,
		addr:              addr,
	}
}

func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(s.basePath+"/", s.serveIndex)
	mux.HandleFunc(s.basePath+"/api/hooks", s.listHooks)
	mux.HandleFunc(s.basePath+"/api/hooks/", s.handleHookByID)
	mux.HandleFunc(s.basePath+"/api/logs", s.listLogs)
	mux.HandleFunc(s.basePath+"/api/custom-hooks", s.handleCustomHooks)
	mux.HandleFunc(s.basePath+"/api/custom-hooks/", s.handleCustomHookByID)
	mux.HandleFunc(s.basePath+"/api/notify-targets", s.handleAllNotifyTargets)
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
	customHooks, _ := LoadCustomHooks(s.customHooksFile)
	customIDs := make(map[string]bool, len(customHooks))
	for _, ch := range customHooks {
		customIDs[ch.ID] = true
	}
	all := rules.GetAllHooks()
	summaries := make([]HookSummary, 0, len(all))
	for _, h := range all {
		summaries = append(summaries, HookSummary{
			ID:             h.ID,
			ExecuteCommand: h.ExecuteCommand,
			HTTPMethods:    h.HTTPMethods,
			HasTriggerRule: h.TriggerRule != nil,
			IsCustom:       customIDs[h.ID],
		})
	}
	okData(w, summaries)
}

func (s *server) handleHookByID(w http.ResponseWriter, r *http.Request) {
	suffix := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, s.basePath+"/api/hooks/"), "/")

	// GET|PUT /{id}/script
	if strings.HasSuffix(suffix, "/script") {
		hookID := strings.TrimSuffix(suffix, "/script")
		if hookID == "" {
			fail(w, http.StatusBadRequest, "hook id is required")
			return
		}
		h := rules.MatchLoadedHook(hookID)
		if h == nil {
			fail(w, http.StatusNotFound, fmt.Sprintf("hook %q not found", hookID))
			return
		}
		scriptPath := h.ExecuteCommand
		switch r.Method {
		case http.MethodGet:
			data, err := os.ReadFile(scriptPath)
			if err != nil {
				fail(w, http.StatusInternalServerError, fmt.Sprintf("read script failed: %v", err))
				return
			}
			okData(w, map[string]string{"content": string(data), "path": scriptPath})
		case http.MethodPut:
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				fail(w, http.StatusBadRequest, "invalid JSON")
				return
			}
			dir := filepath.Dir(scriptPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fail(w, http.StatusInternalServerError, "mkdir failed")
				return
			}
			tmp, err := os.CreateTemp(dir, ".script-*.tmp")
			if err != nil {
				fail(w, http.StatusInternalServerError, "write failed")
				return
			}
			name := tmp.Name()
			if _, err := tmp.WriteString(body.Content); err != nil {
				_ = tmp.Close()
				_ = os.Remove(name)
				fail(w, http.StatusInternalServerError, "write failed")
				return
			}
			_ = tmp.Close()
			_ = os.Chmod(name, 0755)
			if err := os.Rename(name, scriptPath); err != nil {
				_ = os.Remove(name)
				fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
				return
			}
			okMsg(w, "脚本已保存")
		default:
			fail(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// GET|PUT /{id}/targets
	if strings.HasSuffix(suffix, "/targets") {
		hookID := strings.TrimSuffix(suffix, "/targets")
		if hookID == "" {
			fail(w, http.StatusBadRequest, "hook id is required")
			return
		}
		switch r.Method {
		case http.MethodGet:
			targets, err := GetHookTargets(s.notifyTargetsFile, hookID)
			if err != nil {
				fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
				return
			}
			okData(w, targets)
		case http.MethodPut:
			var targets []NotifyTarget
			if err := json.NewDecoder(r.Body).Decode(&targets); err != nil {
				fail(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
				return
			}
			for i, t := range targets {
				if t.ID == "" {
					targets[i].ID = fmt.Sprintf("%d", time.Now().UnixNano()+int64(i))
				}
			}
			if err := SetHookTargets(s.notifyTargetsFile, hookID, targets); err != nil {
				fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
				return
			}
			okMsgData(w, fmt.Sprintf("通知目标已保存（%d 个）", len(targets)), targets)
		default:
			fail(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// POST /{id}/test
	if strings.HasSuffix(suffix, "/test") && r.Method == http.MethodPost {
		hookID := strings.TrimSuffix(suffix, "/test")
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
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			okMsg(w, fmt.Sprintf("Hook \"%s\" 测试成功，通知已发送", hookID))
		} else {
			fail(w, http.StatusBadGateway, fmt.Sprintf("Hook \"%s\" 返回异常状态码 %d", hookID, resp.StatusCode))
		}
		return
	}

	// DELETE /{id}
	hookID := suffix
	if hookID == "" {
		fail(w, http.StatusBadRequest, "hook id is required")
		return
	}
	if r.Method != http.MethodDelete {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if rules.MatchLoadedHook(hookID) == nil {
		fail(w, http.StatusNotFound, fmt.Sprintf("hook %q not found", hookID))
		return
	}

	// 自定义 hook：完整删除（JSON记录 + YAML + 脚本）
	hooks, err := LoadCustomHooks(s.customHooksFile)
	if err != nil {
		fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
		return
	}
	idx := -1
	for i, h := range hooks {
		if h.ID == hookID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		hooks = append(hooks[:idx], hooks[idx+1:]...)
		DeleteCustomHookFiles(s.hooksDir, s.scriptsDir, hookID)
		if err := SaveCustomHooks(s.customHooksFile, hooks); err != nil {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("save failed: %v", err))
			return
		}
	} else {
		// 内置 hook：只删除 YAML 文件
		yamlPath := filepath.Join(s.hooksDir, hookID+".yaml")
		if err := os.Remove(yamlPath); err != nil && !os.IsNotExist(err) {
			fail(w, http.StatusInternalServerError, fmt.Sprintf("delete failed: %v", err))
			return
		}
	}
	// 清理该 hook 的通知目标
	_ = SetHookTargets(s.notifyTargetsFile, hookID, nil)
	okMsg(w, fmt.Sprintf("Hook \"%s\" 已删除", hookID))
}

func (s *server) listLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	okData(w, ListLogs())
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
		_ = SetHookTargets(s.notifyTargetsFile, id, nil)
		okMsg(w, fmt.Sprintf("推送 %q 已删除", id))

	default:
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAllNotifyTargets returns the full hookID→targets map for the UI to compute per-hook counts.
func (s *server) handleAllNotifyTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		fail(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	m, err := LoadHookTargets(s.notifyTargetsFile)
	if err != nil {
		fail(w, http.StatusInternalServerError, fmt.Sprintf("load failed: %v", err))
		return
	}
	okData(w, m)
}
