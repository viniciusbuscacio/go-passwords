package vault

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Vault is an unlocked go-passwords vault held in memory. All mutating
// operations only touch the in-memory state; Save writes the encrypted
// container back to disk atomically.
type Vault struct {
	path     string
	mountKey []byte
	kdf      KDFParams
	salt     []byte
	// mountKeyCiphered is kept so Save can rewrite the header without
	// re-deriving anything; it only changes on ChangeMasterPassword.
	mountKeyCiphered string
	data             payload
}

// ErrVaultExists is returned by Create when the target file already exists.
var ErrVaultExists = errors.New("vault file already exists")

// Create makes a new empty vault at path, protected by masterPassword, and
// writes it to disk.
func Create(path, masterPassword string) (*Vault, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("%w: %s", ErrVaultExists, path)
	}
	salt, err := randomBytes(saltLen)
	if err != nil {
		return nil, err
	}
	mountKey, err := randomBytes(keyLen)
	if err != nil {
		return nil, err
	}
	kdf := DefaultKDFParams()
	derived, err := deriveKey(masterPassword, salt, kdf)
	if err != nil {
		return nil, err
	}
	mkCiphered, err := encrypt(mountKey, derived, encHex)
	if err != nil {
		return nil, err
	}
	v := &Vault{
		path:             path,
		mountKey:         mountKey,
		kdf:              kdf,
		salt:             salt,
		mountKeyCiphered: mkCiphered,
		data:             payload{Secrets: []Secret{}, Categories: []Category{}, Audit: []AuditEntry{}},
	}
	if err := v.Save(); err != nil {
		return nil, err
	}
	return v, nil
}

// Open unlocks the vault at path with masterPassword. A wrong password fails
// the GCM auth tag on the mount key and returns ErrInvalidPassword — no
// password hash is stored anywhere.
func Open(path, masterPassword string) (*Vault, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var h header
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, ErrCorrupted
	}
	if h.Format != FormatName || h.Version != FormatVersion {
		return nil, fmt.Errorf("%w: format %q version %d", ErrCorrupted, h.Format, h.Version)
	}
	salt, err := hex.DecodeString(h.Salt)
	if err != nil || len(salt) != saltLen {
		return nil, ErrCorrupted
	}
	derived, err := deriveKey(masterPassword, salt, h.KDF)
	if err != nil {
		return nil, err
	}
	mountKey, err := decrypt(h.MountKeyCiphered, derived, encHex)
	if err != nil {
		return nil, ErrInvalidPassword
	}
	plain, err := decrypt(h.Payload, mountKey, encBase64)
	if err != nil {
		return nil, ErrCorrupted
	}
	var data payload
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, ErrCorrupted
	}
	return &Vault{
		path:             path,
		mountKey:         mountKey,
		kdf:              h.KDF,
		salt:             salt,
		mountKeyCiphered: h.MountKeyCiphered,
		data:             data,
	}, nil
}

// Save encrypts the payload and writes the container atomically: temp file +
// fsync + rename, guarded by a sidecar lock file so two processes never write
// at the same time.
func (v *Vault) Save() error {
	plain, err := json.Marshal(v.data)
	if err != nil {
		return err
	}
	payloadCiphered, err := encrypt(plain, v.mountKey, encBase64)
	if err != nil {
		return err
	}
	h := header{
		Format:           FormatName,
		Version:          FormatVersion,
		Readme:           readmeText,
		KDF:              v.kdf,
		Salt:             hex.EncodeToString(v.salt),
		MountKeyCiphered: v.mountKeyCiphered,
		Payload:          payloadCiphered,
	}
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	unlock, err := acquireLock(v.path)
	if err != nil {
		return err
	}
	defer unlock()
	return atomicWrite(v.path, raw)
}

// ChangeMasterPassword re-encrypts the mount key under a key derived from the
// new password (fresh salt, current default KDF params). The payload is
// untouched — the mount key itself never changes.
func (v *Vault) ChangeMasterPassword(current, newPassword string) error {
	derived, err := deriveKey(current, v.salt, v.kdf)
	if err != nil {
		return err
	}
	if _, err := decrypt(v.mountKeyCiphered, derived, encHex); err != nil {
		return ErrInvalidPassword
	}
	newSalt, err := randomBytes(saltLen)
	if err != nil {
		return err
	}
	newKDF := DefaultKDFParams()
	newDerived, err := deriveKey(newPassword, newSalt, newKDF)
	if err != nil {
		return err
	}
	mkCiphered, err := encrypt(v.mountKey, newDerived, encHex)
	if err != nil {
		return err
	}
	v.salt = newSalt
	v.kdf = newKDF
	v.mountKeyCiphered = mkCiphered
	v.audit("cli", "change_master_password", "")
	return v.Save()
}

// Path returns the vault file path.
func (v *Vault) Path() string { return v.path }

// Lock wipes the in-memory key material. The Vault must not be used after.
func (v *Vault) Lock() {
	for i := range v.mountKey {
		v.mountKey[i] = 0
	}
	v.mountKey = nil
	v.data = payload{}
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }
