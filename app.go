package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	apiserver "github.com/viniciusbuscacio/go-apiserver"
	"github.com/viniciusbuscacio/go-passwords/internal/config"
	"github.com/viniciusbuscacio/go-passwords/internal/vault"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const actorGUI = "gui"

// App is the thin Wails adapter. Vault logic lives in internal/vault; App
// wires it to the frontend and the REST server and owns process-level state.
type App struct {
	ctx context.Context

	// mu guards cfg and v: Wails-bound methods and the REST server's handlers
	// run on different goroutines.
	mu  sync.Mutex
	cfg config.Config
	v   *vault.Vault

	server *apiserver.Server
	ui     *uiBridge

	lockTimer *time.Timer
}

func NewApp() *App {
	a := &App{}
	a.ui = newUIBridge(a)
	a.server = apiserver.New(a.appInfo, a.ui)
	a.registerAPI()
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mu.Lock()
	a.cfg = config.Load()
	cfg := a.cfg
	a.mu.Unlock()
	if cfg.APIAutoStart {
		_ = a.startServer()
	}
}

// UIAck is called by the frontend to report the resulting screen state after
// executing a ui:command.
func (a *App) UIAck(id string, state string) {
	a.ui.ack(id, json.RawMessage(state))
}

// ---- Vault session ----

// current returns the unlocked vault or an error. Callers touch the vault
// only through this so the auto-lock timer is reset on every use.
func (a *App) current() (*vault.Vault, error) {
	a.mu.Lock()
	v := a.v
	a.mu.Unlock()
	if v == nil {
		return nil, fmt.Errorf("vault is locked")
	}
	a.resetLockTimer()
	return v, nil
}

func (a *App) setVault(v *vault.Vault, path string) {
	a.mu.Lock()
	a.v = v
	a.cfg.LastVault = path
	cfg := a.cfg
	a.mu.Unlock()
	_ = config.Save(cfg)
	a.resetLockTimer()
}

// IsUnlocked reports whether a vault is open.
func (a *App) IsUnlocked() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.v != nil
}

// CreateVault makes a new vault at path and leaves it unlocked.
func (a *App) CreateVault(path, masterPassword string) error {
	if path == "" {
		return fmt.Errorf("choose where to save the vault")
	}
	v, err := vault.Create(path, masterPassword)
	if err != nil {
		return err
	}
	v.RecordUnlock(actorGUI)
	_ = v.Save()
	a.setVault(v, path)
	return nil
}

// UnlockVault opens the vault at path.
func (a *App) UnlockVault(path, masterPassword string) error {
	if path == "" {
		return fmt.Errorf("choose a vault file")
	}
	v, err := vault.Open(path, masterPassword)
	if err != nil {
		return err
	}
	v.RecordUnlock(actorGUI)
	_ = v.Save()
	a.setVault(v, path)
	return nil
}

// LockVault wipes the in-memory keys and tells the frontend.
func (a *App) LockVault() {
	a.mu.Lock()
	v := a.v
	a.v = nil
	a.mu.Unlock()
	a.stopLockTimer()
	if v != nil {
		v.Lock()
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "vault:locked")
	}
}

// notifyUnlocked tells the frontend the vault was opened from outside the GUI
// (the REST API) so it can leave the unlock screen.
func (a *App) notifyUnlocked() {
	runtime.EventsEmit(a.ctx, "vault:unlocked")
}

// notifyChanged tells the frontend the vault content changed from outside the
// GUI (a REST API write) so open lists re-fetch.
func (a *App) notifyChanged() {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "vault:changed")
	}
}

// LastVault returns the most recently opened vault path.
func (a *App) LastVault() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.LastVault
}

// SelectVaultToOpen shows a file-open dialog and returns the chosen path.
func (a *App) SelectVaultToOpen() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Open vault",
		Filters: []runtime.FileFilter{
			{DisplayName: "go-passwords vault (*.gpw)", Pattern: "*.gpw"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

// SelectVaultToCreate shows a file-save dialog and returns the chosen path.
func (a *App) SelectVaultToCreate() (string, error) {
	return runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Create vault",
		DefaultFilename: "vault.gpw",
		Filters: []runtime.FileFilter{
			{DisplayName: "go-passwords vault (*.gpw)", Pattern: "*.gpw"},
		},
	})
}

// ---- Auto-lock ----

func (a *App) resetLockTimer() {
	a.mu.Lock()
	cfg := a.cfg
	t := a.lockTimer
	a.mu.Unlock()
	if t != nil {
		t.Stop()
	}
	if !cfg.AutoLockEnabled || cfg.AutoLockMinutes <= 0 {
		return
	}
	nt := time.AfterFunc(time.Duration(cfg.AutoLockMinutes)*time.Minute, a.LockVault)
	a.mu.Lock()
	a.lockTimer = nt
	a.mu.Unlock()
}

func (a *App) stopLockTimer() {
	a.mu.Lock()
	t := a.lockTimer
	a.lockTimer = nil
	a.mu.Unlock()
	if t != nil {
		t.Stop()
	}
}

// ---- Secrets (bound to the frontend) ----

