# Soren 5 分钟上手

## 前置依赖

确认已安装以下工具（全部只需一次）：

- Go 1.21+：`go version`
- Node 18+：`node -v`
- Rust：`rustc --version`（macOS 首次构建时 Xcode CLI 会自动提示安装）

---

## 第一步：安装前端依赖

```bash
cd Soren/frontend
npm install
```

## 第二步：构建 Go 后端

```bash
npm run build:backend
```

这会把后端编译成 `src-tauri/soren-server`（macOS/Linux）或 `src-tauri/soren-server.exe`（Windows），Tauri 打包时会一并带上。

## 第三步：启动开发模式

```bash
npm run tauri:dev
```

几秒后窗口弹出，右上角会显示"未连接"——这是正常的，因为还没配 API Key。

---

## 确认后端起来了

打开新的终端，运行：

```bash
curl http://localhost:8080/api/health
```

应当返回：

```json
{"status":"ok","storageDriver":"sqlite","cacheDriver":"memory","memoryEnabled":true}
```

数据库文件自动创建在 `~/.soren/soren.db`，什么都不用装。

---

## 配置 API Key

在 `Soren/backend/` 目录新建 `.env` 文件：

```bash
cp backend/.env.example backend/.env
```

然后编辑 `backend/.env`，填入以下之一：

```env
# OpenAI 或兼容接口（如 Ollama、第三方网关）
OPENAI_API_KEY=sk-...

# 或者 Anthropic
ANTHROPIC_API_KEY=sk-ant-...
```

保存后重启后端：`Ctrl+C` 停掉 `tauri:dev`，再重新运行即可。

---

## 验证记忆系统在工作

在 Soren 里聊几轮之后，到「记忆」页查看 AI 记住了什么。也可以直接调 API：

```bash
curl "http://localhost:8080/api/memories/stats" -H "X-User-ID: default-user"
```

---

## 打包成桌面应用

```bash
npm run build:backend   # 先构建后端
npm run tauri:build     # 打包 Tauri 应用
```

产物在 `frontend/src-tauri/target/release/bundle/` 下。
