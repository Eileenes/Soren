# API 文档

## 概述

Soren Backend API 提供了与 AI 助手交互的 RESTful 接口。

**基础 URL**: `http://localhost:8080/api`

## 认证

目前 API 不需要认证。后续版本将添加 JWT 认证。

## 端点

### 健康检查

检查服务器状态。

**请求**

```http
GET /api/health
```

**响应**

```json
{
  "status": "ok"
}
```

---

### 发送消息

向 AI 助手发送消息并获取响应。

**请求**

```http
POST /api/chat
Content-Type: application/json

{
  "conversationId": "550e8400-e29b-41d4-a716-446655440000",
  "message": "你好，Soren！",
  "config": {
    "model": "gpt-3.5-turbo",
    "temperature": 0.7,
    "maxTokens": 2000
  }
}
```

**参数说明**

| 参数 | 类型 | 必需 | 说明 |
|------|------|------|------|
| conversationId | string | 否 | 对话 ID，留空则创建新对话 |
| message | string | 是 | 用户消息内容 |
| config | object | 否 | LLM 配置 |
| config.model | string | 否 | 模型名称 |
| config.temperature | number | 否 | 温度参数（0-1） |
| config.maxTokens | number | 否 | 最大 token 数 |

**响应**

```json
{
  "conversationId": "550e8400-e29b-41d4-a716-446655440000",
  "message": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "conversationId": "550e8400-e29b-41d4-a716-446655440000",
    "role": "assistant",
    "content": "你好！我是 Soren，很高兴为你服务。",
    "timestamp": "2024-10-10T10:30:00Z",
    "createdAt": "2024-10-10T10:30:00Z",
    "updatedAt": "2024-10-10T10:30:00Z"
  }
}
```

---

### 获取对话列表

获取所有对话的列表。

**请求**

```http
GET /api/conversations
```

**响应**

```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "title": "关于 React 的讨论",
    "messages": [],
    "createdAt": "2024-10-10T10:00:00Z",
    "updatedAt": "2024-10-10T10:30:00Z"
  },
  {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "title": "Go 语言学习",
    "messages": [],
    "createdAt": "2024-10-09T15:00:00Z",
    "updatedAt": "2024-10-09T16:00:00Z"
  }
]
```

---

### 获取单个对话

获取指定对话的详细信息，包括所有消息。

**请求**

```http
GET /api/conversations/:id
```

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 对话 ID |

