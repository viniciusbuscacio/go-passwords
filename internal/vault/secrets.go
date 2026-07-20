package vault

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a secret or category does not exist.
var ErrNotFound = errors.New("not found")

// SecretInput carries the writable fields of a secret.
type SecretInput struct {
	Title      string
	Username   string
	Password   string
	URL        string
	Notes      string
	CategoryID string
}

// AddSecret creates a secret and returns it. actor is recorded in the audit
// log ("gui" | "cli" | "api").
func (v *Vault) AddSecret(in SecretInput, actor string) (Secret, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Secret{}, errors.New("title is required")
	}
	if in.CategoryID != "" {
		if _, err := v.categoryByID(in.CategoryID); err != nil {
			return Secret{}, fmt.Errorf("category %s: %w", in.CategoryID, err)
		}
	}
	ts := now()
	s := Secret{
		ID:         uuid.NewString(),
		Title:      in.Title,
		Username:   in.Username,
		Password:   in.Password,
		URL:        in.URL,
		Notes:      in.Notes,
		CategoryID: in.CategoryID,
		CreatedAt:  ts,
		UpdatedAt:  ts,
	}
	v.data.Secrets = append(v.data.Secrets, s)
	v.audit(actor, "set", s.ID)
	return s, nil
}

// UpdateSecret replaces the writable fields of the secret with id.
func (v *Vault) UpdateSecret(id string, in SecretInput, actor string) (Secret, error) {
	if strings.TrimSpace(in.Title) == "" {
		return Secret{}, errors.New("title is required")
	}
	for i := range v.data.Secrets {
		if v.data.Secrets[i].ID == id {
			s := &v.data.Secrets[i]
			s.Title, s.Username, s.Password = in.Title, in.Username, in.Password
			s.URL, s.Notes, s.CategoryID = in.URL, in.Notes, in.CategoryID
			s.UpdatedAt = now()
			v.audit(actor, "set", id)
			return *s, nil
		}
	}
	return Secret{}, fmt.Errorf("secret %s: %w", id, ErrNotFound)
}

// DeleteSecret removes the secret with id.
func (v *Vault) DeleteSecret(id, actor string) error {
	for i := range v.data.Secrets {
		if v.data.Secrets[i].ID == id {
			v.data.Secrets = append(v.data.Secrets[:i], v.data.Secrets[i+1:]...)
			v.audit(actor, "delete", id)
			return nil
		}
	}
	return fmt.Errorf("secret %s: %w", id, ErrNotFound)
}

// GetSecret returns the secret with id (full record, password included).
func (v *Vault) GetSecret(id, actor string) (Secret, error) {
	for _, s := range v.data.Secrets {
		if s.ID == id {
			v.audit(actor, "get", s.ID)
			return s, nil
		}
	}
	return Secret{}, fmt.Errorf("secret %s: %w", id, ErrNotFound)
}

// GetSecretByTitle returns the first secret whose title matches
// case-insensitively.
func (v *Vault) GetSecretByTitle(title, actor string) (Secret, error) {
	for _, s := range v.data.Secrets {
		if strings.EqualFold(s.Title, title) {
			v.audit(actor, "get", s.ID)
			return s, nil
		}
	}
	return Secret{}, fmt.Errorf("secret %q: %w", title, ErrNotFound)
}

// ListSecrets returns all secrets with the password field blanked, optionally
// filtered by q (case-insensitive match on title, username or URL).
func (v *Vault) ListSecrets(q string) []Secret {
	q = strings.ToLower(q)
	out := make([]Secret, 0, len(v.data.Secrets))
	for _, s := range v.data.Secrets {
		if q != "" &&
			!strings.Contains(strings.ToLower(s.Title), q) &&
			!strings.Contains(strings.ToLower(s.Username), q) &&
			!strings.Contains(strings.ToLower(s.URL), q) {
			continue
		}
		s.Password = ""
		out = append(out, s)
	}
	return out
}

// --- Categories ---

func (v *Vault) categoryByID(id string) (Category, error) {
	for _, c := range v.data.Categories {
		if c.ID == id {
			return c, nil
		}
	}
	return Category{}, ErrNotFound
}

// ListCategories returns all categories (never nil, so JSON stays []).
func (v *Vault) ListCategories() []Category {
	out := make([]Category, 0, len(v.data.Categories))
	return append(out, v.data.Categories...)
}

// CategoryPalette is the set of colors offered by the UIs — Fluent-ish
// pastels of similar luminance, harmonized with the app's accent blue, all
// legible as tints on the family's dark and light themes. Auto-assign order
// keeps the first few categories maximally distinct.
var CategoryPalette = []string{
	"#4cc2ff", // blue (accent)
	"#3fbf6f", // green
	"#f7a350", // orange
	"#c58fff", // purple
	"#f47068", // coral
	"#4dd0c4", // teal
	"#f2c94c", // gold
	"#ff7eb6", // pink
}

// AddCategory creates a category. An empty color auto-assigns the next
// palette color.
func (v *Vault) AddCategory(name, color, actor string) (Category, error) {
	if strings.TrimSpace(name) == "" {
		return Category{}, errors.New("name is required")
	}
	if color == "" {
		color = CategoryPalette[len(v.data.Categories)%len(CategoryPalette)]
	}
	c := Category{ID: uuid.NewString(), Name: name, Color: color}
	v.data.Categories = append(v.data.Categories, c)
	v.audit(actor, "category_add", "")
	return c, nil
}

