import { describe, expect, it, vi } from 'vitest';
import type { AxiosRequestConfig } from 'axios';
const captured = vi.hoisted(() => [] as { bot: string; url?: string }[]);
vi.mock('axios', async importOriginal => {
  const actual = await importOriginal<typeof import('axios')>();
  return { ...actual, default: { ...actual.default, create: (options: AxiosRequestConfig) => actual.default.create({ ...options, adapter: async config => {
    captured.push({ bot: String(config.headers.get('X-Bot-ID')), url: config.url });
    return { data: [], status: 200, statusText: 'OK', headers: {}, config };
  } }) } };
});
import { chatAPI } from '../services/api';
import { setActiveBotID } from '../services/botScope';

describe('聊天请求的 Bot 隔离', () => {
  it('侧栏预览使用指定 Bot，普通聊天请求继续跟随当前 Bot', async () => {
    setActiveBotID('soren');
    await chatAPI.getConversations('writer');
    await chatAPI.getConversation('writer-chat', undefined, 'writer');
    await chatAPI.getConversations();
    expect(captured.map(request => request.bot)).toEqual(['writer', 'writer', 'soren']);
  });
});