**响应**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "title": "关于 React 的讨论",
  "messages": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440002",
      "conversationId": "550e8400-e29b-41d4-a716-446655440000",
      "role": "user",
      "content": "React 18 有什么新特性？",
      "timestamp": "2024-10-10T10:00:00Z",
      "createdAt": "2024-10-10T10:00:00Z",
      "updatedAt": "2024-10-10T10:00:00Z"
    },
    {
      "id": "880e8400-e29b-41d4-a716-446655440003",
      "conversationId": "550e8400-e29b-41d4-a716-446655440000",
      "role": "assistant",
      "content": "React 18 引入了并发渲染、自动批处理、新的 Hooks 等特性...",
      "timestamp": "2024-10-10T10:00:15Z",
      "createdAt": "2024-10-10T10:00:15Z",
      "updatedAt": "2024-10-10T10:00:15Z"
    }
  ],
  "createdAt": "2024-10-10T10:00:00Z",
  "updatedAt": "2024-10-10T10:30:00Z"
}
```

---

### 删除对话

删除指定的对话及其所有消息。

**请求**

```http
DELETE /api/conversations/:id
```

**参数**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | string | 对话 ID |

**响应**

```json
{
  "message": "conversation deleted"
}
```

---

## 记忆管理

Soren 会在对话过程中自动积累长期记忆（对话摘要、用户偏好、事实、任务），
以下端点让用户可以查看和管理 AI 记住的内容。

> 说明：用户 ID 优先取 `X-User-ID` 请求头，未提供时使用 `default-user`（桌面端单用户场景）。

### 列出记忆

支持分页与按类型筛选。

**请求**

```http
GET /api/memories?page=1&pageSize=20&type=preference
```

**参数说明**

| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 页码，默认 1 |
| pageSize | int | 每页条数，默认 20，最大 100 |
| type | string | 记忆类型筛选：conversation / preference / task / fact |
| sessionId | string | 按会话筛选 |
| keyword | string | 按关键词过滤内容或摘要 |
| minImportance | int | 最低重要性分数（0-100） |

**响应**

```json
{
  "items": [
    {
      "id": 12,
      "userId": "default-user",
      "sessionId": "550e8400-e29b-41d4-a716-446655440000",
      "contentType": "preference",
      "content": "咖啡: 喜欢拿铁，不喜欢美式",
      "summary": "咖啡",
      "importanceScore": 95,
      "accessCount": 3,
      "lastAccessedAt": "2024-10-10T10:31:00Z",
      "createdAt": "2024-10-10T10:30:00Z",
      "conversationTurn": 1,
      "tags": [{ "id": 4, "name": "咖啡", "tagType": "topic" }]
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 20,
  "enabled": true
}
```

---

### 搜索记忆

**请求**

```http
GET /api/memories/search?q=咖啡&limit=20
```

| 参数 | 类型 | 说明 |
|------|------|------|
| q | string | 搜索关键词（同时兼容 `keyword`） |
| limit | int | 返回上限，默认 20，最大 100 |

**响应**

```json
{
  "items": [],
  "query": "咖啡",
  "total": 0
}
```

---

### 删除记忆

软删除指定记忆，用户可据此纠正 AI 记错的内容。

**请求**

```http
DELETE /api/memories/12
```

**响应**

```json
{ "message": "memory deleted", "id": 12 }
```

---

### 记忆统计

**请求**

```http
GET /api/memories/stats
```

**响应**

```json
{
  "enabled": true,
  "total": 42,
  "byType": { "conversation": 30, "preference": 6, "fact": 4, "task": 2 },
  "highImportance": 8,
  "accessedRecently": 15,
  "totalTags": 37,
  "oldestCreatedAt": "2024-09-01T08:00:00Z",
  "newestCreatedAt": "2024-10-10T10:30:00Z"
}
```

---

### 对话中的记忆注入

`POST /api/chat` 会自动召回相关记忆并注入 system prompt，
响应中额外返回 `memoriesUsed` 字段，便于前端展示"这次用到了哪些记忆"：

```json
{
  "conversationId": "550e8400-e29b-41d4-a716-446655440000",
  "message": { "role": "assistant", "content": "..." },
  "memoriesUsed": ["咖啡", "用户是后端工程师"]
}
```

## 错误处理

API 使用标准的 HTTP 状态码：

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

**错误响应格式**

```json
{
  "error": "错误描述信息"
}
```

## LLM 模型支持

### OpenAI 模型

- `gpt-4`
- `gpt-4-turbo`
- `gpt-3.5-turbo`
- `gpt-3.5-turbo-16k`

### Anthropic 模型

- `claude-3-opus-20240229`
- `claude-3-sonnet-20240229`
- `claude-3-haiku-20240307`
- `claude-2.1`
- `claude-2.0`

## 速率限制

目前没有实施速率限制。生产环境建议添加速率限制中间件。

## 示例

### cURL 示例

```bash
# 健康检查
curl http://localhost:8080/api/health

# 发送消息
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{
    "message": "你好",
    "config": {
      "model": "gpt-3.5-turbo",
      "temperature": 0.7
    }
  }'

# 获取对话列表
curl http://localhost:8080/api/conversations

# 获取单个对话
curl http://localhost:8080/api/conversations/550e8400-e29b-41d4-a716-446655440000

# 删除对话
curl -X DELETE http://localhost:8080/api/conversations/550e8400-e29b-41d4-a716-446655440000
```

### JavaScript 示例

```javascript
// 发送消息
const response = await fetch('http://localhost:8080/api/chat', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    message: '你好',
    config: {
      model: 'gpt-3.5-turbo',
      temperature: 0.7,
    },
  }),
});

const data = await response.json();
console.log(data);
```
