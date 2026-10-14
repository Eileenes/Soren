/**
 * 群聊页
 *
 * 结构复用 ChatPage（消息区 → 骨架屏 → 空状态 → 气泡流 → 错误块 → 回到底部 → 输入区），
 * 差异只在四处：发言人逐条解析、讨论状态与中止入口、@ 补全、自动滚动的实现方式。
 *
 * 输入区固定传 loading={false}：单聊的 loading 会把发送按钮变成「停止生成」，
 * 与状态条上的「中止讨论」语义不同却长得一样，两个都会让人犹豫该点哪个。
 */

import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import type { Bot } from '../services/agents';
import { cn } from '../utils/format';
import { ChatEmptyState } from '../components/chat/ChatEmptyState';
import { ChatInput, type MentionCandidate } from '../components/chat/ChatInput';
import { MessageBubble } from '../components/chat/MessageBubble';
import { OfflineBanner } from '../components/layout/Shell';
import { IconArrowDown, IconStop } from '../components/icons';
import { Button, IconButton, SkeletonList, TypingDots } from '../components/ui';
import { useScrollPosition } from '../hooks/useUtilities';
import { resolveMentionIds, useGroupChat, type UseGroupChatResult } from '../hooks/useGroupChat';
import type { ChatMessage } from '../hooks/useChat';

/**
 * 用户偏离底部多少像素内仍算「贴着底部」
 *
 * 群聊一次回复可能追加多条长消息，DOM 撑高会把容器顶离底部几十到几百像素；
 * 阈值太紧会把这种布局位移误判成用户主动上翻（单聊默认的 80px 在这里不够用）。
 *
 * 同一个值也喂给 useScrollPosition：按钮显隐与自动跟随必须用同一把尺，
 * 否则会出现「回到底部按钮亮着，新消息却仍把视野拽走」的自相矛盾。
 */
const BOTTOM_THRESHOLD = 240;

export interface GroupChatPageProps {
  bots: Bot[];
  chat: UseGroupChatResult;
  offline: boolean;
  onRetryConnection?: () => void;
  retrying?: boolean;
}

/**
 * 群聊路由容器
 *
 * 只做一件事：把群 id 接上 useGroupChat 并渲染页面。Hook 必须在组件里调用，
 * 而 AppContent 需要在选中 Bot 时完全不碰群数据，所以用这层壳把请求范围
 * 限制在真正选中群的时候。
 */
export function GroupChatRoute({ groupId, bots, offline, onRetryConnection, retrying, onSpeakingChange }: {
  groupId: string;
  bots: Bot[];
  offline: boolean;
  onRetryConnection?: () => void;
  retrying?: boolean;
  /** 讨论进行中：标题栏的删除入口据此禁用 */
  onSpeakingChange?: (speaking: boolean) => void;
}) {
  const chat = useGroupChat(groupId);
  useEffect(() => { onSpeakingChange?.(chat.speaking); }, [chat.speaking, onSpeakingChange]);
  return <GroupChatPage bots={bots} chat={chat} offline={offline} onRetryConnection={onRetryConnection} retrying={retrying} />;
}

