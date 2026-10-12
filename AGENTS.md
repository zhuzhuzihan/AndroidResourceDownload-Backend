# Repository Guide (Backend Only)

## Boundaries

- Standalone Go backend (`resdownload.com/backend`); no Android/Gradle code here.
- Entry: `cmd/server/main.go`; routes in `internal/app/router.go`.
- Client contract: `docs/FRONTEND_APPEND_V1_1.md` — verify against `router.go` before assuming end-to-end behavior.

## Commands (repo root)

- CI gate (`.github/workflows/backend-ci.yml`): `go vet ./...`, `go build ./...`, then `go test -race -count=1 ./...`.
- `make test` = race-enabled full suite; `make vet`, `make build`, `make tidy` are thin wrappers.
- Focused: `go test ./internal/integration -run '^TestHealth$'`.
- Integration tests (`internal/integration/`) use temp SQLite (`t.TempDir()`) + `GITHUB_MOCK=true`; no PostgreSQL, Redis, or external WebDAV needed. QQ tests stub `graph.qq.com` locally.
- `go test -race` needs a 48-bit VMA kernel; on 39-bit runtimes it aborts with `ThreadSanitizer: unsupported VMA range` — fall back to plain `go test ./...` locally, let CI run the race gate.
- Toolchain: trust `go.mod`; CI uses `go-version-file: go.mod`.

## Config

- `internal/config/config.go` is the source of truth: defaults + env, YAML only when `CONFIG_FILE` names it. Never auto-loads `.env`; `set -a; source .env; set +a` before `make run-sqlite`.
- SQLite `DATABASE_URL` is resolved via `filepath.Abs`, i.e. relative to CWD — always launch from repo root.
- Startup runs GORM `AutoMigrate` (`internal/repository/db.go`); `migrations/*.sql` is reference only.
- Prod validation: GitHub client ID/secret + `QQ_APP_ID` required, `GITHUB_REDIRECT_URI` must be HTTPS, `GITHUB_APP_REDIRECT_URI` must equal `com.resdownload.android://oauth/callback` exactly.
- Admins: `ADMIN_EMAIL_DOMAINS` or GitHub whitelist via `make seed-admin ID=<github_id> LOGIN=<login>` (`scripts/main.go` → `admin_github_users`).

## API / Auth Traps

- All handlers return `{code,message,data}` (`internal/pkg/response/response.go`); check `code == 0`, not just HTTP 2xx.
- `POST /api/v1/auth/qq/login` (`internal/service/qq_service.go`): server re-fetches OpenID from `qq.me-url` (`QQ_ME_URL`), requires Tencent's `client_id` to equal configured `qq.app-id`, rejects claimed/server OpenID mismatch. Accounts keyed by OpenID in `auth_identities` (provider `qq`); new QQ users get `USER` / `RoleSourceQqDefault`. Override `QQ_ME_URL`/`QQ_USER_INFO_URL` with a stub in tests.
- ECDH issuance v1 (`keyType=x25519-xchacha20poly1305-v1`, `internal/pkg/crypto/ecdh.go`): `POST /webdav/credential` and `POST /update/resolve` switch to the encrypted envelope branch when the request carries `clientPublicKey`; legacy requests without it still get plaintext. `update/resolve` is single-use per JTI (replay → `update_url_expired`). Contract in `docs/FRONTEND_APPEND_V1_1.md` §ECDH; keep `ecdh.go`'s construction comment in sync with the Android implementation.
- Backend only issues role-scoped WebDAV credentials (`POST /webdav/credential`); Android connects to WebDAV directly — do not add a server-side proxy.
