/**
 * 对话列表 Hook
 *
 * 负责对话列表的加载、删除与本地增量更新（发送消息后同步标题和时间），
 * 与具体的消息流状态（useChat）解耦。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { chatAPI, describeError } from '../services/api';
import type { Conversation } from '../types';

export interface UseConversationsResult {
  conversations: Conversation[];
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
  remove: (id: string) => Promise<void>;
  /** 本地更新某条对话（如新消息后刷新标题与时间） */
  patchLocal: (id: string, patch: Partial<Conversation>) => void;
  /** 新对话创建后插入列表头部 */
  prependLocal: (conversation: Conversation) => void;
}

export function useConversations(): UseConversationsResult {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const data = await chatAPI.getConversations();
      if (!mountedRef.current) return;
      // 按更新时间倒序，最近使用的排在前面
      const sorted = [...data].sort(
        (a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime(),
      );
      setConversations(sorted);
      setError(null);
    } catch (err) {
      if (!mountedRef.current) return;
      setError(describeError(err));
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  const remove = useCallback(async (id: string) => {
    await chatAPI.deleteConversation(id);
    setConversations((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const patchLocal = useCallback((id: string, patch: Partial<Conversation>) => {
    setConversations((prev) =>
      prev.map((item) => (item.id === id ? { ...item, ...patch } : item)),
    );
  }, []);

  const prependLocal = useCallback((conversation: Conversation) => {
    setConversations((prev) => {
      if (prev.some((item) => item.id === conversation.id)) return prev;
      return [conversation, ...prev];
    });
  }, []);

  return { conversations, loading, error, reload, remove, patchLocal, prependLocal };
}
