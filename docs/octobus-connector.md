# 致用 OctoBus 连接器

在「应用连接 → 致用 OctoBus」点击「添加」，填写致用平台的个人 Token（不含 `Bearer` 前缀），勾选「可使用的 Bot」，再点击「验证并连接」。地址已预填为 `https://octobus.chaitin.net/capsets/mcp`，无需配置环境变量或安装额外客户端。

连接复用现有 MCP Streamable HTTP 客户端，以 `Authorization: Bearer <个人 Token>` 鉴权，发现工具后才保存。Token 保存在本机系统凭据库，不写入连接元数据。验证失败或服务未提供工具时不保存连接，界面显示错误并保留表单。

连接成功后，从「任务」交办工作，由所选 Bot 使用服务返回的工具与参数定义。可用能力取决于 Token 权限和致用平台当前暴露的 MCP 工具；所有调用沿用现有逐次确认流程。可在卡片中调整 Bot 权限或断开连接。从通用 MCP 入口添加的同地址连接，也会归入致用卡片。

用户提供的 `octobus-grpc` 文档描述了同一平台的 gRPC 发现、reflection 和调用协议。本入口使用用户提供的 MCP 地址，不混用 gRPC 方法路径、reflection 字段或 `connect_rpc` 的 procedure，也不要求用户填写能力包或实例请求头。具体 MCP 工具名及输入字段以服务的 `tools/list` 结果为准，不在客户端预设 CRM/JIRA 方法。

当前沿用通用 MCP 限制：每个连接最多发现 32 个工具，单个 Bot 工具目录总数最多 64 个。授权范围或工具列表发生变化后，需要断开并重新添加，以重新发现工具。

接入检查中，向上述地址发送未携带凭据的 MCP `initialize` 和 `tools/list` 请求均返回 HTTP 401、`TOKEN_REQUIRED`。这只验证了地址可达及 Bearer 鉴权入口；未提供真实个人 Token，因此尚未验证真实账号的握手、工具发现或企业能力调用。gRPC 文档中的业务信封及状态码不能据此推断为 MCP 返回格式。
