# Admin API v1（管理后台）

所有接口均需登录 + `ADMIN` 角色：`Authorization: Bearer <accessToken>`
（沿用现有登录通道：管理员邮箱域名 OTP 或 GitHub 白名单）。
无 token → 401；非 ADMIN → 403（`code=10003`）。另有独立限流
`admin_per_user`（约 30 次突发/分钟）。

响应包络与业务接口一致：`{ code, message, data }`，先看 `code == 0`。

## 版本发布

- `GET /api/v1/admin/versions?limit=50&offset=0`
  → `data: { items: [...], total }`，按 `versionCode` 倒序。`limit` 上限 200。
- `POST /api/v1/admin/versions`
  `{ versionCode, versionName, forceUpdate, changelog, targetUrl }`
  `versionCode` 必须大于当前最大（否则 400 `version_not_newer`，重复 409
  `version_exists`）；`targetUrl` 必须绝对 HTTPS 地址。成功记审计
  `admin.version.publish`。
- `DELETE /api/v1/admin/versions/:code`
  仅允许删**最新版**（即回滚，删后上一版自动成为最新）；删历史版本 409
  `not_latest_version`，不存在 404。成功记审计 `admin.version.delete`。

发版后可用 `GET /api/v1/update/info` 直接验证线上最新。

## 下发监控

- `GET /api/v1/admin/stats/issuance?days=7`（`days` 1..90）
  → `data.days[]: { date(UTC), credentialTotal, credentialEcdh,
  resolveTotal, resolveEcdh, resolveFailed, credentialEcdhRate,
  resolveEcdhRate }`。`key_type` 为空=老明文分支，v1=ECDH 分支。
  ECDH 迁入率与老分支下线看这张表。`resolveFailed` 含过期/重放/篡改
  的失败兑换（失败也落库一行 `resolved=false`，精确到错误码的分布
  看服务端日志）。
- `GET /api/v1/admin/alerts/sessions?windowH=24&minIps=3`
  → 同一用户在窗口内（`windowH` 小时，≤720）登录成功或领取凭据时出现
  ≥ `minIps`（≥2）个不同 IP 即告警：
  `data.alerts[]: { userId, ipCount, ips[], lastSeen }`。
  抓共享账号/盗号用。注意 IP 可能为 NAT 出口，阈值按实际调整。

## 日志查询

- `GET /api/v1/admin/logs/credentials?user_id=&from=&to=&limit=&offset=`
- `GET /api/v1/admin/logs/updates?...&resolved=true|false`
- `GET /api/v1/admin/logs/audit?...&action=`
  `from/to` 为 RFC3339；`limit` 上限 200。返回 `data: { items, total }`，
  按时间倒序。密码类敏感字段本来就不进库，日志可放心展示。

## 红线

- 不返回任何 secret（JWT/凭据加密密钥、WebDAV 密码库里本就没有）。
- 不提供批量导出用户表。
- 所有写操作（发版、删版）强制记 `audit_logs`，操作人=调用 admin。
