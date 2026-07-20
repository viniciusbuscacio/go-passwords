package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	apiserver "github.com/viniciusbuscacio/go-apiserver"
	"github.com/viniciusbuscacio/go-passwords/internal/config"
	"github.com/viniciusbuscacio/go-passwords/internal/vault"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const actorAPI = "api"

// ---- Server lifecycle (bound to the frontend) ----

type APIStatus struct {
	Running     bool   `json:"running"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
	TLS         bool   `json:"tls"`
	Fingerprint string `json:"fingerprint"`
}

func apiURL(cfg config.Config) string {
	host := "127.0.0.1"
	if apiserver.BindHost(cfg.APIAllowlist) == "0.0.0.0" {
		host = apiserver.OutboundIP()
	}
	scheme := "http"
	if cfg.APIHTTPS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, cfg.APIPort)
}

func (a *App) APIState() APIStatus {
	cfg := a.snapshot()
	return APIStatus{
		Running:     a.server.Running(),
		Port:        cfg.APIPort,
		URL:         apiURL(cfg),
		TLS:         cfg.APIHTTPS,
		Fingerprint: a.server.Fingerprint(),
	}
}

// emitAPIState pushes the live server status to the frontend ("api:state"),
// so passive UI like the titlebar indicator stays honest without polling.
func (a *App) emitAPIState() {
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "api:state", a.APIState())
	}
}

func (a *App) startServer() error {
	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	cfg := a.snapshot()
	return a.server.Start(apiserver.Config{
		Port:      cfg.APIPort,
		Key:       cfg.APIKey,
		Allowlist: cfg.APIAllowlist,
		TLS:       cfg.APIHTTPS,
		CertDir:   dir,
		AppName:   "go-passwords",
	})
}

// GetAPIStatus returns the live server status (family API shape).
func (a *App) GetAPIStatus() APIStatus {
	return a.APIState()
}

// StartAPIServer / StopAPIServer back the play button.
func (a *App) StartAPIServer() (APIStatus, error) {
	err := a.startServer()
	a.emitAPIState()
	return a.APIState(), err
}

func (a *App) StopAPIServer() (APIStatus, error) {
	err := a.server.Stop()
	a.emitAPIState()
	return a.APIState(), err
}

// Port range of go-passwords in the family (calc 87xx, notepad 88xx).
const (
	portRangeBase = 8900
	portRangeSpan = 100
)

// pickFreePort returns a bindable port in the family range, different from
// current when possible.
func pickFreePort(current int, host string) int {
	for i := 0; i < portRangeSpan; i++ {
		p := portRangeBase + i
		if p == current {
			continue
		}
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
		if err == nil {
			l.Close()
			return p
		}
	}
	return current
}

// ShuffleAPIPort picks a random free port and restarts the server if running.
func (a *App) ShuffleAPIPort() (APIStatus, error) {
	cur := a.snapshot()
	port := pickFreePort(cur.APIPort, apiserver.BindHost(cur.APIAllowlist))
	if err := a.mutate(func(c *config.Config) { c.APIPort = port }); err != nil {
		return a.APIState(), err
	}
	return a.APIState(), a.applyIfRunning()
}

// SetHTTPS toggles TLS and restarts a running server so the change applies.
func (a *App) SetHTTPS(v bool) (APIStatus, error) {
	if err := a.mutate(func(c *config.Config) { c.APIHTTPS = v }); err != nil {
		return a.APIState(), err
	}
	return a.APIState(), a.applyIfRunning()
}

func (a *App) SetAPIAutoStart(on bool) error {
	return a.mutate(func(c *config.Config) { c.APIAutoStart = on })
}

func (a *App) RotateAPIKey() (string, error) {
	a.mu.Lock()
	a.cfg.APIKey = config.GenerateKey()
	cfg := a.cfg
	a.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		return "", err
	}
	return cfg.APIKey, a.applyIfRunning()
}

// AddAllowlistEntry validates and adds a CIDR, returning the new list.
func (a *App) AddAllowlistEntry(entry string) ([]string, error) {
	normalized, err := apiserver.NormalizeCIDR(entry)
	if err != nil {
		return a.snapshot().APIAllowlist, err
	}
	a.mu.Lock()
	for _, e := range a.cfg.APIAllowlist {
		if e == normalized {
			list := a.cfg.APIAllowlist
			a.mu.Unlock()
			return list, nil
		}
	}
	list := append(append([]string{}, a.cfg.APIAllowlist...), normalized)
	a.cfg.APIAllowlist = list
	cfg := a.cfg
	a.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		return list, err
	}
	return list, a.applyIfRunning()
}

// RemoveAllowlistEntry removes a CIDR; the last entry cannot be removed (a
// keyed server with an empty allowlist would reject everyone).
func (a *App) RemoveAllowlistEntry(entry string) ([]string, error) {
	a.mu.Lock()
	if len(a.cfg.APIAllowlist) <= 1 {
		list := a.cfg.APIAllowlist
		a.mu.Unlock()
		return list, fmt.Errorf("cannot remove the last allowlist entry")
	}
	list := make([]string, 0, len(a.cfg.APIAllowlist))
	for _, e := range a.cfg.APIAllowlist {
		if e != entry {
			list = append(list, e)
		}
	}
	a.cfg.APIAllowlist = list
	cfg := a.cfg
	a.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		return list, err
	}
	return list, a.applyIfRunning()
}

func (a *App) applyIfRunning() error {
	defer a.emitAPIState()
	if !a.server.Running() {
		return nil
	}
	if err := a.server.Stop(); err != nil {
		return err
	}
	return a.startServer()
}

// ---- Domain endpoints (the agent-facing REST API) ----

func (a *App) registerAPI() {
	a.server.HandleExtra("/v1/status", a.handleStatus)
	a.server.HandleExtra("/v1/unlock", a.handleUnlock)
	a.server.HandleExtra("/v1/lock", a.handleLock)
	a.server.HandleExtra("/v1/secrets", a.handleSecrets)
	a.server.HandleExtra("/v1/secrets/{id}", a.handleSecretByID)
	a.server.HandleExtra("/v1/categories", a.handleCategories)
	a.server.HandleExtra("/v1/categories/{id}", a.handleCategoryByID)
	a.server.HandleExtra("/v1/generate", a.handleGenerate)
	a.server.HandleExtra("/v1/export", a.handleExport)
	a.server.HandleExtra("/v1/audit", a.handleAudit)
	a.server.HandleExtra("/v1/update", a.handleUpdate)
}

// decodeBody decodes a JSON request body without DecodeJSON's POST-only
// method check, for PUT endpoints. Same 1 MiB cap and structured error.
func decodeBody(w http.ResponseWriter, r *http.Request, req any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		apiserver.WriteErr(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
		return false
	}
	return true
}

// apiVault returns the unlocked vault or writes 423 Locked.
func (a *App) apiVault(w http.ResponseWriter) *vault.Vault {
	v, err := a.current()
	if err != nil {
		apiserver.WriteErr(w, http.StatusLocked, "vault_locked", "the vault is locked — POST /v1/unlock first")
		return nil
	}
	return v
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	a.mu.Lock()
	unlocked := a.v != nil
	path := a.cfg.LastVault
	a.mu.Unlock()
	apiserver.WriteJSON(w, http.StatusOK, map[string]any{
		"unlocked": unlocked,
		"vault":    path,
		"version":  appVersion,
	})
}

func (a *App) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Vault          string `json:"vault"`
		MasterPassword string `json:"master_password"`
	}
	if !apiserver.DecodeJSON(w, r, &req) {
		return
	}
	path := req.Vault
	if path == "" {
		path = a.LastVault()
	}
	if path == "" {
		apiserver.WriteErr(w, http.StatusBadRequest, "missing_field", "field 'vault' is required (no last vault known)")
		return
	}
	v, err := vault.Open(path, req.MasterPassword)
	if err != nil {
		apiserver.WriteErr(w, http.StatusUnauthorized, "invalid_password", err.Error())
		return
	}
	v.RecordUnlock(actorAPI)
	_ = v.Save()
	a.setVault(v, path)
	if a.ctx != nil {
		// Let the GUI switch from the unlock screen to the vault view.
		a.notifyUnlocked()
	}
	apiserver.WriteJSON(w, http.StatusOK, map[string]any{"unlocked": true, "vault": path})
}

func (a *App) handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	a.LockVault()
	apiserver.WriteJSON(w, http.StatusOK, map[string]any{"unlocked": false})
}

func (a *App) handleSecrets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		v := a.apiVault(w)
		if v == nil {
			return
		}
		apiserver.WriteJSON(w, http.StatusOK, v.ListSecrets(r.URL.Query().Get("q")))
	case http.MethodPost:
		v := a.apiVault(w)
		if v == nil {
			return
		}
		var in vault.SecretInput
		if !apiserver.DecodeJSON(w, r, &in) {
			return
		}
		s, err := v.AddSecret(in, actorAPI)
		if err != nil {
			apiserver.WriteErr(w, http.StatusUnprocessableEntity, "invalid_secret", err.Error())
			return
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusCreated, s)
	default:
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST")
	}
}

func (a *App) handleSecretByID(w http.ResponseWriter, r *http.Request) {
	v := a.apiVault(w)
	if v == nil {
		return
	}
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		s, err := v.GetSecret(id, actorAPI)
		if err != nil {
			apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		_ = v.Save()
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusOK, s)
	case http.MethodPut:
		var in vault.SecretInput
		if !decodeBody(w, r, &in) {
			return
		}
		s, err := v.UpdateSecret(id, in, actorAPI)
		if err != nil {
			apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		if err := v.DeleteSecret(id, actorAPI); err != nil {
			apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET, PUT or DELETE")
	}
}

func (a *App) handleCategories(w http.ResponseWriter, r *http.Request) {
	v := a.apiVault(w)
	if v == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		apiserver.WriteJSON(w, http.StatusOK, v.ListCategories())
	case http.MethodPost:
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if !apiserver.DecodeJSON(w, r, &req) {
			return
		}
		c, err := v.AddCategory(req.Name, req.Color, actorAPI)
		if err != nil {
			apiserver.WriteErr(w, http.StatusUnprocessableEntity, "invalid_category", err.Error())
			return
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusCreated, c)
	default:
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST")
	}
}

func (a *App) handleCategoryByID(w http.ResponseWriter, r *http.Request) {
	v := a.apiVault(w)
	if v == nil {
		return
	}
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodPut:
		var req struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if !decodeBody(w, r, &req) {
			return
		}
		if req.Name != "" {
			if err := v.RenameCategory(id, req.Name, actorAPI); err != nil {
				apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
		}
		if req.Color != "" {
			if err := v.SetCategoryColor(id, req.Color, actorAPI); err != nil {
				apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusOK, map[string]bool{"renamed": true})
	case http.MethodDelete:
		if err := v.DeleteCategory(id, actorAPI); err != nil {
			apiserver.WriteErr(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		if err := v.Save(); err != nil {
			apiserver.WriteErr(w, http.StatusInternalServerError, "save_failed", err.Error())
			return
		}
		a.notifyChanged()
		apiserver.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "PUT or DELETE")
	}
}

func (a *App) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST only")
		return
	}
	var req struct {
		Length  int   `json:"length"`
		Symbols *bool `json:"symbols"`
	}
	if !apiserver.DecodeJSON(w, r, &req) {
		return
	}
	o := vault.DefaultGenerateOptions()
	if req.Length > 0 {
		o.Length = req.Length
	}
	if req.Symbols != nil {
		o.Symbols = *req.Symbols
	}
	p, err := vault.GeneratePassword(o)
	if err != nil {
		apiserver.WriteErr(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
		return
	}
	apiserver.WriteJSON(w, http.StatusOK, map[string]string{"password": p})
}

func (a *App) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	v := a.apiVault(w)
	if v == nil {
		return
	}
	dump, err := v.Export(actorAPI)
	if err != nil {
		apiserver.WriteErr(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	_ = v.Save()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(dump)
}

func (a *App) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apiserver.WriteErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
		return
	}
	v := a.apiVault(w)
	if v == nil {
		return
	}
	apiserver.WriteJSON(w, http.StatusOK, v.AuditLog())
}
