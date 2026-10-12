/**
 * Soren 后端 API 客户端
 *
 * 设计要点：
 * 1. 所有错误统一归一化为 ApiError，包含面向用户的中文提示，
 *    后端未启动时给出"请先启动后端"的可操作提示，而不是让调用方拿到 axios 的原始异常。
 * 2. 返回值全部带具体类型，不使用 any。
 * 3. 请求头统一带 X-User-ID（后端 resolveUserID 会读取，缺省为 default-user）。
 */

import { getActiveBotID } from './botScope';
import axios, { AxiosError, type AxiosInstance, type AxiosRequestConfig } from 'axios';
import type {
  ChannelListResponse,
  ChannelStatus,
  ChatRequest,
  ChatResponse,
  Conversation,
  HealthStatus,
  MemoryEntry,
  MemoryListParams,
  MemoryListResponse,
  MemorySearchResponse,
  MemoryStats,
} from '../types';

/** 默认后端地址，可通过 .env 的 VITE_API_BASE_URL 覆盖 */
export const DEFAULT_API_BASE_URL =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? 'http://localhost:8080';

/** 桌面端单用户场景的默认用户标识 */
export const DEFAULT_USER_ID =
  (import.meta.env.VITE_USER_ID as string | undefined) ?? 'default-user';

/** 归一化后的 API 错误 */
export class ApiError extends Error {
  readonly status: number;
  readonly detail?: string;
  readonly isOffline: boolean;

  constructor(message: string, status: number, detail?: string, isOffline = false) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.detail = detail;
    this.isOffline = isOffline;
  }

  /** 类型守卫：判断任意异常是否为 ApiError */
  static isApiError(value: unknown): value is ApiError {
    return value instanceof ApiError;
  }
}

/** 后端 /api/health 可达但服务未就绪时也可能是 5xx，统一按离线友好提示处理 */
function isNetworkFailure(error: AxiosError): boolean {
  // 无 response 说明请求没到达服务端：后端未启动、端口不通、DNS 失败或超时
  if (!error.response) return true;
  const status = error.response.status;
  return status === 502 || status === 503 || status === 504;
}

/** 从后端错误响应体中提取 error 字段 */
function extractDetail(error: AxiosError): string | undefined {
  const data = error.response?.data;
  if (data && typeof data === 'object' && 'error' in data) {
    const raw = (data as { error?: unknown }).error;
    if (typeof raw === 'string') return raw;
  }
  if (typeof data === 'string' && data.trim() !== '') return data;
  return undefined;
}

/** 将任意异常转换为 ApiError */
export function normalizeError(error: unknown): ApiError {
  if (ApiError.isApiError(error)) return error;

  if (axios.isAxiosError(error)) {
    const axiosError = error as AxiosError;
    const detail = extractDetail(axiosError);

    if (isNetworkFailure(axiosError)) {
      return new ApiError(
        `无法连接到 Soren 后端服务（${DEFAULT_API_BASE_URL}）。请确认服务已启动后重试。`,
        0,
        detail ?? axiosError.message,
        true,
      );
    }

    const status = axiosError.response?.status ?? 0;
    const fallbackByStatus: Record<number, string> = {
      400: '请求参数有误，请检查输入内容。',
      404: '请求的资源不存在，可能已被删除。',
      500: '后端服务内部错误，请稍后重试。',
    };

    return new ApiError(
      fallbackByStatus[status] ?? `请求失败（HTTP ${status}）`,
      status,
      detail,
      false,
    );
  }

  if (error instanceof Error) {
    return new ApiError(error.message || '发生未知错误', 0, error.message, false);
  }

  return new ApiError('发生未知错误，请重试。', 0, String(error), false);
}

/** 生成可读的错误文案：优先后端 detail，便于排查 */
export function describeError(error: unknown): string {
  const normalized = normalizeError(error);
  if (normalized.detail && normalized.status !== 0) {
    return `${normalized.message}（${normalized.detail}）`;
  }
  return normalized.message;
}

// ────────────────────────────────────────────────────────────
// axios 实例
// ────────────────────────────────────────────────────────────

let currentBaseUrl = DEFAULT_API_BASE_URL;

const http: AxiosInstance = axios.create({
  baseURL: DEFAULT_API_BASE_URL,
  timeout: 60000, // LLM 生成较慢，给足超时
  headers: { 'Content-Type': 'application/json' },
});

http.interceptors.request.use((config) => {
  config.headers.set?.('X-Bot-ID', getActiveBotID());
  return config;
}, undefined, { synchronous: true });

http.interceptors.response.use(
  (response) => response,
  (error: unknown) => Promise.reject(normalizeError(error)),
);

