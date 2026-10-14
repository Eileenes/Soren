/**
 * 群聊数据 Hook
 *
 * 两个入口：
 * - useGroupList：侧栏群列表（标题、成员摘要、最近一条预览）
 * - useGroupChat：单个群的详情、历史与发言
 *
 * 两条后端契约决定了这里的写法：
 * 1. SpeakResult.messages 是本次发言落库的整段（含用户自己那条），
 *    因此替换乐观消息，并按服务端 ID 合并，不做内容级去重——
 *    用户连续发两条相同内容是合法的，去重会吞掉第二条。
 * 2. 群历史只能从 GET /api/groups/:id 取，没有第二个入口。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { describeError, normalizeError } from '../services/api';
import { groupsAPI } from '../services/groups';
import { toChatMessage, type ChatMessage } from './useChat';
import type { GroupDetailView, GroupSpeakInput, GroupView, SpeakResult } from '../types';

/** 群聊的成员上限与轮次上限（后端 group.MinMembers/MaxMembers/MaxRoundsCap） */
export const GROUP_MIN_MEMBERS = 2;
export const GROUP_MAX_MEMBERS = 5;
export const GROUP_MAX_ROUNDS = 3;
export const GROUP_DEFAULT_ROUNDS = 3;

/** 群列表：侧栏用 */
export interface UseGroupListResult {
  groups: GroupView[];
  loading: boolean;
  error: string | null;
  reload: () => Promise<void>;
}

