# 开发文档

## 系统架构

Soren AI Assistant 采用前后端分离架构：

### 前端架构

```
Tauri Application
├── React UI Layer
│   ├── Components (可复用组件)
│   ├── Pages (页面组件)
│   ├── Services (API 通信)
│   └── Hooks (状态管理)
└── Tauri Core (Rust)
    ├── Window Management
    ├── System Integration
    └── IPC Bridge
```

### 后端架构

```
Go Backend Server
├── HTTP Layer (Gin)
│   ├── API Handlers
│   ├── Middleware
│   └── Router
├── Business Logic
│   ├── LLM Providers
│   ├── Conversation Manager
│   └── Configuration
└── Data Layer
    ├── PostgreSQL (持久化)
    ├── Redis (缓存)
    └── GORM (ORM)
```

## LLM 统一接口设计

### 接口定义

所有 LLM 提供商实现统一的 `Provider` 接口：

```go
type Provider interface {
    Chat(ctx context.Context, messages []Message, options *Options) (*Response, error)
    StreamChat(ctx context.Context, messages []Message, options *Options, stream chan<- StreamChunk) error
}
```

### 支持的提供商

1. **OpenAI Provider**
   - 支持 GPT-3.5, GPT-4 等模型
   - 兼容 OpenAI API 标准

2. **Anthropic Provider**
   - 支持 Claude 系列模型
   - 自动处理 system prompt

### 添加新提供商

创建新文件 `internal/llm/newprovider.go`：

```go
package llm

type NewProvider struct {
    APIKey  string
    BaseURL string
    client  *http.Client
}

func NewNewProvider(apiKey, baseURL string) *NewProvider {
    return &NewProvider{
        APIKey:  apiKey,
        BaseURL: baseURL,
        client:  &http.Client{},
    }
}

func (p *NewProvider) Chat(ctx context.Context, messages []Message, options *Options) (*Response, error) {
    // 实现聊天逻辑
}
```

## 数据库设计

### 表关系

```
conversations (对话表)
    ├── id (UUID, PK)
    ├── title (VARCHAR)
    ├── created_at (TIMESTAMP)
    └── updated_at (TIMESTAMP)

messages (消息表)
    ├── id (UUID, PK)
    ├── conversation_id (UUID, FK -> conversations.id)
    ├── role (VARCHAR: user/assistant/system)
    ├── content (TEXT)
    ├── timestamp (TIMESTAMP)
    ├── created_at (TIMESTAMP)
    └── updated_at (TIMESTAMP)

llm_configs (LLM配置表)
    ├── id (INTEGER, PK)
    ├── provider (VARCHAR: openai/anthropic)
    ├── model (VARCHAR)
    ├── api_key (VARCHAR)
    ├── base_url (VARCHAR)
    ├── temperature (DECIMAL)
    ├── max_tokens (INTEGER)
    ├── created_at (TIMESTAMP)
    └── updated_at (TIMESTAMP)
```

### 索引策略

```sql
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at DESC);
```

## API 请求流程

```
1. 客户端发送请求
   ├── POST /api/chat
   └── Body: { message, conversationId?, config? }

2. API Handler 处理
   ├── 验证请求参数
   ├── 获取或创建对话
   └── 保存用户消息

3. LLM Provider 调用
   ├── 构建消息历史
   ├── 调用 LLM API
   └── 处理响应

4. 保存并返回
   ├── 保存助手消息
   ├── 更新对话时间
   └── 返回响应给客户端
```

## 前端状态管理

使用 React Hooks 进行状态管理：

### useChat Hook

```typescript
const {
  conversations,       // 对话列表
  currentConversation, // 当前对话
  loading,             // 加载状态
  error,               // 错误信息
  sendMessage,         // 发送消息
  loadConversations,   // 加载对话列表
  deleteConversation   // 删除对话
} = useChat();
```

## 错误处理

### 后端错误处理

```go
// 统一错误响应格式
type ErrorResponse struct {
    Error string `json:"error"`
}

// 在处理器中使用
if err != nil {
    c.JSON(http.StatusInternalServerError, ErrorResponse{
        Error: "failed to process request",
    })
    return
}
```

### 前端错误处理

```typescript
try {
  await sendMessage(content);
} catch (error) {
  console.error('Failed to send message:', error);
  // 显示错误提示
}
```

## 性能优化

### 后端优化

1. **数据库连接池**
   - 使用 GORM 自动管理连接池
   - 合理设置最大连接数

2. **Redis 缓存**
   - 缓存热门对话
   - 缓存 LLM 配置

3. **并发控制**
   - 使用 context 控制超时
   - 限制并发请求数

### 前端优化

1. **虚拟滚动**
   - 对长对话使用虚拟滚动
   - 按需加载消息

2. **防抖和节流**
   - 搜索框使用防抖
   - 滚动事件使用节流

3. **代码分割**
   - 使用 React.lazy 懒加载页面
   - 按路由分割代码

## 安全性

### API 安全

1. **CORS 配置**
   - 限制允许的源
   - 配置允许的方法和头

2. **输入验证**
   - 验证所有用户输入
   - 使用 Gin 的验证器

3. **SQL 注入防护**
   - 使用 GORM 参数化查询
   - 永远不要拼接 SQL

### 密钥管理

1. **环境变量**
   - API 密钥存储在环境变量
   - 不要提交 .env 文件

2. **加密存储**
   - 数据库中的密钥应加密
   - 使用 bcrypt 或类似算法

## 测试

### 后端测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/api

# 生成覆盖率报告
go test -cover ./...
```

### 前端测试

```bash
# 运行测试
npm test

# 生成覆盖率报告
npm test -- --coverage
```

## 部署

### 后端部署

```bash
# 构建二进制文件
go build -o soren-server cmd/server/main.go

# 使用 Docker
docker build -t soren-backend .
docker run -p 8080:8080 soren-backend
```

### 前端部署

```bash
# 构建 Tauri 应用
npm run tauri:build

# 生成的安装包位于
# frontend/src-tauri/target/release/bundle/
```

## 监控和日志

### 日志级别

- ERROR: 错误信息
- WARN: 警告信息
- INFO: 一般信息
- DEBUG: 调试信息

### 日志格式

```
[时间] [级别] [消息]
2024-10-10 10:30:00 INFO Server started on port 8080
```

## 故障排查

### 常见问题

1. **数据库连接失败**
   - 检查 DATABASE_URL 配置
   - 确认 PostgreSQL 服务运行中

2. **LLM API 调用失败**
   - 验证 API 密钥
   - 检查网络连接

3. **前端无法连接后端**
   - 确认后端服务运行中
   - 检查 CORS 配置

## 开发工具推荐

### 后端开发

- IDE: VS Code with Go extension
- 调试: Delve debugger
- API 测试: Postman / curl

### 前端开发

- IDE: VS Code with React/TypeScript extensions
- 调试: Chrome DevTools
- 状态检查: React Developer Tools

## 代码规范

### Go 代码规范

- 使用 `gofmt` 格式化代码
- 遵循 Effective Go 指南
- 添加必要的注释

### TypeScript 代码规范

- 使用 ESLint 和 Prettier
- 遵循 Airbnb Style Guide
- 为组件添加 Props 类型定义

## 贡献流程

1. Fork 项目
2. 创建特性分支
3. 提交代码
4. 创建 Pull Request
5. 代码审查
6. 合并到主分支
