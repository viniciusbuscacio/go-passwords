package vault

// FormatName identifies a go-passwords vault file.
const FormatName = "go-passwords-vault"

// FormatVersion is the current vault format version.
const FormatVersion = 3

// readmeText is the mini-manual embedded in every vault header so the file is
// self-describing: anyone opening it in a text editor sees what it is and how
// to recover the data (with the master password). The app ignores this field
// on read and rewrites the canonical text on save.
var readmeText = []string{
	"This file is an encrypted go-passwords vault. Without the master password it is unreadable.",
	"To decrypt: key = Argon2id(master_password, salt, kdf params below, keyLen=32);",
	"mount_key = AES-256-GCM-decrypt(mount_key_ciphered, key); data = AES-256-GCM-decrypt(payload, mount_key).",
	"Fields are 'iv:tag:ciphertext' (mount key in hex, payload in base64). Decrypted payload is plain JSON.",
	"Recovery guide + ready-to-run scripts (Go and Python):",
	"https://github.com/viniciusbuscacio/go-passwords/blob/main/docs/FORMAT.md",
}

// header is the cleartext part of the vault file. It carries only what is
// needed to unlock: crypto parameters and the encrypted mount key. No user
// data, no counts, no timestamps.
type header struct {
	Format           string    `json:"format"`
	Version          int       `json:"version"`
	Readme           []string  `json:"_readme"`
	KDF              KDFParams `json:"kdf"`
	Salt             string    `json:"salt"`               // hex
	MountKeyCiphered string    `json:"mount_key_ciphered"` // iv:tag:ct hex
	Payload          string    `json:"payload"`            // iv:tag:ct base64
}

// payload is the encrypted body: everything the vault stores.
type payload struct {
	Secrets    []Secret     `json:"secrets"`
	Categories []Category   `json:"categories"`
	Audit      []AuditEntry `json:"audit"`
}

// Secret is one stored credential. All fields live inside the encrypted
// payload, so none of them ever touch disk in cleartext.
type Secret struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	URL        string `json:"url,omitempty"`
	Notes      string `json:"notes,omitempty"`
	CategoryID string `json:"category_id,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Category groups secrets.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AuditEntry records one operation. It references secrets by ID only — never
// titles or values — so the log can never leak content.
type AuditEntry struct {
	TS       string `json:"ts"`
	Actor    string `json:"actor"`  // gui | cli | api
	Action   string `json:"action"` // unlock | unlock_failed | get | set | delete | ...
	SecretID string `json:"secret_id,omitempty"`
}

// maxAuditEntries caps the audit log; oldest entries are dropped beyond it.
const maxAuditEntries = 10000
