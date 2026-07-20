// Command smoke is the end-to-end smoke test for the go-passwords agent
// control plane. It discovers the API key, port and scheme from the app's
// config file (or takes them via -base-url / -api-key), then exercises every
// layer an agent depends on:
//
//   - /v1/health              reachability + auth
//   - /v1/ax                  the app descriptor / accessibility tree contract
//   - lock state              423 vault_locked, 401 invalid_password, unlock
//   - /v1/secrets, /v1/categories, /v1/generate, /v1/export, /v1/audit
//   - /v1/ui/*                driving the REAL UI (press, input, state)
//   - the password field NEVER reaches the UI bridge (masked, length only)
//   - structured errors       unknown_testid, missing_field, disabled_control
//   - AX <-> DOM coverage: every unconditional testid advertised in /v1/ax is
//     reachable on screen (vault, editor, categories, settings, master
//     password, api, and the three unlock sub-screens)
//
// Run it while the app is open with the REST server started and a THROWAWAY
// test vault as the last-opened vault — the smoke unlocks it, writes a secret
// and a category, and deletes them again, but never run it against a vault
// you care about:
//
//	go run ./tools/smoke -master-password <password of the open test vault>
//	GO_PW_MASTER_PASSWORD=... go run ./tools/smoke
//
// The vault is left LOCKED when the run finishes. Exit code is 0 when every
// check passes, non-zero otherwise (1 = a check failed or the server is
// unreachable, 2 = missing API key or master password).
package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const appDir = "go-passwords"

type config struct {
	APIPort  int    `json:"apiPort"`
	APIKey   string `json:"apiKey"`
	APIHTTPS bool   `json:"apiHttps"`
}

// loadConfig reads the app's config.json from the per-user config dir — the
// same location the app writes. Missing or unreadable config means zeroes.
func loadConfig() config {
	var s config
	dir, err := os.UserConfigDir()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(filepath.Join(dir, appDir, "config.json"))
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}

// client is a minimal JSON API client. HTTP-level errors (4xx/5xx) are
// returned as (status, parsed body), not as Go errors — the checks assert on
// them; only transport failures return err.
type client struct {
	base string
	key  string
	http *http.Client
}

func newClient(base, key, pin string) *client {
	tlsCfg := &tls.Config{
		// The app serves a self-signed cert; trust-on-first-use for a local
		// smoke run. With -pin the public key is verified for real below.
		InsecureSkipVerify: true,
	}
	if pin != "" {
		tlsCfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
			got := "sha256//" + base64.StdEncoding.EncodeToString(sum[:])
			if got != pin {
				return fmt.Errorf("TLS pin mismatch: server has %s", got)
			}
			return nil
		}
	}
	return &client{
		base: strings.TrimRight(base, "/"),
		key:  key,
		http: &http.Client{
			Timeout:   15 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		},
	}
}

func (c *client) call(method, path string, body any, key string) (int, any, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if key == "" {
		key = c.key
	}
	req.Header.Set("X-API-Key", key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	var parsed any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &parsed); err != nil {
			parsed = string(raw)
		}
	}
	return resp.StatusCode, parsed, nil
}

func (c *client) get(path string) (int, any, error) { return c.call("GET", path, nil, "") }

func (c *client) press(testid string) (int, any, error) {
	return c.call("POST", "/v1/ui/press", map[string]string{"testid": testid}, "")
}

func (c *client) input(testid, value string) (int, any, error) {
	return c.call("POST", "/v1/ui/input", map[string]string{"testid": testid, "value": value}, "")
}

// checks tallies pass/fail and prints one line per assertion.
type checks struct{ passed, failed int }

func (ch *checks) ok(name string, cond bool, detail string) bool {
	mark := "PASS"
	if !cond {
		mark = "FAIL"
		ch.failed++
		if detail != "" {
			name += "  -> " + detail
		}
	} else {
		ch.passed++
	}
	fmt.Printf("[%s] %s\n", mark, name)
	return cond
}

// ---- JSON helpers ----

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func errCode(v any) string {
	e := asMap(asMap(v)["error"])
	code, _ := e["code"].(string)
	return code
}

func view(state any) string {
	v, _ := asMap(state)["view"].(string)
	return v
}

func controls(state any) map[string]bool {
	out := map[string]bool{}
	list, _ := asMap(state)["controls"].([]any)
	for _, v := range list {
		if s, ok := v.(string); ok {
			out[s] = true
		}
	}
	return out
}

func collectTestids(node map[string]any, out map[string]bool) {
	if tid, _ := node["testid"].(string); tid != "" {
		out[tid] = true
	}
	children, _ := node["children"].([]any)
	for _, c := range children {
		if m := asMap(c); m != nil {
			collectTestids(m, out)
		}
	}
}

