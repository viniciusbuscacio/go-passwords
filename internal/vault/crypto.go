package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// KDFParams are stored in the vault header so they can be hardened in the
// future without breaking older vaults: unlock always reads them from the
// file, never from constants.
type KDFParams struct {
	Algo      string `json:"algo"`
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
}

// DefaultKDFParams are used when creating a new vault.
func DefaultKDFParams() KDFParams {
	return KDFParams{Algo: "argon2id", Time: 3, MemoryKiB: 64 * 1024, Threads: 4}
}

const (
	keyLen   = 32
	saltLen  = 16
	nonceLen = 12
)

var (
	// ErrInvalidPassword is returned when the master password cannot decrypt
	// the mount key (the GCM auth tag fails).
	ErrInvalidPassword = errors.New("invalid master password")
	// ErrCorrupted is returned when the file structure cannot be parsed.
	ErrCorrupted = errors.New("vault file is corrupted or not a go-passwords vault")
)

func deriveKey(password string, salt []byte, p KDFParams) ([]byte, error) {
	if p.Algo != "argon2id" {
		return nil, fmt.Errorf("%w: unsupported kdf %q", ErrCorrupted, p.Algo)
	}
	return argon2.IDKey([]byte(password), salt, p.Time, p.MemoryKiB, p.Threads, keyLen), nil
}

func randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// encrypt seals plaintext with AES-256-GCM and encodes it as "iv:tag:ciphertext".
// enc selects the text encoding of the three parts (hex or base64).
func encrypt(plaintext, key []byte, enc encoding) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce, err := randomBytes(nonceLen)
	if err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)
	ct, tag := sealed[:len(sealed)-gcm.Overhead()], sealed[len(sealed)-gcm.Overhead():]
	return enc.encode(nonce) + ":" + enc.encode(tag) + ":" + enc.encode(ct), nil
}

// decrypt reverses encrypt. A wrong key or tampered data fails the GCM auth
// tag and returns an error.
func decrypt(s string, key []byte, enc encoding) ([]byte, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return nil, ErrCorrupted
	}
	nonce, err1 := enc.decode(parts[0])
	tag, err2 := enc.decode(parts[1])
	ct, err3 := enc.decode(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return nil, ErrCorrupted
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, ErrCorrupted
	}
	return gcm.Open(nil, nonce, append(ct, tag...), nil)
}

type encoding int

const (
	encHex encoding = iota
	encBase64
)

func (e encoding) encode(b []byte) string {
	if e == encHex {
		return hex.EncodeToString(b)
	}
	return base64.StdEncoding.EncodeToString(b)
}

func (e encoding) decode(s string) ([]byte, error) {
	if e == encHex {
		return hex.DecodeString(s)
	}
	return base64.StdEncoding.DecodeString(s)
}
