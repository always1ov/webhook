package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/robfig/cron/v3"
)

type CronJob struct {
	HookID   string `json:"hookId"`
	Schedule string `json:"schedule"` // standard cron expression, e.g. "*/5 * * * *"
	Enabled  bool   `json:"enabled"`
}

type CronJobMap map[string]CronJob // keyed by hookID

var (
	globalCron     *cron.Cron
	globalCronMu   sync.Mutex
	globalCronFile string
	globalCronAddr string
	cronEntryIDs   = map[string]cron.EntryID{}
)

func StartCronScheduler(cronFile, addr string) {
	globalCronFile = cronFile
	globalCronAddr = addr
	globalCron = cron.New()
	globalCron.Start()
	reloadCronJobs()
}

func reloadCronJobs() {
	globalCronMu.Lock()
	defer globalCronMu.Unlock()

	// remove all existing entries
	for _, id := range cronEntryIDs {
		globalCron.Remove(id)
	}
	cronEntryIDs = map[string]cron.EntryID{}

	jobs, err := LoadCronJobs(globalCronFile)
	if err != nil {
		return
	}
	for _, job := range jobs {
		if !job.Enabled || job.Schedule == "" {
			continue
		}
		hookID := job.HookID
		addr := globalCronAddr
		entryID, err := globalCron.AddFunc(job.Schedule, func() {
			triggerHookHTTP(hookID, addr)
		})
		if err == nil {
			cronEntryIDs[hookID] = entryID
		}
	}
}

func triggerHookHTTP(hookID, addr string) {
	url := fmt.Sprintf("http://%s/hooks/%s?msg=定时触发&title=定时任务", addr, hookID)
	resp, err := http.Get(url) // #nosec G107
	if err == nil {
		_ = resp.Body.Close()
	}
}

func LoadCronJobs(file string) (CronJobMap, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return CronJobMap{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m CronJobMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = CronJobMap{}
	}
	return m, nil
}

func SetCronJob(file, hookID, schedule string, enabled bool) error {
	m, err := LoadCronJobs(file)
	if err != nil {
		return err
	}
	if schedule == "" {
		delete(m, hookID)
	} else {
		m[hookID] = CronJob{HookID: hookID, Schedule: schedule, Enabled: enabled}
	}
	if err := saveCronJobs(file, m); err != nil {
		return err
	}
	reloadCronJobs()
	return nil
}

func DeleteCronJob(file, hookID string) error {
	m, err := LoadCronJobs(file)
	if err != nil {
		return err
	}
	delete(m, hookID)
	if err := saveCronJobs(file, m); err != nil {
		return err
	}
	reloadCronJobs()
	return nil
}

func saveCronJobs(file string, m CronJobMap) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".cron-jobs-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	_ = tmp.Close()
	return os.Rename(name, file)
}

// ValidateCronSchedule checks if a cron expression is valid.
func ValidateCronSchedule(expr string) error {
	_, err := cron.ParseStandard(expr)
	return err
}