func main() {
	baseURL := flag.String("base-url", "", "e.g. https://127.0.0.1:8123 (default: derived from config.json)")
	apiKey := flag.String("api-key", "", "X-API-Key value (default: read from config.json)")
	master := flag.String("master-password", "", "master password of the OPEN THROWAWAY test vault (default: env GO_PW_MASTER_PASSWORD)")
	pin := flag.String("pin", "", `TLS public-key pin to verify, e.g. "sha256//..." (default: accept the self-signed cert)`)
	flag.Parse()

	s := loadConfig()
	key := *apiKey
	if key == "" {
		key = s.APIKey
	}
	if key == "" {
		fmt.Println("No API key found. Pass -api-key or open the app once so it writes config.json.")
		os.Exit(2)
	}
	pw := *master
	if pw == "" {
		pw = os.Getenv("GO_PW_MASTER_PASSWORD")
	}
	if pw == "" {
		fmt.Println("No master password. Pass -master-password or set GO_PW_MASTER_PASSWORD (throwaway test vault only!).")
		os.Exit(2)
	}
	base := *baseURL
	if base == "" {
		scheme := "http"
		if s.APIHTTPS {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://127.0.0.1:%d", scheme, s.APIPort)
	}

	fmt.Println("Target:", base)
	c := newClient(base, key, *pin)
	ch := &checks{}

	// --- health + auth ---
	st, body, err := c.get("/v1/health")
	if !ch.ok("health reachable", err == nil && st == 200, fmt.Sprintf("status=%d err=%v", st, err)) {
		fmt.Println("Server not reachable — is the app open with the REST server started?")
		os.Exit(1)
	}
	status, _ := asMap(body)["status"].(string)
	ch.ok("health body ok", status == "ok", fmt.Sprint(body))

	st, body, _ = c.call("GET", "/v1/health", nil, "wrong-key")
	ch.ok("auth rejects bad key (401 unauthorized)",
		st == 401 && errCode(body) == "unauthorized", fmt.Sprintf("status=%d body=%v", st, body))

	// --- ax contract ---
	st, ax, _ := c.get("/v1/ax")
	ch.ok("ax reachable", st == 200, fmt.Sprintf("status=%d", st))
	axDoc := asMap(ax)
	_, hasSchema := axDoc["schemaVersion"]
	ch.ok("ax has schemaVersion", hasSchema, "")
	caps, _ := axDoc["capabilities"].([]any)
	hasSecrets := false
	for _, v := range caps {
		if v == "secrets" {
			hasSecrets = true
		}
	}
	ch.ok("ax advertises capabilities", hasSecrets, "")
	codes, _ := axDoc["errors"].([]any)
	hasLocked := false
	for _, v := range codes {
		if code, _ := asMap(v)["code"].(string); code == "vault_locked" {
			hasLocked = true
		}
	}
	ch.ok("ax documents vault_locked", hasLocked, "")

	axTestids := map[string]bool{}
	if tree := asMap(axDoc["axTree"]); tree != nil {
		collectTestids(tree, axTestids)
	}
	ch.ok("ax tree exposes testids", len(axTestids) > 20, fmt.Sprintf("count=%d", len(axTestids)))

	// --- locked behavior ---
	st, _, _ = c.call("POST", "/v1/lock", nil, "")
	ch.ok("lock accepted", st == 200, fmt.Sprintf("status=%d", st))

	st, body, _ = c.get("/v1/secrets")
	ch.ok("locked vault -> 423 vault_locked",
		st == 423 && errCode(body) == "vault_locked", fmt.Sprintf("status=%d body=%v", st, body))

	st, body, _ = c.call("POST", "/v1/unlock", map[string]string{"master_password": "definitely-wrong"}, "")
	ch.ok("wrong master password -> 401 invalid_password",
		st == 401 && errCode(body) == "invalid_password", fmt.Sprintf("status=%d body=%v", st, body))

	st, body, _ = c.call("POST", "/v1/unlock", map[string]string{"master_password": pw}, "")
	unlocked, _ := asMap(body)["unlocked"].(bool)
	if !ch.ok("unlock with the test password", st == 200 && unlocked, fmt.Sprintf("status=%d body=%v", st, body)) {
		fmt.Println("Cannot unlock — is the throwaway test vault the app's last-opened vault?")
		os.Exit(1)
	}

	st, body, _ = c.get("/v1/status")
	stUnlocked, _ := asMap(body)["unlocked"].(bool)
	ch.ok("status reports unlocked", st == 200 && stUnlocked, fmt.Sprint(body))

	_, state, _ := c.get("/v1/ui/state")
	ch.ok("GUI followed the REST unlock (vault view)", view(state) == "vault", fmt.Sprintf("view=%q", view(state)))

	// --- generate ---
	st, body, _ = c.call("POST", "/v1/generate", map[string]any{"length": 24, "symbols": true}, "")
	gen, _ := asMap(body)["password"].(string)
	ch.ok("generate returns a 24-char password", st == 200 && len(gen) == 24, fmt.Sprintf("status=%d len=%d", st, len(gen)))

	// --- secrets CRUD over REST ---
	secretPass := "smoke-S3cret!" + gen[:8]
	st, body, _ = c.call("POST", "/v1/secrets", map[string]string{
		"title": "SMOKE test secret", "username": "smoke-user", "password": secretPass,
		"url": "https://example.com", "notes": "created by tools/smoke",
	}, "")
	created := asMap(body)
	secretID, _ := created["id"].(string)
	ch.ok("create secret (201)", st == 201 && secretID != "", fmt.Sprintf("status=%d body=%v", st, body))

	st, body, _ = c.get("/v1/secrets?q=SMOKE")
	list, _ := body.([]any)
	listHasIt := false
	listLeaks := false
	for _, it := range list {
		m := asMap(it)
		if m["id"] == secretID {
			listHasIt = true
			if _, has := m["password"]; has {
				listLeaks = true
			}
		}
	}
	ch.ok("list finds it (filtered)", st == 200 && listHasIt, fmt.Sprintf("status=%d n=%d", st, len(list)))
	ch.ok("list carries NO password field", !listLeaks, "")

	st, body, _ = c.get("/v1/secrets/" + secretID)
	full := asMap(body)
	ch.ok("get by id returns the full secret", st == 200 && full["password"] == secretPass, fmt.Sprintf("status=%d", st))

	st, body, _ = c.call("PUT", "/v1/secrets/"+secretID, map[string]string{
		"title": "SMOKE test secret v2", "username": "smoke-user", "password": secretPass,
	}, "")
	ch.ok("update secret", st == 200 && asMap(body)["title"] == "SMOKE test secret v2", fmt.Sprintf("status=%d body=%v", st, body))

	st, body, _ = c.call("POST", "/v1/secrets", map[string]string{"title": ""}, "")
	ch.ok("empty title -> 422 invalid_secret",
		st == 422 && errCode(body) == "invalid_secret", fmt.Sprintf("status=%d body=%v", st, body))

	// --- categories CRUD over REST ---
	st, body, _ = c.call("POST", "/v1/categories", map[string]string{"name": "SMOKE category"}, "")
	catID, _ := asMap(body)["id"].(string)
	ch.ok("create category (201)", st == 201 && catID != "", fmt.Sprintf("status=%d body=%v", st, body))

	st, _, _ = c.call("PUT", "/v1/categories/"+catID, map[string]string{"name": "SMOKE category v2"}, "")
	ch.ok("rename category", st == 200, fmt.Sprintf("status=%d", st))

	// --- export carries the secret (cleartext contract) ---
	st, body, _ = c.get("/v1/export")
	dump, _ := json.Marshal(body)
	ch.ok("export contains the secret", st == 200 && bytes.Contains(dump, []byte("SMOKE test secret v2")), fmt.Sprintf("status=%d", st))

	// --- audit has entries, ids only ---
	st, body, _ = c.get("/v1/audit")
	audit, _ := body.([]any)
	auditDump, _ := json.Marshal(body)
	ch.ok("audit log has entries", st == 200 && len(audit) > 0, fmt.Sprintf("status=%d n=%d", st, len(audit)))
	ch.ok("audit never contains the password value", !bytes.Contains(auditDump, []byte(secretPass)), "")

	// --- drive the real UI ---
	// Park on the vault view whatever was on screen (back is a 404 no-op when
	// already there).
	c.press("back")
	c.press("back")
	_, state, _ = c.get("/v1/ui/state")
	ch.ok("ui state reachable on vault view", view(state) == "vault", fmt.Sprintf("view=%q", view(state)))
	ch.ok("REST-created secret appears in the GUI list", controls(state)["secret-"+secretID],
		fmt.Sprintf("want secret-%s", secretID))

	// Open the editor on our secret; collect coverage; and assert the ONE rule
	// that matters most: the password value must never cross the UI bridge.
	_, state, _ = c.press("secret-" + secretID)
	editorControls := controls(state)
	ch.ok("row press opens the editor", editorControls["save-secret"], fmt.Sprint(view(state)))
	stateDump, _ := json.Marshal(state)
	ch.ok("password value never crosses the UI bridge", !bytes.Contains(stateDump, []byte(secretPass)), "")
	seen := map[string]bool{}
	for t := range editorControls {
		seen[t] = true
	}
	_, state, _ = c.press("close-editor")
	for t := range controls(state) {
		seen[t] = true
	}

	// Categories view.
	_, state, _ = c.press("open-categories")
	for t := range controls(state) {
		seen[t] = true
	}
	c.press("cancel-categories")

	// Settings view (+ the updater flow: pressing Check must settle into a
	// structured outcome — up to date, available, or a clean error).
	_, state, _ = c.press("open-settings")
	for t := range controls(state) {
		seen[t] = true
	}
	st, _, _ = c.press("update-check")
	ch.ok("pressing update-check is accepted", st == 200, fmt.Sprintf("status=%d", st))
	deadline := time.Now().Add(20 * time.Second)
	var upd map[string]any
	for {
		_, body, _ = c.get("/v1/update")
		upd = asMap(body)
		if b, _ := upd["checking"].(bool); !b || time.Now().After(deadline) {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	checking, _ := upd["checking"].(bool)
	updErr, _ := upd["error"].(string)
	_, hasAvailable := upd["available"].(bool)
	ch.ok("update check settles with a structured outcome",
		!checking && (updErr != "" || hasAvailable), fmt.Sprint(upd))

	// Change-master-password view (looked at, never submitted).
	_, state, _ = c.press("nav-master-password")
	for t := range controls(state) {
		seen[t] = true
	}
	c.press("back")

	// API server view + the disabled-control contract.
	_, state, _ = c.press("nav-api")
	for t := range controls(state) {
		seen[t] = true
	}
	st, body, _ = c.press("add-ip") // 'New IP' is empty, so Add is disabled
	ch.ok("disabled control -> 409 disabled_control",
		st == 409 && errCode(body) == "disabled_control", fmt.Sprintf("status=%d body=%v", st, body))
	c.press("back")
	c.press("back")

	// --- structured UI errors ---
	st, body, _ = c.press("does-not-exist")
	ch.ok("unknown testid -> 404 unknown_testid",
		st == 404 && errCode(body) == "unknown_testid", fmt.Sprintf("status=%d body=%v", st, body))

	st, body, _ = c.call("POST", "/v1/ui/key", map[string]string{"key": ""}, "")
	ch.ok("empty key -> 400 missing_field",
		st == 400 && errCode(body) == "missing_field", fmt.Sprintf("status=%d body=%v", st, body))

	// --- cleanup (REST) ---
	st, _, _ = c.call("DELETE", "/v1/secrets/"+secretID, nil, "")
	ch.ok("delete secret", st == 200, fmt.Sprintf("status=%d", st))
	st, _, _ = c.call("DELETE", "/v1/categories/"+catID, nil, "")
	ch.ok("delete category", st == 200, fmt.Sprintf("status=%d", st))
	st, body, _ = c.get("/v1/secrets/" + secretID)
	ch.ok("deleted secret is gone (404 not_found)",
		st == 404 && errCode(body) == "not_found", fmt.Sprintf("status=%d body=%v", st, body))

	// --- lock again + unlock-screen coverage ---
	st, _, _ = c.call("POST", "/v1/lock", nil, "")
	ch.ok("lock at the end", st == 200, fmt.Sprintf("status=%d", st))
	_, state, _ = c.get("/v1/ui/state")
	ch.ok("GUI followed the REST lock (unlock view)", view(state) == "unlock", fmt.Sprintf("view=%q", view(state)))
	for t := range controls(state) { // start hub
		seen[t] = true
	}
	_, state, _ = c.press("recent-vault-0") // password screen
	for t := range controls(state) {
		seen[t] = true
	}
	_, state, _ = c.press("back-to-start")
	_, state, _ = c.press("goto-create") // create form
	for t := range controls(state) {
		seen[t] = true
	}
	c.press("back-to-start")

	// --- AX <-> DOM coverage ---
	// "<...>" testids are dynamic patterns; conditional controls only exist in
	// specific states (an error shown, HTTPS on, an update available, the
	// delete confirmation step).
	conditional := map[string]bool{
		"unlock-error": true, "editor-error": true, "server-error": true, "ip-error": true,
		"fingerprint": true, "copy-fingerprint": true, "curl-example": true, "copy-curl": true,
		"update-notes": true, "update-install": true, "update-skip": true, "update-later": true,
		"confirm-delete": true,
	}
	var missing []string
	for t := range axTestids {
		if !seen[t] && !strings.Contains(t, "<") && !conditional[t] {
			missing = append(missing, t)
		}
	}
	sort.Strings(missing)
	ch.ok("every unconditional ax testid is reachable on screen", len(missing) == 0,
		fmt.Sprintf("missing=%v", missing))

	fmt.Printf("\n%d passed, %d failed\n", ch.passed, ch.failed)
	if ch.failed > 0 {
		os.Exit(1)
	}
}
