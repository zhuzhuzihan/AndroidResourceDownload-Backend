# App 更新加密下发接入（ECDH v1）

面向 Android 端实现。`keyType = "x25519-xchacha20poly1305-v1"`。
后端实现以 `internal/pkg/crypto/ecdh.go` 为准；本节向量以
`internal/pkg/crypto/ecdh_test.go` 为准，如有出入以后端代码为准。

## 1. 流程

```
GET /api/v1/update/info  (Authorization 必带)
  → data: { versionCode, versionName, forceUpdate, changelog,
            encryptedUrl, expiresAt, signature }
  → 本地三段版本比较（规则不变，见 §7），需要更新才继续
POST /api/v1/update/resolve { encryptedUrl, clientPublicKey, keyType }
  → data: { expiresIn, encryptedUrl: { serverPublicKey, nonce, ciphertext, keyType } }
  → 本地 ECDH 解密 → { versionCode, targetUrl, exp, jti }
  → 在内存中使用 targetUrl（系统浏览器打开 / 下载），遵守 exp
```

`encryptedUrl`（info 返回的）**单次有效**：同一值第二次 resolve 返回
`code=30001 update_url_expired`。不要缓存、不要重试复用，过期就重新
`GET info`。

## 2. 请求 / 响应字段

`POST /api/v1/update/resolve` 请求（JSON）：

| 字段 | 说明 |
|---|---|
| `encryptedUrl` | 必填，`GET info` 原样返回的字符串 |
| `clientPublicKey` | 本次临时 X25519 公钥，base64url（无填充）裸 32 字节 |
| `keyType` | 固定 `"x25519-xchacha20poly1305-v1"`（缺省亦可，显式传最稳） |

响应 `data`（加密分支**没有** `url` 字段，不得回退去读它）：

| 字段 | 说明 |
|---|---|
| `expiresIn` | 建议有效秒数，仅参考 |
| `encryptedUrl.serverPublicKey` | 服务端本次临时公钥，base64url 裸 32 字节 |
| `encryptedUrl.nonce` | base64url，24 字节 |
| `encryptedUrl.ciphertext` | base64url，密文 + 16 字节 tag |
| `encryptedUrl.keyType` | 回显，必须与请求一致否则中止 |

## 3. 客户端解密步骤（与后端逐行对应）

1. 每次 resolve **新生成**临时 X25519 keypair（`clientPriv` 32 字节随机，
   `clientPub = X25519(clientPriv, basepoint)`），用完清零。
2. `shared = X25519(clientPriv, serverPub)`；全零结果视为失败。
3. `salt = serverPub || clientPub`（**此顺序**，各 32 字节，共 64 字节）。
4. `info = "webdavbox-ecdh-v1|update"`（UTF-8 精确字节；凭据分支是
   `"webdavbox-ecdh-v1|credential"`，不可混用）。
5. `key = HKDF-SHA256(salt, shared, info, 32)`。
6. `明文 = XChaCha20-Poly1305-Open(key, nonce, ciphertext, aad)`，
   其中 `aad = "x25519-xchacha20poly1305-v1"` 的 UTF-8 字节。
   失败即中止，不区分原因、不重试同一 envelope。
7. 明文是 JSON：`{ "versionCode": 42, "targetUrl": "https://…",
   "exp": 173…（unix 秒）, "jti": "…" }`。校验 `exp > now`，
   `versionCode` 与 info 一致，然后使用 `targetUrl`。

## 4. 平台实现（BouncyCastle 轻量 API，不手写算法）

按仓库规矩（`AGENTS.md` §Crypto Rules）：加密原语一律用受审计的库，
只手写组装胶水。Android 侧用 `org.bouncycastle:bcprov-jdk18on`，
以下三个类即覆盖全部所需原语（均为纯 Java，无 minSdk 限制）：

| 步骤 | BC 类 | 用法 |
|---|---|---|
| X25519 | `X25519Agreement` + `X25519PrivateKeyParameters` / `X25519PublicKeyParameters` | 临时 keypair 由 `SecureRandom` 32 字节生成；`agreement.calculateAgreement(peerPub, shared, 0)` 得 32 字节共享密钥；全零结果视为失败 |
| HKDF-SHA256 | `HKDFBytesGenerator` + `SHA256Digest` | `init(HKDFParameters(shared, salt, info))`，`generateBytes(key, 0, 32)`；`salt = serverPub \|\| clientPub`（§3 顺序），`info` 见 §3 |
| XChaCha20-Poly1305 | `org.bouncycastle.crypto.modes.XChaCha20Poly1305` | `init(false, AEADParameters(KeyParameter(key), 128, nonce, aad))`，`aad` 为 keyType 的 UTF-8 字节；`doFinal` 抛异常即解密失败，中止 |