/** 更新运行时 baseURL（设置页修改后端地址后调用） */
export function setApiBaseUrl(baseUrl: string): void {
  const trimmed = baseUrl.trim().replace(/\/+$/, '');
  currentBaseUrl = trimmed === '' ? DEFAULT_API_BASE_URL : trimmed;
  http.defaults.baseURL = currentBaseUrl;
}

/** 读取当前生效的 baseURL */
export function getApiBaseUrl(): string {
  return currentBaseUrl;
}

/** 将前端的 LLMOptions（驼峰）转换为后端 llm.Options（下划线） */
function toBackendOptions(
  config: ChatRequest['config'],
): Record<string, unknown> | undefined {
  if (!config) return undefined;
  const out: Record<string, unknown> = {};
  if (config.model) out.model = config.model;
  if (typeof config.temperature === 'number') out.temperature = config.temperature;
  if (typeof config.max_tokens === 'number') out.max_tokens = config.max_tokens;
  return Object.keys(out).length > 0 ? out : undefined;
}

// ────────────────────────────────────────────────────────────
// 健康检查
// ────────────────────────────────────────────────────────────

export const healthAPI = {
  /** GET /api/health */
  async check(signal?: AbortSignal): Promise<HealthStatus> {
    const { data } = await http.get<HealthStatus>('/api/health', { signal, timeout: 5000 });
    return data;
  },
};

// ────────────────────────────────────────────────────────────
// 聊天
// ────────────────────────────────────────────────────────────

export const chatAPI = {
  /** POST /api/chat */
  async sendMessage(request: ChatRequest, signal?: AbortSignal): Promise<ChatResponse> {
    const { data } = await http.post<ChatResponse>('/api/chat', {
      conversationId: request.conversationId,
      message: request.message,
      config: toBackendOptions(request.config),
    }, { signal });
    return data;
  },

  /** GET /api/conversations */
  async getConversations(): Promise<Conversation[]> {
    const { data } = await http.get<Conversation[]>('/api/conversations');
    return data;
  },

  /** GET /api/conversations/:id */
  async getConversation(id: string, signal?: AbortSignal): Promise<Conversation> {
    const { data } = await http.get<Conversation>(`/api/conversations/${id}`, { signal });
    return data;
  },

  /** DELETE /api/conversations/:id */
  async deleteConversation(id: string): Promise<void> {
    await http.delete(`/api/conversations/${id}`);
  },
};

// ────────────────────────────────────────────────────────────
// 记忆管理
// ────────────────────────────────────────────────────────────

export const memoryAPI = {
  /** GET /api/memories */
  async list(params: MemoryListParams = {}): Promise<MemoryListResponse> {
    const { data } = await http.get<MemoryListResponse>('/api/memories', {
      params: {
        page: params.page,
        pageSize: params.pageSize,
        type: params.type,
        sessionId: params.sessionId,
        keyword: params.keyword,
        minImportance: params.minImportance,
      },
    });
    return data;
  },

  /** GET /api/memories/search */
  async search(query: string, limit = 20): Promise<MemorySearchResponse> {
    const { data } = await http.get<MemorySearchResponse>('/api/memories/search', {
      params: { q: query, limit },
    });
    return data;
  },

  /** GET /api/memories/stats */
  async stats(): Promise<MemoryStats> {
    const { data } = await http.get<MemoryStats>('/api/memories/stats');
    return data;
  },

  /** DELETE /api/memories/:id */
  async remove(id: number): Promise<{ message: string; id: number }> {
    const { data } = await http.delete<{ message: string; id: number }>(
      `/api/memories/${id}`,
    );
    return data;
  },
};

// ────────────────────────────────────────────────────────────
// 消息渠道
// ────────────────────────────────────────────────────────────

export const channelAPI = {
  /** GET /api/channels */
  async list(): Promise<ChannelStatus[]> {
    const { data } = await http.get<ChannelListResponse>('/api/channels');
    return data.channels ?? [];
  },

  /** GET /api/channels/:name */
  async get(name: string): Promise<ChannelStatus> {
    const { data } = await http.get<ChannelStatus>(`/api/channels/${name}`);
    return data;
  },

  /** POST /api/channels/:name/enable */
  async enable(name: string): Promise<ChannelStatus> {
    const { data } = await http.post<ChannelStatus>(`/api/channels/${name}/enable`);
    return data;
  },

  /** POST /api/channels/:name/disable */
  async disable(name: string): Promise<ChannelStatus> {
    const { data } = await http.post<ChannelStatus>(`/api/channels/${name}/disable`);
    return data;
  },
};

/** 供调试使用：直接发起一次 GET 探测后端可达性 */
export async function pingBackend(
  baseUrl?: string,
  config?: AxiosRequestConfig,
): Promise<boolean> {
  try {
    await http.get('/api/health', { ...config, timeout: 4000, baseURL: baseUrl });
    return true;
  } catch {
    return false;
  }
}

export type { MemoryEntry };
