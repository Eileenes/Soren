import { useCallback, useEffect, useRef, useState } from 'react';
import { chatAPI, describeError, normalizeError } from '../services/api';
import { getActiveBotID } from '../services/botScope';
import { connectionRequest } from '../services/connections';
import { getSettings } from '../services/settingsStore';
import type { Message } from '../types';

export type MessageStatus = 'sending' | 'sent' | 'failed';
export interface ChatMessage extends Message { status: MessageStatus; streaming?: boolean; memoriesUsed?: string[]; }
export function toChatMessage(message: Message, extra?: Partial<ChatMessage>): ChatMessage { return { ...message, status: 'sent', ...extra }; }
export interface UseChatOptions { onConversationCreated?: (id: string, firstMessage: string) => void; }
export interface UseChatResult {
  messages: ChatMessage[]; conversationId: string | null; loading: boolean; typing: boolean;
  loadingHistory: boolean; error: string | null; offline: boolean;
  send: (content: string) => Promise<void>; regenerate: (id?: string) => Promise<void>;
  stop: () => void; skipTyping: () => void; reset: () => void;
  loadConversation: (id: string) => Promise<void>; removeMessage: (id: string) => void;
}
const newId = (prefix: string) => prefix + '-' + crypto.randomUUID();

/** A request owns a generation and AbortController. Late responses never cross conversations. */
export function useChat({ onConversationCreated }: UseChatOptions = {}): UseChatResult {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [conversationId, setConversationId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const controller = useRef<AbortController | null>(null);
  const generation = useRef(0);
  const busy = useRef(false);
  const retry = useRef<{ content: string; conversationId: string | null; id: string; botId: string }>();
  const cancel = useCallback(() => { generation.current += 1; controller.current?.abort(); controller.current = null; busy.current = false; }, []);
  useEffect(() => () => cancel(), [cancel]);

  const send = useCallback(async (input: string) => {
    const content = input.trim();
    if (!content || busy.current) return;
    cancel();
    const request = new AbortController();
    controller.current = request;
    const version = generation.current;
    busy.current = true;
    setLoading(true); setLoadingHistory(false); setError(null);
    const userId = newId('user'); const pendingId = newId('assistant');
    const timestamp = new Date().toISOString();
    const common = { conversationId: conversationId ?? '', timestamp, createdAt: timestamp, updatedAt: timestamp };
    const retrying = retry.current?.content === content && retry.current?.conversationId === conversationId && retry.current?.botId === getActiveBotID();
    setMessages(previous => [...(retrying ? previous.filter(item => !(item.role === 'user' && item.status === 'failed' && item.content === content)) : previous),
      { ...common, id: userId, role: 'user', content, status: 'sent' },
      { ...common, id: pendingId, role: 'assistant', content: '', status: 'sending' },
    ]);
    try {
      const settings = getSettings().llm;
      if (!retrying || !retry.current) retry.current = { content, conversationId, id: crypto.randomUUID(), botId: getActiveBotID() };
      const response = await chatAPI.sendMessage({ requestId: retry.current.id, conversationId: conversationId ?? undefined, message: content, config: { model: settings.model || undefined, temperature: settings.temperature, max_tokens: settings.maxTokens } }, request.signal);
      if (request.signal.aborted || generation.current !== version) return;
      retry.current = undefined;
      setConversationId(response.conversationId); setOffline(false);
      setMessages(previous => previous.map(item => item.id === pendingId ? toChatMessage(response.message, { memoriesUsed: response.memoriesUsed }) : item));
      if (!conversationId) onConversationCreated?.(response.conversationId, content);
    } catch (err) {
      if (request.signal.aborted || generation.current !== version) return;
      setOffline(normalizeError(err).isOffline); setError(describeError(err));
      setMessages(previous => previous.filter(item => item.id !== pendingId).map(item => item.id === userId ? { ...item, status: 'failed' } : item));
    } finally {
      if (generation.current === version) { busy.current = false; setLoading(false); }
    }
  }, [cancel, conversationId, onConversationCreated]);

  const stop = useCallback(() => {
    const pending = retry.current;
    if (busy.current && pending) void connectionRequest('POST', `/chat/requests/${pending.id}/cancel`, {}, pending.botId).catch(e => setError(describeError(e)));
    retry.current = undefined;
    cancel(); setLoading(false); setLoadingHistory(false);
    setMessages(previous => previous.filter(item => item.status !== 'sending'));
  }, [cancel]);
  const reset = useCallback(() => { retry.current = undefined; cancel(); setMessages([]); setConversationId(null); setLoading(false); setLoadingHistory(false); setError(null); }, [cancel]);
  const loadConversation = useCallback(async (id: string) => {
    retry.current = undefined; cancel(); const request = new AbortController(); controller.current = request;
    const version = generation.current; busy.current = true;
    setLoading(false); setLoadingHistory(true); setError(null); setMessages([]); setConversationId(id);
    try {
      const conversation = await chatAPI.getConversation(id, request.signal);
      if (request.signal.aborted || generation.current !== version) return;
      setMessages((conversation.messages ?? []).map(message => toChatMessage(message))); setOffline(false);
    } catch (err) {
      if (request.signal.aborted || generation.current !== version) return;
      setOffline(normalizeError(err).isOffline); setError(describeError(err));
    } finally { if (generation.current === version) { busy.current = false; setLoadingHistory(false); } }
  }, [cancel]);
  const removeMessage = useCallback((id: string) => setMessages(previous => previous.filter(item => item.id !== id)), []);
  // The backend has no regenerate endpoint. Do not fabricate replacement semantics.
  const regenerate = useCallback(async () => { setError('当前版本不支持替换已保存的回复，请发送补充要求。'); }, []);
  return { messages, conversationId, loading, typing: false, loadingHistory, error, offline, send, regenerate, stop, skipTyping: () => {}, reset, loadConversation, removeMessage };
}
