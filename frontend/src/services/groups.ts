/**
 * 群聊 API 客户端
 *
 * 路由说明：群接口注册在后端根路由上而不是 /api 分组，因为 /api 分组挂了
 * BotContext 中间件会强制解析 X-Bot-ID，而群聊的身份来自成员而不是单个 Bot。
 * 路径仍是 /api/groups，所以直接复用 api.ts 的共享实例即可；请求拦截器自动
 * 附加的 X-Bot-ID 在这些路由上被后端忽略。
 */

import { http, toBackendOptions } from './api';
import type {
  GroupCreateInput,
  GroupDetailView,
  GroupUpdateInput,
  GroupView,
  SpeakResult,
} from '../types';

export const groupsAPI = {
  /** GET /api/groups */
  async list(signal?: AbortSignal): Promise<GroupView[]> {
    const { data } = await http.get<GroupView[]>('/api/groups', { signal, timeout: 15000 });
    return data;
  },

  /** POST /api/groups */
  async create(input: GroupCreateInput): Promise<GroupView> {
    const { data } = await http.post<GroupView>('/api/groups', input, { timeout: 15000 });
    return data;
  },

  /** GET /api/groups/:id —— 群历史的唯一入口 */
  async detail(id: string, signal?: AbortSignal): Promise<GroupDetailView> {
    const { data } = await http.get<GroupDetailView>(`/api/groups/${id}`, { signal, timeout: 15000 });
    return data;
  },

  /**
   * PATCH /api/groups/:id
   *
   * 入参字段全部可选：后端是指针类型，未传的字段保持原值，
   * 因此不要发全量覆盖的请求体。
   */
  async update(id: string, input: GroupUpdateInput): Promise<GroupView> {
    const { data } = await http.patch<GroupView>(`/api/groups/${id}`, input, { timeout: 15000 });
    return data;
  },

  /** DELETE /api/groups/:id */
  async remove(id: string): Promise<{ message: string }> {
    const { data } = await http.delete<{ message: string }>(`/api/groups/${id}`, { timeout: 15000 });
    return data;
  },

  /**
   * POST /api/groups/:id/messages
   *
   * 一次调用是阻塞的：自主模式会在服务端跑完所有轮次才返回，
   * 所以返回的 messages 是本次落库的整段（含用户自己那条），前端整段替换。
   * 自主模式最长 30 次模型调用，超时给足。
   */
  async speak(
    id: string,
    input: {
      content: string;
      mentions: string[];
      mode: string;
      requestId: string;
      config?: Parameters<typeof toBackendOptions>[0];
    },
    signal?: AbortSignal,
  ): Promise<SpeakResult> {
    const { data } = await http.post<SpeakResult>(
      `/api/groups/${id}/messages`,
      { ...input, config: toBackendOptions(input.config) },
      { signal, timeout: 10 * 60 * 1000 },
    );
    return data;
  },

  /** POST /api/groups/:id/cancel */
  async cancel(id: string): Promise<{ status: string }> {
    const { data } = await http.post<{ status: string }>(`/api/groups/${id}/cancel`, {}, { timeout: 15000 });
    return data;
  },
};