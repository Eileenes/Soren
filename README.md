<p align="center"><strong>简体中文</strong> · <a href="README.en.md">English</a></p>

<div align="center">
  <img src="assets/app-icon-1024.png" alt="Soren Logo" width="120" height="120" />
  <h1>Soren</h1>
  <p><strong>你的私人 AI 空间。</strong></p>
  <p>让不同的 Bot 各司其职，记住你的偏好，连接常用应用，把想法变成可查看的成果。</p>
  <p>本地记忆 · 多 Bot 协作 · 应用连接 · 手机联动</p>
  <p>
    <a href="https://github.com/Eileenes/Soren/releases/latest">下载 macOS</a> ·
    <a href="#开始使用">开始使用</a> ·
    <a href="#手机如何关联-mac">手机联动</a> ·
    <a href="#从源码运行">开发与构建</a> ·
    <a href="#文档导航">文档</a>
  </p>
  <sub>开发预览版 0.4.3 · macOS Apple Silicon · iOS 开发验证中</sub>
</div>

---

Soren 是一个以 Mac 为执行中心的个人 AI 助手。你可以和它对话，也可以创建工作、创作、生活等不同职责的 Bot，交办任务、查看执行过程，并保存成果。

对话、Bot 配置和记忆默认保存在本机。安装后无需注册 Soren 账号或登录 GitHub，配置自己的模型服务即可开始使用。需要操作其他应用时，再为指定 Bot 授予连接权限。

**当前可以使用 Mac 本地功能。** 手机扫码配对、加密中继和 iOS 工程已实现并完成本地验证，公共连接服务与正式手机分发尚未上线，默认安装包暂不能直接远程关联手机。

## Soren 可以做什么

| 能力 | 使用方式 |
| --- | --- |
| 多 Bot | 自定义名称、内置头像、职责和模型，按工作场景切换助手 |
| 本地记忆 | 每个 Bot 拥有独立记忆与对话历史，支持查看、搜索、添加、修改和删除记忆 |
| 协作任务 | 双方授权后邀请其他 Bot 参与任务，按步骤交换成果，由发起 Bot 汇总 |
| 应用连接 | 接入 Gmail、Notion 和兼容的远程 MCP 服务，为不同 Bot 分配权限 |
| 可查看的执行过程 | 查看任务步骤、工具参数与执行结果；确认需要审批的操作，导出 Markdown 成果 |
| 模型选择 | 支持 OpenAI 兼容协议和 Anthropic，获取服务商模型列表，也可手动填写模型 ID |
| 日常使用 | 浅色、深色与跟随系统外观；关闭窗口后可在菜单栏继续运行 |
| 消息渠道 | 可选接入飞书长连接和钉钉 Stream，也支持配置认证后的 Webhook 模式 |

记忆目前以规则提取、标签和时序召回为基础。普通「对话」用于交流；需要使用连接器或保存文件成果时，请从「任务」发起。

## 开始使用

### 1. 安装并打开

拿到 `Soren-0.4.3-macOS-arm64.dmg` 后，打开安装包，将 **Soren** 拖入「应用程序」，再启动应用。首次使用引导可以跳过，也可从应用顶部重新打开。