**红线：只用 `org.bouncycastle.crypto.*` 轻量 API，不要注册 JCE
Provider**（`Security.addProvider`）。Android 系统自带老版本 BC，
注册会冲突；直接 new 这些类当普通 Java 类用即可。

不选 Tink 的原因：Tink 不暴露裸 X25519 密钥协商（只有 ECIES/HPKE，
和本线格式对不上），且其 AEAD 默认带 5 字节 key-id 前缀，需配
`NO_PREFIX` 才互通——引入它只解决三分之一问题。JCE 的 `XDH`
（API 31+）可备选做 X25519，但为统一仍建议全走 BC。

依赖体积：bcprov jar 本体较大，R8/minify 后只保留用到的类，
实际增量以 App 仓实测为准。

## 5. 测试向量（十六进制）

X25519（BoringSSL 向量）：

```
in:     668fb9f76ad971c81ac900071a1560bce2ca00cac7e67af99348913761434014
base:   db5f32b7f841e7a1a00968effded12735fc47a3eb13b579aacadeae80939a7dd
expect: 090d85e599ea8e2beeb61304d37be10ec5c905f9927d32f42a9a0afb3e0b4074
```

即 `X25519(in, base) == expect`。

XChaCha20-Poly1305（draft-irtf-cfrg-xchacha 附录 A.3.1）：

```
key:   808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9f
nonce: 404142434445464748494a4b4c4d4e4f5051525354555657
aad:   50515253c0c1c2c3c4c5c6c7
pt:    4c616469657320616e642047656e746c656d656e206f662074686520636c617373206f66202739393a204966204920636f756c64206f6666657220796f75206f6e6c79206f6e652074697020666f7220746865206675747572652c2073756e73637265656e20776f756c642062652069742e
ct:    bd6d179d3e83d43b9576579493c0e939572a1700252bfaccbed2902c21396cbb731c7f1b0b4aa6440bf3a82f4eda7e39ae64c6708c54c216cb96b72e1213b4522f8c9ba40db5d945b11b69b982c1bb9e3f3fac2bc369488f76b2383565d3fff921f9664c97637da9768812f615c68b13b52ec0875924c1c7987947deafd8780acf49
```

HKDF-SHA256：用 RFC 5869 §A.3 Test Case 3（80 字节 `00…4f` 作 IKM、
80 字节 `60…af` 作 salt、80 字节 `b0…ff` 作 info，`L=42`），OKM 以
RFC 原文为准。

## 6. 联调检查清单

1. 三个向量各自通过（X25519 / XChaCha20 / HKDF）。
2. 真机走完整流程：info → resolve（带公钥）→ 本地解密 →
   `targetUrl` 与 seeded 版本一致；响应中**无** `url` 字段。
3. 同一 `encryptedUrl` resolve 两次，第二次 `code=30001`。
4. 篡改 `ciphertext` 任一字节 → 解密失败（App 侧报错，不发第二次请求）。
5. 跨 scope：用凭据分支的 envelope 调更新解密（或反之）→ 必须失败
   （后端测试 `TestECDHScopeIsolation` 同理）。
6. 老版本 App（不带 `clientPublicKey`）仍拿到明文 `url`，行为不变。

## 7. 不变的规则与红线

- 版本比较：`BuildConfig.VERSION_NAME` 三段数字逐段比，仅远端更高才提示；
  打开/下载走 HTTPS 校验（沿用现有逻辑）。
- 外层响应包络仍是 `{ code, message, data }`，先看 `code == 0`。
  常见错误：`30001 update_url_expired`（过期或已兑换）、
  `30002 update_url_invalid`（篡改/公钥非法/keyType 不支持）、
  `40001 rate_limited`（resolve 限流约每用户 5 次突发，不要紧循环重试）。
- targetUrl、解密中间值只驻进程内存：不写 Room/DataStore/文件/日志；
  `exp` 过期即丢弃；每次请求换新 keypair，私钥字节用完后清零；禁止把长期密钥内置进 APK。
- WebDAV 凭据的加密分支同构（scope 换成 `credential`，接口
  `POST /api/v1/webdav/credential`），另行接入时照此办理。
