package vault

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPassword = "correct horse battery staple"

func newTestVault(t *testing.T) *Vault {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.gpw")
	v, err := Create(path, testPassword)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return v
}

func TestRoundTrip(t *testing.T) {
	v := newTestVault(t)
	cat, err := v.AddCategory("Banks", "", "cli")
	if err != nil {
		t.Fatalf("AddCategory: %v", err)
	}
	want, err := v.AddSecret(SecretInput{
		Title:      "AWS Token",
		Username:   "admin",
		Password:   "AKIA-secret",
		URL:        "https://aws.amazon.com",
		Notes:      "prod account",
		CategoryID: cat.ID,
	}, "cli")
	if err != nil {
		t.Fatalf("AddSecret: %v", err)
	}
	if err := v.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	re, err := Open(v.Path(), testPassword)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := re.GetSecretByTitle("aws token", "cli") // case-insensitive
	if err != nil {
		t.Fatalf("GetSecretByTitle: %v", err)
	}
	if got != want {
		t.Errorf("secret mismatch:\n got %+v\nwant %+v", got, want)
	}
	cats := re.ListCategories()
	if len(cats) != 1 || cats[0] != cat {
		t.Errorf("categories mismatch: %+v", cats)
	}
}

func TestWrongPasswordFailsClean(t *testing.T) {
	v := newTestVault(t)
	if _, err := Open(v.Path(), "wrong password"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword, got %v", err)
	}
}

func TestNothingLeaksInFile(t *testing.T) {
	v := newTestVault(t)
	if _, err := v.AddSecret(SecretInput{Title: "SuperBank", Username: "vini@example.com", Password: "hunter2"}, "cli"); err != nil {
		t.Fatal(err)
	}
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(v.Path())
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"SuperBank", "vini@example.com", "hunter2"} {
		if strings.Contains(string(raw), needle) {
			t.Errorf("cleartext %q found in vault file", needle)
		}
	}
	// The header must stay parseable and carry the self-describing readme.
	var h header
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatalf("header not parseable: %v", err)
	}
	if len(h.Readme) == 0 || h.Format != FormatName {
		t.Errorf("missing _readme or format in header: %+v", h)
	}
}

func TestCorruptedFile(t *testing.T) {
	v := newTestVault(t)
	raw, _ := os.ReadFile(v.Path())

	t.Run("truncated", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "trunc.gpw")
		os.WriteFile(p, raw[:len(raw)/2], 0o600)
		if _, err := Open(p, testPassword); !errors.Is(err, ErrCorrupted) {
			t.Fatalf("want ErrCorrupted, got %v", err)
		}
	})
	t.Run("flipped payload byte", func(t *testing.T) {
		var h header
		json.Unmarshal(raw, &h)
		// Flip one character inside the base64 payload body.
		i := strings.LastIndex(h.Payload, ":") + 5
		b := []byte(h.Payload)
		if b[i] == 'A' {
			b[i] = 'B'
		} else {
			b[i] = 'A'
		}
		h.Payload = string(b)
		mod, _ := json.Marshal(h)
		p := filepath.Join(t.TempDir(), "tampered.gpw")
		os.WriteFile(p, mod, 0o600)
		if _, err := Open(p, testPassword); !errors.Is(err, ErrCorrupted) {
			t.Fatalf("want ErrCorrupted, got %v", err)
		}
	})
	t.Run("not json", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "garbage.gpw")
		os.WriteFile(p, []byte("this is not a vault"), 0o600)
		if _, err := Open(p, testPassword); !errors.Is(err, ErrCorrupted) {
			t.Fatalf("want ErrCorrupted, got %v", err)
		}
	})
}

func TestChangeMasterPassword(t *testing.T) {
	v := newTestVault(t)
	s, _ := v.AddSecret(SecretInput{Title: "Entry", Password: "p1"}, "cli")
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangeMasterPassword("wrong", "new"); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("want ErrInvalidPassword for wrong current, got %v", err)
	}
	const newPassword = "new master password"
	if err := v.ChangeMasterPassword(testPassword, newPassword); err != nil {
		t.Fatalf("ChangeMasterPassword: %v", err)
	}
	if _, err := Open(v.Path(), testPassword); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("old password still works: %v", err)
	}
	re, err := Open(v.Path(), newPassword)
	if err != nil {
		t.Fatalf("Open with new password: %v", err)
	}
	got, err := re.GetSecret(s.ID, "cli")
	if err != nil || got.Password != "p1" {
		t.Fatalf("data lost after password change: %+v %v", got, err)
	}
}

func TestCreateRefusesExisting(t *testing.T) {
	v := newTestVault(t)
	if _, err := Create(v.Path(), "x"); !errors.Is(err, ErrVaultExists) {
		t.Fatalf("want ErrVaultExists, got %v", err)
	}
}

