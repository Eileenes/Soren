# Soren 0.3：本地连接器与任务执行

交付日期：2026-10-12。Apple Silicon / macOS 12+，沿用 `~/.soren` 中的本地数据。此版本完成本地执行基础，真实第三方授权仍需用户配置应用凭据并联调。

> 0.3.2 更新：设置已移除 GitHub 登录表单，本地使用无需账号。模型列表和最新配置见 [本地配置交付](LOCAL-SETTINGS-DELIVERY.md)。下文 GitHub 部分记录 0.3 实现，不再是用户的配置步骤。

## 已实现

| 功能 | 当前范围 |
| --- | --- |
| 多 Bot | 自定义职责、内置头像、独立私有记忆、显式共享资料、指定参与者的协作任务；不会自动无限创建 Bot |
| 工具任务 | 在「任务」页由模型选择工具、读取结果、继续处理；工具记录和 Markdown 成果持久化 |
| Gmail | Google 桌面 OAuth + PKCE，创建纯文本单收件人草稿、确认后发送；支持令牌刷新 |
| Notion | 内部集成 Token，搜索已授权页面、读取第一层内容、确认后创建子页面 |
| MCP | 官方 Go SDK v1.7.0，远程公网 HTTPS Streamable HTTP，可选 Bearer Token；所有调用都要求确认 |
| Bot 授权 | 每个连接分别分配给指定 Bot；协作不自动继承其他 Bot 的连接权限 |
| 本地身份 | GitHub OAuth Device Flow，绑定当前本地空间的 GitHub 身份 |
| 桌面后台 | 关闭窗口后保留菜单栏与后端，菜单可重新打开或完全退出；Mac 休眠时任务不能持续运行 |
| 界面 | 桌面、390px 手机宽度，浅色/深色连接管理与授权界面 |

连接凭据保存在 macOS Keychain，SQLite 只保存连接元数据；既有模型凭据仍保存在权限受限的本地配置文件，尚未迁移至 Keychain。

## 第一次使用

1. 安装 `release/Soren-0.3.0-macOS-arm64.dmg` 并打开应用。升级前完全退出旧版；不要删除 `~/.soren`。
2. 「设置 → 模型与连接」填入可用的模型接口、模型 ID 和 Key。需要服务商支持工具调用；未配置模型时只有明确标记的演示流程。
3. 「我的 Bot」设置职责和头像；「应用连接」添加连接，勾选允许使用它的 Bot。
4. 进入「任务」创建任务，例如「整理本周计划，保存为 Markdown」。要调用连接器时，也从任务页开始，普通聊天和飞书/钉钉消息尚未使用这个工具循环。
5. 任务出现「等待确认」后，展开查看连接账号与完整参数，批准具体操作，再点击「继续任务」。拒绝后继续会终止该操作及任务。修改连接授权会使原批准失效。
6. 成果可直接预览；桌面下载会写入 `~/.soren/exports` 并在 Finder 中显示，浏览器使用普通文件下载。

### GitHub 身份

在 GitHub 注册 OAuth App，启用 **Device Flow**，将 Client ID 填入 Soren 设置中的账号面板。应用打开 GitHub 验证页，输入显示的短码并授权；无需把 GitHub Client Secret 写入桌面端。文档：[GitHub Device Flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps)。

当前绑定的是单个本地空间所有者，不是在线账号系统。其他 GitHub 账号不能自动接管已有空间；没有账号也可使用本地功能。GitHub 授权不能替代 Gmail 或 Notion 的服务授权。

### Gmail

在 Google Cloud 项目中启用 Gmail API、配置 OAuth 同意屏幕并创建 **Desktop app** 客户端。测试状态时将自己的 Google 账号列为测试用户。把 Client ID 和 Client Secret 填入 Gmail 连接弹窗，然后在系统浏览器授权。回调是本机 Go 服务的 `/oauth/google/callback`，默认端口 8080，授权期间保持 Soren 运行。

当前 scope 为 `gmail.compose`，用于草稿与发送；不提供收件箱阅读、附件或日历操作。Google 的应用审核、用户限制与令牌政策由实际云项目状态决定。文档：[原生应用 OAuth](https://developers.google.com/identity/protocols/oauth2/native-app)。

### Notion

创建内部集成，将 Token 填入 Notion 连接弹窗；在 Notion 中将需要使用的页面连接到该集成。创建页面时指定已授权的父页面 ID。集成需具备读取和插入内容权限。文档：[Notion 授权](https://developers.notion.com/guides/get-started/authorization)。

### MCP

输入服务实际提供的公网 HTTPS Streamable HTTP endpoint，可填写 Bearer Token。当前没有 stdio 子进程、任意本地 HTTP、旧式 SSE 或 MCP OAuth 授权发现，因此不能把需要 OAuth 的服务填入地址后就视为已接通。Notion 本版使用直接 API；Canva 尚未接入。先前开源比较见 [调研记录](CONNECTOR-RESEARCH-AND-GAPS.md)，本文件代表实际交付范围。

致用 OctoBus 已提供独立卡片：填写个人 Token 并选择 Bot 后验证连接，服务地址自动配置。使用与联调范围见 [致用 OctoBus 连接器](octobus-connector.md)。

## 恢复与确认

- 确认绑定具体工具调用、参数、连接版本和所属 Bot；执行前再次检查任务状态及授权。
- 已成功调用恢复时复用记录，不重复发送；外部调用超时或进程中断后标记「结果待核实」，需要到服务端核实，不能自动当作失败后重新发送。
- 每个步骤最多 12 轮模型调用，每次回复最多 8 个工具调用，单 Bot 工具目录最多 64 项。尚未实现精确费用统计或金额预算。
- Gmail 草稿和 Notion 读取无需逐次确认；发邮件、创建 Notion 页面以及所有 MCP 调用需要确认。连接权限由后端执行，不由模型决定。

## 仍缺什么

手机目前只有适配后的界面，**不能远程控制本机**。下一阶段采用「同 GitHub 账号 + 可信设备配对 + 在线中继 + Mac 主动连接」：Mac 保存并执行 Bot，手机查看同一实例、批准操作、获取成果；Mac 离线时显示离线。还需云端会话、设备密钥、撤销、事件续传和部署，现有 `deploy` 目录是准备材料，不是已上线服务。本机 API 明确限制回环访问，不能通过开放端口代替账号鉴权。

此外尚缺：普通聊天/渠道统一工具执行、真实账号验收、Canva 和图片/视频生成、定时/事件触发、自动创建临时 Agent、完整记忆冲突处理与备份恢复、Apple 签名公证和自动更新。Bot 协作仍是选定参与者的任务步骤，不是任意 Bot 的自由群聊。

## 验证

- Go 全包测试、连接器/任务/API 竞态检测，覆盖工具参数、审批归属、拒绝、撤权、去重、未知结果、OAuth state/PKCE、刷新、身份绑定和本地访问限制。
- 前端 22 项测试、TypeScript/Vite 构建，Rust release 构建。
- 隔离 SQLite + 本地模拟模型响应 + 真实 Go 工具执行的浏览器验收：生成并保存 Markdown 成果、连接弹窗、GitHub 表单、手机宽度和深浅主题；截图在 `docs/previews/*03*.png`。
- 上述不等于真实 GitHub/Google/Notion 授权成功，也不等于发送了真实邮件；尚未提供这些服务的测试账号和应用凭据。
- 安装包为 ad-hoc 签名，尚未 Apple 公证。本轮新增依赖许可证随 App 的 `third-party` 资源分发。
