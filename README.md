# AndroidResourceDownload-Backend

Android WebDAV 文件客户端配套后端服务。负责认证、角色识别、WebDAV 凭据加密下发、App 更新 URL 加密下发。

> 来源：从 `coolzyd9107/AndroidResourceDownload` 的 `backend/` 目录独立拆出（快照拆分，不含 Android 客户端代码）。客户端契约见 `docs/FRONTEND_APPEND_V1_1.md`。

GitHub OAuth App 的 callback 必须指向后端 HTTPS 地址，例如 `https://api.example.com/api/v1/auth/github/callback`。后端回到 Android 时只携带 90 秒有效的一次性 code；session token 和 WebDAV 凭据不会写入跳转 URL。

- 语言：Go（以 `go.mod` 声明的版本为准）
- Web 框架：Gin
- 数据库：PostgreSQL（生产）/ SQLite（本地开发与单元测试）
- 缓存/限流：内存令牌桶（默认）/ Redis（生产可选）
- API 契约：见 `docs/FRONTEND_APPEND_V1_1.md`

## 快速开始（本地 SQLite）

```bash
go mod download
make run-sqlite
```

服务默认监听 `:8080`，健康检查 `GET /health`，所有业务接口前缀 `/api/v1`。

## 配置

所有配置通过环境变量覆盖；默认值见 `config.example.yaml`。常用环境变量：

| 变量 | 默认 | 说明 |
|---|---|---|
| `APP_ENV` | `dev` | dev / prod |
| `SERVER_PORT` | `8080` | HTTP 端口 |
| `DATABASE_DRIVER` | `sqlite` | `sqlite` 或 `postgres` |
| `DATABASE_URL` | `data/dev.db` | SQLite 路径 或 PG DSN |
| `JWT_SECRET` | _required_ | access/refresh token 签名密钥 |
| `CREDENTIAL_SECRET` | _required_ | WebDAV 凭据加密密钥 |
| `UPDATE_URL_SECRET` | _required_ | 更新 URL 加密/签名密钥 |
| `EMAIL_MODE` | `console` | `console` / `smtp` |
| `GITHUB_CLIENT_ID` | _empty_ | GitHub OAuth |
| `GITHUB_CLIENT_SECRET` | _empty_ | GitHub OAuth |
| `GITHUB_REDIRECT_URI` | _required_ | 后端 HTTPS callback，例如 `/api/v1/auth/github/callback` |
| `GITHUB_APP_REDIRECT_URI` | `com.resdownload.android://oauth/callback` | callback 完成后返回 App 的固定 URI，必须与 Android 端完全一致 |
| `QQ_APP_ID` | _empty_ | QQ 互联 AppID；`POST /api/v1/auth/qq/login` 用它向 `graph.qq.com` 复核客户端上报的 access token 与 OpenID |
| `QQ_ME_URL` / `QQ_USER_INFO_URL` | graph.qq.com 对应端点 | 仅测试时覆盖 |
| `ADMIN_EMAIL_DOMAINS` | _empty_ | 可选的管理员邮箱域名列表；默认通过 GitHub 白名单授予管理员权限 |
| `WEBDAV_BASE_URL` | `https://dav.example.com` | WebDAV 服务地址 |
| `WEBDAV_READONLY_USERNAME` / `WEBDAV_READONLY_PASSWORD` | _empty_ | 普通用户凭据 |
| `WEBDAV_ADMIN_USERNAME` / `WEBDAV_ADMIN_PASSWORD` | _empty_ | 管理员凭据 |

注意：服务端不会自动加载 `.env` 文件。复制 `.env.example` 为 `.env` 后，需 `set -a; source .env; set +a`（或自行 export）再启动；也可用 `CONFIG_FILE` 指向 YAML 配置文件。

## 常用命令

```bash
make help            # 查看所有命令
make build           # 编译到 bin/server
make test            # 运行单元测试
make vet             # go vet
make run-sqlite      # 以 SQLite 启动
make run-pg          # 以 PostgreSQL 启动（需 DATABASE_DRIVER=postgres）
make seed-admin ID=1234567 LOGIN=alice  # 添加 GitHub 管理员白名单
```

## 部署

```bash
cd deployments
docker compose up -d
```

包含 PostgreSQL、Redis、后端、Nginx。HTTPS 终止由 Nginx 处理。

## 目录结构

```
.
├── cmd/server/          入口
├── internal/app/        路由与 HTTP server
├── internal/config/     Viper 配置加载
├── internal/middleware/ 中间件
├── internal/handler/    HTTP handler
├── internal/service/    业务服务
├── internal/repository/ 数据访问
├── internal/model/      GORM 模型
├── internal/dto/        请求/响应 DTO
├── internal/ratelimit/  限流
├── internal/pkg/        通用工具（jwt/crypto/logger/validator/response）
├── migrations/          SQL 迁移（参考，实际启动走 GORM AutoMigrate）
├── deployments/         Docker / Nginx
└── docs/                客户端契约（FRONTEND_APPEND_V1_1.md）
```

## 已知风险

- WebDAV 静态账号：所有 USER 共用一个只读账号，管理员同理；操作无法精确到个人。MVP 范围，后续可换动态临时账号。
- 加密预共享密钥：`CREDENTIAL_SECRET`、`UPDATE_URL_SECRET` 需妥善保管，泄露后凭据/更新链接可被解密。
- 当前不实现真实 WebDAV 代理；后端只下发凭据，由 Android 客户端直连 WebDAV。
