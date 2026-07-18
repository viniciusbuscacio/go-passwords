# The go-passwords vault format (v3)

This document is the complete, self-sufficient specification of the `.gpw`
vault file. With it (and the master password) anyone can recover the data in
any language — no go-passwords code required. Ready-to-run recovery tools
live in [`tools/recover/`](../tools/recover/): `recover.go` (Go) and
`recover.py` (Python), both independent implementations of this spec.

## Design goals

- **Everything encrypted.** The file reveals nothing without the password —
  not even how many secrets it holds. Only crypto parameters are cleartext.
- **Boring, standard crypto.** Argon2id + AES-256-GCM, available in every
  language, so the format outlives the app.
- **Self-describing.** The header embeds a `_readme` mini-manual: opening the
  file in a text editor tells you what it is and how to recover it.
- **Single portable file.** Copy it, back it up, sync it however you want.

## File layout

A `.gpw` file is a UTF-8 JSON document:

```json
{
  "format": "go-passwords-vault",
  "version": 3,
  "_readme": ["<recovery mini-manual, informative only>"],
  "kdf": { "algo": "argon2id", "time": 3, "memory_kib": 65536, "threads": 4 },
  "salt": "<hex, 16 bytes>",
  "mount_key_ciphered": "<iv:tag:ciphertext, hex>",
  "payload": "<iv:tag:ciphertext, base64>"
}
```

| Field | Meaning |
|---|---|
| `format`, `version` | Must be `go-passwords-vault` / `3`. |
| `_readme` | Embedded recovery notes. Ignored on read, rewritten on save. |
| `kdf` | Argon2id parameters. **Always read them from the file** — they may differ between vaults and may be hardened in future versions. |
| `salt` | Argon2id salt, hex-encoded (16 bytes). |
| `mount_key_ciphered` | The 32-byte mount key, AES-256-GCM-encrypted with the derived key. Parts are **hex**. |
| `payload` | The entire vault body, AES-256-GCM-encrypted with the mount key. Parts are **base64** (standard, padded). |

### The `iv:tag:ciphertext` triplet

Every encrypted field is one string with three encoded parts joined by `:`:

- `iv` — the 12-byte AES-GCM nonce
- `tag` — the 16-byte GCM authentication tag
- `ciphertext` — the encrypted bytes

Note for most crypto libraries (Go, Python, ...): they expect and produce
`ciphertext || tag` concatenated — split or join accordingly.

## Decryption recipe

1. `key = Argon2id(password, salt, time, memory_kib, threads, keyLen=32)`
2. `mount_key = AES-256-GCM-open(mount_key_ciphered, key)` — 32 bytes.
   Failure here = wrong master password (there is **no password hash** stored;
   the GCM auth tag is the verifier).
3. `data = AES-256-GCM-open(payload, mount_key)` — failure here = corrupted
   or tampered file.
4. `data` is plain JSON:

```json
{
  "secrets": [
    { "id": "<uuid>", "title": "...", "username": "...", "password": "...",
      "url": "...", "notes": "...", "category_id": "<uuid, optional>",
      "created_at": "<RFC3339 UTC>", "updated_at": "<RFC3339 UTC>" }
  ],
  "categories": [ { "id": "<uuid>", "name": "..." } ],
  "audit":      [ { "ts": "...", "actor": "gui|cli|api", "action": "...",
                    "secret_id": "<uuid, optional>" } ]
}
```

The audit log references secrets by ID only — never titles or values — and is
capped at 10,000 entries (oldest dropped).

## Writing a vault

- Fresh random 12-byte IV for **every** encryption, always.
- The mount key is generated once (32 random bytes) at vault creation and
  never changes; changing the master password only re-encrypts
  `mount_key_ciphered` under a new salt + derived key.
- Writes are atomic: temp file in the same directory + fsync + rename. A
  sidecar `<vault>.lock` file (created with O_EXCL) guards concurrent writers;
  locks older than 30 s are considered stale leftovers from a crash.
- File permissions: `0600`.

## Threat model notes

- Offline attack on the file must go through Argon2id (memory-hard) — there is
  no faster verifier (no bcrypt/scrypt hash stored anywhere).
- GCM authenticates both ciphertexts: any bit flip in the payload or the
  mount key is detected at open time.
- What the file does leak: that it is a go-passwords vault, its KDF
  parameters, and its total size. Nothing else.
