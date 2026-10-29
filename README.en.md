> **Version note:** This guide describes Developer Preview **0.4.3**. For installation instructions and capabilities of the current `main` branch, see the [main README](README.md). The content below is retained as documentation for 0.4.3.

<p align="center"><a href="README.md">简体中文</a> · <strong>English</strong></p>

<div align="center">
  <img src="assets/app-icon-1024.png" alt="Soren Logo" width="120" height="120" />
  <h1>Soren</h1>
  <p><strong>Your personal AI space.</strong></p>
  <p>Give each Bot a role, let it remember your preferences, connect your apps, and turn ideas into results you can inspect.</p>
  <p>Local memory · Multi-Bot collaboration · App connections · Mobile access</p>
  <p>
    <a href="https://github.com/Eileenes/Soren/releases/latest">Download macOS</a> ·
    <a href="#getting-started">Get started</a> ·
    <a href="#connect-your-phone-to-your-mac">Mobile access</a> ·
    <a href="#run-from-source">Develop and build</a> ·
    <a href="#documentation">Documentation</a>
  </p>
  <sub>Developer Preview 0.4.3 · macOS Apple Silicon · iOS under development and validation</sub>
</div>

---

Soren is a personal AI assistant that runs tasks on your Mac. Chat with it, or create Bots for work, creativity, and daily life. Assign tasks, follow their execution, and save the results.

Conversations, Bot settings, and memory are stored locally by default. You do not need a Soren account or GitHub login to get started: configure your own model provider and begin using the app. Grant individual Bots access to other apps when needed.

**Local Mac features are available now.** QR pairing, encrypted relay transport, and the iOS project are implemented and have been tested locally. The public connection service and official mobile distribution are not live yet, so the default build cannot currently connect to a phone remotely.

## What Soren can do

| Feature | How it works |
| --- | --- |
| Multiple Bots | Customize names, built-in avatars, responsibilities, and models for different tasks |
| Local memory | Each Bot has separate memory and conversation history; view, search, add, edit, and delete memories |
| Collaborative tasks | With mutual permission, invite other Bots to exchange results step by step; the initiating Bot prepares the final response |
| App connections | Connect Gmail, Notion, and compatible remote MCP services, with permissions per Bot |
| Inspectable execution | Inspect task steps, tool arguments, and results; approve operations that require confirmation and export Markdown artifacts |
| Model selection | Use OpenAI-compatible APIs or Anthropic; fetch provider models or enter a model ID manually |
| Everyday use | Light, dark, and system appearance; keep the app running in the menu bar after closing its window |
| Messaging channels | Optionally connect Feishu through a persistent connection or DingTalk Stream; authenticated webhooks are also supported |

Memory currently uses rule-based extraction, tags, and chronological recall. Use **Chat (对话)** for conversation and **Tasks (任务)** when you need connectors or saved artifacts.

## Getting started

The current app interface uses Chinese labels. This guide includes the original labels for key actions so you can find them in the app.

### 1. Install and open

Open `Soren-0.4.3-macOS-arm64.dmg`, drag **Soren** into Applications, and launch it. You can skip the first-use setup guide and reopen it from the top of the app later.