func TestSecretCRUDAndSearch(t *testing.T) {
	v := newTestVault(t)
	a, _ := v.AddSecret(SecretInput{Title: "GitHub", Username: "vini", Password: "gh"}, "cli")
	v.AddSecret(SecretInput{Title: "GitLab", Username: "vini", Password: "gl"}, "cli")

	if got := v.ListSecrets(""); len(got) != 2 {
		t.Fatalf("want 2 secrets, got %d", len(got))
	}
	for _, s := range v.ListSecrets("") {
		if s.Password != "" {
			t.Errorf("ListSecrets leaked password for %s", s.Title)
		}
	}
	if got := v.ListSecrets("github"); len(got) != 1 || got[0].Title != "GitHub" {
		t.Errorf("search failed: %+v", got)
	}

	if _, err := v.UpdateSecret(a.ID, SecretInput{Title: "GitHub", Password: "gh2"}, "cli"); err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	got, _ := v.GetSecret(a.ID, "cli")
	if got.Password != "gh2" {
		t.Errorf("update not applied: %+v", got)
	}
	if err := v.DeleteSecret(a.ID, "cli"); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, err := v.GetSecret(a.ID, "cli"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	if err := v.DeleteSecret("nope", "cli"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestCategoryDeleteDetachesSecrets(t *testing.T) {
	v := newTestVault(t)
	cat, _ := v.AddCategory("Work", "", "cli")
	s, _ := v.AddSecret(SecretInput{Title: "VPN", CategoryID: cat.ID}, "cli")
	if err := v.DeleteCategory(cat.ID, "cli"); err != nil {
		t.Fatal(err)
	}
	got, _ := v.GetSecret(s.ID, "cli")
	if got.CategoryID != "" {
		t.Errorf("secret still points to deleted category: %+v", got)
	}
}

func TestAuditRotation(t *testing.T) {
	v := newTestVault(t)
	s, _ := v.AddSecret(SecretInput{Title: "X", Password: "p"}, "cli")
	for i := 0; i < maxAuditEntries+50; i++ {
		v.audit("cli", "get", s.ID)
	}
	log := v.AuditLog()
	if len(log) != maxAuditEntries {
		t.Fatalf("want %d audit entries, got %d", maxAuditEntries, len(log))
	}
	// Newest entry must be the last one appended.
	if log[len(log)-1].Action != "get" {
		t.Errorf("unexpected tail entry: %+v", log[len(log)-1])
	}
}

func TestAuditNeverStoresContent(t *testing.T) {
	v := newTestVault(t)
	v.AddSecret(SecretInput{Title: "SecretTitle", Password: "SecretValue"}, "cli")
	raw, _ := json.Marshal(v.AuditLog())
	if strings.Contains(string(raw), "SecretTitle") || strings.Contains(string(raw), "SecretValue") {
		t.Error("audit log contains secret content")
	}
}

func TestExportImport(t *testing.T) {
	v := newTestVault(t)
	cat, _ := v.AddCategory("Cloud", "", "cli")
	v.AddSecret(SecretInput{Title: "One", Password: "1", CategoryID: cat.ID}, "cli")
	v.AddSecret(SecretInput{Title: "Two", Password: "2"}, "cli")
	dump, err := v.Export("cli")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(dump), `"One"`) {
		t.Fatal("export missing data")
	}

	v2 := newTestVault(t)
	n, err := v2.Import(dump, "cli")
	if err != nil || n != 2 {
		t.Fatalf("Import: n=%d err=%v", n, err)
	}
	if got, err := v2.GetSecretByTitle("Two", "cli"); err != nil || got.Password != "2" {
		t.Fatalf("imported secret wrong: %+v %v", got, err)
	}
	if len(v2.ListCategories()) != 1 {
		t.Errorf("categories not imported")
	}
	if _, err := v2.Import([]byte("not json"), "cli"); err == nil {
		t.Error("want error importing garbage")
	}
}

func TestLockFileBlocksConcurrentWrite(t *testing.T) {
	v := newTestVault(t)
	unlock, err := acquireLock(v.Path())
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Save(); !errors.Is(err, ErrLocked) {
		unlock()
		t.Fatalf("want ErrLocked, got %v", err)
	}
	unlock()
	if err := v.Save(); err != nil {
		t.Fatalf("Save after unlock: %v", err)
	}
}

func TestAtomicWriteLeavesNoTmp(t *testing.T) {
	v := newTestVault(t)
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(v.Path() + ".tmp"); !os.IsNotExist(err) {
		t.Error("tmp file left behind")
	}
	if _, err := os.Stat(v.Path() + ".lock"); !os.IsNotExist(err) {
		t.Error("lock file left behind")
	}
}

func TestGeneratePassword(t *testing.T) {
	p, err := GeneratePassword(DefaultGenerateOptions())
	if err != nil || len(p) != 16 {
		t.Fatalf("got %q err %v", p, err)
	}
	q, _ := GeneratePassword(DefaultGenerateOptions())
	if p == q {
		t.Error("two generated passwords are identical")
	}
	digits, err := GeneratePassword(GenerateOptions{Length: 32, Digits: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			t.Fatalf("non-digit %q in digits-only password", c)
		}
	}
	if _, err := GeneratePassword(GenerateOptions{Length: 8}); err == nil {
		t.Error("want error with no classes enabled")
	}
	if _, err := GeneratePassword(GenerateOptions{Length: 0, Lower: true}); err == nil {
		t.Error("want error with zero length")
	}
}

func TestKDFParamsReadFromFile(t *testing.T) {
	// A vault written with weaker (legacy) params must still open: unlock
	// reads params from the header, never from constants.
	v := newTestVault(t)
	v.kdf = KDFParams{Algo: "argon2id", Time: 1, MemoryKiB: 8 * 1024, Threads: 1}
	derived, err := deriveKey(testPassword, v.salt, v.kdf)
	if err != nil {
		t.Fatal(err)
	}
	mk, err := encrypt(v.mountKey, derived, encHex)
	if err != nil {
		t.Fatal(err)
	}
	v.mountKeyCiphered = mk
	if err := v.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(v.Path(), testPassword); err != nil {
		t.Fatalf("vault with custom KDF params failed to open: %v", err)
	}
}
