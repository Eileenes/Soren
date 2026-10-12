# Soren macOS 构建与本地运行

## 构建

当前项目使用 Tauri 2 + React + Go。桌面应用会启动内置 Go 服务并在退出时回收；默认 SQLite 与内存缓存，无需安装 PostgreSQL、Redis 或 Go 运行时。

开发所需工具：Node.js 20+、Go 1.21+、Rust stable、Xcode Command Line Tools。将项目从其他操作系统迁移后，先在当前机器重新安装前端依赖，避免使用 Linux 的 esbuild / Tauri 原生模块。

```sh
cd /path/to/Soren
npm --prefix frontend ci
npm run build:mac
```

产物目录：

- `frontend/src-tauri/target/release/bundle/macos/Soren.app`
- `frontend/src-tauri/target/release/bundle/dmg/`

`build:mac` 使用本机 Rust 目标架构。本次采用 Go 1.25.1 构建，安装包最低系统设为 macOS 12，和 [Go 1.26.9 的 macOS 支持范围](https://go.dev/doc/go1.25#darwin)及 sidecar 二进制的 `minos 12.0` 一致。Apple Silicon 产物只能直接用于 Apple Silicon Mac；本次不包含 Intel 或 iOS/Android 原生安装包。若需要 Intel 包，需要先安装对应 Rust target，再执行 `bash scripts/build.sh --target x86_64-apple-darwin --bundles app,dmg`。

后端按 [Tauri 官方 sidecar 规范](https://v2.tauri.app/develop/sidecar/)生成 `binaries/soren-server-<target-triple>`，由 `externalBin` 随 App 一起打包。安装后的后端位于 `Soren.app/Contents/MacOS/soren-server`。

## 使用

打开 DMG，将 Soren 拖入 Applications，再双击启动。当前包使用 ad-hoc 本地签名，没有 Developer ID 签名或 Apple 公证；本机可运行，跨机器下载分发可能出现系统安全确认。正式公开发行需要开发者证书和公证。

首次启动可打开界面和管理本地数据；真实 AI 对话需要在设置中填写服务商、API Key 和接口地址。保存后桌面进程重启内置服务，新配置立即用于后续请求。空 API Key 只在服务商与接口地址均未改变时保留旧值；更改地址需重新填写，避免将旧凭据发送到另一端点。

## 数据与配置

所有用户数据位于 `~/.soren`，不会写入 App 安装目录：

| 文件 | 用途 |
| --- | --- |
| `soren.db` | 本地对话与长期记忆 |
| `provider.json` | 桌面设置保存的服务商、接口地址、API Key |
| `backend.log` | 当前服务日志 |
| `backend.previous.log` | 上次轮转的日志（启动时超过 5 MB） |
| `.env`（可选） | 飞书、钉钉等高级服务端配置，由 Go 自动读取 |

目录权限为 `0700`，配置和日志文件以 `0600` 创建。API Key 保存在当前用户可读的本地配置文件中；当前版本尚未集成 macOS Keychain，因此这不是加密凭据库。UI 读取配置状态时不会返回 API Key。

设置中的桌面服务商配置优先于 `.env` 里的 OpenAI / Anthropic 凭据；API 端口固定为 `127.0.0.1:8080`，以匹配当前前端。端口被其他程序占用时会报错，不会接管或杀死第三方进程。日志和桌面配置状态可用于诊断；关闭冲突程序后重启 Soren。

## 开发命令

```sh
npm run dev           # Tauri 桌面开发；自动构建并启动后端
npm run dev:web       # 只启动前端浏览器开发服务器
npm run dev:backend   # 单独启动 Go 服务，读取 backend/.env
npm run build:backend
cargo test --manifest-path frontend/src-tauri/Cargo.toml
```

不要同时运行桌面 App 和单独的 `dev:backend`，它们默认都使用端口 8080。可将独立 Go 服务的 `SERVER_PORT` 设为其他端口用于测试。

## IPC 接口

- `get_desktop_config` → `{ provider, baseURL, configured, dataDir, backendReady, startupError }`
- `save_desktop_config({ config: { provider, baseURL, apiKey? } })` → 保存成功并重启后的同一结构，错误时 reject
- `open_data_directory` → 用系统文件管理器打开数据目录

支持的 provider 为 `openai` 与 `anthropic`；`openai` 也用于自定义 OpenAI 兼容接口。网页模式不具有上述桌面 IPC，不能通过前端假装保存服务端凭据。

## 本次验证

2026-10-11 重新构建完成（包含最新图标、深色主题和精简文案）：

- 可分发副本：`release/Soren-0.1.0-macOS-arm64.dmg`，约 10.76 MiB。
- SHA-256：`3824410cd3d3481072ab070a311051604352aa765bef8039132ccbffc83b07ec`。
- `npm run build:mac` 成功；包含 `.app`、Go sidecar 与最新 ICNS。
- `codesign --verify --deep --strict` 通过；这是 ad-hoc 签名，不代表 Apple 公证。
- `hdiutil verify` 校验通过。
- 从最终 App 包内直接启动 Go sidecar，使用独立临时数据库，`/api/health` 返回健康及真实的 `local-echo` 演示状态；SIGTERM 后正常退出，退出码 0。
- 最新前端 22 项测试通过，桌面 1360×880 与手机 390×844 明暗页面实际截图检查通过；无页面 JS 异常、手机横向溢出，主题刷新保留，抽屉 Escape 正常。
- 原生窗口截图验收未完成：系统辅助功能/录屏权限仍待授权。这里不将浏览器截图作为原生窗口验收证据。此前桌面进程的服务已验证可达，退出后端口释放；最终包仍需用户在原生窗口中确认显示。
- 未使用真实模型 API Key 或第三方账号，邮件/媒体生成/多 Bot/跨设备功能不包含在本安装包内。


## 0.2 更新

最新包为 `release/Soren-0.2.0-macOS-arm64.dmg`，约 10.92 MiB；SHA-256 `95fa1d21f45276b17f1ad96c899e2c856415bc743d762184c2fa58d9ad6b13a1`。签名与 DMG 完整性通过；已启动目标 App 并验证内置服务和 Bot API。具体功能、测试与限制见 [0.2 交付](MULTIBOT-DELIVERY.md)。
