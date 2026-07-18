// recover.go — standalone recovery tool for go-passwords vaults.
//
// This file intentionally does NOT import the go-passwords code: it is an
// independent implementation of the vault format, so your data stays
// recoverable even if the main app is gone. Only the Go standard library and
// golang.org/x/crypto are used.
//
// Usage:
//
//	go run recover.go <vault.gpw>
//
// It asks for the master password and prints the decrypted JSON to stdout.
package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/argon2"
)

type headerFile struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	KDF     struct {
		Algo      string `json:"algo"`
		Time      uint32 `json:"time"`
		MemoryKiB uint32 `json:"memory_kib"`
		Threads   uint8  `json:"threads"`
	} `json:"kdf"`
	Salt             string `json:"salt"`
	MountKeyCiphered string `json:"mount_key_ciphered"`
	Payload          string `json:"payload"`
}

func die(format string, v ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", v...)
	os.Exit(1)
}

// decrypt opens an "iv:tag:ciphertext" string with AES-256-GCM.
// decode converts each part from its text encoding (hex or base64).
func decrypt(s string, key []byte, decode func(string) ([]byte, error)) ([]byte, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected iv:tag:ciphertext")
	}
	iv, err := decode(parts[0])
	if err != nil {
		return nil, err
	}
	tag, err := decode(parts[1])
	if err != nil {
		return nil, err
	}
	ct, err := decode(parts[2])
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, append(ct, tag...), nil)
}

func main() {
	if len(os.Args) != 2 {
		die("usage: go run recover.go <vault.gpw>")
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		die("%v", err)
	}
	var h headerFile
	if err := json.Unmarshal(raw, &h); err != nil {
		die("not a go-passwords vault: %v", err)
	}
	if h.Format != "go-passwords-vault" || h.Version != 3 || h.KDF.Algo != "argon2id" {
		die("unsupported format %q version %d kdf %q", h.Format, h.Version, h.KDF.Algo)
	}
	salt, err := hex.DecodeString(h.Salt)
	if err != nil {
		die("bad salt: %v", err)
	}

	fmt.Fprint(os.Stderr, "Master password: ")
	password, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	password = strings.TrimRight(password, "\r\n")

	key := argon2.IDKey([]byte(password), salt, h.KDF.Time, h.KDF.MemoryKiB, h.KDF.Threads, 32)
	mountKey, err := decrypt(h.MountKeyCiphered, key, hex.DecodeString)
	if err != nil {
		die("wrong master password")
	}
	payload, err := decrypt(h.Payload, mountKey, base64.StdEncoding.DecodeString)
	if err != nil {
		die("payload corrupted: %v", err)
	}
	var pretty any
	if err := json.Unmarshal(payload, &pretty); err != nil {
		die("payload is not JSON: %v", err)
	}
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Println(string(out))
}
