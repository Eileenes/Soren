# Soren 配置参考

所有配置通过环境变量读取。在 `backend/` 目录创建 `.env` 文件（参考 `backend/.env.example`），或直接在启动命令前设置环境变量。

---

## 存储层

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `STORAGE_DRIVER` | `sqlite` | 存储驱动：`sqlite`（嵌入式，开箱即用）或 `postgres` |
| `SOREN_DATA_DIR` | `~/.soren` | 数据目录，SQLite 文件和其他本地数据放在此处 |
| `SQLITE_PATH` | `~/.soren/soren.db` | SQLite 数据库文件路径，覆盖 `SOREN_DATA_DIR` 下的默认位置 |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/soren?sslmode=disable` | PostgreSQL 连接串，仅 `STORAGE_DRIVER=postgres` 时生效 |

切换到 PostgreSQL 的完整步骤：

1. 安装 PostgreSQL 并创建数据库：`createdb soren`
2. 在 `.env` 中设置：

```env
STORAGE_DRIVER=postgres
DATABASE_URL=postgres://用户名:密码@localhost:5432/soren?sslmode=disable
```

3. 重启后端，GORM 会自动执行数据库迁移。

---

## 缓存层

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `CACHE_DRIVER` | `memory` | 缓存驱动：`memory`（进程内，无需额外服务）或 `redis` |
| `REDIS_URL` | `localhost:6379` | Redis 地址，仅 `CACHE_DRIVER=redis` 时生效 |

---

## 长期记忆系统

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `MEMORY_ENABLED` | `true` | 是否启用长期记忆系统 |
| `MEMORY_RECALL_LIMIT` | `10` | 每轮对话注入到上下文的召回记忆条数上限 |

---

## LLM 服务

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `OPENAI_API_KEY` | 无 | OpenAI 或兼容接口的 API Key |
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | API 基础地址，可改为第三方网关或本地模型 |
| `ANTHROPIC_API_KEY` | 无 | Anthropic API Key |
| `ANTHROPIC_BASE_URL` | `https://api.anthropic.com` | Anthropic API 地址 |

优先级：`ANTHROPIC_API_KEY` > `OPENAI_API_KEY` > 本地回显模式。

### 接入第三方 OpenAI 兼容服务

```env
OPENAI_API_KEY=你的key
OPENAI_BASE_URL=https://你的网关地址/v1
```

### 接入本地 Ollama

```env
OPENAI_API_KEY=ollama
OPENAI_BASE_URL=http://localhost:11434/v1
```

---

## 服务器

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP 服务监听端口 |

---

## 飞书渠道（可选）

在飞书开放平台创建自建应用，开启机器人能力，订阅 `im.message.receive_v1` 事件。

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `FEISHU_ENABLED` | `false` | 是否启用飞书渠道 |
| `FEISHU_APP_ID` | 无 | 应用 AppID，如 `cli_xxxxx` |
| `FEISHU_APP_SECRET` | 无 | 应用 AppSecret |
| `FEISHU_VERIFICATION_TOKEN` | 无 | 事件订阅 Verification Token |
| `FEISHU_ENCRYPT_KEY` | 无 | 事件订阅 Encrypt Key（Webhook 模式必填） |
| `FEISHU_MODE` | `ws` | 接入模式：`ws`（长连接，不需要公网）或 `webhook` |
| `FEISHU_BASE_URL` | `https://open.feishu.cn` | 飞书开放平台地址 |
| `FEISHU_WEBHOOK_PORT` | `8081` | Webhook 模式下的监听端口（需与主服务端口不同） |

推荐使用 `FEISHU_MODE=ws`，这是长连接模式，无需公网 IP，适合桌面端。

---

## 钉钉渠道（可选）

在钉钉开放平台创建企业内部应用，开启机器人能力。接收模式选择 Stream。

| 变量名 | 默认值 | 说明 |
|---|---|---|
| `DINGTALK_ENABLED` | `false` | 是否启用钉钉渠道 |
| `DINGTALK_CLIENT_ID` | 无 | 应用 AppKey（新版控制台称为 ClientID） |
| `DINGTALK_CLIENT_SECRET` | 无 | 应用 AppSecret（新版控制台称为 ClientSecret） |
| `DINGTALK_ROBOT_CODE` | 无 | 机器人 RobotCode，通常与 AppKey 相同 |
| `DINGTALK_MODE` | `ws` | 接入模式：`ws`（Stream 长连接）或 `webhook` |
| `DINGTALK_WEBHOOK_SECRET` | 无 | 机器人加签密钥，Webhook 模式必填 |
| `DINGTALK_WEBHOOK_PORT` | `8082` | Webhook 模式下的监听端口 |

推荐使用 `DINGTALK_MODE=ws`（Stream 模式），无需公网 IP。

---

## 完整 .env 示例

```env
# LLM（至少填一个）
OPENAI_API_KEY=sk-...
# ANTHROPIC_API_KEY=sk-ant-...

# 服务器
SERVER_PORT=8080

# 存储（默认 SQLite，无需改动）
# STORAGE_DRIVER=sqlite
# SOREN_DATA_DIR=~/.soren

# 飞书（可选）
# FEISHU_ENABLED=true
# FEISHU_APP_ID=cli_xxx
# FEISHU_APP_SECRET=xxx
# FEISHU_VERIFICATION_TOKEN=xxx
# FEISHU_MODE=ws

# 钉钉（可选）
# DINGTALK_ENABLED=true
# DINGTALK_CLIENT_ID=xxx
# DINGTALK_CLIENT_SECRET=xxx
# DINGTALK_MODE=ws
```
