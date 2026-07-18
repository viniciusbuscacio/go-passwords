package main

// appInfo builds the app descriptor / accessibility tree served at GET /v1/ax:
// what the app is, its domain API, and every view and control with testid,
// action and risk level, so an agent knows where to "click" and what to treat
// with care.

const axSchemaVersion = 1

// appVersion is stamped by the release workflow via -ldflags.
var appVersion = "0.1.0"

const (
	riskSafe        = "safe"
	riskNavigation  = "navigation"
	riskExternal    = "external"
	riskSensitive   = "sensitive"
	riskDestructive = "destructive"
)

type axNode struct {
	Role        string   `json:"role"`
	Name        string   `json:"name"`
	ID          string   `json:"id,omitempty"`
	Testid      string   `json:"testid,omitempty"`
	Description string   `json:"description,omitempty"`
	Action      string   `json:"action,omitempty"`
	Keyboard    string   `json:"keyboard,omitempty"`
	Risk        string   `json:"risk,omitempty"`
	OpenedBy    string   `json:"openedBy,omitempty"`
	Children    []axNode `json:"children,omitempty"`
}

type apiEndpoint struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Body    any    `json:"body,omitempty"`
	Returns any    `json:"returns,omitempty"`
	Auth    string `json:"auth"`
}

type errorInfo struct {
	Code    string `json:"code"`
	Status  int    `json:"status"`
	Meaning string `json:"meaning"`
}

type appInfoDTO struct {
	SchemaVersion int           `json:"schemaVersion"`
	Version       string        `json:"version"`
	App           string        `json:"app"`
	Description   string        `json:"description"`
	HowToUse      string        `json:"howToUse"`
	Capabilities  []string      `json:"capabilities"`
	API           []apiEndpoint `json:"api"`
	Errors        []errorInfo   `json:"errors"`
	AXTree        axNode        `json:"axTree"`
}

