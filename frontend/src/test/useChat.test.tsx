import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
const api = vi.hoisted(() => ({ send: vi.fn(), history: vi.fn() }));
vi.mock('../services/api', () => ({ chatAPI: { sendMessage: api.send, getConversation: api.history }, describeError: (e: Error) => e.message, normalizeError: () => ({ isOffline: false }) }));
import { useChat } from '../hooks/useChat';
const message = { id: 'reply', conversationId: 'old', role: 'assistant' as const, content: 'late reply', timestamp: '', createdAt: '', updatedAt: '' };
beforeEach(() => vi.clearAllMocks());
describe('请求与会话隔离', () => {
  it('重置时中止旧请求，旧结果不能覆盖新对话', async () => {
    let resolve!: (value: unknown) => void;
    api.send.mockImplementation(() => new Promise(r => { resolve = r; }));
    const { result } = renderHook(() => useChat());
    let completion!: Promise<void>;
    act(() => { completion = result.current.send('old request'); });
    const signal = api.send.mock.calls[0][1] as AbortSignal;
    act(() => result.current.reset());
    expect(signal.aborted).toBe(true);
    await act(async () => { resolve({ conversationId: 'old', message }); await completion; });
    expect(result.current.messages).toEqual([]);
    expect(result.current.conversationId).toBeNull();
    expect(result.current.loading).toBe(false);
  });
  it('快速切换历史时，只保留最后选择的会话', async () => {
    let resolveFirst!: (value: unknown) => void;
    api.history.mockImplementationOnce(() => new Promise(r => { resolveFirst = r; })).mockResolvedValueOnce({ id: 'new', messages: [{ ...message, id: 'new-message', content: 'new content' }] });
    const { result } = renderHook(() => useChat());
    let first!: Promise<void>;
    act(() => { first = result.current.loadConversation('old'); });
    await act(async () => { await result.current.loadConversation('new'); });
    await act(async () => { resolveFirst({ id: 'old', messages: [message] }); await first; });
    expect(result.current.conversationId).toBe('new');
    expect(result.current.messages[0].content).toBe('new content');
  });
  it('停止后发送新消息，旧请求的 finally 不会解锁新请求', async () => {
    let resolveOld!: (value: unknown) => void;
    let resolveNew!: (value: unknown) => void;
    api.send.mockImplementationOnce(() => new Promise(r => { resolveOld = r; })).mockImplementationOnce(() => new Promise(r => { resolveNew = r; }));
    const { result } = renderHook(() => useChat());
    let old!: Promise<void>; let next!: Promise<void>;
    act(() => { old = result.current.send('first'); });
    act(() => result.current.stop());
    act(() => { next = result.current.send('second'); });
    await act(async () => { resolveOld({ conversationId: 'old', message }); await old; });
    expect(result.current.loading).toBe(true);
    await act(async () => { resolveNew({ conversationId: 'new', message: { ...message, content: 'new reply' } }); await next; });
    expect(result.current.loading).toBe(false);
    expect(result.current.messages.filter(item => item.role === 'assistant').map(item => item.content)).toEqual(['new reply']);
  });
});
