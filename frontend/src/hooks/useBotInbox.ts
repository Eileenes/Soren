import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { Bot } from '../services/agents';
import { chatAPI } from '../services/api';
import type { Conversation } from '../types';

export interface BotInbox { conversations: Conversation[]; preview: string; latestId?: string; failed?: boolean; }

/** Explicit Bot scopes keep previews separate from whichever Bot is currently selected. */
export function useBotInbox(bots: Bot[]) {
  const idsKey = JSON.stringify(bots.filter(bot => bot.status !== 'archived').map(bot => bot.id).sort());
  const ids = useMemo<string[]>(() => JSON.parse(idsKey), [idsKey]);
  const [inbox, setInbox] = useState<Record<string, BotInbox>>({});
  const cache = useRef<Record<string, { revision: string; preview: string }>>({});
  const generation = useRef(0);
  const reload = useCallback(async () => {
    const version = ++generation.current;
    const rows = await Promise.all(ids.map(async id => {
      try {
        const conversations = (await chatAPI.getConversations(id)).sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt));
        const latest = conversations[0];
        let preview = latest?.title ?? '';
        if (latest) {
          const revision = `${latest.id}:${latest.updatedAt}`;
          if (cache.current[id]?.revision === revision) preview = cache.current[id].preview;
          else {
            try {
              const detail = await chatAPI.getConversation(latest.id, undefined, id);
              preview = detail.messages?.filter(message => message.role === 'user' || message.role === 'assistant').slice(-1)[0]?.content || preview;
              if (version === generation.current) cache.current[id] = { revision, preview };
            } catch { /* The conversation title remains a useful preview if detail is unavailable. */ }
          }
        }
        return [id, { conversations, preview: preview.replace(/\s+/g, ' ').trim(), latestId: latest?.id }] as const;
      } catch {
        return [id, { conversations: [], preview: '', failed: true }] as const;
      }
    }));
    if (version === generation.current) setInbox(Object.fromEntries(rows));
  }, [ids]);
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => { await reload(); if (!disposed) timer = setTimeout(poll, 5000); };
    void poll();
    return () => { disposed = true; generation.current++; clearTimeout(timer); };
  }, [reload]);
  return { inbox, reload };
}
