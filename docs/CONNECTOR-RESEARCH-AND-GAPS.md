# 连接器调研与产品缺口

调研日期：2026-10-12。范围：GitHub README、许可证、部分接口与凭据实现，以及 Soren 当前源码。未安装第三方项目，未授权账号，未完成真实服务联调；下列为候选选型，不代表已经集成。

## 选型结论

保留 React / Tauri / Go / SQLite。在 Go 后端建设统一工具执行层，采用 MCP 客户端与直接 API 适配两种接入方式。连接器复用外部项目，Soren 管理连接身份、Bot 权限、确认、执行状态和成果。

| 项目 | 已核对内容 | 对 Soren 的用途与限制 |
| --- | --- | --- |
| [官方 MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) | README、go.mod、LICENSE | 首选协议库候选，支持客户端/服务端。当前 main 的 go.mod 要求 Go 1.25.0，Soren 声明 Go 1.21；实施时选择并锁定发布版本、升级工具链并验证。README 提示客户端 OAuth 支持仍有实验性部分。 |
| [Google Workspace MCP](https://github.com/taylorwilsdon/google_workspace_mcp) | LICENSE、pyproject.toml、gmail/gmail_tools.py、auth/credential_store.py | 可本地运行的社区项目，MIT，Python 依赖；适合作为 Gmail/日历/Drive 原型候选。本地目录凭据实现写 JSON，需要适配凭据存储、隔离连接账号和控制工具权限，不能直接作为已完成的桌面集成。 |
| [Activepieces](https://github.com/activepieces/activepieces) | README、LICENSE、Gmail index/auth/send-email-action | 参考动作参数、OAuth 和连接器组织方式。社区代码主要 MIT，仓库指定企业目录为单独许可证；复用时逐文件核对。TypeScript pieces 依赖其框架，不能直接复制进 Go 运行。 |
| [Goose](https://github.com/aaif-goose/goose) | README、仓库元数据 | 本地 Agent、桌面/CLI/API 与 MCP 扩展的架构参考。Apache-2.0；当前仅做文档层比较，未评估用它替换 Soren 执行引擎的成本。 |
| [Nango](https://github.com/NangoHQ/nango) | README、LICENSE | OAuth、凭据刷新和代理能力值得参考。存在云与自托管方案，ELv2；不把它当无条件可嵌入的宽松许可库。当前阶段暂不引入整个平台。 |
| [Composio](https://github.com/ComposioHQ/composio) | README、仓库元数据 | 多工具接入、按用户会话、动态发现工具值得参考。当前仓库是 SDK monorepo，快速开始依赖服务 API Key，并提供托管 MCP endpoint；SDK 源码可用不等于完整服务能本地部署。作为未来可选托管适配。 |

## 从实现中得到的具体结论

1. Activepieces 的 [Gmail 动作](https://github.com/activepieces/activepieces/blob/main/packages/pieces/community/gmail/src/lib/actions/send-email-action.ts) 使用 `draft` 参数分支调用 drafts.create 或 messages.send。Soren 不能只凭工具名称判断风险，需要检查实际操作参数，或对外拆成固定语义工具。批准应绑定账号、收件人、正文和附件版本；内容变更后原批准失效。
2. Google Workspace MCP 的 [CredentialStore](https://github.com/taylorwilsdon/google_workspace_mcp/blob/main/auth/credential_store.py) 提供可替换存储抽象，本地目录实现会写入 JSON。可研究接 Keychain 或 Soren 凭据代理，不能把“在本地”误认为“已经使用系统凭据库”。
3. SDK/协议负责连接和调用，不能替代 Soren 的 Bot 权限、账号绑定或任务执行策略。
4. 导入第三方工具时应固定版本、保留所需许可证声明。工具数量不是选型标准；授权、刷新、失败恢复、打包和实际成功率更重要。

## 第一版建议

- MCP Go SDK：先用锁定版本验证本地 stdio 与远程 HTTP 服务发现、调用、超时和断连，测试 OAuth 兼容性。
- Notion：优先验证[官方 MCP](https://developers.notion.com/guides/mcp/overview)，限于明确授权能力；若权限粒度不能满足需求，再使用直接 API 适配。
- Gmail：用社区 MCP 做受控原型，评估 Python 打包、账号隔离、凭据存储；最终在复用 MCP 与直接 Gmail API 之间按验证结果选。第一条真实流程是创建草稿、展示结果、绑定具体内容批准、发送。
- Canva：在文字工具流程完成后，验证[官方 MCP/API](https://www.canva.dev/docs/apps/)提供的具体能力。设计生成与导出按异步任务处理。
- 不要求普通用户自己安装 Python、Node 或手动运行命令。若采用本地 MCP 子进程，需要由应用安装、启动、停止、升级并显示故障。

## Soren 除连接器之外的缺口

### 必须与连接器同时补齐

1. **工具执行循环**：当前 llm.Provider 的消息/响应只有文本。增加结构化工具请求、调用结果回传、错误处理、轮数/时长/费用上限。统一聊天、协作和渠道入口。
2. **连接与权限模型**：增加 Connection、BotToolGrant、ToolInvocation、Approval 等持久化对象。连接账号由服务端绑定；模型不能通过随意填写账号参数选择其他凭据。多个 Bot 协作不自动继承对方权限。
3. **外部操作恢复**：当前 Run.RequestID 去重只保护创建任务。邮件已发出但响应丢失时，不能靠重跑步骤防重复；记录调用状态、服务端回执和待核实状态，按各产品能力核对，禁止盲目重试写操作。
4. **外部内容边界**：邮件、Notion 页面和工具描述可包含指令文本，但不能提升权限或改变已批准操作。权限由代码执行，不能只靠提示词。
5. **真实验收**：现有演示/测试不能证明真实 OAuth、令牌刷新、读写、取消和恢复可用。要有用户授权的测试账号验收以及可重复的本地故障测试。

### 形成日常可用产品所需

- **后台服务**：当前 main.rs 在窗口销毁和退出时停止后端。需要区分关闭窗口、后台运行和完全退出；明确休眠/断网恢复行为。
- **手机**：同账号、可信设备、传输通道、状态续传与撤销尚未实现。手机控制同一 Mac 实例，不独立复制一套 Bot。Mac 离线时不能显示任务已执行。
- **记忆**：当前为规则提取与召回，需要来源、更新/冲突纠正、遗忘与备份；协作任务目前不读取私有记忆，后续要实现私有上下文使用与可分享结果的边界。
- **成果管理**：目前主要是文本结果，需要文件、Notion 链接、邮件回执、设计导出记录与手机可访问的成果视图。
- **触发器**：定时执行、邮件到达等事件尚缺。先做本地定时任务；外部 webhook 需要在线接收端，轮询则需游标、限流和离线补偿。
- **成本**：每任务限制模型调用与工具次数，并记录失败/取消前已经发生的费用。多个 Bot 只在有明确分工收益时调用。
- **工程交付**：当前目录不是 Git 仓库，应建立版本管理后再扩大改动；补数据库迁移/备份恢复、依赖版本管理、Mac 签名公证与更新渠道。当前桌面包为 Apple Silicon、ad-hoc 签名。

## 建议验收顺序

1. 一个 Bot 使用真实 Notion 资料生成 Gmail 草稿，全程可追踪。
2. 批准发送后显示真实回执；模拟超时和重复请求不盲目重发。
3. 多 Bot 按职责协作，未授权 Bot 无法访问连接或借协作绕过权限。
4. 关闭 Mac 界面仍能后台完成任务，重启后状态准确。
5. 手机连接同一个实例，查看进度、批准操作、查看成果；离线状态真实。

完成以上步骤后，再扩展 Canva、图像/视频、更多应用和自主创建临时 Agent。
