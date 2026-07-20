package main

// The in-app updater's thin Wails adapter (family pattern — see go-notepad
// docs/updater-design.md). All real work lives in the shared
// github.com/viniciusbuscacio/go-updates library; this file owns the app-side
// state machine and pushes every change to the frontend as an "update:state"
// event, so the UI and GET /v1/update always agree.

import (
	"fmt"
	"net/http"
	"os"
	"time"

	apiserver "github.com/viniciusbuscacio/go-apiserver"
	"github.com/viniciusbuscacio/go-passwords/internal/config"
	updater "github.com/viniciusbuscacio/go-updates"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	updateRepo     = "viniciusbuscacio/go-passwords"
	laterCooldown  = 7 * 24 * time.Hour
	autoCheckEvery = 24 * time.Hour
)

// UpdateInfo is the single update snapshot shared by the frontend (binding +
// "update:state" event) and GET /v1/update.
type UpdateInfo struct {
	Checking   bool   `json:"checking"`
	Installing bool   `json:"installing"`
	Progress   string `json:"progress"`
	Available  bool   `json:"available"`
	Version    string `json:"version"`
	Notes      string `json:"notes"`
	Current    string `json:"current"`
	CheckedAt  string `json:"checkedAt"`
	Error      string `json:"error"`
	Notify     bool   `json:"notify"`
}

func (a *App) updateClient() *updater.Client {
	return &updater.Client{
		BaseURL: os.Getenv("GO_PASSWORDS_UPDATE_URL"),
		Repo:    updateRepo,
		App:     "go-passwords",
	}
}

func (a *App) setUpdState(fn func(*UpdateInfo)) UpdateInfo {
	a.mu.Lock()
	fn(&a.updState)
	a.updState.Current = appVersion
	info := a.updState
	cfg := a.cfg
	a.mu.Unlock()
	info.Notify = computeNotify(info, cfg)
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "update:state", info)
	}
	return info
}

func computeNotify(info UpdateInfo, cfg config.Config) bool {
	if !info.Available || info.Installing {
		return false
	}
	if cfg.UpdateSkipped != "" && cfg.UpdateSkipped == "v"+info.Version {
		return false
	}
	if until, err := time.Parse(time.RFC3339, cfg.UpdateLaterUntil); err == nil && time.Now().Before(until) {
		return false
	}
	return true
}

// GetUpdateInfo returns the current snapshot without touching the network.
func (a *App) GetUpdateInfo() UpdateInfo {
	a.mu.Lock()
	info := a.updState
	info.Current = appVersion
	cfg := a.cfg
	a.mu.Unlock()
	info.Notify = computeNotify(info, cfg)
	return info
}

// CheckForUpdates asks GitHub for the latest release, right now.
func (a *App) CheckForUpdates() UpdateInfo {
	a.setUpdState(func(u *UpdateInfo) {
		u.Checking = true
		u.Error = ""
	})
	rel, newer, err := a.updateClient().Check(appVersion)
	now := time.Now().UTC().Format(time.RFC3339)
	_ = a.mutate(func(c *config.Config) { c.UpdateLastCheck = now })
	return a.setUpdState(func(u *UpdateInfo) {
		u.Checking = false
		u.CheckedAt = now
		if err != nil {
			u.Error = err.Error()
			return
		}
		u.Available = newer
		u.Version = rel.Version
		u.Notes = rel.Notes
		a.updRelease = rel
	})
}

// InstallUpdate downloads, verifies and applies the available update, then
// relaunches the new binary and closes this one.
func (a *App) InstallUpdate() error {
	a.mu.Lock()
	rel := a.updRelease
	installing := a.updState.Installing
	a.mu.Unlock()
	if installing {
		return fmt.Errorf("an install is already in progress")
	}
	if rel == nil || !a.GetUpdateInfo().Available {
		return fmt.Errorf("no update available to install; check for updates first")
	}

	a.setUpdState(func(u *UpdateInfo) {
		u.Installing = true
		u.Error = ""
	})
	client := a.updateClient()
	err := client.Install(rel, func(stage string) {
		a.setUpdState(func(u *UpdateInfo) { u.Progress = stage })
	})
	if err != nil {
		a.setUpdState(func(u *UpdateInfo) {
			u.Installing = false
			u.Progress = ""
			u.Error = err.Error()
		})
		return err
	}
	if err := client.Relaunch(); err != nil {
		a.setUpdState(func(u *UpdateInfo) {
			u.Installing = false
			u.Progress = ""
			u.Error = "updated, but the new version could not be started: " + err.Error() +
				"; it will be used the next time you open the app"
		})
		return err
	}
	wruntime.Quit(a.ctx)
	return nil
}

// SkipUpdateVersion silences the currently available version.
func (a *App) SkipUpdateVersion() error {
	info := a.GetUpdateInfo()
	if !info.Available {
		return fmt.Errorf("no update available to skip")
	}
	if err := a.mutate(func(c *config.Config) { c.UpdateSkipped = "v" + info.Version }); err != nil {
		return err
	}
	a.setUpdState(func(u *UpdateInfo) {})
	return nil
}

// RemindUpdateLater snoozes the notice for 7 days.
func (a *App) RemindUpdateLater() error {
	until := time.Now().Add(laterCooldown).UTC().Format(time.RFC3339)
	if err := a.mutate(func(c *config.Config) { c.UpdateLaterUntil = until }); err != nil {
		return err
	}
	a.setUpdState(func(u *UpdateInfo) {})
	return nil
}

// SetUpdateAutoCheck toggles the once-a-day automatic check.
func (a *App) SetUpdateAutoCheck(v bool) error {
	return a.mutate(func(c *config.Config) { c.UpdateAutoCheck = v })
}

// maybeAutoCheck runs the startup check when the user opted in and the last
// one is old enough.
func (a *App) maybeAutoCheck() {
	cfg := a.snapshot()
	if !cfg.UpdateAutoCheck {
		return
	}
	if last, err := time.Parse(time.RFC3339, cfg.UpdateLastCheck); err == nil && time.Since(last) < autoCheckEvery {
		return
	}
	a.CheckForUpdates()
}

// handleUpdate serves GET /v1/update: the same snapshot the UI renders.
func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	apiserver.WriteJSON(w, http.StatusOK, a.GetUpdateInfo())
}
