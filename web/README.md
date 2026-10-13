# Admin WebUI (`web/`)

Flutter Web 管理后台，只调 `/api/v1/admin/*`（契约见 `docs/ADMIN_API_V1.md`）。

## 工具链（钉死）

- Flutter 版本见 `.fvmrc`（当前 3.47.7）；`material_3_expressive` 精确钉
  `pubspec.yaml`（当前 1.1.6，无 `^`）。
- 注意该包要求 `import package:material_ui/material_ui.dart`，
  **不要** `import package:flutter/material.dart`。

## 本地运行

```bash
cd web
flutter pub get
flutter run -d chrome \
  --dart-define=API_BASE_URL=http://localhost:8080
```

后端本地起 SQLite（仓库根）：`set -a; source .env; set +a && make run-sqlite`。

## 构建（产物只从 CI 出，不手动发生产）

```bash
flutter build web --release \
  --dart-define=API_BASE_URL=https://api.example.com
```

产物 `build/web/` 由 Nginx 静态托管（见 `deployments/`，待接入）。
