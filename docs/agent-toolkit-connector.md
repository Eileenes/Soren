# Agent 工具包连接器

接入规范来源：[Agent 工具包接入指南](https://agent-toolkit.app.baizhi.cloud/access-guide)。

在「设置 → 应用连接 → Agent 工具包」点击「添加」，填写在服务商「API Key 管理」创建的 Key，并勾选可使用的 Bot。点击「验证并连接」后，Soren 通过 Streamable HTTP 连接 `https://agent-toolkit.app.baizhi.cloud/mcp`，使用 `Authorization: Bearer <API Key>` 鉴权，读取工具列表后保存连接。没有可用工具或验证失败时不会保存。

此入口是现有 MCP 连接器的预设：持久化类型仍为 `mcp`，复用系统凭据库、工具发现、Bot 权限和逐次调用确认。服务地址自动填写，用户不需要手动配置。之前从通用 MCP 入口添加的同地址连接，也会归入 Agent 工具包卡片，可继续调整 Bot 权限或断开连接。

工具范围由服务商和该 Key 的权限决定。当前沿用通用 MCP 的限制：每个连接读取最多 32 个工具，单个 Bot 总工具数最多 64 个。不内置未经服务发现的工具或虚构功能。

未提供真实 API Key 时，可验证表单、请求映射和本地授权流程，不能声称已完成真实账号的连接或工具调用。
