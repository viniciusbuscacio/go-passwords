# go-passwords — agent notes

Password manager of the [go-apps](https://github.com/viniciusbuscacio/go-apps)
family: encrypted single-file vault (`.gpw`), Wails GUI, `go-pw-cli` CLI and
an embedded REST API.

**Before changing anything, read the family rules** — engineering:
[go-apps/AGENTS.md](https://github.com/viniciusbuscacio/go-apps/blob/main/AGENTS.md)
(local sibling checkout: `../go-apps/AGENTS.md`) — UI/visuals:
[go-design](https://github.com/viniciusbuscacio/go-design)
(`../go-design/README.md`). go-notepad is the family's visual reference.

App specifics:

- Vault format v3 (Argon2id + AES-256-GCM container, atomic writes, audit
  log with rotation) lives in `internal/vault` — pure Go, heavily tested.
  The open format spec is `docs/FORMAT.md`; standalone recovery scripts in
  `tools/recover/` are tested in CI and must keep opening current vaults.
- Secrets never reach the frontend unasked: the UI bridge reports password
  fields by length only; clipboard copy happens in Go (`CopySecretField`).
- REST API port range **8900–8999**; endpoints under `/v1/secrets`,
  `/v1/categories`, `/v1/unlock|lock`, `/v1/generate`, `/v1/export|import`,
  `/v1/audit`. A locked vault answers `423 Locked`.
- CLI (`cmd/go-pw-cli`) is stateless per invocation; env vars `GO_PW_VAULT`
  and `GO_PW_MASTER_PASSWORD`.
- Content zoom via `.zoom-host` (Ctrl +/−/0, Ctrl+wheel), 50–200%.
- Gate before commit: `go vet ./...`, `go test ./...`, `wails build`.
