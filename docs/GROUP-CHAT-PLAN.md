# Soren 群聊功能实施方案

状态：v3（已过两轮 verifier 评审） · 目标版本 0.5.0 · 基线 commit `4061c7e`

> **v2 修订说明**：v1 经独立评审判定 FAIL，存在 3 处事实错误与 6 个阻塞缺口。本版逐条修正，改动处均标 `[v2]`。
>
> **v3 修订说明**：v2 经第二轮复核判定 PARTIAL，主体可施工但残留 1 处阻塞性内部矛盾。本版补齐，改动处均标 `[v3]`：
> 1. **【原阻塞】SQLite 单连接下的事务嵌套死锁**——外层事务调用自带事务的 `convRepo.AddMessage` 会因拿不到第二条连接而挂死，已在 §3 约束 3 写明禁止并给出单事务写法
> 2. 同群 `running` 互斥的 TOCTOU 缺口 → §5.5 事务内检查 + 插入
> 3. `422` 为永不可触发的死规则 → 删除，乘积 ≤ 30 降级为推导出的成本不变量
> 4. DELETE 未清理 `GroupMember` / `GroupTurn`、未先 cancel → §6 补清理顺序
> 5. 群内各 Bot 的模型选择未定义 → §7.6 沿用单聊解析顺序
> 6. 引用修正：`chat.go:81` → `chat.go:83`；Bot ID 生成处 `api/bot.go:149`（实为 SharedNote）→ `agent/service.go:165`
> 7. 基线包数修正：15 → 14 个包 ok + 2 个 no test files

---

## 1. 目标

在左侧菜单加号入口新增「创建群聊」，允许多个 Bot 进入同一个会话共同讨论。两种发言模式：

- **提及模式（mention）**：用户 `@` 指定 Bot，只有被点名的 Bot 回复。
- **自主模式（auto）**：Bot 之间按轮次自动接续讨论，用户设轮次上限，可随时中止。

## 2. 已确认的产品决策

以下四条已由产品负责人拍板，实现时不得自行更改。

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | **群聊不要求 Bot 双向协作授权** | Bot 都属于同一个本地用户，用户在群里点名是自己的权限。不复用 `Run` 的 `CollaborationEnabled` + `Collaborators` 互相勾选校验（`backend/internal/agent/service.go:261`）。 |
| D2 | 轮次上限**默认 3，硬上限 6**，群成员**上限 5** | 见 §4。 |
| D3 | **群聊不设独立记忆**。每个 Bot 把群聊中自己的发言记入**自己的私有记忆** | `agent.Owner(botID)` 已按 Bot 隔离记忆（`backend/internal/agent/service.go:114`），无需新表。 |
| D4 | **两种模式一次做全**，不分期 | 提及模式与自主模式共用同一套编排器与数据层。 |

### D3 的实现含义

群聊中 Bot B 发言后，调用：

```go
memory.ProcessTurn(agent.Owner(botID), conversationID, userMessage, botBReply, turn)
```

召回时同样按 Bot 各自召回：

```go
memory.RecallRelevantMemories(ctx, agent.Owner(botID), conversationID, currentMessage, 10)
```

**不得使用 `api.resolveUserID(c)`**——它读的是 `X-Bot-ID` 请求头对应的单个 owner（`backend/internal/api/bot.go:31` 写入 `memoryOwner`），群聊里必须逐 Bot 显式计算。

副作用（已知且接受）：Bot A 的私有记忆里会包含 Bot B 在群里说的话，因此 A 之后单独与你对话时可能引用群里的讨论。这是预期行为。

## 3. 现状基线

架构：Tauri 壳 + Go / Gin 后端 + React 19 / TypeScript 前端，本地 SQLite。

### 已有资产（复用，不重写）

| 资产 | 位置 | 复用方式 |
| --- | --- | --- |
| 多 Bot 顺序执行编排器 | `backend/internal/agent/service.go:304` `execute()` | 自主模式骨架：并发槽 `slots`(cap 3, `service.go:91`)、`cancellations` 取消、共享资料交集校验（`service.go:202-207`）、**每步重查 Bot 状态**（`service.go:295-300`）全部照搬 |
| 对话 / 消息 CRUD | `backend/internal/repository/conversation.go` | 群聊复用，见 §5.3 |
| 私有记忆读写 | `backend/internal/memory/manager.go` | 见 D3。两者均为纯 Go，**不产生额外 LLM 调用** |
| 消息气泡 / 输入框 | `frontend/src/components/chat/` | 群聊页复用，见 §8 |
| Bot 头像组件 | `frontend/src/components/BotAvatar.tsx` | 群成员叠放 |

### 已知约束（实现时必须遵守）

1. **`Message` 表没有发言人字段**。Bot 身份目前靠 `X-Bot-ID` 请求头传递（`backend/internal/api/bot.go:21`），一次群聊发言会产生 N 条 `assistant` 消息但请求头只有一个值。**必须加字段**。→ §5.1