export function GroupChatPage({ bots, chat, offline, onRetryConnection, retrying = false }: GroupChatPageProps) {
  const { group, messages, loadingHistory, speaking, cancelling, error, send, cancel, reload } = chat;
  const scrollRef = useRef<HTMLDivElement>(null);
  const { atBottom, scrollToBottom } = useScrollPosition(scrollRef, BOTTOM_THRESHOLD);
  /**
   * 用户是否仍贴着底部
   *
   * 单独用 ref 记，不读 useScrollPosition 暴露的 atBottom：那个 state 会被
   * 布局撑高引发的 scroll 事件同步改写，一次追加多条消息就会在自动滚动前翻成
   * false，于是消息渲染了却停在视野之外。这里只在真实滚动时更新它。
   */
  const stickToBottom = useRef(true);
  /** mention 模式额外显示各位发言人的思考占位 */
  const [targets, setTargets] = useState<string[]>([]);

  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    const handleScroll = () => {
      stickToBottom.current =
        element.scrollHeight - element.scrollTop - element.clientHeight <= BOTTOM_THRESHOLD;
    };
    // 挂载时先量一次：历史消息由骨架屏换成真实气泡后，内容高度是重新布局出来的
    handleScroll();
    element.addEventListener('scroll', handleScroll, { passive: true });
    return () => element.removeEventListener('scroll', handleScroll);
  }, []);

  /**
   * 新消息自动跟随
   *
   * 用 useLayoutEffect 而非 useEffect：它在 DOM 变更提交后同步执行，早于绘制与
   * scroll 事件派发，此刻 ref 里还是更新前的用户位置，不会被这次撑高布局引发的
   * scroll 事件抢先翻成 false。仍然只在用户本来就在底部时跟随——上翻看历史时
   * 不该被强行拽回。
   */
  useLayoutEffect(() => {
    if (!stickToBottom.current || messages.length === 0) return;
    const element = scrollRef.current;
    if (!element) return;
    element.scrollTo({ top: element.scrollHeight, behavior: 'smooth' });
    // 只认条数：stickToBottom 是 ref 不会触发重渲染，messages 的引用每轮都变，
    // 进了依赖反而会在内容替换时多滚一次
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages.length, targets.length]);

  const members = group?.members ?? [];
  const names = useMemo(() => members.map(member => member.name), [members]);
  const candidates = useMemo<MentionCandidate[]>(
    () => members.map(member => ({ id: member.botId, name: member.name, avatar: member.avatar })),
    [members],
  );

  const handleSend = useCallback((content: string) => {
    const ids = resolveMentionIds(content, members);
    // 未指定提及时后端按全员依次回复，前端的思考占位要与之一致
    setTargets(ids.length ? ids : members.map(member => member.botId));
    void send(content);
  }, [send, members]);

  const isEmpty = messages.length === 0 && !loadingHistory && !error;
  const showStrip = Boolean(group) && speaking;

  const pendingMessages: ChatMessage[] = group?.mode !== 'auto' && speaking
    ? members.filter(member => targets.includes(member.botId)).map(member => ({
      id: `pending-${member.botId}`,
      conversationId: '',
      role: 'assistant',
      content: '',
      status: 'sending',
      speakerBotId: member.botId,
      speakerName: member.name,
      timestamp: '',
      createdAt: '',
      updatedAt: '',
    }))
    : [];

  return (
    <div className="chat-page has-messages">
      {offline && <OfflineBanner onRetry={onRetryConnection} retrying={retrying} />}

      <div ref={scrollRef} className="chat-scroll" aria-live="polite" aria-busy={speaking}>
        <div className="chat-content">
          {loadingHistory ? (
            <SkeletonList rows={3} className="pt-4" />
          ) : isEmpty ? (
            <ChatEmptyState variant="group" group={group ?? undefined} members={candidates} />
          ) : (
            <div className="flex flex-col gap-5">
              {messages.map(message => (
                <MessageBubble key={message.id} bots={bots} mentionNames={names} message={message} />
              ))}
              {pendingMessages.map(message => (
                <MessageBubble key={message.id} bots={bots} message={message} pending />
              ))}

              {error && (
                <div role="alert" className="flex items-start gap-3 rounded-md border border-error/30 bg-error/10 px-4 py-3">
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium text-error">这次发言没有完整成功</p>
                    <p className="mt-1 break-words text-xs leading-5 text-ink-2">{error}</p>
                  </div>
                  <Button size="sm" variant="secondary" onClick={() => void reload()}>重试</Button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* ── 回到底部 ────────────────────────────────────── */}
      {!atBottom && messages.length > 0 && (
        <div className="pointer-events-none absolute bottom-[168px] left-1/2 z-10 -translate-x-1/2">
          <button
            type="button"
            onClick={() => scrollToBottom('smooth')}
            aria-label="回到底部"
            className={cn(
              'control-icon pointer-events-auto flex items-center justify-center',
              'glass-panel-strong text-ink-2 shadow-lg',
              'transition-all duration-base ease-ease hover:scale-105 hover:text-ink-1',
              'animate-fade-in',
            )}
          >
            <IconArrowDown size={16} />
          </button>
        </div>
      )}

      <ChatInput
        placeholder={group ? `发消息到 ${group.title}，@ 可以指定 Bot` : '发消息…'}
        onSend={handleSend}
        disabled={offline || !group || loadingHistory || speaking}
        loading={false}
        mentionCandidates={candidates}
        above={showStrip && group ? (
          <div className="mb-2 flex h-9 items-center gap-2 text-ink-2">
            <TypingDots />
            <span className="text-xs tabular-nums">{group.mode === 'auto' ? `讨论中 · 最多 ${group.maxRounds} 轮` : '正在回复'}</span>
            <span className="flex-1" />
            <IconButton
              label="中止讨论"
              onClick={() => void cancel()}
              disabled={cancelling}
              className="!text-error hover:bg-surface-3"
            >
              <IconStop size={16} />
            </IconButton>
          </div>
        ) : undefined}
      />
    </div>
  );
}