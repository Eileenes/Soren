<div align="center">

<img src="assets/app-icon-1024.png" alt="Soren Logo" width="112" height="112" />

# Soren

### 从一次对话，到一组各司其职的 AI 助手。

为工作、创作与生活创建不同的 Bot：保留记忆、一起讨论、调用应用，保存可查看的成果。<br />
个人空间在本机运行；公司空间连接自部署服务，使用公司模型与独立的用户空间。

![Source](https://img.shields.io/badge/source-0.4.8-6366F1?style=flat-square)
![Desktop](https://img.shields.io/badge/desktop-Tauri_2-334155?style=flat-square)
![Frontend](https://img.shields.io/badge/frontend-React_18-149ECA?style=flat-square)
![Backend](https://img.shields.io/badge/backend-Go-F59E0B?style=flat-square)

**[下载已发布版本](https://github.com/Eileenes/Soren/releases)** · **[快速开始](#quick-start)** · **[应用连接](#connections)** · **[使用限制](#limitations)** · **[完整文档](#docs)**

[English — 0.4.3 guide](README.en.md)

</div>

---

**先确认你使用的版本：** 本页介绍 `main` 分支的 **0.4.8 源码**。当前公开 Release 最新为 **0.4.3**，提供 macOS Apple Silicon DMG 和 SHA-256 校验文件；0.4.8 源码中的聊天工具调用、空间共享连接、首次空间选择等改动，不会自动出现在 0.4.3 安装包中。体验下文的新功能需从源码运行或自行构建。版本记录中的 Windows／Linux 包说明不代表这些安装包已在公开 Release 提供。

## 能做什么

| 对话与记忆 | Bot 与协作 | 应用与成果 |
| --- | --- | --- |
| 单聊、会话历史与流式回复 | 自定义名称、头像、职责与模型 | 在单聊、群聊和任务中调用工具 |
| 按 Bot 保存、搜索和编辑记忆 | 群聊中 @ 指定 Bot，或邀请成员讨论 | 查看调用参数、结果与待确认操作 |
| 个人空间本地存储，公司空间独立保存 | 多 Bot 分步协作，由发起者汇总成果 | 保存 Markdown，下载连接器生成的文件 |
| OpenAI 兼容接口与 Anthropic | 职责版本、动作策略、暂停与归档 | Gmail、Notion、远程 MCP 和创作服务连接 |

另外提供共享资料、Bot 模板发布与导入、飞书／钉钉消息渠道，以及开发中的 iOS 配对链路。模板、公司空间、手机联动分别需要相应服务，具体范围见下文。

**模型和外部服务需自行配置。** Soren 不附带模型额度或第三方服务账户；未配置模型 Key 时使用本地回显模式，只能体验界面和数据保存。

<a id="quick-start"></a>

## 快速开始

### 1. 选择安装包或源码

**使用现有安装包：** 从 [Releases](https://github.com/Eileenes/Soren/releases) 下载 0.4.3 macOS ARM64 DMG，将 Soren 拖入「应用程序」。该包采用本地签名，尚未经过 Apple 公证。它使用旧版操作流程：从「任务」调用工具，并按 Bot 分配连接权限；可参考 [0.4.3 英文指南](README.en.md)。

**使用当前源码功能：** 准备 Node.js **22.12+**、Go **1.26.9+**、Rust/Cargo；macOS 还需 Xcode Command Line Tools。在 macOS 开发环境执行：

```bash
git clone https://github.com/Eileenes/Soren.git
cd Soren
npm --prefix frontend ci
npm run dev
```

开发命令会编译内置 Go 服务，启动 Tauri 窗口和前端热更新。下列操作以 **0.4.8 源码客户端**为准。

开发版与安装版默认共用 `~/.soren/`。需要单独体验时，在启动前设置数据目录，例如：

```bash
SOREN_DATA_DIR="$HOME/.soren-dev" npm run dev
```

### 2. 选择使用空间

首次打开先选择空间，再点击「确认」：

| 空间 | 如何进入 | 数据与模型 |
| --- | --- | --- |
| 个人空间 | 选择「个人空间」，无需 Soren 或 GitHub 登录 | 数据保存在本机；自行配置模型 |
| 公司空间 | 填写管理员提供的服务器地址，通过浏览器完成公司登录 | 数据保存在公司服务器；模型由管理员提供 |

首次界面默认选中公司空间，个人使用者需主动切换到个人空间。之后可从顶部「使用空间」入口切换；个人资料不会自动上传到公司。

### 3. 连接模型，完成第一个任务

个人空间进入「设置 → 模型」，选择服务商或兼容接口，填写地址与 API Key，获取模型列表或手动填写模型 ID，再保存连接。保存会重启本地服务，建议先结束正在执行的任务。

选择默认 Bot，在单聊或「任务」中输入：

> 帮我制定一份三天学习计划，每天包括目标、行动和检查项。调用本地成果工具，保存为 Markdown，标题叫“三天学习计划”。

此流程无需 Gmail 或 Notion。模型必须支持工具调用，且 Bot 的动作策略允许保存成果。以成功的工具记录和实际文件为准；模型在文字里说“已完成”不等于文件已经创建。

### 4. 添加自己的 Bot

在「我的 Bot」创建助手，填写职责与边界，选择模型和记忆设置；模型留空时继承全局配置。可以分别创建工作助手、创作助手和生活助手。

在「配置 Bot」调整工具动作的「禁用／每次确认／允许」。需要共用的内容放入「共享资料」，选择允许访问的 Bot；私有记忆与会话仍按 Bot 隔离。

## 从单聊到协作

**选择空间 → 创建 Bot → 配置模型与应用 → 单聊／群聊／任务 → 确认操作 → 查看成果**

| 入口 | 适合什么需求 | 当前行为 |
| --- | --- | --- |
| 单聊 | 持续交流、使用工具、积累个人记忆 | 在消息中查看调用记录、审批、继续与取消 |
| 群聊 | 让不同职责的 Bot 共同讨论 | @ 谁由谁回复；直接提问全员回复一轮；邀请讨论最多三轮 |
| 任务 | 明确目标、分步骤执行和交付 | 可邀请其他 Bot 协作，按步骤传递结果并汇总 |
| Bot 模板 | 把职责与配置交给其他使用者 | 发布版本链接，预览后导入独立副本；不包含聊天、私有记忆或凭据 |

协作仍受双方的协作设置和动作策略约束。群聊遇到待确认操作时暂停本轮讨论；处理该操作后，发送下一条消息继续。模板需要单独部署服务，链接持有者可查看和导入，副本不自动更新；公司空间暂不支持模板发布与导入。[模板服务与边界 →](docs/bot-template-service.md)

<a id="connections"></a>

## 连接常用应用

在「应用连接」添加所需服务。**0.4.5 起，启用的连接供当前用户、当前空间内的所有 Bot 使用**，包括以后新建的 Bot；不再逐 Bot 勾选连接。Bot 动作策略和每次操作确认仍然生效，连接停用或断开后全空间停止使用该连接。

| 连接 | 当前能力 | 需要什么／边界 |
| --- | --- | --- |
| Gmail | 创建纯文本草稿、确认后发送邮件 | Google 桌面 OAuth 客户端与授权；不读取收件箱；公司模式暂未接入 |
| Notion | 按标题搜索、读取页面第一层内容、创建子页面 | 内部集成 Token，并向集成授权页面 |
| 远程 MCP | 发现服务工具并调用 | 公网 HTTPS Streamable HTTP 地址，可选 Bearer Token；不支持本地 stdio 或 MCP OAuth |
| Agent 工具包／致用 OctoBus | 预设 MCP 地址，连接时发现工具 | 对应 API Key／个人 Token；能力以服务返回和账户权限为准 |
| AI 音乐 | 创作方案、生成、查询与下载音乐 | 百智云 AI 音乐 API Key；生成、重试和删除需确认 |
| AI 生图 | 文生图、图生图、查询与保存图片 | 百智云 AI 生图 API Key；生成和重试消耗积分，需确认 |
| 视频 API | 查询规格、文生视频、图生视频、查询与下载 | 对应 API Key；图生视频接受公网 HTTPS 原图 URL，暂不上传本地原图 |
| 云浏览器 | 创建和查询远程运行、人工处理链接、保存下载 | CapyBrowser API Key；启动计费运行等操作需确认 |
| 设计提示词 | 查询设计系统、视觉风格、方向与模板 | 对应 API Key；当前工具为只读查询和模板下载 |
| 本地成果 | 创建、列出、读取与导出 Markdown | 不需要外部连接，仍受 Bot 动作策略约束 |

Gmail 发送、Notion 新建页面、MCP 调用和付费生成等操作需要确认。界面出现待确认记录时，核对账户、内容和参数，再批准或拒绝。外部操作结果不确定时不会自动重复提交。

每个 MCP 连接最多发现 32 个工具，单 Bot 的工具目录总数上限为 64；连接过多时需要停用不用的服务。各连接器有模拟接口测试，**不代表所有真实账户和业务动作都已完成联调**。服务价格、额度和可用工具以对应服务为准。

## 公司部署与手机联动

| 能力 | 现有实现 | 使用条件 |
| --- | --- | --- |
| 公司空间 | 独立 Go 企业服务、PostgreSQL、OAuth／IngressHub SSO、按用户隔离空间 | 管理员部署 HTTPS 服务与模型；桌面和服务端需同步升级 |
| Bot 模板服务 | 独立 Go 服务、SQLite、发布凭据、版本链接与导入 | 管理员部署服务；发布者配置身份，接收者预先配置服务地址 |
| iOS 联动 | 扫码配对、桌面确认、加密中继、任务和记忆入口 | 需构建 iOS 工程、准备签名和中继；公共手机分发尚未开放 |

公司服务目前面向单实例试点，未完成大规模容量验收；不提供多实例持久调度或完整管理控制台。Gmail OAuth、飞书／钉钉渠道、模板分享和手机企业登录尚未接入公司模式。[企业部署 →](docs/ENTERPRISE-DEPLOYMENT.md) · [IngressHub SSO →](docs/INGRESSHUB-DEPLOYMENT.md)

手机配对不依赖同一个 GitHub 账号。桌面必须保持运行、联网且未休眠；中继传输加密消息，不代替桌面执行任务或备份记忆。当前没有公开 TestFlight 下载，真实 iPhone 与公网生产验收仍待完成。[手机联动 →](docs/MOBILE-040-DELIVERY.md) · [服务部署 →](deploy/README.md) · [iOS 分发 →](docs/IOS-DISTRIBUTION.md)

<a id="limitations"></a>

## 使用条件与当前限制

- **源码与发行包不同步**：本页功能基于 0.4.8 源码，公开下载仍为 0.4.3 macOS ARM64。其他桌面平台的构建支持和版本记录不能替代可下载安装包及目标系统验收。
- **记忆不是无限或完全准确**：当前使用规则提取、标签和时序召回，可在界面核对、修改和删除；协作任务不会直接加载每个 Bot 的私有长期记忆。
- **模型必须具备相应能力**：文本聊天可用不代表工具调用可用；没有有效模型连接时为回显模式。模型与连接器费用由各服务收取。
- **个人本地存储不代表完全离线**：调用云模型或连接器时，请求所需内容会发送给对应服务；公司空间的数据保存在公司服务器。
- **移动端仍在开发**：Android、手机完整聊天、手机编辑 Bot、推送通知和离线编辑尚未交付。

个人空间默认使用 SQLite 和进程内缓存，无需另装 PostgreSQL 或 Redis。默认数据目录为 `~/.soren/`：

| 内容 | 保存位置 |
| --- | --- |
| 个人对话、Bot、记忆、任务与成果 | 本地 SQLite 数据库 |
| 个人模型连接 | 当前用户的数据目录内 `provider.json` |
| 个人连接器凭据、公司登录令牌 | 系统凭据库 |
| 导出的成果与下载文件 | 数据目录内 `exports/`；连接器文件另有各自目录 |
| 公司空间数据与连接器凭据 | 公司数据库和文件目录；凭据由服务端密钥加密保存 |

本地记录不是磁盘加密；公司管理员管理的数据库也不是管理员不可读的端到端存储。凭据不要写进聊天、公开截图、Git 或 `VITE_*` 配置。[安全约定 →](SECURITY.md)

## 开发与构建

<details>
<summary><b>单独调试 Web 界面和后端</b></summary>

完成前端依赖安装后，在根目录打开两个终端：

```bash
# 终端一：本地后端，默认 127.0.0.1:8080
npm run dev:backend
```

```bash
# 终端二：Web 界面，http://localhost:1420
npm run dev:web
```

需要真实模型时，按 [backend/.env.example](backend/.env.example) 创建 `backend/.env`，仅填写所用服务商的 Key。浏览器模式不提供完整桌面原生能力；不要再启动另一个占用 8080 端口的后端。

</details>

<details>
<summary><b>构建桌面安装包</b></summary>

在 macOS 开发环境、项目根目录执行：

```bash
npm run release:check
npm run build:mac
```

产物位于 `frontend/src-tauri/target/release/bundle/`。带显式 `--target` 参数时，产物位于相应 target 子目录。`build:mac` 会编译内置 Go 服务并生成 app／DMG；仓库 GitHub Actions 自动发布流程当前仅覆盖 macOS。签名、校验与发布步骤见 [macOS 构建](docs/MACOS-BUILD.md) 和 [发布指南](docs/RELEASING.md)。

Windows／Linux 构建还需要对应 Tauri 系统依赖和工具链，不能仅凭 macOS 构建成功推断通过跨平台安装验收。

</details>

<details>
<summary><b>开发 iOS 客户端</b></summary>

安装完整 Xcode、Python 3、XcodeGen 和对应 iOS Rust 编译目标，并先安装 `frontend` 的 npm 依赖：

```bash
npm --prefix mobile run ios:init
npm --prefix mobile run ios:dev
```

生成的 Xcode 工程不提交 Git。真机与分发需要 Apple 开发者签名；跨网络连接需配置公共中继。详见 [iOS 分发](docs/IOS-DISTRIBUTION.md)。

</details>

<details>
<summary><b>运行现有检查</b></summary>

在项目根目录执行：

```bash
npm --prefix frontend test
npm --prefix frontend run build
(cd backend && go test -race ./... && go vet ./...)
npm run build:backend
cargo test --locked --manifest-path frontend/src-tauri/Cargo.toml
npm run release:check
```

这些命令分别覆盖现有前端测试、类型检查与构建、Go 测试与静态检查、桌面 Rust 测试及版本配置一致性；不代替真实模型、外部账户、企业身份服务或手机分发验收。各版本已有验证记录见 [发布记录](docs/releases)。

</details>

<details>
<summary><b>目录结构</b></summary>

```text
Soren/
├── backend/
│   ├── cmd/server/       # 桌面本地服务
│   ├── cmd/enterprise/   # 公司空间服务
│   ├── cmd/templates/    # Bot 模板服务
│   ├── cmd/relay/        # 加密中继
│   └── internal/        # Bot、群聊、任务、记忆、连接器与设备配对
├── frontend/
│   ├── src/             # React 界面与移动入口
│   └── src-tauri/       # Rust 桌面壳、内置后端与第三方声明
├── mobile/src-tauri/    # iOS 原生壳、扫码与安全存储
├── deploy/              # 中继、企业与模板服务部署配置
├── assets/              # Logo 与图标
├── docs/                # 使用、配置、架构与发布记录
├── scripts/             # 开发、构建与验证脚本
└── release/             # 本地产物说明；安装包不随源码分发
```

</details>

<a id="docs"></a>

## 文档与维护

| 文档 | 内容 |
| --- | --- |
| [0.4.8 发布记录](docs/releases/v0.4.8.md) · [0.4.4 聊天工具](docs/releases/v0.4.4.md) · [0.4.5 连接共享](docs/releases/v0.4.5.md) | 当前源码的版本变化与验证边界 |
| [日常操作指南](docs/USER-GUIDE.md) · [English 0.4.3](README.en.md) | 较早版本的使用说明，操作入口可能与当前源码不同 |
| [连接器说明](docs/CONNECTORS-DELIVERY.md) · [Agent 工具包](docs/agent-toolkit-connector.md) · [OctoBus](docs/octobus-connector.md) · [视频 API](docs/video-api-connector.md) | 接入要求与能力范围；旧文档的逐 Bot 勾选规则已由空间共享取代 |
| [Bot 模板服务](docs/bot-template-service.md) | 部署、发布、导入、版本与访问边界 |
| [企业部署](docs/ENTERPRISE-DEPLOYMENT.md) · [IngressHub](docs/INGRESSHUB-DEPLOYMENT.md) | 公司服务、身份认证、用户隔离与运维 |
| [手机联动](docs/MOBILE-040-DELIVERY.md) · [中继部署](deploy/README.md) · [iOS 分发](docs/IOS-DISTRIBUTION.md) | 配对实现、部署条件与待完成事项 |
| [配置参考](docs/CONFIGURATION.md) · [故障排查](docs/TROUBLESHOOTING.md) | 环境变量与常见启动问题 |
| [发布指南](docs/RELEASING.md) · [macOS 构建](docs/MACOS-BUILD.md) | 打包、签名、校验与 GitHub Release |
| [多 Bot 与移动规划](docs/MULTI-BOT-AND-MOBILE-PLAN.md) · [架构审查](docs/ARCHITECTURE-REVIEW.md) · [0.4.1 审查](docs/REVIEW-041.md) | 历史设计与验证背景，不代表所有规划均已交付 |
| [安全约定](SECURITY.md) | 凭据处理、开发检查与运行边界 |

反馈问题时请[提交 Issue](https://github.com/Eileenes/Soren/issues)，注明源码／安装包版本、系统、所用空间、复现步骤和预期结果；移除日志中的凭据与私人信息。

## 来源与授权

项目采用 Tauri、React、Go 等组件，依赖保留各自的许可证。桌面相关第三方声明见 [third-party/README.txt](frontend/src-tauri/third-party/README.txt)，移动端附带 [TweetNaCl 许可证](mobile/third-party/tweetnacl-LICENSE)。应用连接图标的来源见[图标来源记录](docs/connection-logo-sources.md)。

**当前仓库没有给 Soren 项目整体授予许可证的根目录 LICENSE。** 第三方依赖许可证不等于 Soren 自身的使用或再分发授权；公开源码不自动授予额外权利。模型、应用服务、品牌和素材分别遵循各自条款。