2. **`App.tsx:315` 用 `key={activeID}`（Bot ID）作为 `AppContent` 的 key**。群聊不是 Bot，塞进去会导致切换时状态被清空。**必须先做前置重构**。→ §8.1

3. **SQLite 单连接**：`sqlDB.SetMaxOpenConns(1)`（`backend/internal/store/database.go:77`）。所有写操作串行，自主模式多轮落库会变慢。不要为此引入连接池。

   `[v3]` **⚠️ 由此产生一条硬性禁令：群聊落库不得嵌套既有仓储方法。** `conversation.go:64-71` 的 `AddMessage` 自身开了 `Transaction`。若在外层 `db.Transaction(...)` 里调用它，内层 `Begin` 需要**第二条连接**，而池里只有 1 条且被外层持有 → 请求阻塞至超时。

   群聊落库必须在**单个事务内**直接执行：

   ```go
   err := db.Transaction(func(tx *gorm.DB) error {
       if err := tx.Create(message).Error; err != nil { return err }
       return tx.Model(&GroupChat{}).Where("id = ?", groupID).
           Updates(map[string]any{
               "last_message_at":      now,
               "last_message_preview": preview,
           }).Error
   })
   ```

   **不得**写成 `db.Transaction(func(tx) { convRepo.AddMessage(...) })`。单聊路径继续用既有 `AddMessage` 不受影响（它不在外层事务里）。

4. `[v2]` **本次 schema 变更没有任何自动备份**。`backend/cmd/server/main.go:41` 的备份守卫条件是
   `HasTable(Conversation) && !HasColumn(Conversation, "bot_id")`——这是**一次性历史迁移守卫**（补 Bot 归属）。任何已有安装的 `bot_id` 早已存在，条件恒为 false。
   → **Agent A 必须在 `AutoMigrate` 之前自行实现备份**：检测到 SQLite 且本次为新 schema 版本时，先 `VACUUM INTO` 到 `cfg.SQLitePath + ".before-groups-<timestamp>"` 并 `chmod 0600`，失败则 `log.Fatalf` 拒绝启动。不要依赖现有守卫。

5. **非流式**：`/api/chat` 一次性返回，前端 `useChat` 的 `typing` 恒为 `false`（`frontend/src/hooks/useChat.ts:86`）。群聊沿用非流式，但必须有轮次进度反馈（见 §8.4）。

6. **`useBotInbox` 每 5 秒轮询每个 Bot 的对话列表**（`frontend/src/hooks/useBotInbox.ts:43`）。群聊列表单独取，不并入这个轮询，避免放大请求量。`GetAllForBot` 不 Preload Messages（`conversation.go:137-141`），现网靠二次请求拿 preview——群聊**不得沿用这个 N+1 模式**，见 §5.4。

7. `[v2]` **移动端不受影响，且不支持群聊**。`mobile/client.ts` 走 Tauri `invoke` 而非 HTTP，`MobileApp.tsx` 只 import `MarkdownView` 与 `ui`。**Agent C 不得修改 `mobile/` 下任何文件**，并在 UI 上明确群聊为桌面端能力。

## 4. 术语：什么是「轮次」

**一轮 = 群内每个 Bot 各发言一次。**

以 3 个 Bot（产品 / 研发 / 设计）为例，用户发一句话后：

```
用户：这个功能要不要先做移动端？

── 第 1 轮 ──
产品：我建议先做移动端，理由是……
研发：（看到产品的话）移动端成本高，建议先补 API。
设计：（看到前两段）我同意产品，但要留出深色模式的迭代位。

── 第 2 轮 ──
产品：（看到大家分歧）那我调整一下，改为……
研发：（看到产品调整）这样可以，API 我提前介入。
设计：……

── 第 3 轮 ──
（到上限停止）
```

关键点：

- 第 1 轮第 N 个 Bot 发言时，能看到本轮前面所有 Bot 说的话。
- 第 2 轮每个 Bot 都能看到第 1 轮的全部内容。
- `MaxRounds = 3`、3 个 Bot 的一次发言 = **9 次 LLM 调用**。
- 成本护栏公式：`成员数 × MaxRounds`，上限 5 × 6 = 30 次调用。服务端发起前必须校验。

### 允许 Bot 弃权

强制每个 Bot 每轮都发言，第三轮必然重复。引入 `[SKIP]` 约定：Bot 判断无内容可补充时，只输出 `[SKIP]` 五个字符，编排器跳过该条且不落库。

- 不增加任何 LLM 调用成本（复用同一次响应）。
- 解析规则：`strings.TrimSpace(reply) == "[SKIP]"` 才跳过。前缀包含不算，避免误吞正文。
- prompt 必须写明「若无需补充，只输出 `[SKIP]`，不要附加任何其他文字」。
- 某轮所有 Bot 都 `SKIP` → 提前结束整个讨论。

## 5. 数据模型

### 5.1 Message 新增三个字段

`backend/internal/models/models.go`