从 [GitHub Releases](https://github.com/Eileenes/Soren/releases) 下载 macOS 安装包及 SHA-256 校验文件。Git 源码不包含安装包；本机构建产物在 `release/` 目录，自行构建请看[从源码运行](#从源码运行)。当前 macOS 包采用本地签名，尚未经过 Apple 公证。

### 2. 连接一个模型

在「设置 → 模型」中：

1. 选择模型服务商，或选择「兼容接口」。
2. 填写接口地址和 API Key。
3. 点击「获取模型」并选择一个模型；服务商不支持列表时，可手动填写模型 ID。
4. 点击「保存模型连接」，等待本地服务重启，然后到「对话」发送一条消息。

没有配置 Key 时，Soren 使用本地回显模式，可体验界面和数据保存，但不会调用真实模型。执行工具任务需要所选模型支持工具调用。修改连接会重启本地服务，建议先结束正在运行的任务。

### 3. 完成第一个任务

选择默认的 Soren，打开「任务」，输入：

> 帮我制定一份三天学习计划，每天包括目标、行动和检查项。调用本地成果工具，保存为 Markdown，标题叫“三天学习计划”。

点击「开始任务」，展开执行记录。任务成功后，可在「本地成果」中预览内容并下载 Markdown。这个流程只需要模型连接，不需要额外接入 Gmail 或 Notion。

### 4. 按需连接应用

进入「应用连接」，选择服务并完成配置，再勾选「可使用的 Bot」。之后从「任务」交办工作，例如整理 Notion 页面或起草 Gmail 邮件。

## 创建你的 Bot

打开「我的 Bot → 新建 Bot」，选择头像，填写名称与职责，按需开启独立记忆，点击「保存 Bot」。模型字段留空时继承全局配置。

| Bot 示例 | 职责示例 |
| --- | --- |
| 工作助手 | 整理项目资料、拆解行动计划、起草邮件，清楚区分事实与建议 |
| 创作助手 | 规划选题、润色文案、编写分镜和图像提示词，保持一致的表达风格 |
| 生活助手 | 整理待办、规划日常安排、记录偏好，让建议更符合你的习惯 |

**职责决定怎么工作，权限决定能操作什么。** 在职责里写“发送邮件”不会自动获得邮箱访问权限，仍需连接 Gmail 并授权给该 Bot。

每个 Bot 的私有记忆与历史按 Bot 隔离。需要共用的内容放入「共享资料」，明确选择允许访问的 Bot。

### 让多个 Bot 一起完成任务

1. 在 Bot A 的配置中开启「允许协作」，选择 Bot B。
2. 在 Bot B 中也开启协作并选择 A，保存双方配置。
3. 切换到 A，在「任务 → 邀请协作」选择 B，填写共同目标并开始任务。

当前最多邀请 3 个其他 Bot，按步骤传递成果，由发起 Bot 最后汇总。各参与者只能使用自己获准的连接。协作任务不会直接加载各 Bot 的私有长期记忆，必要资料请写进任务目标，或放入所有参与者均获准访问的共享资料。

## 连接常用应用

| 连接 | 当前能力 | 配置要求 |
| --- | --- | --- |
| Gmail | 创建纯文本草稿、确认后发送邮件 | Google 桌面 OAuth 客户端及浏览器授权；暂不读取收件箱 |
| Notion | 按标题搜索、读取页面第一层内容、创建子页面 | Notion 内部集成 Token，并向集成授权相关页面 |
| 远程 MCP | 发现工具并在任务中调用 | 公网 HTTPS Streamable HTTP 地址，可选 Bearer Token；暂不支持 MCP OAuth 或本地 stdio |
| 本地成果 | 创建、读取和导出 Markdown | 无需外部账号，配置支持工具调用的模型即可 |

Gmail 发送、Notion 创建页面及 MCP 调用需要确认。遇到待确认任务，请核对账号和完整参数，点击「批准此操作」，再点击「继续任务」。Gmail 草稿和本地成果可直接创建。

连接可随时调整 Bot 权限或断开。当前为开发预览版，外部服务仍需自行准备相应应用凭据；详细步骤见[连接器配置说明](docs/CONNECTORS-DELIVERY.md)。

## 手机如何关联 Mac

桌面「设置 → 关联设备」提供 iOS 下载入口。当前 TestFlight 链接尚未就绪，入口显示待开放；发布后可通过按钮或安装二维码下载，再扫描配对码关联。发布方配置步骤见 [iOS 分发说明](docs/IOS-DISTRIBUTION.md)。

手机和 Mac 访问同一个本地空间，任务仍由 Mac 执行。关联依靠**扫码与 Mac 确认**，不要求登录同一 GitHub 账号。

产品方部署公共连接服务并完成 iOS 分发后，用户的操作流程为：

1. Mac 打开「设置 → 关联设备」，开启连接并生成二维码。
2. 手机打开 Soren，扫描 Mac 二维码。
3. 核对两端确认码，在 Mac 上允许关联。
4. 手机选择 Bot，发起任务、查看结果和记忆，或审批待确认操作。

普通用户无需部署服务器或手动填写 Mac IP。Mac 需要保持运行、联网且未休眠；中继只负责传输加密消息，不执行任务，也不备份记忆。

**目前的交付范围：**

- 已完成配对、加密传输、手机任务与记忆入口，以及 iOS 模拟器验证。
- 公共中继、真实 iPhone 扫码验收、正式签名和手机分发尚待完成，因此默认安装包暂不开放远程关联。
- Android、手机完整聊天、手机编辑 Bot、推送通知和离线编辑尚未交付。

开发者可用本地中继验证完整链路，见[连接服务部署指南](deploy/README.md)；权限与实现细节见[手机联动说明](docs/MOBILE-040-DELIVERY.md)。

## 数据与隐私

Soren 默认使用 SQLite 和进程内缓存，无需安装 PostgreSQL 或 Redis。本地数据位于 `~/.soren/`：

| 内容 | 保存位置 |
| --- | --- |
| 对话、Bot、记忆、任务与成果 | 本地 SQLite 数据库 |
| 模型连接配置 | 当前系统用户私有配置文件 |
| Gmail、Notion、MCP 凭据 | 系统 Keychain |
| 手机身份 | iOS Keychain；Mac 配对身份位于私有文件 |
| 下载的 Markdown | `~/.soren/exports/` |

使用云端模型或外部连接器时，完成请求所需的内容会发送给对应服务。本地存储不代表所有计算都离线完成。API Key 不应放入聊天、截图、Git 仓库或 `VITE_*` 环境变量。

开发与提交规则见 [SECURITY.md](SECURITY.md)。

## 从源码运行

### 开发环境

| 依赖 | 要求 |
| --- | --- |
| Node.js | 22.12 或更高版本 |
| Go | 1.26.9 或更高版本 |
| Rust | Rust 工具链与 Cargo |
| macOS 构建工具 | Xcode Command Line Tools；iOS 开发还需要完整 Xcode |

当前主要验证平台为 **macOS Apple Silicon**。其他桌面平台尚未完成发行验收。

### 启动桌面开发版

```bash
git clone https://github.com/Eileenes/Soren.git
cd Soren
npm --prefix frontend ci
npm run dev
```

开发命令会编译内置 Go 服务并启动 Tauri 窗口和前端热更新。桌面开发版与正式版默认使用相同的 `~/.soren` 数据目录；需要隔离实验数据时，可设置 `SOREN_DATA_DIR`。

### 仅调试 Web 界面与后端

在项目根目录分别打开两个终端：

```bash
# 终端一：后端，默认监听 127.0.0.1:8080
npm run dev:backend
```

```bash
# 终端二：前端，访问 http://localhost:1420
npm run dev:web
```

浏览器模式不提供桌面凭据保存和 Finder 导出等原生能力。需要真实模型时，参考 [backend/.env.example](backend/.env.example) 配置 `backend/.env`，仅填写使用的服务商 Key，将其他服务商 Key 留空，再启动后端。不要同时运行两个占用 8080 端口的后端。

### 构建 macOS 安装包

```bash
npm run build:mac
```

构建结果：

```text
frontend/src-tauri/target/release/bundle/
├── macos/Soren.app
└── dmg/Soren_0.4.3_aarch64.dmg
```

构建命令会自动编译后端并打包。签名和安装说明见 [macOS 构建指南](docs/MACOS-BUILD.md)。

### 开发 iOS 客户端

先安装完整 Xcode，并准备 Python 3、XcodeGen 与对应的 iOS Rust 编译目标；初始化脚本会调用这些工具生成工程。

```bash
npm --prefix mobile run ios:init
npm --prefix mobile run ios:dev
```

iOS 工程由脚本生成，不随 Git 提交。模拟器用于开发验证；真机安装和分发需要 Apple 开发者签名，跨网络联动还需要可访问的公共连接服务。完整流程见[部署指南](deploy/README.md)。

### 运行检查

```bash
# 前端测试、类型检查和生产构建
npm --prefix frontend test
npm --prefix frontend run build

# Go 测试与静态检查
(cd backend && go test -race ./... && go vet ./...)

# 桌面 Rust 测试
cargo test --locked --manifest-path frontend/src-tauri/Cargo.toml
```

0.4.1 基线的验证结果、已修复问题与剩余依赖告警见 [0.4.1 审查记录](docs/REVIEW-041.md)。

## 项目结构

```text
Soren/
├── backend/
│   ├── cmd/server/       # Mac 本地服务
│   ├── cmd/relay/        # 独立加密中继
│   └── internal/         # Bot、任务、记忆、连接器、渠道与设备配对
├── frontend/
│   ├── src/              # React 桌面界面与移动入口
│   └── src-tauri/        # Rust 桌面壳与内置后端管理
├── mobile/src-tauri/     # iOS 原生壳、扫码与安全存储
├── deploy/               # 中继部署与反向代理配置
├── assets/               # 品牌 Logo 与图标
├── docs/                 # 使用说明、架构和交付记录
├── scripts/              # 开发、构建与端到端验证脚本
└── release/              # 本地产物目录，安装包不上传 Git
```

技术栈：**Tauri 2 · React 18 · TypeScript · Go · SQLite**。

## 常见问题

**需要登录才能用吗？**

本地使用不需要 Soren 或 GitHub 登录。模型凭据、Gmail 授权等只用于对应服务。

**为什么 Bot 没有真的发送邮件或保存成果？**

从「任务」发起，确认模型支持工具调用、连接已授权给当前 Bot，并完成必要的审批。以成功工具记录和实际成果为准。

**关闭 Mac 窗口后还会运行吗？**

会留在菜单栏后台运行。菜单栏选择「退出 Soren」才会完全退出；电脑休眠或断网会影响任务与手机联动。

**手机现在可以直接扫码用吗？**

默认发行构建尚未接入公共服务，暂时不能。开发者可以按部署指南验证本地链路，普通用户需等待公共服务与手机分发就绪。

**未来还会支持什么？**

后续方向包括正式手机分发、更完整的移动交互、更多连接器，以及图片/视频生成和定时任务。Canva、日历和独立媒体生成流程目前尚未内置。

## 文档导航

- [版本发布](docs/RELEASING.md) / [iOS 分发](docs/IOS-DISTRIBUTION.md)：GitHub Release、TestFlight 和桌面下载入口。

- [日常操作指南](docs/USER-GUIDE.md)：模型配置、Bot、任务和应用连接。
- [连接器说明](docs/CONNECTORS-DELIVERY.md)：Gmail、Notion 与 MCP 的接入范围。
- [手机联动说明](docs/MOBILE-040-DELIVERY.md) / [部署指南](deploy/README.md)：配对、权限与服务部署。
- [配置参考](docs/CONFIGURATION.md) / [故障排查](docs/TROUBLESHOOTING.md)：环境变量和常见启动问题。
- [多 Bot 与移动规划](docs/MULTI-BOT-AND-MOBILE-PLAN.md) / [架构审查](docs/ARCHITECTURE-REVIEW.md)：设计与演进背景。
- [安全约定](SECURITY.md) / [0.4.1 审查记录](docs/REVIEW-041.md)：凭据处理、验证与当前限制。

历史方案与交付文档保留了各版本的状态；当前使用范围以本页和最新审查记录为准。