Download the macOS installer and SHA-256 checksum from [GitHub Releases](https://github.com/Eileenes/Soren/releases). Installers are release assets, not part of the Git source history. Local builds are stored in `release/`. To build your own, see [Run from source](#run-from-source). The current macOS package is locally signed and has not been notarized by Apple.

### 2. Connect a model

Go to **Settings → Model (设置 → 模型)**:

1. Choose a model provider, or select **Compatible API (兼容接口)**.
2. Enter the API endpoint and API key.
3. Click **Fetch models (获取模型)** and select a model. If the provider does not support listing models, enter the model ID manually.
4. Click **Save model connection (保存模型连接)**, wait for the local service to restart, then send a message in **Chat (对话)**.

Without a configured key, Soren uses a local echo mode for trying the interface and data persistence; it does not call a real model. Tool-based tasks require a model that supports tool calling. Updating the connection restarts the local service, so finish any running tasks first.

### 3. Complete your first task

Select the default Soren Bot, open **Tasks (任务)**, and enter:

> Create a three-day study plan with a goal, actions, and checkpoints for each day. Use the local artifact tool to save it as Markdown titled “Three-Day Study Plan”.

Click **Start task (开始任务)** and expand the execution log. After a successful run, preview the result under **Local artifacts (本地成果)** and download the Markdown file. This workflow only requires a model connection; Gmail and Notion are optional.

### 4. Connect apps as needed

Open **App connections (应用连接)**, choose a service, complete its setup, and select the **Allowed Bots (可使用的 Bot)**. Then use Tasks to assign work such as organizing Notion pages or drafting Gmail messages.

## Create your Bots

Open **My Bots → New Bot (我的 Bot → 新建 Bot)**, choose an avatar, enter a name and responsibilities, optionally enable independent memory, and click **Save Bot (保存 Bot)**. Leave the model field blank to inherit the global configuration.

| Example Bot | Example responsibilities |
| --- | --- |
| Work assistant | Organize project information, break down action plans, and draft emails while clearly distinguishing facts from suggestions |
| Creative assistant | Plan topics, polish copy, and write storyboards and image prompts with a consistent voice |
| Daily-life assistant | Organize to-dos, plan daily activities, and remember preferences to make suggestions more relevant |

**Responsibilities define how a Bot works; permissions define what it can access.** Writing “send emails” in its instructions does not grant mailbox access. You still need to connect Gmail and authorize that Bot.

Private memory and history are isolated per Bot. Put information that should be shared in **Shared materials (共享资料)** and explicitly select the Bots allowed to access it.

### Have multiple Bots work together

1. In Bot A's settings, enable **Allow collaboration (允许协作)** and select Bot B.
2. Enable collaboration for Bot B and select A, then save both configurations.
3. Switch to A, choose B under **Tasks → Invite collaborators (任务 → 邀请协作)**, enter a shared goal, and start the task.

You can currently invite up to three other Bots. They pass results sequentially, and the initiating Bot produces the final response. Each participant can only use its own authorized connections. Collaborative tasks do not directly load the Bots' private long-term memory. Include necessary context in the task goal or in shared materials that every participant is allowed to access.

## Connect your apps

| Connection | Current capabilities | Setup requirements |
| --- | --- | --- |
| Gmail | Create plain-text drafts and send emails after confirmation | A Google desktop OAuth client and browser authorization; inbox reading is not supported yet |
| Notion | Search by title, read first-level page content, and create subpages | A Notion internal integration token and permission to access the relevant pages |
| Remote MCP | Discover tools and call them within tasks | A public HTTPS Streamable HTTP endpoint, with an optional bearer token; MCP OAuth and local stdio are not supported yet |
| Local artifacts | Create, read, and export Markdown | No external account needed; configure a model that supports tool calling |

Sending Gmail messages, creating Notion pages, and calling MCP tools require confirmation. When a task awaits approval, check the account and full arguments, click **Approve this operation (批准此操作)**, and then **Continue task (继续任务)**. Gmail drafts and local artifacts can be created directly.

You can change Bot permissions or disconnect a service at any time. This is a developer preview, so you currently need to supply the relevant application credentials for external services. See the [connector setup guide](docs/CONNECTORS-DELIVERY.md) for details.

## Connect your phone to your Mac

The desktop **Settings → Linked devices (设置 → 关联设备)** section now includes an iOS download entry. It shows a pending state until a TestFlight link is available. Once published, use the button or installation QR code to install, then scan the separate pairing code. See [iOS distribution](docs/IOS-DISTRIBUTION.md) for publisher setup.

Your phone and Mac access the same local workspace, while the Mac continues to execute tasks. Pairing uses **a QR code and approval on the Mac**; it does not require a shared GitHub account.

Once the product operator deploys the public connection service and distributes the iOS app, users will be able to:

1. Open **Settings → Linked devices (设置 → 关联设备)** on the Mac, enable the connection, and generate a QR code.
2. Open Soren on the phone and scan the Mac's QR code.
3. Compare the confirmation codes on both devices and approve pairing on the Mac.
4. Select a Bot on the phone to start tasks, view results and memories, or approve pending operations.

End users will not need to host a server or manually enter their Mac's IP address. The Mac must remain running, online, and awake. The relay only transports encrypted messages; it does not execute tasks or back up memory.

**Current delivery status:**

- Pairing, encrypted transport, mobile task and memory views, and iOS simulator validation are complete.
- The public relay, QR scanning acceptance tests on a physical iPhone, official signing, and mobile distribution are pending. Remote pairing is therefore not enabled in the default installer yet.
- Android, full mobile chat, editing Bots on mobile, push notifications, and offline editing have not been delivered.

Developers can validate the complete flow with a local relay. See the [connection service deployment guide](deploy/README.md) and the [mobile integration notes](docs/MOBILE-040-DELIVERY.md) for setup, permissions, and implementation details.

## Data and privacy

Soren uses SQLite and an in-process cache by default. PostgreSQL and Redis are not required. Local data is stored under `~/.soren/`:

| Data | Storage |
| --- | --- |
| Conversations, Bots, memories, tasks, and artifacts | Local SQLite database |
| Model connection settings | A private configuration file for the current system user |
| Gmail, Notion, and MCP credentials | System Keychain |
| Phone identity | iOS Keychain; the Mac's pairing identity is stored in a private file |
| Downloaded Markdown | `~/.soren/exports/` |

When you use a cloud model or an external connector, content needed to fulfill the request is sent to that service. Local storage does not mean that all computation happens offline. Do not put API keys in chats, screenshots, Git repositories, or `VITE_*` environment variables.

See [SECURITY.md](SECURITY.md) for development and contribution security rules.

## Run from source

### Prerequisites

| Dependency | Requirement |
| --- | --- |
| Node.js | 22.12 or later |
| Go | 1.26.9 or later |
| Rust | Rust toolchain and Cargo |
| macOS build tools | Xcode Command Line Tools; iOS development also requires the full Xcode installation |

The primary validated platform is **macOS Apple Silicon**. Other desktop platforms have not completed release acceptance testing.

### Start the desktop development app

```bash
git clone https://github.com/Eileenes/Soren.git
cd Soren
npm --prefix frontend ci
npm run dev
```

The development command compiles the bundled Go service and starts the Tauri window with frontend hot reload. Development and release desktop builds use the same `~/.soren` data directory by default. Set `SOREN_DATA_DIR` to keep experimental data separate.

### Run only the web interface and backend

Open two terminals at the repository root:

```bash
# Terminal 1: backend, listening on 127.0.0.1:8080 by default
npm run dev:backend
```

```bash
# Terminal 2: frontend, available at http://localhost:1420
npm run dev:web
```

Browser mode does not provide native desktop capabilities such as credential storage and Finder export. To use a real model, configure `backend/.env` using [backend/.env.example](backend/.env.example). Fill in only the key for your chosen provider, leave other provider keys empty, and start the backend. Do not run two backends that both use port 8080.

### Build the macOS installer

```bash
npm run build:mac
```

Build output:

```text
frontend/src-tauri/target/release/bundle/
├── macos/Soren.app
└── dmg/Soren_0.4.3_aarch64.dmg
```

The build command automatically compiles the backend and packages the app. See the [macOS build guide](docs/MACOS-BUILD.md) for signing and installation details.

### Develop the iOS client

Install the full Xcode application, Python 3, XcodeGen, and the appropriate iOS Rust targets. The initialization script uses these tools to generate the project.

```bash
npm --prefix mobile run ios:init
npm --prefix mobile run ios:dev
```

The iOS project is generated by scripts and is not committed to Git. Use the simulator for development validation. Installation and distribution on physical devices require Apple developer signing; access across networks also requires a reachable public connection service. See the [deployment guide](deploy/README.md) for the complete workflow.

### Run checks

```bash
# Frontend tests, type checking, and production build
npm --prefix frontend test
npm --prefix frontend run build

# Go tests and static checks
(cd backend && go test -race ./... && go vet ./...)

# Desktop Rust tests
cargo test --locked --manifest-path frontend/src-tauri/Cargo.toml
```

See the [0.4.1 review notes](docs/REVIEW-041.md) for the 0.4.1 baseline validation results, fixes, and remaining dependency warnings.

## Project structure

```text
Soren/
├── backend/
│   ├── cmd/server/       # Local Mac service
│   ├── cmd/relay/        # Independent encrypted relay
│   └── internal/         # Bots, tasks, memory, connectors, channels, and pairing
├── frontend/
│   ├── src/              # React desktop interface and mobile entry point
│   └── src-tauri/        # Rust desktop shell and bundled backend management
├── mobile/src-tauri/     # Native iOS shell, QR scanning, and secure storage
├── deploy/               # Relay deployment and reverse proxy configuration
├── assets/               # Brand logo and icons
├── docs/                 # User guides, architecture, and delivery notes
├── scripts/              # Development, build, and end-to-end validation scripts
└── release/              # Local build artifacts; installers are not committed
```

Built with **Tauri 2 · React 18 · TypeScript · Go · SQLite**.

## Frequently asked questions

**Do I need to sign in?**

Local use does not require a Soren or GitHub login. Model credentials and Gmail authorization are only used for their respective services.

**Why did my Bot not actually send an email or save an artifact?**

Start from Tasks. Check that your model supports tool calling, the current Bot has permission to use the connection, and all required approvals are complete. Verify the successful tool execution records and actual outputs.

**Does Soren keep running when I close the Mac window?**

Yes, it continues running in the menu bar. Choose **Quit Soren (退出 Soren)** from the menu bar to exit completely. Sleep or loss of network connectivity can interrupt tasks and mobile access.

**Can I scan a code and use my phone right now?**

Not with the default release build, which is not connected to a public service yet. Developers can validate the local flow using the deployment guide. Other users need to wait for the public service and mobile distribution.

**What is planned next?**

Planned work includes official mobile distribution, more complete mobile interactions, additional connectors, image and video generation, and scheduled tasks. Canva, calendar integration, and dedicated media generation workflows are not built in yet.

## Documentation

The detailed guides linked below are currently in Chinese.

- [Release workflow](docs/RELEASING.md) / [iOS distribution](docs/IOS-DISTRIBUTION.md): GitHub Releases, TestFlight, and desktop download configuration.
- [User guide](docs/USER-GUIDE.md): model setup, Bots, tasks, and app connections.
- [Connector guide](docs/CONNECTORS-DELIVERY.md): current Gmail, Notion, and MCP support.
- [Mobile integration](docs/MOBILE-040-DELIVERY.md) / [Deployment guide](deploy/README.md): pairing, permissions, and service deployment.
- [Configuration reference](docs/CONFIGURATION.md) / [Troubleshooting](docs/TROUBLESHOOTING.md): environment variables and common startup issues.
- [Multi-Bot and mobile plan](docs/MULTI-BOT-AND-MOBILE-PLAN.md) / [Architecture review](docs/ARCHITECTURE-REVIEW.md): design background and evolution.
- [Security guidelines](SECURITY.md) / [0.4.1 review notes](docs/REVIEW-041.md): credential handling, validation, and current limitations.

Historical plans and delivery notes reflect the state of their respective versions. For current capabilities, refer to this README and the latest review notes.
