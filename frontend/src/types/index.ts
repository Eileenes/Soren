/**
 * Soren 共享类型定义
 *
 * 字段名严格对齐后端 JSON 输出（见 backend/internal/api 与 internal/memory/models.go），
 * 不做驼峰/下划线转换，避免联调时出现"静默 undefined"。
 */

// ────────────────────────────────────────────────────────────
// 对话与消息
// ────────────────────────────────────────────────────────────

/** 消息角色 */
export type MessageRole = 'user' | 'assistant' | 'system';

/**
 * 单条消息
 *
 * 后端返回的时间字段是 RFC3339 字符串，读取端需自行 new Date() 转换，
 * 因此这里保留 string 类型，不假装是 Date。
 */
export interface Message {
  toolRunId?: string;
  id: string;
  conversationId: string;
  role: MessageRole;
  content: string;
  /** 后端 models.Message 的 json 字段名 */
  timestamp: string;
  createdAt: string;
  updatedAt: string;
  /** 群聊发言 Bot；单聊与用户消息为空。后端 omitempty */
  speakerBotId?: string;
  /** 发言人名称冗余存储，Bot 改名或删除后仍可正确显示 */
  speakerName?: string;
}

/** 对话 */
export interface Conversation {
  id: string;
  title: string;
  messages: Message[];
  createdAt: string;
  updatedAt: string;
}

// ────────────────────────────────────────────────────────────
// 聊天接口
// ────────────────────────────────────────────────────────────

/** LLM 调用参数（对应后端 llm.Options） */
export interface LLMOptions {
  model?: string;
  temperature?: number;
  /** 后端 json tag 为 max_tokens */
  max_tokens?: number;
}

/** POST /api/chat 请求体 */
export interface ChatRequest {
  requestId?: string;
  conversationId?: string;
  message: string;
  config?: LLMOptions;
}

/** POST /api/chat 响应体 */
export interface ChatResponse {
  conversationId: string;
  message: Message;
  /** 本轮注入 system prompt 的记忆摘要，用于展示"AI 想起了什么" */
  memoriesUsed?: string[];
}

// ────────────────────────────────────────────────────────────
// 群聊
// ────────────────────────────────────────────────────────────

/** 发言模式：mention=你 @ 谁谁回答；auto=群内 Bot 依次自主发言 */
export type GroupMode = 'mention' | 'auto';

/** 群成员展示信息 */
export interface GroupMemberView {
  botId: string;
  name: string;
  avatar: string;
  status: string;
}

/** GET /api/groups、POST /api/groups、PATCH /api/groups/:id 的响应体 */
export interface GroupView {
  id: string;
  title: string;
  topic: string;
  mode: GroupMode;
  maxRounds: number;
  lastMessageAt?: string;
  /** 后端已含「发言人名：」前缀并截断，前端只做单行省略 */
  lastMessagePreview?: string;
  members: GroupMemberView[];
  createdAt: string;
  updatedAt: string;
  /**
   * 是否置顶
   *
   * GET /api/groups 的顺序已经是「置顶在前（pinnedAt 倒序）、未置顶在后（updatedAt 倒序）」，
   * 前端按返回顺序渲染，不再自己排一次。
   */
  pinned: boolean;
  /** 置顶时间，未置顶时为空；由后端维护，前端不传 */
  pinnedAt?: string | null;
}

/**
 * GET /api/groups/:id 的响应体
 *
 * 群历史只有这一个入口：群对话挂在 BotID = "group:<id>" 上，
 * 而 /api/conversations 走 X-Bot-ID 中间件解析不出真实 Bot，会 404。
 */
export interface GroupDetailView extends GroupView {
  conversationId?: string;
  messages: Message[];
  latestTurn?: { requestId: string; status: GroupSpeakStatus; error?: string; mode: GroupMode };
}

/** 一次群发言的结果状态 */
export type GroupSpeakStatus = 'done' | 'cancelled' | 'failed' | 'running' | 'interrupted';

/**
 * POST /api/groups/:id/messages 的响应体
 *
 * messages 是本次发言落库的整段（含用户自己那条），不是群全部历史，
 * 前端替换乐观消息并按服务端 ID 合并，不做内容级去重。
 */