func (a *App) ListSecrets(q string) ([]vault.Secret, error) {
	v, err := a.current()
	if err != nil {
		return nil, err
	}
	return v.ListSecrets(q), nil
}

func (a *App) GetSecret(id string) (vault.Secret, error) {
	v, err := a.current()
	if err != nil {
		return vault.Secret{}, err
	}
	s, err := v.GetSecret(id, actorGUI)
	if err != nil {
		return vault.Secret{}, err
	}
	_ = v.Save() // persist the audit entry
	return s, nil
}

// SaveSecret adds (id == "") or updates a secret and persists the vault.
func (a *App) SaveSecret(id string, in vault.SecretInput) (vault.Secret, error) {
	v, err := a.current()
	if err != nil {
		return vault.Secret{}, err
	}
	var s vault.Secret
	if id == "" {
		s, err = v.AddSecret(in, actorGUI)
	} else {
		s, err = v.UpdateSecret(id, in, actorGUI)
	}
	if err != nil {
		return vault.Secret{}, err
	}
	return s, v.Save()
}

func (a *App) DeleteSecret(id string) error {
	v, err := a.current()
	if err != nil {
		return err
	}
	if err := v.DeleteSecret(id, actorGUI); err != nil {
		return err
	}
	return v.Save()
}

// CopySecretField puts one field of a secret on the clipboard without the
// value ever passing through the frontend.
func (a *App) CopySecretField(id, field string) error {
	v, err := a.current()
	if err != nil {
		return err
	}
	s, err := v.GetSecret(id, actorGUI)
	if err != nil {
		return err
	}
	_ = v.Save()
	var val string
	switch field {
	case "username":
		val = s.Username
	case "password":
		val = s.Password
	case "url":
		val = s.URL
	default:
		return fmt.Errorf("unknown field %q", field)
	}
	return runtime.ClipboardSetText(a.ctx, val)
}

// ---- Categories ----

func (a *App) ListCategories() ([]vault.Category, error) {
	v, err := a.current()
	if err != nil {
		return nil, err
	}
	return v.ListCategories(), nil
}

func (a *App) AddCategory(name string) (vault.Category, error) {
	v, err := a.current()
	if err != nil {
		return vault.Category{}, err
	}
	c, err := v.AddCategory(name, actorGUI)
	if err != nil {
		return vault.Category{}, err
	}
	return c, v.Save()
}

func (a *App) RenameCategory(id, name string) error {
	v, err := a.current()
	if err != nil {
		return err
	}
	if err := v.RenameCategory(id, name, actorGUI); err != nil {
		return err
	}
	return v.Save()
}

func (a *App) DeleteCategory(id string) error {
	v, err := a.current()
	if err != nil {
		return err
	}
	if err := v.DeleteCategory(id, actorGUI); err != nil {
		return err
	}
	return v.Save()
}

// ---- Tools ----

func (a *App) GeneratePassword(length int, symbols bool) (string, error) {
	o := vault.DefaultGenerateOptions()
	if length > 0 {
		o.Length = length
	}
	o.Symbols = symbols
	return vault.GeneratePassword(o)
}

func (a *App) ChangeMasterPassword(current, newPassword string) error {
	v, err := a.current()
	if err != nil {
		return err
	}
	if newPassword == "" {
		return fmt.Errorf("new password must not be empty")
	}
	return v.ChangeMasterPassword(current, newPassword)
}

// ExportVault dumps cleartext JSON to a user-chosen file.
func (a *App) ExportVault() (string, error) {
	v, err := a.current()
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export vault (CLEARTEXT!)",
		DefaultFilename: "go-passwords-export.json",
		Filters:         []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	dump, err := v.Export(actorGUI)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, dump, 0o600); err != nil {
		return "", err
	}
	_ = v.Save()
	return path, nil
}

// ImportVault merges an export dump chosen by the user; returns how many
// secrets were imported.
func (a *App) ImportVault() (int, error) {
	v, err := a.current()
	if err != nil {
		return 0, err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import export file",
		Filters: []runtime.FileFilter{{DisplayName: "JSON", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return 0, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	n, err := v.Import(raw, actorGUI)
	if err != nil {
		return 0, err
	}
	return n, v.Save()
}

func (a *App) AuditLog() ([]vault.AuditEntry, error) {
	v, err := a.current()
	if err != nil {
		return nil, err
	}
	return v.AuditLog(), nil
}

// ---- Settings ----

func (a *App) snapshot() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

func (a *App) GetSettings() config.Config {
	return a.snapshot()
}

func (a *App) SetTheme(theme string) error {
	if theme != "light" {
		theme = "dark"
	}
	a.mu.Lock()
	a.cfg.Theme = theme
	cfg := a.cfg
	a.mu.Unlock()
	return config.Save(cfg)
}

func (a *App) SetAutoLock(enabled bool, minutes int) error {
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 720 {
		minutes = 720
	}
	a.mu.Lock()
	a.cfg.AutoLockEnabled = enabled
	a.cfg.AutoLockMinutes = minutes
	cfg := a.cfg
	a.mu.Unlock()
	if err := config.Save(cfg); err != nil {
		return err
	}
	a.resetLockTimer()
	return nil
}

func (a *App) Version() string {
	return appVersion
}
