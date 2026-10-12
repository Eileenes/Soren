# Soren

新增免账号扫码配对、加密中继、手机任务/记忆入口及 iOS 工程。公共服务与真机分发尚未上线，详见 [交付说明](docs/MOBILE-040-DELIVERY.md) 和 [部署步骤](deploy/README.md)。

> 当前开发版本 **0.4.1**：[审查与修复记录](docs/REVIEW-041.md)。本地安装包为 `release/Soren-0.4.1-macOS-arm64.dmg`（不上传 Git）；手机公网联动尚待公共服务部署与真机分发。

0.3.2 统一应用内下拉菜单，支持获取服务商模型与跟随系统外观，移除本地设置中的 GitHub 登录入口。本地安装后无需账号或部署 Soren 服务器，配置模型即可使用。扫码手机 App 方案见 [移动端配对调研](docs/MOBILE-PAIRING-RESEARCH.md)。

第一次使用请看 [操作说明与手机关联说明](docs/USER-GUIDE.md)。

Soren 是一个带**长期记忆**的桌面端 AI 助手。它采用类 Muse 的时序记忆方案，能记住你的偏好、工作背景和历史任务；支持 OpenAI 兼容协议和 Anthropic 协议；可以接入飞书和钉钉作为消息平台机器人。

桌面应用由 Tauri 2.0（Rust）+ React 18 构建，后端服务用 Go 1.26.9 编写。默认使用嵌入式 SQLite，**不需要安装 PostgreSQL 或 Redis**，双击即用。

---

## 快速开始

### 前置依赖

- **Go 1.26.9+** — 编译后端：https://go.dev/dl/
- **Node 22.12+** — 前端开发：https://nodejs.org/
- **Rust + Cargo** — 编译 Tauri 桌面端：https://rustup.rs/
- **系统依赖（Linux）**：`sudo apt-get install libwebkit2gtk-4.1-dev libayatana-appindicator3-dev librsvg2-dev`
- **系统依赖（macOS）**：Xcode Command Line Tools（`xcode-select --install`）

### 开发模式（推荐先用这个跑起来）

```bash
# 1. 进入前端目录
cd frontend
npm install

# 2. 构建 Go 后端（产物放到 src-tauri/ 下供 Tauri 打包）
npm run build:backend

# 3. 启动开发模式（会同时启动后端和前端热更新）
npm run tauri:dev
```

应用窗口出现后，即可使用。没有配置 API Key 时会进入明确标记的本地回显模式，可以体验界面和存储；真实模型对话仍需要有效凭据。

### 配置模型

在 macOS 应用中打开「设置 → 模型与连接」，选择 OpenAI、Anthropic 或兼容接口，填写接口地址、API Key 和模型 ID，点击「保存模型连接」。应用会保存到当前用户私有目录并重启内置服务。Key 不会回传前端或写入浏览器存储。

没有配置 Key 时界面会明确显示“本地体验模式”，回复为回显，不具有模型推理能力。独立 Go 开发仍可用 `backend/.env`（参考 `.env.example`）。

### 打包为桌面应用

```bash
cd frontend
npm install
npm run build:backend   # 先构建后端二进制
npm run tauri:build     # 打包 Tauri 应用
```

产物在 `frontend/src-tauri/target/release/bundle/` 下。

---

## 本轮重构与产品边界

- 暖白晨光与墨绿暮色主题，设置持久化；桌面侧栏、手机抽屉和自适应聊天输入。
- 原创薄荷青绿渐变眨眼图标，以及完整 macOS 图标资源。
- 真实桌面模型配置、内置 Go 服务、关闭窗口后菜单栏后台运行，完全退出时回收进程。
- 聊天、历史、基础规则记忆与飞书/钉钉消息接入；取消/切换会话不会混入过期响应。
- 任务页支持模型调用本地成果工具，以及授权后的 Gmail 草稿/发送、Notion 搜索/读取/创建页面、远程 MCP 工具；外部写入需要确认。普通聊天与渠道尚未接入此工具循环。
- 本地使用无需登录。早期 GitHub 身份绑定保留在后端但不再作为用户配置入口；云端会话、Canva、日历、图片/视频生成和定时触发尚未实现。
- 手机端已实现 iOS 工程、扫码配对和端到端加密的任务/记忆入口，已完成模拟器与本地中继验证；公共中继部署、真机签名分发和 Android 尚待完成。默认本地 API 不向局域网开放。

详细方案：[架构审查与演进](docs/ARCHITECTURE-REVIEW.md)，安装构建：[macOS 构建说明](docs/MACOS-BUILD.md)，界面验证：[重构验收记录](docs/REDESIGN-DELIVERY.md)。

## 功能一览

**长期记忆**：AI 会记住你的偏好（如回答风格）、事实（职业背景）、任务和对话摘要。时序优先、标签索引、多路召回——参见 `docs/memory-system-design.md`。

**多模型支持**：统一接口同时支持 OpenAI 兼容协议和 Anthropic 协议，可通过环境变量切换服务商或接入自建网关、Ollama 等。

**消息平台集成**：飞书（WebSocket 长连接）和钉钉（Stream 模式），默认关闭，配置凭据后启用。桌面端无需公网 IP。

**记忆管理界面**：可查看 AI 记住了什么、按类型筛选、搜索、删除——记忆对你可见且可控。

**零配置存储**：默认嵌入式 SQLite（`~/.soren/soren.db`），无需任何数据库服务。需要时可切换到 PostgreSQL。

**优雅降级**：没有 API Key 时进入本地回显模式，记忆系统照常工作，不崩溃不白屏。

---

## 项目结构

```
Soren/
├── backend/          # Go 后端服务
│   ├── cmd/server/   # 入口 main.go
│   ├── internal/
│   │   ├── api/      # HTTP 处理器（聊天、记忆、渠道）
│   │   ├── cache/    # 缓存抽象（内存 / Redis）
│   │   ├── channel/  # 消息渠道（飞书、钉钉）
│   │   ├── config/   # 配置加载
│   │   ├── llm/      # LLM 接口（OpenAI、Anthropic、Mock）
│   │   ├── memory/   # 长期记忆系统（核心）
│   │   ├── models/   # 数据库模型
│   │   ├── repository/ # 数据访问层
│   │   └── store/    # 存储抽象（SQLite / PostgreSQL）
│   └── .env.example
├── frontend/         # Tauri 2.0 + React 桌面端
│   ├── src/          # React 源码
│   ├── src-tauri/    # Rust / Tauri 配置
│   └── package.json
├── docs/             # 详细文档
├── assets/           # Logo、图标
└── scripts/          # 构建脚本
```

---

## 常见问题

**后端没起来，前端显示"未连接"**：先手动跑 `cd backend && go run ./cmd/server` 看错误输出；最常见原因是端口 8080 被占（`lsof -i :8080`）或 `.env` 路径不对。

**tauri:dev 白屏**：通常是 CSP 问题或 Vite dev server 未启动。检查终端输出，确认 `http://localhost:1420` 可访问。

**Cargo.toml 报 shell-open feature 不存在**：这是旧模板遗留问题，Tauri 2.0 已移除该 feature，本项目使用 `tray-icon` feature 支持菜单栏；参考当前 Cargo.toml。

详细排查见 `docs/TROUBLESHOOTING.md`，完整配置参考见 `docs/CONFIGURATION.md`。

- [多 Bot 与移动端联动规划](docs/MULTI-BOT-AND-MOBILE-PLAN.md)
