# Repository Guide (Backend Only)

## Boundaries

- This repository is the standalone Go backend extracted from `coolzyd9107/AndroidResourceDownload` (`backend/` snapshot). There is no Android/Gradle code here.
- Server wiring: `cmd/server/main.go`, routes in `internal/app/router.go`.
- Client/backend contract: `docs/FRONTEND_APPEND_V1_1.md`. Verify against `router.go` before assuming a feature works end-to-end.

## Verification

- Backend CI-equivalent gate from repo root: `go vet ./...`, `go build ./...`, then `go test -race -count=1 ./...`. `make test` runs the same race-enabled test command.
- Focus backend tests with `go test ./internal/integration -run '^TestHealth$'`. Integration tests use temporary SQLite and mock GitHub; they require no PostgreSQL, Redis, or external WebDAV service.
- Trust `go.mod` for the Go toolchain version. The workflow uses `go-version-file: go.mod`.
- `go test -race` requires a 48-bit VMA kernel; on 39-bit devices (e.g. Android/Termux proot) it aborts at runtime with `ThreadSanitizer: unsupported VMA range`. Verify locally with plain `go test ./...` and let CI run the race gate.

## Runtime Configuration

- Backend configuration is defaults plus environment variables, or YAML only when `CONFIG_FILE` names it. The server does NOT auto-load `.env`; export/source variables before `make run-sqlite`. SQLite paths are resolved relative to the process working directory, so launch from the repo root.
- The GitHub OAuth callback is fixed as `com.resdownload.android://oauth/callback` on the Android side; the backend `GITHUB_APP_REDIRECT_URI` must match exactly.
- Server startup runs GORM `AutoMigrate`; files in `migrations/` are reference SQL, not the executed migration path.
- Admins are granted by admin email domain (`ADMIN_EMAIL_DOMAINS`) or the seeded GitHub whitelist: `make seed-admin ID=<github_id> LOGIN=<login>`.

## Current Integration Traps

- Login is GitHub OAuth + QQ. The backend still serves `/api/v1/auth/email/code|login` (slated for removal — do not extend it). `POST /api/v1/auth/qq/login` re-validates the provider token against `graph.qq.com/oauth2.0/me`, requires the configured `QQ_APP_ID` (`qq.app-id`) to match Tencent's `client_id`, and keys accounts by Tencent's OpenID via the generic `auth_identities` table (provider `qq`); QQ users are always `USER` with role source `qq_default`. Integration tests override `QQ_ME_URL`/`QQ_USER_INFO_URL` with a local stub server.
- Backend API responses use `{code,message,data}` even when HTTP status is successful. Clients must decode the envelope rather than treating HTTP 2xx alone as success.
- Preserve the credential boundary: the backend only issues role credentials; Android talks directly to WebDAV afterwards.
