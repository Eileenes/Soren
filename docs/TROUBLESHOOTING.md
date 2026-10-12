# Soren 常见问题排查

---

## 后端无法启动

症状：前端显示"未连接"，或直接没有窗口弹出。

最快定位：

```bash
cd Soren/backend
go run ./cmd/server
```

看终端输出的错误信息，常见原因：

**端口 8080 被占用**：`lsof -i :8080`（macOS/Linux），找到占用的进程后 `kill <PID>` 或改后端端口：

```env
SERVER_PORT=9090
```

同时在前端 `.env`（或设置页）把 API 地址改为 `http://localhost:9090`。

**`.env` 文件位置不对**：Go 服务加载的是 `backend/.env`，不是项目根目录的 `.env`。

**SQLite 路径没有写权限**：默认数据目录是 `~/.soren/`，如果因为权限问题无法创建，在 `.env` 里指定一个有权限的目录：

```env
SOREN_DATA_DIR=/tmp/soren-data
```

---

## 前端显示"未连接"

1. 先确认后端在跑：`curl http://localhost:8080/api/health`，能返回 JSON 就是后端正常。
2. 如果后端正常但前端还是显示未连接，检查设置页里的 API 地址是否和后端端口一致。
3. 如果在 Tauri 包装的应用里看到这个，可能是 CSP 拦截了请求。打开开发者控制台（Tauri dev 模式下按 F12）看 Network 标签是否有 CSP 报错。

---

## tauri dev 白屏

打开开发者工具（F12），查看 Console 和 Network。常见原因：

**Vite 还没启动**：`tauri:dev` 会同时启动 Vite（端口 1420）和 Rust 进程，Rust 进程有时比 Vite 更快到位，导致 WebView 加载到空页面。等 3-5 秒后按 F5 刷新。

**CSP 错误**：控制台有 `Content Security Policy` 报错。检查 `src-tauri/tauri.conf.json` 里是否有 `devCsp` 字段。如果没有，补上（详见 `docs/CONFIGURATION.md` 中关于 CSP 的说明）。

**node_modules 未安装**：`cd frontend && npm install`。

---

## Rust 编译报错

**`error: feature shell-open not found in package tauri`**：这是 Tauri 1.x 遗留配置，2.0 不再有这个 feature。在 `src-tauri/Cargo.toml` 中把：

```toml
tauri = { version = "2.0", features = ["shell-open"] }
```

改为：

```toml
tauri = { version = "2.0", features = [] }
```

**找不到 tauri 或 tauri-build**：`cd frontend && npm install` 会触发 `@tauri-apps/cli` 的安装，这个包会管理 Rust crate 的下载。如果单独 `cargo build` 报错，用 `npm run tauri:dev` 代替。

**macOS 找不到 WebKit 头文件**：运行 `xcode-select --install` 安装 Command Line Tools。

---

## 找不到 soren-server 可执行文件

症状：应用启动但后端没有自动拉起，日志显示"未找到 soren-server"。

解决步骤：

1. 先手动构建后端：`cd frontend && npm run build:backend`
2. 确认产物在 `frontend/src-tauri/soren-server`（macOS/Linux）或 `soren-server.exe`（Windows）
3. 如果文件存在但还是找不到，用环境变量手动指定：

```bash
SOREN_BACKEND_BIN=/path/to/soren-server npm run tauri:dev
```

---

## 记忆没有生效

诊断步骤：

1. 确认记忆系统已启用：`curl http://localhost:8080/api/health` 返回的 `memoryEnabled` 应为 `true`。
2. 聊几轮后查看记忆列表：`curl "http://localhost:8080/api/memories?limit=5" -H "X-User-ID: default-user"`，看是否有数据。
3. 记忆提取是异步的，在 AI 回复之后约 1-3 秒才会完成提取和存储。
4. 如果一直没有记忆，在 `.env` 里确认 `MEMORY_ENABLED` 没有被设为 `false`。

---

## 飞书 / 钉钉连接失败

**通用排查**：先查后端日志，用 `journalctl` 或直接看终端输出。

**飞书**：
- `FEISHU_APP_ID` 和 `FEISHU_APP_SECRET` 必须填写，两者都空时渠道不会启动。
- WebSocket 长连接模式（`FEISHU_MODE=ws`）需要在飞书开放平台的"事件订阅"里开启"使用长连接接收事件"，而不是配置 webhook 地址。
- 如果使用 webhook 模式，服务需要外部可访问的地址，且飞书回调端口（默认 8081）不能与主服务端口相同。

**钉钉**：
- `DINGTALK_CLIENT_ID` 对应钉钉控制台的 AppKey，`DINGTALK_CLIENT_SECRET` 对应 AppSecret。
- Stream 模式（`DINGTALK_MODE=ws`）需要在钉钉开放平台的"消息接收模式"里选择"Stream 模式"而不是 HTTP 回调。
- 如果应用没有开通机器人能力，Stream 连接会建立成功但收不到消息。
