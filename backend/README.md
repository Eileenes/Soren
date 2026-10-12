# Soren Backend

Go + Gin 后端服务。

## 开发

```bash
# 安装依赖
go mod download

# 运行服务
go run cmd/server/main.go

# 构建
go build -o soren-server cmd/server/main.go

# 运行测试
go test ./...
```

## 项目结构

```
cmd/
└── server/         # 服务器入口

internal/
├── api/            # API 处理器（聊天、记忆、渠道）
├── cache/          # 缓存抽象：进程内缓存（默认）+ Redis（可选）
├── channel/        # 消息渠道：飞书、钉钉接入与消息路由
├── memory/         # 长期记忆系统：存储、多路召回、重排序、提取
├── store/          # 存储抽象：SQLite（默认）/ PostgreSQL
├── llm/            # LLM 统一接口
├── models/         # 数据模型
├── repository/     # 数据仓储
├── middleware/     # 中间件
└── config/         # 配置管理

pkg/                # 公共包
```

## 存储与缓存

桌面端默认**零配置**运行：

- **存储**：嵌入式 SQLite（纯 Go 驱动 `modernc.org/sqlite`，无需 cgo、无需安装数据库），
  数据库文件位于 `~/.soren/soren.db`
- **缓存**：进程内缓存（TTL + LRU），无需安装 Redis

服务端部署时可切换到外部依赖：

```bash
STORAGE_DRIVER=postgres DATABASE_URL=postgres://... \
CACHE_DRIVER=redis REDIS_URL=localhost:6379 \
go run cmd/server/main.go
```

## 长期记忆系统

记忆按类型分类（`conversation` / `preference` / `task` / `fact`），
采用「时序优先 + 标签索引 + 多路召回 + 重排序」的方案，不依赖向量数据库：

1. **写入**：对话结束后异步提取偏好、事实、任务与对话摘要，自动计算重要性分数并抽取标签
2. **召回**：时序召回、标签召回、会话召回、高重要性召回四路并发执行
3. **重排序**：相关性 + 时间衰减 + 重要性加权 + 多样性调整，取 Top K 注入 system prompt

记忆可通过 `GET /api/memories` 等端点查看与管理（详见 `docs/API.md`）。
设计文档见 `docs/memory-system-design.md`。

## 消息渠道（飞书 / 钉钉）

把 Soren 接入飞书或钉钉，用户可以直接在聊天窗口里对话，
消息由 `internal/channel` 统一路由到 LLM，回复再发回原平台，
对话历史与桌面端界面共享同一份存储。

渠道是**完全可选**的：未配置时不会启动，也不影响服务本身启动。

### 接入模式

两个渠道都默认使用**长连接**，桌面端与内网环境无需公网 IP：

| 渠道 | 默认模式 | 备选模式 |
| --- | --- | --- |
| 飞书 | WebSocket 长连接 | Webhook 事件订阅（需公网地址 + Verification Token） |
| 钉钉 | Stream 长连接 | Outgoing Webhook（需公网地址 + 加签密钥） |

Webhook 模式会监听独立端口（默认飞书 `8081`、钉钉 `8082`），必须与 `SERVER_PORT` 错开。

### 飞书配置

1. 在[飞书开放平台](https://open.feishu.cn)创建**企业自建应用**，开通「机器人」能力
2. 订阅事件 `im.message.receive_v1`，权限需包含「读取与发送单聊、群组消息」
3. 在 `.env` 中填写：

```bash
FEISHU_ENABLED=true
FEISHU_APP_ID=cli_xxx
FEISHU_APP_SECRET=xxx
FEISHU_VERIFICATION_TOKEN=xxx   # 事件订阅页面的 Verification Token
FEISHU_ENCRYPT_KEY=             # 可选，配置后事件加密推送并校验回调签名
FEISHU_MODE=ws                  # ws（默认，长连接）| webhook
```

### 钉钉配置

1. 在[钉钉开放平台](https://open-dev.dingtalk.com)创建**企业内部应用**，添加「机器人」能力并发布
2. 消息接收模式选择 **Stream 模式**（推荐）或 HTTP 模式
3. 在 `.env` 中填写：

```bash
DINGTALK_ENABLED=true
DINGTALK_CLIENT_ID=xxx          # 应用的 AppKey / ClientID
DINGTALK_CLIENT_SECRET=xxx      # 应用的 AppSecret / ClientSecret
DINGTALK_ROBOT_CODE=            # 留空则自动使用 CLIENT_ID
DINGTALK_MODE=ws                # ws（默认，Stream 长连接）| webhook
DINGTALK_WEBHOOK_SECRET=        # 可选，Webhook 模式下的加签密钥
```

### 渠道管理 API

```bash
# 列出所有渠道及状态
curl http://localhost:8080/api/channels

# 启用 / 禁用渠道（channel 取值为 feishu 或 dingtalk）
curl -X POST http://localhost:8080/api/channels/feishu/enable
curl -X POST http://localhost:8080/api/channels/feishu/disable
```

## 环境变量

复制 `.env.example` 到 `.env` 并配置（全部可选，缺省即为零配置的桌面端模式）：

```bash
cp .env.example .env
```
