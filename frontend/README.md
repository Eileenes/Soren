# Soren Frontend

Soren 桌面端 AI 助手前端：Tauri 2 + React 18 + TypeScript + Tailwind CSS。

## 常用命令

```bash
npm install          # 安装依赖
npm run dev          # 开发模式（Web，端口 1420）
npm run tauri:dev    # 开发模式（Tauri 桌面窗口）
npm run typecheck    # 类型检查（tsc --noEmit）
npm test             # 运行组件测试（vitest run）
npm run build        # 生产构建（tsc + vite build）
npm run tauri:build  # 打包桌面应用
```

## 目录结构

```
src/
├── components/
│   ├── ui/            # 基础组件：按钮、表单、模态框、Toast、卡片
│   ├── layout/        # 应用外壳：TitleBar、Sidebar、PageContainer
│   ├── chat/          # 聊天：消息气泡、Markdown、输入框、空状态
│   ├── memories/      # 记忆卡片
│   ├── Logo.tsx       # 品牌标识（Logomark / Logo / LogoHero）
│   └── icons.tsx      # 手写图标集（不依赖图标库）
├── pages/             # ChatPage / MemoriesPage / ChannelsPage / SettingsPage
├── hooks/             # 数据与交互 Hooks
├── services/          # api.ts（HTTP 客户端）、settingsStore.ts（内存设置）
├── styles/index.css   # 主题变量、滚动条、Markdown 排版
├── test/              # Vitest + Testing Library 测试
├── types/index.ts     # 与后端 JSON 对齐的类型定义
└── utils/format.ts    # 时间、数字、重要度等展示格式化
```

## 设计系统落地方式

- **主题变量**：`src/styles/index.css` 中定义 `--soren-surface-*` / `--soren-ink-*` / `--soren-line-*`
  三组 RGB 变量，`.dark` 类切换深色（默认）与浅色。Tailwind 的 `surface-*`、`ink-*`、
  `line-*` 颜色直接映射到这些变量，因此同一套类名在两种主题下都成立。
- **兼容层**：`bg-primary`、`text-primary`、`border-light` 等设计文档原始 token 名称保留在
  `tailwind.config.js` 中，同样指向主题变量。
- **品牌元素**：`bg-gradient-primary`（#667eea → #764ba2）、`bg-gradient-accent`
  （#f093fb → #f5576c）、`bg-aurora`（页面底层光晕）、`glass-panel`（玻璃拟态）。
- **动效**：`animate-caret`（打字光标）、`animate-bounce-dot`（思考中）、`animate-shimmer`
  （骨架屏）、`animate-glow-pulse`、`animate-float`、`animate-spin-slow` 等。
- **注意**：Tailwind 默认透明度刻度只支持 5 的倍数（`/10`、`/15`、`/20`…）。
  写 `/8`、`/12`、`/35` 这类值不会报错但会被静默丢弃，样式完全不生效。

## 与后端对接的约定

- 类型定义严格对齐后端 JSON 字段（`contentType`、`importanceScore`、`memoriesUsed` 等），
  不做驼峰转换，避免联调时出现静默 `undefined`。
- `/api/chat` 是一次性返回（无 SSE 端点），**打字机效果由前端逐字揭示实现**，
  揭示速度按文本长度自适应，长回复也能在约 1.2 秒内播完。
- 后端未启动时，`services/api.ts` 会把网络异常归一化为带中文提示的 `ApiError`，
  界面展示可操作的重试条，不会白屏或往 console 抛错。

## 已知限制

- 设置项保存在内存中（约束要求不使用 localStorage），重启后恢复默认；
  接入 Tauri Store 时只需替换 `services/settingsStore.ts` 的两个持久化钩子。
- 渠道凭据后端只从环境变量读取，前端表单用于生成 `.env` 配置片段，不做写入。
- LLM 的 API Key / Base URL 同属后端启动期配置，前端仅生成配置片段。