export interface SpeakResult {
  mode?: GroupMode;
  conversationId: string;
  status: GroupSpeakStatus;
  messages: Message[];
  /** 说明本轮哪些 Bot 未发言 */
  error: string;
}

/** POST /api/groups 请求体 */
export interface GroupCreateInput {
  title: string;
  topic: string;
  botIds: string[];
  mode: GroupMode;
  maxRounds: number;
}

/** PATCH /api/groups/:id 请求体：字段可选，未传的保持原值 */
export interface GroupUpdateInput {
  title?: string;
  topic?: string;
  botIds?: string[];
  mode?: GroupMode;
  maxRounds?: number;
  /** 置顶开关。pinnedAt 由后端维护，不要传 */
  pinned?: boolean;
}

/** POST /api/groups/:id/messages 请求体 */
export interface GroupSpeakInput {
  content: string;
  mentions: string[];
  mode?: GroupMode;
  /** 幂等键，必填，同键重复提交返回同一批消息 */
  requestId: string;
  config?: LLMOptions;
}

// ────────────────────────────────────────────────────────────
// 记忆系统
// ────────────────────────────────────────────────────────────

/** 记忆类型 */
export type MemoryType = 'conversation' | 'preference' | 'task' | 'fact';

/** 全部记忆类型，用于筛选器与分组渲染 */
export const MEMORY_TYPES: readonly MemoryType[] = [
  'conversation',
  'preference',
  'task',
  'fact',
] as const;

/** 记忆类型的中文展示名 */
export const MEMORY_TYPE_LABELS: Record<MemoryType, string> = {
  conversation: '对话',
  preference: '偏好',
  task: '任务',
  fact: '事实',
};

/** 标签类型 */
export type TagType = 'preference' | 'topic' | 'entity' | 'skill';

/** 记忆标签 */
export interface MemoryTag {
  id: number;
  name: string;
  tagType: TagType;
}

/** 一条长期记忆 */
export interface MemoryEntry {
  id: number;
  userId: string;
  sessionId: string;
  contentType: MemoryType;
  content: string;
  summary: string;
  /** 0-100 */
  importanceScore: number;
  accessCount: number;
  /** 可能为 null（后端未 omitempty） */
  lastAccessedAt: string | null;
  createdAt: string;
  updatedAt: string;
  /** 后端 omitempty，未设置时字段缺失 */
  expiresAt?: string | null;
  parentMemoryId?: number | null;
  conversationTurn: number;
  isExplicitlyStated: boolean;
  /** 后端 omitempty，无标签时字段缺失 */
  tags?: MemoryTag[];
}

/** GET /api/memories 查询参数 */
export interface MemoryListParams {
  page?: number;
  pageSize?: number;
  type?: MemoryType;
  sessionId?: string;
  keyword?: string;
  minImportance?: number;
}

/** GET /api/memories 响应体 */
export interface MemoryListResponse {
  items: MemoryEntry[];
  total: number;
  page: number;
  pageSize: number;
  /** 记忆系统总开关状态 */
  enabled: boolean;
}

/** GET /api/memories/search 响应体 */
export interface MemorySearchResponse {
  items: MemoryEntry[];
  query: string;
  total: number;
  /** 记忆系统被禁用时后端会返回该字段 */
  enabled?: boolean;
}

/** GET /api/memories/stats 响应体 */
export interface MemoryStats {
  enabled: boolean;
  total: number;
  /** 各类型条数，键为 MemoryType */
  byType: Record<string, number>;
  highImportance: number;
  accessedRecently: number;
  totalTags: number;
  oldestCreatedAt?: string | null;
  newestCreatedAt?: string | null;
}

// ────────────────────────────────────────────────────────────
// 消息渠道
// ────────────────────────────────────────────────────────────

/** 渠道标识 */
export type ChannelName = 'feishu' | 'dingtalk';

/** 渠道接入模式：ws=长连接 / webhook=回调 */
export type ChannelMode = 'ws' | 'webhook';

/** 渠道运行状态（对应后端 channel.Status） */
export interface ChannelStatus {
  name: string;
  type: string;
  enabled: boolean;
  running: boolean;
  mode: string;
  configured: boolean;
  description: string;
  /** omitempty，无错误时缺失 */
  lastError?: string;
  /** omitempty，未启动时为 Go 零值时间（会被序列化） */
  startedAt?: string;
}