export function useGroupList(): UseGroupListResult {
  const [groups, setGroups] = useState<GroupView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const generation = useRef(0);

  const reload = useCallback(async () => {
    const version = ++generation.current;
    try {
      const data = await groupsAPI.list();
      if (version === generation.current) { setGroups(data ?? []); setError(null); }
    } catch (err) {
      if (version === generation.current) setError(describeError(err));
    } finally {
      if (version === generation.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
    // 群预览（lastMessagePreview）靠服务端冗余字段，发言后需要刷新
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => { await reload(); if (!disposed) timer = setTimeout(poll, 10000); };
    timer = setTimeout(poll, 10000);
    return () => { disposed = true; generation.current++; clearTimeout(timer); };
  }, [reload]);

  return { groups, loading, error, reload };
}

/** 单个群：详情页用 */
export interface UseGroupChatResult {
  group: GroupView | null;
  turn?: GroupDetailView['latestTurn'];
  messages: ChatMessage[];
  loadingHistory: boolean;
  /** 一次发言进行中（阻塞式调用，服务端跑完所有轮次才返回） */
  speaking: boolean;
  /** 已请求中止，等待在途请求收敛 */
  cancelling: boolean;
  error: string | null;
  offline: boolean;
  send: (content: string) => Promise<void>;
  cancel: () => Promise<void>;
  reload: () => Promise<void>;
}

export function useGroupChat(groupId: string): UseGroupChatResult {
  const [group, setGroup] = useState<GroupView | null>(null);
  const [turn, setTurn] = useState<GroupDetailView['latestTurn']>();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(true);
  const [speaking, setSpeaking] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const generation = useRef(0);
  const lifetime = useRef(0);
  const busy = useRef(false);
  const inFlight = useRef(false);
  const remoteRunning = useRef(false);
  // 网络结果不确定时保留完整请求，重试不重新计算 mentions/mode。
  const pending = useRef<GroupSpeakInput | null>(null);

  const load = useCallback(async (options?: { background?: boolean; keepError?: boolean }) => {
    const version = ++generation.current;
    if (!options?.background) setLoadingHistory(true);
    if (!options?.keepError) setError(null);
    try {
      const detail: GroupDetailView = await groupsAPI.detail(groupId);
      if (version !== generation.current) return;
      setGroup(detail);
      setTurn(detail.latestTurn);
      setMessages((detail.messages ?? []).map(message => toChatMessage(message)));
      setOffline(false);
      const wasRunning = remoteRunning.current;
      remoteRunning.current = detail.latestTurn?.status === 'running';
      if (wasRunning && detail.latestTurn && !remoteRunning.current) {
        if (detail.latestTurn.requestId === pending.current?.requestId) pending.current = null;
        setError(detail.latestTurn.error || null);
      }
      busy.current = inFlight.current || remoteRunning.current;
      setSpeaking(busy.current);
      if (!busy.current) setCancelling(false);
      if (detail.latestTurn?.error) setError(detail.latestTurn.error);
    } catch (err) {
      if (version !== generation.current) return;
      setOffline(normalizeError(err).isOffline);
      setError(describeError(err));
      // 查询失败不能断言讨论结束，保留中止入口并继续轮询。
    } finally {
      if (version === generation.current && !options?.background) setLoadingHistory(false);
    }
  }, [groupId]);

  useEffect(() => {
    lifetime.current += 1;
    busy.current = false; inFlight.current = false; remoteRunning.current = false;
    pending.current = null;
    setGroup(null); setTurn(undefined); setMessages([]); setSpeaking(false); setCancelling(false);
    void load();
    return () => { lifetime.current += 1; generation.current += 1; };
  }, [load]);

  // 切回群时从服务端恢复运行状态；运行中只查询详情，不重复提交模型请求。
  useEffect(() => {
    if (!speaking) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      await load({ background: true, keepError: true });
      if (!disposed) timer = setTimeout(poll, 1000);
    };
    timer = setTimeout(poll, 1000);
    return () => { disposed = true; clearTimeout(timer); };
  }, [speaking, load]);

  const send = useCallback(async (content: string) => {
    const text = content.trim();
    if (!text || busy.current || !group) return;
    const request: GroupSpeakInput = pending.current?.content === text ? pending.current : {
      content: text, mentions: resolveMentionIds(text, group.members),
      requestId: crypto.randomUUID(),
    };
    const activeLifetime = lifetime.current;
    const isCurrent = () => lifetime.current === activeLifetime;
    const optimisticId = `local-${request.requestId}`;
    const timestamp = new Date().toISOString();
    generation.current += 1;
    pending.current = request;
    busy.current = true; inFlight.current = true;
    setSpeaking(true); setCancelling(false); setError(null);
    setTurn({ requestId: request.requestId, mode: 'mention', status: 'running' });
    setMessages(previous => [...previous.filter(item => item.id !== optimisticId), {
      id: optimisticId, conversationId: group.id, role: 'user', content: text,
      timestamp, createdAt: timestamp, updatedAt: timestamp, status: 'sending',
    }]);
    try {
      const result: SpeakResult = await groupsAPI.speak(groupId, request);
      if (!isCurrent()) return;
      generation.current += 1;
      remoteRunning.current = result.status === 'running';
      setTurn({ requestId: request.requestId, mode: result.mode ?? 'mention', status: result.status, error: result.error });
      if (!remoteRunning.current) pending.current = null;
      // 同一条服务端消息只能出现一次；内容相同但 ID 不同的消息仍然保留。
      setMessages(previous => {
        const merged = new Map(previous.filter(item => item.id !== optimisticId).map(item => [item.id, item]));
        for (const item of result.messages) merged.set(item.id, toChatMessage(item));
        return [...merged.values()];
      });
      setOffline(false);
      if (result.error) setError(result.error);
    } catch (err) {
      if (!isCurrent()) return;
      const normalized = normalizeError(err);
      setOffline(normalized.isOffline); setError(describeError(err));
      // 收到明确的 HTTP 失败后允许下一次发起新轮次；仅网络结果不确定时复用幂等键。
      if (normalized.status > 0) pending.current = null;
      await load({ background: true, keepError: true });
    } finally {
      if (isCurrent()) {
        inFlight.current = false;
        busy.current = remoteRunning.current;
        setSpeaking(busy.current);
        if (!busy.current) setCancelling(false);
      }
    }
  }, [group, groupId, load]);

  const cancel = useCallback(async () => {
    if (!busy.current) return;
    const activeLifetime = lifetime.current;
    setCancelling(true);
    try {
      await groupsAPI.cancel(groupId);
      if (lifetime.current === activeLifetime) await load({ background: true, keepError: true });
    } catch (err) {
      if (lifetime.current !== activeLifetime) return;
      setError(describeError(err)); setCancelling(false);
    }
  }, [groupId, load]);

  const reload = useCallback(() => load(), [load]);
  return { group, turn, messages, loadingHistory, speaking, cancelling, error, offline, send, cancel, reload };
}

/* ────────────────────────────────────────────────────────────
 * @ 提及解析（纯函数，供气泡高亮、补全与发言请求共用）
   ──────────────────────────────────────────────────────────── */

/** 文本中命中的一个 @成员名 */
export interface MentionHit {
  name: string;
  start: number;
  end: number;
}

/** @ 后的合法名字字符：中文、字母、数字等；空白与常见标点结束名字 */
const MENTION_TAIL = /[^\s@,，。！？!?;:：;、()（）[\]]/;
/** 从 @ 之后连续取完整一段名字字符（必须锚定起点，否则只能拿到第一个字） */
const MENTION_TAIL_RUN = /^[^\s@,，。！？!?;:：;、()（）[\]]*/;

/**
 * 找出文本里所有匹配到群成员名的 @提及
 *
 * 只匹配当前群成员名：@ 后跟非成员名（邮箱、代码片段、"@someone"）一律不高亮，
 * 误伤正文比漏高亮更糟。名字按长度降序匹配，避免「小助手」与「小助手二号」互相截断。
 */
export function findMentionHits(content: string, names: string[]): MentionHit[] {
  const candidates = names
    .filter(name => name.trim().length > 0)
    .sort((a, b) => b.length - a.length);
  const hits: MentionHit[] = [];
  for (let index = 0; index < content.length; index += 1) {
    if (content[index] !== '@') continue;
    const token = MENTION_TAIL_RUN.exec(content.slice(index + 1))![0];
    const name = candidates.find(item => token.startsWith(item));
    if (name) {
      hits.push({ name, start: index, end: index + 1 + name.length });
      index += name.length;
    }
  }
  return hits;
}

/** 把文本里的 @提及解析成群成员 botId（仅成员名命中才算） */
export function resolveMentionIds(content: string, members: { botId: string; name: string }[]): string[] {
  const names = members.map(member => member.name);
  const ids: string[] = [];
  for (const hit of findMentionHits(content, names)) {
    const member = members.find(item => item.name === hit.name);
    if (member && !ids.includes(member.botId)) ids.push(member.botId);
  }
  return ids;
}

/** 光标前的 @补全查询；不在补全状态返回 null */
export interface MentionQuery {
  /** 已输入的前缀 */
  query: string;
  /** @ 的位置 */
  start: number;
  /** 光标位置 */
  end: number;
}

/**
 * 找出光标前正在输入的 @
 *
 * 只认「@ 后面紧跟连续的名字字符」，输入空格、中文逗号或退格删光即失效。
 */
export function activeMentionQuery(value: string, caret: number): MentionQuery | null {
  const before = value.slice(0, caret);
  const at = before.lastIndexOf('@');
  if (at < 0) return null;
  const token = before.slice(at + 1);
  if (token && !MENTION_TAIL.test(token[0])) return null;
  if (token.includes('@')) return null;
  return { query: token, start: at, end: caret };
}