```go
SpeakerBotID string `gorm:"type:varchar(36);index" json:"speakerBotId,omitempty"`
SpeakerName  string `gorm:"type:varchar(100)" json:"speakerName,omitempty"`
```

- `SpeakerBotID`：群聊中该条消息的发言 Bot。单聊留空，不影响既有数据。
- `SpeakerName`：**必须冗余存储**。不能只存 ID 再运行时 join `Bot` 表——用户改名或删除 Bot 后，历史消息身份会错乱。

`ReplyToBotID` 已从 v1 删除——前端可以用「用户消息中的 `@` 文本」在渲染时定位，无需单独存字段。少一个字段少一处不一致的来源。

### 5.2 群聊模型

新建 `backend/internal/models/group.go`

```go
type GroupChat struct {
    ID              uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
    Title           string    `gorm:"type:varchar(255);not null" json:"title"`
    Topic           string    `gorm:"type:text" json:"topic"`
    Mode            string    `gorm:"type:varchar(20);not null;default:mention" json:"mode"`
    MaxRounds       int       `gorm:"not null;default:3" json:"maxRounds"`
    // [v2] 冗余预览字段，避免 §3 约束 6 的 N+1 查询
    LastMessageAt      *time.Time `json:"lastMessageAt,omitempty"`
    LastMessagePreview string     `gorm:"type:varchar(200)" json:"lastMessagePreview,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}