/** GET /api/channels 响应体 */
export interface ChannelListResponse {
  channels: ChannelStatus[];
}

/**
 * 渠道配置字段定义
 *
 * 字段名与 backend/internal/config/config.go 中的 FeishuConfig / DingTalkConfig 一一对应。
 * 注意：后端目前只支持通过环境变量配置渠道，没有写入配置的接口，
 * 因此表单仅做本地校验与展示，保存行为需后端补充 PUT /api/channels/:name 后接入。
 */
export interface FeishuChannelConfig {
  appId: string;
  appSecret: string;
  verificationToken: string;
  encryptKey: string;
  mode: ChannelMode;
  baseUrl: string;
  webhookPort: number;
}

export interface DingTalkChannelConfig {
  clientId: string;
  clientSecret: string;
  robotCode: string;
  mode: ChannelMode;
  webhookSecret: string;
  webhookPort: number;
}

/** 渠道表单字段描述，供通用表单渲染 */
export interface ChannelFieldSpec {
  key: string;
  label: string;
  /** 输入类型 */
  type: 'text' | 'password' | 'number' | 'select';
  placeholder?: string;
  helper?: string;
  /** type=select 时的选项 */
  options?: Array<{ value: string; label: string }>;
  required?: boolean;
  /** 环境变量名提示，便于用户直接改 .env */
  envKey: string;
}

// ────────────────────────────────────────────────────────────
// 健康检查与设置
// ────────────────────────────────────────────────────────────

/** GET /api/health 响应体 */
export interface HealthStatus {
  model?: string;
  llmProvider?: string;
  demoMode?: boolean;
  status: string;
  storageDriver?: string;
  cacheDriver?: string;
  memoryEnabled?: boolean;
  cacheHealthy?: boolean;
}

/** 前端侧连接状态机 */
export type ConnectionState = 'unknown' | 'connecting' | 'online' | 'offline';

/** LLM 服务商 */
export type LLMProviderId = 'openai' | 'anthropic' | 'custom';

/**
 * 前端 LLM 配置
 *
 * 后端 /api/chat 的 config 只接受 model / temperature / max_tokens，
 * provider、apiKey、baseURL 属于后端启动期配置（环境变量），
 * 前端保存后通过 /api/chat 的 config 字段生效的只有 model/temperature/maxTokens。
 */
export interface LLMSettings {
  provider: LLMProviderId;
  apiKey: string;
  baseUrl: string;
  model: string;
  temperature: number;
  maxTokens: number;
}

/** 主题 */
export type ThemeMode = 'dark' | 'light' | 'system';

/** 界面语言 */
export type LanguageCode = 'zh-CN' | 'en-US';

/** 通用设置 */
export interface GeneralSettings {
  theme: ThemeMode;
  language: LanguageCode;
  /** 是否在侧栏展示记忆入口的统计角标 */
  showMemoryBadge: boolean;
}

/** 应用全部设置 */
export interface AppSettings {
  llm: LLMSettings;
  general: GeneralSettings;
}

// ────────────────────────────────────────────────────────────
// 导航
// ────────────────────────────────────────────────────────────

/** 顶层视图标识（不引入 react-router，用 state 切换） */
export type ViewId = 'chat' | 'memories' | 'channels' | 'settings' | 'bots' | 'tasks' | 'shared' | 'history';

/**
 * 侧栏当前选中对象
 *
 * 群聊与单聊是两种不同的页面实体，各自持有独立状态，
 * 因此 AppContent 的挂载 key 用 type + id，切走再切回不互相污染。
 */
export type Selection =
  | { type: 'bot'; id: string }
  | { type: 'group'; id: string };

// ────────────────────────────────────────────────────────────
// 统一错误
// ────────────────────────────────────────────────────────────

/** 归一化后的 API 错误 */
export interface ApiErrorShape {
  /** HTTP 状态码，网络层失败时为 0 */
  status: number;
  /** 面向用户的中文提示 */
  message: string;
  /** 后端原始 error 字段 */
  detail?: string;
  /** 是否为"后端未启动/不可达" */
  isOffline: boolean;
}