// SetCategoryColor changes a category's color.
func (v *Vault) SetCategoryColor(id, color, actor string) error {
	for i := range v.data.Categories {
		if v.data.Categories[i].ID == id {
			v.data.Categories[i].Color = color
			v.audit(actor, "category_color", "")
			return nil
		}
	}
	return fmt.Errorf("category %s: %w", id, ErrNotFound)
}

// RenameCategory changes a category's name.
func (v *Vault) RenameCategory(id, name, actor string) error {
	for i := range v.data.Categories {
		if v.data.Categories[i].ID == id {
			v.data.Categories[i].Name = name
			v.audit(actor, "category_rename", "")
			return nil
		}
	}
	return fmt.Errorf("category %s: %w", id, ErrNotFound)
}

// DeleteCategory removes a category; secrets that used it keep no category.
func (v *Vault) DeleteCategory(id, actor string) error {
	for i := range v.data.Categories {
		if v.data.Categories[i].ID == id {
			v.data.Categories = append(v.data.Categories[:i], v.data.Categories[i+1:]...)
			for j := range v.data.Secrets {
				if v.data.Secrets[j].CategoryID == id {
					v.data.Secrets[j].CategoryID = ""
				}
			}
			v.audit(actor, "category_delete", "")
			return nil
		}
	}
	return fmt.Errorf("category %s: %w", id, ErrNotFound)
}

// --- Audit ---

func (v *Vault) audit(actor, action, secretID string) {
	v.data.Audit = append(v.data.Audit, AuditEntry{TS: now(), Actor: actor, Action: action, SecretID: secretID})
	if n := len(v.data.Audit); n > maxAuditEntries {
		v.data.Audit = append([]AuditEntry(nil), v.data.Audit[n-maxAuditEntries:]...)
	}
}

// AuditLog returns the audit entries, newest last (never nil, so JSON stays []).
func (v *Vault) AuditLog() []AuditEntry {
	out := make([]AuditEntry, 0, len(v.data.Audit))
	return append(out, v.data.Audit...)
}

// RecordUnlock appends an unlock event (call after Open succeeds) so the
// audit trail covers session starts too.
func (v *Vault) RecordUnlock(actor string) {
	v.audit(actor, "unlock", "")
}

// --- Export / import ---

// ExportData is the cleartext dump structure (also the import format).
type ExportData struct {
	Format     string     `json:"format"`
	Secrets    []Secret   `json:"secrets"`
	Categories []Category `json:"categories"`
}

// Export returns the vault content as cleartext JSON. Handle with care.
func (v *Vault) Export(actor string) ([]byte, error) {
	v.audit(actor, "export", "")
	return json.MarshalIndent(ExportData{
		Format:     "go-passwords-export",
		Secrets:    v.data.Secrets,
		Categories: v.data.Categories,
	}, "", "  ")
}

// Import merges secrets and categories from an export dump. Entries keep
// their IDs unless they collide with existing ones, in which case new IDs are
// assigned.
func (v *Vault) Import(raw []byte, actor string) (int, error) {
	var in ExportData
	if err := json.Unmarshal(raw, &in); err != nil {
		return 0, fmt.Errorf("invalid import file: %w", err)
	}
	haveCat := map[string]bool{}
	for _, c := range v.data.Categories {
		haveCat[c.ID] = true
	}
	for _, c := range in.Categories {
		if c.ID == "" || haveCat[c.ID] {
			c.ID = uuid.NewString()
		}
		haveCat[c.ID] = true
		v.data.Categories = append(v.data.Categories, c)
	}
	haveSec := map[string]bool{}
	for _, s := range v.data.Secrets {
		haveSec[s.ID] = true
	}
	count := 0
	ts := now()
	for _, s := range in.Secrets {
		if s.ID == "" || haveSec[s.ID] {
			s.ID = uuid.NewString()
		}
		haveSec[s.ID] = true
		if s.CreatedAt == "" {
			s.CreatedAt = ts
		}
		s.UpdatedAt = ts
		v.data.Secrets = append(v.data.Secrets, s)
		count++
	}
	v.audit(actor, "import", "")
	return count, nil
}

// --- Password generation ---

// GenerateOptions controls GeneratePassword.
type GenerateOptions struct {
	Length  int
	Symbols bool
	Digits  bool
	Upper   bool
	Lower   bool
}

// DefaultGenerateOptions is 16 chars with all character classes.
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{Length: 16, Symbols: true, Digits: true, Upper: true, Lower: true}
}

// GeneratePassword builds a random password with crypto/rand.
func GeneratePassword(o GenerateOptions) (string, error) {
	if o.Length <= 0 {
		return "", errors.New("length must be positive")
	}
	var alphabet string
	if o.Lower {
		alphabet += "abcdefghijklmnopqrstuvwxyz"
	}
	if o.Upper {
		alphabet += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	}
	if o.Digits {
		alphabet += "0123456789"
	}
	if o.Symbols {
		alphabet += "!@#$%^&*()-_=+[]{};:,.?"
	}
	if alphabet == "" {
		return "", errors.New("no character classes enabled")
	}
	var b strings.Builder
	for i := 0; i < o.Length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}