func (a *App) appInfo() any {
	return appInfoDTO{
		SchemaVersion: axSchemaVersion,
		Version:       appVersion,
		App:           "go-passwords",
		Description:   "An offline password manager for humans and AI agents. The vault is a single fully-encrypted file (Argon2id + AES-256-GCM).",
		HowToUse: "Everything requires the vault to be unlocked; locked-state calls return 423 vault_locked. " +
			"Unlock with POST /v1/unlock {vault?, master_password} (omit 'vault' to use the last opened one), lock with POST /v1/lock. " +
			"Secrets: GET /v1/secrets lists WITHOUT passwords (optional ?q= filter); GET /v1/secrets/{id} returns the full record including the password (risk: sensitive); " +
			"POST /v1/secrets creates {title, username?, password?, url?, notes?, categoryId?}; PUT /v1/secrets/{id} updates; DELETE /v1/secrets/{id} is destructive and permanent. " +
			"Every read and write is recorded in an audit log inside the vault (GET /v1/audit) — by id only, never content. " +
			"To operate the real UI instead, use /v1/ui/* with the testids in this axTree; check each control's risk before pressing. " +
			"Errors are structured as {\"error\":{code,message,status}}.",
		Capabilities: []string{"secrets", "categories", "generate", "export", "audit", "ui.state", "ui.press", "ui.key", "ui.input"},
		Errors: []errorInfo{
			{Code: "invalid_json", Status: 400, Meaning: "request body is not valid JSON"},
			{Code: "missing_field", Status: 400, Meaning: "a required field was empty or absent"},
			{Code: "unauthorized", Status: 401, Meaning: "invalid or missing X-API-Key header"},
			{Code: "invalid_password", Status: 401, Meaning: "the master password could not open the vault"},
			{Code: "forbidden", Status: 403, Meaning: "the client IP is not in the allowlist"},
			{Code: "not_found", Status: 404, Meaning: "no secret/category with that id"},
			{Code: "unknown_testid", Status: 404, Meaning: "no control on screen has that testid"},
			{Code: "method_not_allowed", Status: 405, Meaning: "wrong HTTP method for this endpoint"},
			{Code: "disabled_control", Status: 409, Meaning: "the control exists but is currently disabled"},
			{Code: "invalid_secret", Status: 422, Meaning: "the secret payload was rejected (e.g. empty title)"},
			{Code: "vault_locked", Status: 423, Meaning: "the vault is locked — POST /v1/unlock first"},
			{Code: "ui_timeout", Status: 503, Meaning: "the UI did not respond in time"},
		},
		API: []apiEndpoint{
			{Method: "GET", Path: "/v1/status", Returns: map[string]any{"unlocked": true, "vault": "path", "version": "x.y.z"}, Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/unlock", Body: map[string]string{"vault": "(optional) path", "master_password": "..."}, Returns: map[string]any{"unlocked": true}, Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/lock", Returns: map[string]any{"unlocked": false}, Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/secrets?q=", Returns: "secrets WITHOUT passwords", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/secrets", Body: map[string]string{"title": "required", "username": "", "password": "", "url": "", "notes": "", "categoryId": ""}, Returns: "the created secret", Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/secrets/{id}", Returns: "the full secret including password", Auth: "X-API-Key header"},
			{Method: "PUT", Path: "/v1/secrets/{id}", Body: "same shape as POST", Returns: "the updated secret", Auth: "X-API-Key header"},
			{Method: "DELETE", Path: "/v1/secrets/{id}", Returns: map[string]bool{"deleted": true}, Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/categories", Returns: "[{id,name}]", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/categories", Body: map[string]string{"name": "..."}, Returns: "the created category", Auth: "X-API-Key header"},
			{Method: "PUT", Path: "/v1/categories/{id}", Body: map[string]string{"name": "..."}, Auth: "X-API-Key header"},
			{Method: "DELETE", Path: "/v1/categories/{id}", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/generate", Body: map[string]any{"length": 16, "symbols": true}, Returns: map[string]string{"password": "..."}, Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/export", Returns: "the whole vault as CLEARTEXT JSON — handle with extreme care", Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/audit", Returns: "audit entries {ts,actor,action,secret_id}", Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/health", Returns: map[string]string{"status": "ok"}, Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/ax", Returns: "this document", Auth: "X-API-Key header"},
			{Method: "GET", Path: "/v1/ui/state", Returns: "current on-screen state", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/ui/press", Body: map[string]string{"testid": "add-secret"}, Returns: "resulting on-screen state", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/ui/key", Body: map[string]string{"key": "Enter"}, Returns: "resulting on-screen state", Auth: "X-API-Key header"},
			{Method: "POST", Path: "/v1/ui/input", Body: map[string]string{"testid": "edit-title", "value": "GitHub"}, Returns: "resulting on-screen state", Auth: "X-API-Key header"},
		},
		AXTree: axNode{
			Role: "application",
			Name: "go-passwords",
			Children: []axNode{
				{
					Role:        "toolbar",
					Name:        "Title bar",
					Description: "Always visible, on every view.",
					Children: []axNode{
						{Role: "button", Name: "Lock", Testid: "lock-now", Action: "lock the vault immediately (visible while unlocked)", Risk: riskSafe},
						{Role: "button", Name: "Settings", Testid: "open-settings", Action: "open the Settings view", Risk: riskNavigation},
						{Role: "button", Name: "Minimize", Testid: "window-minimize", Action: "minimize the window", Risk: riskSafe},
						{Role: "button", Name: "Maximize", Testid: "window-maximize", Action: "maximize/restore the window", Risk: riskSafe},
						{Role: "button", Name: "Close", Testid: "window-close", Action: "close the app", Risk: riskDestructive},
					},
				},
				{
					Role:        "view",
					Name:        "Unlock",
					ID:          "unlock",
					Description: "Shown while the vault is locked. Open an existing vault or create a new one.",
					Children: []axNode{
						{Role: "text", Name: "Vault path", Testid: "vault-path", Description: "the vault file that will be opened"},
						{Role: "button", Name: "Browse", Testid: "browse-vault", Action: "pick a vault file (native dialog — NOT reachable via /v1/ui, use POST /v1/unlock instead)", Risk: riskSafe},
						{Role: "textbox", Name: "Master password", Testid: "master-password", Action: "type the master password", Risk: riskSensitive},
						{Role: "button", Name: "Unlock", Testid: "unlock-btn", Action: "open the vault with the typed password", Keyboard: "Enter", Risk: riskSensitive},
						{Role: "button", Name: "Create new vault", Testid: "goto-create", Action: "switch to the create-vault form", Risk: riskNavigation},
						{Role: "textbox", Name: "New password", Testid: "create-password", Action: "type the new vault's master password (create form)", Risk: riskSensitive},
						{Role: "textbox", Name: "Confirm password", Testid: "create-password2", Action: "confirm the new password (create form)", Risk: riskSensitive},
						{Role: "button", Name: "Choose location & create", Testid: "create-btn", Action: "pick where to save (native dialog) and create the vault", Risk: riskSensitive},
						{Role: "text", Name: "Error", Testid: "unlock-error", Description: "why the unlock/create failed; rendered only after a failure"},
					},
				},
				{
					Role:        "view",
					Name:        "Vault",
					ID:          "vault",
					Description: "Main screen while unlocked: search, list, and a detail editor.",
					Children: []axNode{
						{Role: "textbox", Name: "Search", Testid: "search", Action: "filter the list by title/username/url", Risk: riskSafe},
						{Role: "button", Name: "Add secret", Testid: "add-secret", Action: "open the editor with an empty secret", Risk: riskSafe},
						{Role: "list", Name: "Secrets", Testid: "secret-list", Description: "one row per secret; rows have testid secret-<id>"},
						{Role: "button", Name: "Secret row", Testid: "secret-<id>", Action: "open that secret in the editor", Risk: riskSafe},
						{Role: "textbox", Name: "Title", Testid: "edit-title", Action: "the secret's title (required)", Risk: riskSafe},
						{Role: "textbox", Name: "Username", Testid: "edit-username", Risk: riskSafe},
						{Role: "textbox", Name: "Password", Testid: "edit-password", Description: "masked by default", Risk: riskSensitive},
						{Role: "button", Name: "Show/hide password", Testid: "toggle-password", Action: "reveal or mask the password field", Risk: riskSensitive},
						{Role: "button", Name: "Generate", Testid: "generate-password", Action: "fill the password field with a random password", Risk: riskSafe},
						{Role: "button", Name: "Copy username", Testid: "copy-username", Action: "copy the username to the clipboard", Risk: riskSafe},
						{Role: "button", Name: "Copy password", Testid: "copy-password", Action: "copy the password to the clipboard (never shown)", Risk: riskSensitive},
						{Role: "textbox", Name: "URL", Testid: "edit-url", Risk: riskSafe},
						{Role: "textbox", Name: "Notes", Testid: "edit-notes", Risk: riskSafe},
						{Role: "combobox", Name: "Category", Testid: "edit-category", Action: "assign a category", Risk: riskSafe},
						{Role: "button", Name: "Save", Testid: "save-secret", Action: "save the editor's secret", Risk: riskSafe},
						{Role: "button", Name: "Delete", Testid: "delete-secret", Action: "delete the secret PERMANENTLY (asks to confirm; confirm button is confirm-delete)", Risk: riskDestructive},
						{Role: "button", Name: "Confirm delete", Testid: "confirm-delete", Action: "the confirmation step of delete", Risk: riskDestructive},
						{Role: "button", Name: "Close editor", Testid: "close-editor", Action: "close the editor without saving", Risk: riskSafe},
						{Role: "text", Name: "Editor error", Testid: "editor-error", Description: "why a save failed; rendered only after a failure"},
					},
				},
				{
					Role:        "view",
					Name:        "Settings",
					ID:          "settings",
					OpenedBy:    "open-settings",
					Description: "Theme, auto-lock, categories, master password, export/import, REST API.",
					Children: []axNode{
						{Role: "button", Name: "Back", Testid: "back", Action: "return to the previous view", Risk: riskNavigation},
						{Role: "switch", Name: "Dark mode", Testid: "theme-switch", Action: "toggle dark/light theme", Risk: riskSafe},
						{Role: "switch", Name: "Auto-lock", Testid: "autolock-switch", Action: "toggle locking the vault after inactivity", Risk: riskSensitive},
						{Role: "textbox", Name: "Auto-lock minutes", Testid: "autolock-minutes", Action: "minutes of inactivity before locking", Risk: riskSafe},
						{Role: "table", Name: "Categories", Testid: "category-list", Description: "rows carry testids cat-rename-<id> / cat-delete-<id>"},
						{Role: "textbox", Name: "New category", Testid: "new-category", Risk: riskSafe},
						{Role: "button", Name: "Add category", Testid: "add-category", Risk: riskSafe},
						{Role: "textbox", Name: "Current master password", Testid: "mp-current", Risk: riskSensitive},
						{Role: "textbox", Name: "New master password", Testid: "mp-new", Risk: riskSensitive},
						{Role: "textbox", Name: "Confirm new master password", Testid: "mp-confirm", Risk: riskSensitive},
						{Role: "button", Name: "Change master password", Testid: "mp-change", Action: "re-encrypt the vault key under the new password", Risk: riskSensitive},
						{Role: "button", Name: "Export", Testid: "export-vault", Action: "save the whole vault as CLEARTEXT JSON (native dialog)", Risk: riskSensitive},
						{Role: "button", Name: "Import", Testid: "import-vault", Action: "merge an export file into the vault (native dialog)", Risk: riskSensitive},
						{Role: "button", Name: "REST API Server", Testid: "nav-api", Action: "open the REST API server settings", Risk: riskNavigation},
						{Role: "link", Name: "GitHub", Testid: "open-github", Action: "open the project on GitHub", Risk: riskExternal},
						{Role: "text", Name: "Version", Testid: "app-version"},
					},
				},
				{
					Role:        "view",
					Name:        "REST API Server",
					ID:          "api",
					Description: "Opened from Settings → REST API Server. The server is OFF by default.",
					Children: []axNode{
						{Role: "button", Name: "Back", Testid: "back", Action: "return to Settings", Risk: riskNavigation},
						{Role: "button", Name: "Start/Stop", Testid: "toggle-server", Action: "start or stop the REST server", Risk: riskSensitive},
						{Role: "status", Name: "Server status", Testid: "status", Description: "Running or Stopped"},
						{Role: "switch", Name: "Start automatically", Testid: "autostart", Action: "toggle starting the server on app launch", Risk: riskSensitive},
						{Role: "table", Name: "Allowed IPs", Testid: "allowlist", Description: "CIDR allowlist; rows carry testid remove-<cidr>"},
						{Role: "textbox", Name: "New IP", Testid: "new-ip", Risk: riskSafe},
						{Role: "button", Name: "Add IP", Testid: "add-ip", Risk: riskSensitive},
						{Role: "text", Name: "Access key", Testid: "api-key", Description: "the API key (masked)"},
						{Role: "button", Name: "Copy key", Testid: "copy-key", Action: "copy the API key", Risk: riskSensitive},
						{Role: "button", Name: "Rotate key", Testid: "rotate-key", Action: "generate a new API key", Risk: riskSensitive},
					},
				},
			},
		},
	}
}