type GroupMember struct {
    GroupID  uuid.UUID `gorm:"type:varchar(36);primary_key" json:"groupId"`
    BotID    string    `gorm:"type:varchar(36);primary_key;index" json:"botId"`
    JoinedAt time.Time `json:"joinedAt"`
}
```

`[v2]` **`GroupTurn` 表 —— 幂等键的落点（v1 缺失导致幂等不可实现）**

```go
type GroupTurn struct {
    ID         uuid.UUID `gorm:"type:varchar(36);primary_key" json:"id"`
    RequestID  string    `gorm:"type:varchar(36);not null;uniqueIndex" json:"requestId"`
    GroupID    uuid.UUID `gorm:"type:varchar(36);index;not null" json:"groupId"`
    Mode       string    `gorm:"type:varchar(20);not null" json:"mode"`
    Content    string    `gorm:"type:text;not null" json:"content"`
    Status     string    `gorm:"type:varchar(20);index;not null" json:"status"` // running|done|cancelled|failed|interrupted
    MessageIDs string    `gorm:"type:text" json:"-"`  // JSON 数组，本次产出的消息 ID
    Error      string    `gorm:"type:text" json:"error,omitempty"`
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`
}
```

- 幂等：按 `RequestID` 查。同键同参数 → 返回已有 `MessageIDs` 对应的消息；同键不同参数 → **409**（不是 400）。
- 并发：同群同时只允许一个 `status = "running"` 的 GroupTurn。
- **服务重启收敛**：进程启动时把 `status = "running"` 的行更新为 `interrupted`，并在对应会话末尾追加一条 `role = "system"` 的中断说明。参照 `agent/service.go:106` 对孤儿 Run 的处理。

在 `backend/cmd/server/main.go:49` 的 `AutoMigrate` 中加入这三个模型。

### 5.3 Conversation 的群编码约定

**不新建平行对话表**，群对话复用 `Conversation`，用 `BotID` 字段承载群身份：

```
Conversation.BotID == "group:" + groupID
ExternalKey        == ""   // 必须留空
```

复用收益：`GetAllForBot` / `GetByIDForBot` / `AddMessage` 零改动复用，对话历史、时间戳排序自动正确。

`[v2]` **v1 关于 `Delete` 的断言是错的**（已核实）。`repository/conversation.go:52-61` 的 `Delete(ctx, id)` 按**单个 ID** 删除，消息清理是**手写事务**，`models.go:32` 的 `foreignKey:ConversationID` 没有 `constraint:OnDelete:CASCADE`，AutoMigrate 不生成级联约束。仓库层**不存在按 BotID 批量删除的方法**。

→ 删除群必须新写 `DeleteConversationsForBot(ctx, botID)`：先 `SELECT id WHERE bot_id = ?` 再逐个调既有 `Delete`，或一次事务内批量删。**不能假设级联**。

`[v2]` **群对话严禁经过 `GetOrCreateConversation`**。该方法按 `external_key` 查询并创建（`conversation.go:77-103`），群对话必须直接 `convRepo.Create` 且 `ExternalKey` 留空，否则会与飞书/钉钉渠道会话串号。

**必须验证的隔离性**：Bot ID 由服务端 `uuid.NewString()` 生成（`backend/internal/agent/service.go:165` `b.ID = uuid.NewString()`，注意 `api/bot.go:149` 那个 `n.ID = uuid.NewString()` 属于 `SharedNote`，不是 Bot），不可能以 `group:` 开头，前缀命名空间安全。Agent A 仍需在 Bot 创建处加一条注释说明该前缀为保留命名空间。

### 5.4 群列表预览（[v2] 新增）

`GET /api/groups` 返回的每项包含 `lastMessageAt` 与 `lastMessagePreview`。

- 预览格式：`发言人名：内容`，截断 200 字。
- 写入时机：`GroupChatRepository` 在 `AddMessage` 的同一事务内更新 `GroupChat` 的冗余字段。
- **禁止**在列表接口里对每个群再查一次对话详情——`MaxOpenConns(1)` 下这就是锁竞争。

## 6. API 设计

`[v2]` **路由必须注册在根 `r` 上，不能挂 `apiGroup`。**

v1 的前提是错的：`main.go:156-157` 是

```go
apiGroup := r.Group("/api")
apiGroup.Use(api.BotContext(agents))
```

Gin 的 group 级中间件对该 group 上注册的**所有**路由生效，而 `BotContext`（`bot.go:19-34`）在 `GetBot` 失败时 `c.AbortWithStatusJSON(404)`。挂到 `apiGroup` 的群路由**必然**经过 `X-Bot-ID` 解析。

仓库里已有正确先例：`main.go:160` 的 `api.RegisterOAuthCallback(r, tools)` 注册在根 `r` 上，正是为了绕开 `BotContext`。群路由照此办理：

```go
groupHandler := api.NewGroupHandler(...)   // 在 main.go 中构造
api.RegisterGroups(r, groupHandler)        // 注册到根 r
```

这样群路由的 context 里**根本不存在 bot**，§10 的「不调用 `botID(c)`」才成为可证伪的验收项（`botID(c)` 在 context 无 bot 时会兜底返回 `DefaultBotID`，挂着中间件的测试会假通过）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/groups` | 群聊列表，含成员摘要与 `lastMessagePreview` |
| POST | `/api/groups` | 创建 `{title, topic, botIds[], mode, maxRounds}` |
| GET | `/api/groups/:id` | 详情，含完整成员列表 |
| PATCH | `/api/groups/:id` | 更新标题/主题/成员/模式/轮次上限 |
| DELETE | `/api/groups/:id` | 删除群及 `BotID = "group:<id>"` 的全部对话与消息 |
| POST | `/api/groups/:id/messages` | 发言并触发回复 |
| POST | `/api/groups/:id/cancel` | 中止进行中的自主讨论 |

### 发言请求

```jsonc
// POST /api/groups/:id/messages
{
  "content": "@研发 这个接口的并发怎么处理",
  "mentions": ["<botId>"],        // 可选；auto 模式忽略
  "mode": "mention",              // 可选，不传用群默认
  "requestId": "<uuid>"           // 幂等键，必填
}
```

```jsonc
// 200
{
  "conversationId": "<uuid>",
  "status": "done",               // done | cancelled | failed
  "messages": [ /* 本次产生的消息，含 speakerBotId / speakerName */ ],
  "error": ""                     // 部分失败时说明哪些 Bot 失败
}
```

### 错误码（[v3] 修订：删除死规则）

| 状态码 | 场景 |
| --- | --- |
| 400 | 参数校验失败：成员数越界、`maxRounds` 越界、Bot 不存在或非 active、成员重复、提及了非群成员 |
| 404 | 群不存在，或 `requestId` 命中但群不匹配 |
| 409 | `requestId` 重复但参数不同；同群已有进行中的讨论 |
| 500 | 全部 Bot 调用均失败 |

`[v3]` v2 定义的 `422 | 成员数 × maxRounds > 30` **是一条永不可触发的死规则**：在 `2 ≤ n ≤ 5` 与 `1 ≤ maxRounds ≤ 6` 之下，乘积恒 ≤ 30，已被 400 的两条边界完全覆盖。故删除 `422`，不单独校验乘积。

`成员数 × maxRounds ≤ 30` 保留为**推导出的成本不变量**，用于 §4 的预算说明与代码注释，不作为独立的校验分支。任何能凑出 31 的入参，必然先违反成员数或轮次上限并被 400 拦下。

### 服务端校验（缺一不可）

- 群成员数量 `2 ≤ n ≤ 5`
- `maxRounds` 落在 `1..6`（auto 模式才有意义，mention 模式忽略但仍校验范围）
- 由上两条推出 `成员数 × maxRounds ≤ 30`，无需独立校验（见上）
- 所有 `botIds` 存在且 `status == "active"`，不重复
- 提及模式下 `mentions` 非空且全部是群成员；为空时按群内全员依次回复
- 同群同时只允许一个进行中的讨论（按 §5.5 的事务实现）
- 消息长度沿用现有 32000 字限制（`chat.go:68`）

### 删除群时的清理顺序（[v3] 补齐）

`DELETE /api/groups/:id` 必须按顺序处理，**否则编排器会继续往已删除的对话里写消息**：

1. 若存在 `status == "running"` 的 GroupTurn，先触发 cancel 并等待编排器退出
2. 删除该群的 `GroupMember` 与 `GroupTurn` 行
3. 调 `DeleteConversationsForBot(ctx, "group:"+id)` 清理对话与消息（见 §5.3，**无级联，必须显式写**）
4. 删除 `GroupChat` 行

Bot 的**私有记忆一律保留**——`repository/conversation.go` 的 `Delete` 只动对话与消息，不碰 memories 表。

## 7. 编排引擎

新建 `backend/internal/group/orchestrator.go`。

### 7.1 系统提示词

每个 Bot 在群聊中收到的 system prompt = 单聊 prompt 结构（`chat.go:165`）+ 群聊专用段：

```
你是 {bot.Name}。以下是用户定义的职责：
{bot.Instructions}

你正在一个群聊中参与讨论，其他参与者：{成员名列表}。
群聊主题：{group.Topic}

讨论规则：
- 其他参与者的观点你未必认同。有不同意见就直接指出并说明理由，不要附和。
- 不要重复其他 Bot 已经说过的内容。
- 如果无需补充内容，只输出 [SKIP]，不要附加任何其他文字。
- 保持简洁，控制在 {n} 字以内。
```

「不要附和」这条是**功能成败关键**。多 Bot 讨论最常见的失败模式是模型之间互相恭维、迅速趋同，导致讨论产出无效内容。必须在 prompt 层面强制立场差异。

共享资料（`agent.Service.SharedContext`）沿用单聊逻辑，仅注入群成员共同获准的部分。

### 7.2 提及模式

```
按 requestId 做幂等检查（§5.2）
创建 GroupTurn{Status: "running"}
保存用户消息（SpeakerBotID 空）
解析 mentions → 去重 → 按群成员顺序
for bot in targets:
    复查 bot 状态（§7.5）
    llmMessages = systemPrompt(bot) + 群历史截至此刻
    reply = LLM(llmMessages, optionsFor(bot))
    if reply == [SKIP]: continue
    落库(reply, speakerBotID=bot.ID, speakerName=bot.Name)
    更新 GroupChat 预览（§5.4）
GroupTurn.Status = "done"，写回 MessageIDs
返回全部新消息
```

### 7.3 自主模式

```
按 requestId 做幂等检查
创建 GroupTurn{Status: "running"}
保存用户消息
for round := 1; round <= MaxRounds; round++:
    spokeThisRound := 0
    for bot in members:
        if ctx.Err() != nil:
            GroupTurn.Status = "cancelled"
            追加一条 role="system" 消息说明讨论已中止
            return 已产生的部分消息      // 不报错
        // [v2] 每步重查——Bot 可能在本轮中途被归档/移出群
        live, err := GetBot(bot.ID)
        if err != nil || live.Status != "active" || !仍在群内:
            记录到 GroupTurn.Error, continue
        llmMessages = systemPrompt(bot) + 群历史截至此刻
        reply, err := LLM(llmMessages, optionsFor(bot))
        if err != nil:
            // [v2] 单 Bot 失败不中断整场
            记录到 GroupTurn.Error, continue
        if reply == [SKIP]: continue
        落库(reply, speakerBotID=bot.ID, speakerName=bot.Name)
        spokeThisRound++
    if spokeThisRound == 0: break   // 全员弃权或全员失败，提前结束
GroupTurn.Status = "done"，写回 MessageIDs
```

- 顺序执行，不并发（模型无共享状态，并发只会打乱发言顺序）。
- 每步之间重新读一次群历史，保证「后面的人看得到前面的话」。
- 复用 `agent.Service` 的并发槽与取消机制思路，但**不复用 `Run` 表**——群聊是交互式会话，不是任务记录。

### 7.4 `[v2]` 失败路径处理

| 情况 | 处理 | 用户可见结果 |
| --- | --- | --- |
| 单个 Bot LLM 调用报错 | 跳过该 Bot，记入 `GroupTurn.Error`，继续下一个 | 已产生的发言正常显示，响应 `status: "done"` + `error` 文本说明 |
| 全部 Bot 都报错 | `GroupTurn.Status = "failed"` | HTTP 500 + 已产生的消息 |
| Bot 在讨论中途被归档 / 移出群 | 每步执行前重新 `GetBot` 校验 `status == "active"` 且仍在 `GroupMember` 中 | 跳过该 Bot，记入 `GroupTurn.Error` |
| 用户中途点中止 | `context.Cancel()` | `GroupTurn.Status = "cancelled"`，**返回已产生的部分消息不报错**，追加一条 `role = "system"` 的「讨论已中止」说明 |
| 服务进程重启 | 启动时把 `running` 的 GroupTurn 收敛为 `interrupted`（参照 `agent/service.go:106`），追加 `role = "system"` 说明 | 重启后打开群聊，末尾显示中断说明，不留半截无标记的发言 |

**`role = "system"` 是 v1 缺失的落点**。`MessageBubble.tsx:120-128` 已有非 user/assistant 消息的分支，会渲染成居中灰色 pill——直接复用，不要为此伪造 assistant 消息。`speakerBotId` 留空。

### 7.5 记忆写入

**全部发言完成后再统一写入**，不要每条实时写：

- 自主模式下 30 次调用逐条触发记忆提取会拖慢响应，且 SQLite 单连接下争抢明显。
- **每个发言过的 Bot 各调一次**，且必须逐 Bot 校验开关（v1 漏掉了这层，**会写穿用户的隐私设置**）：

```go
if h.memory != nil && h.memory.Enabled() && bot.MemoryEnabled {
    go h.memory.ProcessTurn(agent.Owner(bot.ID), convID, userMessage, botReply, turn)
}
```

依据：`chat.go:83` 是 `memoryEnabled := h.memory != nil && h.memory.Enabled() && (bot == nil || bot.MemoryEnabled)`；`Bot.MemoryEnabled` 是真实字段（`service.go:31`）；而 `ProcessTurn` 内部只复查全局 `m.Enabled()`（`manager.go:320`），**不查 per-bot**。不补这层判断，关闭了记忆的 Bot 会被写入群聊内容。

- 未发言（`[SKIP]` 或调用失败）的 Bot 不写记忆。
- `turn` 取值：`chat.go:226` 是 `turn := len(conv.Messages) + 2`。群聊里沿用「本轮开始前的消息总数 + 2」，保证同一轮内各 Bot 的 turn 一致且递增。

### 7.6 群内各 Bot 的模型选择（[v3] 补齐）

`optionsFor(bot)` 必须沿用单聊的模型解析顺序（`chat.go:175-197`）：

```
默认模型 = agents.DefaultModel
若 bot.Model != "" → 用 bot.Model          // 每个 Bot 可配独立模型
否则若请求带了 config.model → 用 config.model
temperature / maxTokens 同理，非零才覆盖
```

**群聊里每个 Bot 可以用不同模型**，这是既有能力（Bot 表有 `Model` 字段，`service.go:31` 附近），不要为了省事统一成一个模型。这直接影响 §4 的耗时与成本预期——群成员可能跨不同服务商，串行调用的总时长也随之变化。

## 8. 前端设计

### 8.1 前置重构（必须先做）

`App.tsx:315` 的 `key={activeID}` 必须改为按「当前选中对象」挂载：

```ts
type Selection = { type: 'bot'; id: string } | { type: 'group'; id: string };
```

`AppContent` 的 `key` 改为 `sel.type + ':' + sel.id`。群聊页与单聊页各自持有独立状态，切走再切回不互相污染。

`[v2]` 同步处理两处 v1 遗漏：

- **`App.tsx:313` 的自动回退**：`useEffect(() => { if (!bots.some(b => b.id === activeID && b.status !== 'archived')) choose('soren') })`。选中对象可以是群之后，这行会把用户静默弹回 Soren。必须加 `sel.type === 'bot'` 判断，群选中时跳过。
- **localStorage 持久化**：`App.tsx:300/307` 的 `soren.active-bot` 只存 bot id。新增 `soren.active-group`，启动时优先恢复 group（群不存在则回落 `soren.active-bot`）。

### 8.2 加号下拉

`Sidebar.tsx:51` 的加号从直接调 `onCreateBot` 改为弹出下拉菜单，两项：

- 创建 Bot → 现有 `BotEditor` 弹窗，行为不变
- 创建群聊 → 新建 `CreateGroupDialog`

菜单用现有 `inbox-tool` 视觉体系，不引入新控件样式。键盘可达（↑↓ 选择、Enter 确认、Esc 关闭），点击外部关闭。

### 8.3 侧栏群聊分组

Bot 列表下方：`1px` 分割线 + 小标题「群聊 N」+ 群列表。

群列表项视觉：

- 左侧：最多 3 个 Bot 头像横向叠放（重叠 8px，后一个用 `bg-surface-*` 描边分隔），超出显示 `+N`
- 主文本：群名
- 副文本：`lastMessagePreview`（已含「发言人名：」前缀）
- 群列表项**不加彩色标签、不加特殊圆角**，与 Bot 项保持同一套行高与内边距

### 8.4 群聊页

复用 `ChatPage` 结构，差异点：

| 位置 | 改动 |
| --- | --- |
| 消息气泡 | **发言人头像与名称必须来自 `message.speakerBotId`**，由组件在传入的 bots 列表中解析；`speakerBotId` 缺失时回退到现有 `bot` prop（单聊行为不变）。名称渲染为头像右侧、气泡上方的 `12px / text-ink-3` 一行 |
| @ 高亮 | 用户消息中的 `@Bot名` 用 `text-primary-start` + 轻描边内联标记。**不用背景色块**——会切碎消息流 |
| 输入框 | `ChatInput` 输入 `@` 时在光标上方弹出候选列表（键盘 + 鼠标可选），回车确认 |
| 讨论状态 | `auto` 模式下，输入框上方显示 `TypingDots` + 「讨论中 2/3 轮」+ 中止按钮 |
| 空状态 | 群名、主题、成员头像列表 + 一句引导。`ChatEmptyState` 的变体 |

`[v2]` v1 只让 `MessageBubble` 加一个 `speakerName` 文本标签，但该组件现在只有 `bot?: Bot` 单个 prop（`MessageBubble.tsx:20`，由 `ChatPage.tsx:111` 传入当前活动 Bot）。群聊里所有发言都是 `role: assistant`，只加文字标签会导致 **N 条气泡共用同一个头像**，达不到 §10 的验收项。必须实现头像解析——`ChatMessage extends Message`（`useChat.ts:7`），`speakerBotId` 数据能流到组件，缺的只是渲染规则。

### 8.5 视觉红线

沿用现有 `bubble-user` / `bubble-assistant` / `text-primary-start` / `BotAvatar` / `TypingDots`。

**禁止**：紫蓝渐变、玻璃拟态、发光边框、霓虹赛博朋克、渐变文字标题、大圆角+厚阴影、装饰性插画。界面中文必须是功能性表述，不写解释性或抒情文案。

中止按钮**与输入框同高、同圆角**，不做专属造型——它是操作区的一个动作，不是主角。

## 9. 任务拆分与文件所有权

三个 agent 并行，目录集互不重叠。

### Agent A — 后端（coder）

| 文件 | 动作 |
| --- | --- |
| `backend/cmd/server/main.go` | 改：备份守卫 + AutoMigrate 加 3 模型 + 在根 `r` 挂 7 个路由 |
| `backend/internal/models/models.go` | 改：Message 加 2 字段 |
| `backend/internal/models/group.go` | 新增 |
| `backend/internal/repository/conversation.go` | 改：新增 `DeleteConversationsForBot` |
| `backend/internal/repository/group.go` | 新增 |
| `backend/internal/group/orchestrator.go` | 新增 |
| `backend/internal/api/group.go` | 新增 |
| `backend/internal/api/group_test.go` | 新增 |

### Agent B — UI 规范（venus）

| 文件 | 动作 |
| --- | --- |
| `docs/GROUP-CHAT-UI-SPEC.md` | 新增 |

不改动任何产品代码。先读 `frontend/tailwind.config.js`、`docs/design-system.md`、`MessageBubble.tsx`、`Sidebar.tsx` 现有样式再出规范，不另起炉灶。

### Agent C — 前端（coder）

依赖 Agent A 的接口契约（本文档 §6）与 Agent B 的视觉规范。

| 文件 | 动作 |
| --- | --- |
| `frontend/src/types/index.ts` | 改：Message 加 2 字段，新增 GroupChat / GroupMember |
| `frontend/src/services/api.ts` | 改：暴露共享 axios 实例与拦截器行为，供 `groups.ts` 复用 |
| `frontend/src/services/groups.ts` | 新增 |
| `frontend/src/hooks/useGroupChat.ts` | 新增 |
| `frontend/src/App.tsx` | 改：§8.1 前置重构 + 自动回退修正 + localStorage |
| `frontend/src/components/layout/Sidebar.tsx` | 改：加号下拉 + 群聊分组 |
| `frontend/src/components/layout/TitleBar.tsx` | 改：群聊下显示群名 |
| `frontend/src/components/SearchPalette.tsx` | 改：可搜到群聊 |
| `frontend/src/components/CreateGroupDialog.tsx` | 新增 |
| `frontend/src/components/chat/ChatInput.tsx` | 改：@ 补全 |
| `frontend/src/components/chat/MessageBubble.tsx` | 改：发言人头像与名称解析 |
| `frontend/src/components/chat/ChatEmptyState.tsx` | 改：群聊变体 |
| `frontend/src/pages/GroupChatPage.tsx` | 新增 |
| `frontend/src/test/groupChat.test.tsx` | 新增 |
| `frontend/src/test/App.test.tsx` | **改**：v1 未认领，但 `App.tsx:300/307` 的 `soren.active-bot` 在此文件 :289/:297/:309/:352 被断言，§8.1 重构必然影响它，不改会违反「现有测试无回归」 |

`frontend/src/mobile/**` **不在任何人范围内**，禁止修改。

## 10. 验收标准

### 后端

- [ ] `go build ./...` 与 `go test ./...` 全绿，现有测试无回归（基线已确认：14 个包 ok + 2 个 no test files）
- [ ] 群路由注册在根 `r` 上；请求不带 `X-Bot-ID` 也能正常返回（证明绕过了 `BotContext`）
- [ ] 迁移前备份文件已生成且权限 0600（构造新库验证备份路径被执行）
- [ ] 提及模式：`@A @B` → A、B 各回复一条，两条 `speakerBotId` 正确且互不相同
- [ ] 自主模式：3 Bot × 3 轮 = 恰好 9 次 LLM 调用（或因 `[SKIP]` 提前结束）。**测试必须注入 fake Provider 并计数**（`agent.Service` 的 `Provider` 字段支持，`service.go:91`）
- [ ] 第 2 轮 Bot 能看到第 1 轮全部发言（两 Bot 用例，断言第二次调用的上下文包含第一次的输出）
- [ ] 中止：自主讨论中途调 cancel，`status == "cancelled"`，已产生的消息仍返回且已落库，末尾有 `role="system"` 的中止说明
- [ ] **单 Bot LLM 报错不中断整场**：3 Bot 中第 2 个返回错误，第 3 个仍发言，最终 `status == "done"` 且 `error` 非空
- [ ] **Bot 中途归档被跳过**：讨论进行中把某 Bot 设为 archived，其轮次被跳过并记入 `error`
- [ ] **重启收敛**：构造 `status = "running"` 的 GroupTurn，启动服务后变为 `interrupted`
- [ ] 越界拒绝：`maxRounds=7`、成员 6 人均返回 400（`成员数 × 轮次 > 30` 已被这两条边界蕴含，不单列用例，见 §6）
- [ ] **落库无嵌套**：群落库走单事务（§3 约束 3），测试中不得出现阻塞；开启 SQLite 单连接下调 `MaxOpenConns(1)` 跑一遍全流程验证不死锁
- [ ] **同群并发互斥**：两个 goroutine 同时对同一群发起发言，其一得 409（§5.5）
- [ ] **per-Bot 记忆开关**：`MemoryEnabled=false` 的 Bot 发言后，其记忆中**没有**该轮内容；开启的 Bot 有
- [ ] 幂等：同 `requestId` 重复提交返回同一批消息，不重复调用 LLM；同键不同参数返回 409
- [ ] 删除群：群对话与全部消息被清理（**验证无级联假设**），Bot 的私有记忆**保留**
- [ ] 群对话 `ExternalKey` 恒为空，渠道会话（飞书/钉钉）不受影响
- [ ] 群聊 handler 不调用 `botID(c)` / `resolveUserID(c)` / `currentBot(c)`

### 前端

- [ ] `npm --prefix frontend test` 全绿，**包含改过的 `App.test.tsx`**（基线已确认：11 文件 62 测试）
- [ ] 加号弹出两项菜单，键盘可完整操作
- [ ] 群聊消息头像与发言人名**逐条正确**（N 条不同发言者 = N 个不同头像）
- [ ] 群聊列表项与 Bot 项行高、内边距一致；无新增色彩体系
- [ ] 切换群聊后状态不串；切换群↔Bot 后状态不互相污染
- [ ] `@` 补全可用鼠标和键盘选中
- [ ] 群聊作为选中项被持久化，重启后能恢复
- [ ] 窄屏（<760px）下侧栏群聊分组不溢出
- [ ] `mobile/` 目录零改动

### 端到端（手工）

- [ ] 真实启动前后端，用一个真实模型完成「建群 → @ 发言 → 自主讨论 2 轮 → 中止」全流程，截图确认渲染

> 端到端项**无法在 CI 执行**（需要 API Key），标注为手工凭据项。
> 「行高/内边距一致」「同高同圆角」在 jsdom 下无法断言计算样式（Tailwind 是类名不是内联样式），改为类名断言或人工目检。

## 11. 风险清单

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 模型间趋同（互相附和） | 自主讨论产出无效内容 | §7.1 prompt 强制立场差异；`[SKIP]` 允许弃权；轮次默认压到 3 |
| 30 次串行 LLM 调用耗时过长 | 用户等待数分钟且无中间反馈 | 轮次进度反馈（§8.4）；中止随时可用；流式留待后续版本 |
| 本次 schema 变更无备份 | 迁移失败可能损坏用户数据 | §3 约束 4：Agent A 必须自行实现备份，失败则拒绝启动 |
| 群聊把他人 Bot 的发言写进私有记忆 | 跨上下文信息渗漏 | D3 已确认接受；§7.5 已补 per-Bot 开关校验 |
| SQLite 单连接写入串行 | 多轮落库变慢 | 不改连接池；记忆写入统一延后到发言结束；列表预览用冗余字段避免 N+1 |
| `Conversation.BotID` 语义被占用 | 后续维护困惑 | 注释说明 `group:` 保留命名空间；Bot ID 为 uuid，天然不冲突 |
| 服务重启留下半截发言 | 用户看到无来源的孤立回复 | §5.2 启动收敛为 `interrupted` + `system` 消息说明 |

## 12. 未决事项（已核实，均不阻塞开工）

1. 群聊是否需要独立的「对话历史」入口？`ViewId` 含 `'history'`（`types/index.ts:300`），但 `ConversationHistoryPage.tsx:12` 只遍历 `bots`，群对话进不了历史页。侧栏群列表本身是入口，延后可接受。
2. 群聊内是否允许 Bot 调用连接器工具？当前设计**默认不允许**——群聊复用 `llm.Provider` 但不调 `AttachTools`，天然无工具。延后无风险。
3. 自主模式是否需要「指定发言顺序」而非固定成员顺序。纯产品增强，延后无风险。
