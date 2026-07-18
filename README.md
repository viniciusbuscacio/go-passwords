# go-passwords

**An offline password manager built for humans and AI agents.** Part of the
[go-apps](https://github.com/viniciusbuscacio/go-apps) family — Go, single
binaries, no accounts, no cloud, no telemetry.

The successor of
[passwordplease](https://github.com/viniciusbuscacio/passwordplease),
rewritten in Go around a fully encrypted vault format.

## Status

**v0.1 in development.** Working today: the vault core (`internal/vault`) and
the CLI (`go-pw-cli`). Coming next: the desktop GUI (Wails, family style) and
the embedded REST API for agents (`serve`).

## Why

- **100% offline** — the vault is one local file. Copy it, back it up, sync
  it however you want.
- **Everything encrypted** — the whole file is an AES-256-GCM container;
  keys derived with Argon2id. Without the master password it reveals nothing,
  not even how many secrets it holds.
- **Agent-friendly** — a small static CLI binary with env vars
  (`GO_PW_VAULT`, `GO_PW_MASTER_PASSWORD`) and `--json` output; a REST API
  (coming) so agents can drive the vault programmatically. Every operation is
  recorded in an audit log inside the vault.
- **Recoverable forever** — the format is open and boring on purpose:
  [docs/FORMAT.md](docs/FORMAT.md) specifies it completely, the file header
  embeds a recovery mini-manual, and [`tools/recover/`](tools/recover/) ships
  independent Go and Python scripts that decrypt a vault with nothing but the
  master password. All of it exercised in CI.

## CLI quick start

```sh
go build -o gpw ./cmd/go-pw-cli

export GO_PW_VAULT=~/secrets.gpw
export GO_PW_MASTER_PASSWORD=...   # or use the interactive prompt

./gpw init
./gpw set "AWS Token" --username admin --password "AKIA..." --url https://aws.amazon.com
./gpw list
./gpw get "AWS Token" --field password
./gpw get "AWS Token" --json
./gpw generate --length 24
./gpw audit
```

Run `./gpw help` for everything: categories, export/import,
change-master-password, status.

## The vault format

One `.gpw` file: a cleartext JSON header carrying only crypto parameters
(Argon2id settings, salt, the encrypted mount key) plus a single encrypted
payload with all data — secrets, categories and the audit log. Master
password verification is the GCM auth tag on the mount key; no password hash
is stored anywhere. Full spec: [docs/FORMAT.md](docs/FORMAT.md).

## Security model in one paragraph

Your master password goes through Argon2id (64 MiB, memory-hard) to derive a
key that decrypts a random 32-byte mount key; the mount key decrypts the
payload. Fresh random IV on every write, atomic file writes (temp + rename),
sidecar lock against concurrent writers, `0600` permissions. Offline attacks
have no shortcut past Argon2id. What the file leaks: that it is a vault, its
KDF parameters and its size — nothing else.

## Development

```sh
go test ./...   # vault core test suite
go vet ./...
```

Design decisions and family conventions live in the
[go-apps](https://github.com/viniciusbuscacio/go-apps) hub; visual identity in
[go-design](https://github.com/viniciusbuscacio/go-design).

## Roadmap

- [x] Vault format v3 (encrypted container) + core
- [x] CLI `go-pw-cli`
- [x] Recovery scripts (Go + Python) tested in CI
- [ ] Desktop GUI (Wails, go-design look)
- [ ] Embedded REST API for agents (`serve`, go-apiserver)
- [ ] Installers + self-update (go-installer, go-updates)
- [ ] Touch ID (macOS), Windows Hello

## License

[MIT](LICENSE)
