// Package config persists the GUI's app-level preferences (theme, auto-lock,
// last vault). Nothing sensitive lives here — secrets stay in the vault.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Theme            string `json:"theme"`   // "dark" | "light"
	Opacity          int    `json:"opacity"` // window opacity, 20..100
	AutoLockEnabled  bool   `json:"autoLockEnabled"`
	AutoLockMinutes  int    `json:"autoLockMinutes"`
	LastVault        string   `json:"lastVault"`
	RecentVaults     []string `json:"recentVaults"` // MRU, max 3
	GeneratorLength  int    `json:"generatorLength"`
	GeneratorSymbols bool   `json:"generatorSymbols"`
	ToastSeconds     int    `json:"toastSeconds"` // toast duration; 0 disables toasts

	// Embedded REST API (go-apiserver). Off by default — a password manager
	// opens no port the user didn't ask for.
	APIAutoStart bool     `json:"apiAutoStart"`
	APIPort      int      `json:"apiPort"`
	APIKey       string   `json:"apiKey"`
	APIAllowlist []string `json:"apiAllowlist"` // CIDRs
	APIHTTPS     bool     `json:"apiHttps"`

	// In-app updater (go-updates). AutoCheck is opt-in: the app makes no
	// network call the user didn't ask for.
	UpdateAutoCheck  bool   `json:"updateAutoCheck"`
	UpdateSkipped    string `json:"updateSkippedVersion"`
	UpdateLaterUntil string `json:"updateLaterUntil"`
	UpdateLastCheck  string `json:"updateLastAutoCheck"`
}

// DefaultPort range for go-passwords (family: go-calc 87xx, go-notepad 88xx).
const defaultPort = 8940

func Default() Config {
	return Config{
		Theme:            "dark",
		Opacity:          100,
		AutoLockEnabled:  true,
		AutoLockMinutes:  5,
		GeneratorLength:  16,
		GeneratorSymbols: true,
		ToastSeconds:     3,
		APIAutoStart:     false,
		APIPort:          defaultPort,
		APIKey:           GenerateKey(),
		APIAllowlist:     []string{"127.0.0.1/32"},
	}
}

// GenerateKey returns a fresh random API key.
func GenerateKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ConfigDir returns the app's per-user config directory.
func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "go-passwords"), nil
}

func path() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() Config {
	cfg := Default()
	p, err := path()
	if err != nil {
		return cfg
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return cfg
	}
	// Unmarshal over the defaults so missing fields keep their default value.
	_ = json.Unmarshal(raw, &cfg)
	if cfg.GeneratorLength <= 0 {
		cfg.GeneratorLength = 16
	}
	if cfg.Opacity < 20 || cfg.Opacity > 100 {
		cfg.Opacity = 100
	}
	if cfg.ToastSeconds < 0 {
		cfg.ToastSeconds = 0
	}
	if cfg.ToastSeconds > 60 {
		cfg.ToastSeconds = 60
	}
	if cfg.APIKey == "" {
		cfg.APIKey = GenerateKey()
	}
	if cfg.APIPort == 0 {
		cfg.APIPort = defaultPort
	}
	if len(cfg.APIAllowlist) == 0 {
		cfg.APIAllowlist = []string{"127.0.0.1/32"}
	}
	return cfg
}

func Save(cfg Config) error {
	p, err := path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}
