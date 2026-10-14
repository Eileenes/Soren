/**
 * 聊天页
 *
 * 组成：消息列表（自动滚动 + 回到底部）→ 离线提示 → 输入框。
 * 首次进入显示欢迎空状态与示例 prompt。
 */

import type { Bot } from '../services/agents';
import { useCallback, useEffect, useRef, useState } from 'react';
import { cn } from '../utils/format';
import { ChatEmptyState } from '../components/chat/ChatEmptyState';
import { ChatInput } from '../components/chat/ChatInput';
import { MessageBubble } from '../components/chat/MessageBubble';
import { OfflineBanner } from '../components/layout/Shell';
import { IconArrowDown } from '../components/icons';
import { Button, SkeletonList } from '../components/ui';
import { useScrollPosition } from '../hooks/useUtilities';
import type { UseChatResult } from '../hooks/useChat';

export interface ChatPageProps {
 bot?: Bot;
  chat: UseChatResult;
  demoMode?: boolean;
  onModelSettings?: () => void;
  /** 后端是否离线 */
  offline: boolean;
  /** 重新检测后端 */
  onRetryConnection?: () => void;
  retrying?: boolean;
}

export function ChatPage({
  bot,
  chat,
  offline,
  onRetryConnection,
  retrying = false,
  demoMode,
  onModelSettings,
}: ChatPageProps) {
  const {
    messages,
    loading,
    typing,
    loadingHistory,
    error,
    send,
    stop,
    removeMessage,
  } = chat;

  const [draft, setDraft] = useState<{ text: string; key: number }>();
  const scrollRef = useRef<HTMLDivElement>(null);
  const { atBottom, scrollToBottom } = useScrollPosition(scrollRef);

  /**
   * 新消息时自动滚动
   *
   * 仅当用户本来就在底部时才自动跟随——用户往上翻看历史时不应该被强行拽回底部。
   */
  useEffect(() => {
    if (!atBottom || messages.length === 0) return;
    const element = scrollRef.current;
    if (!element) return;
    element.scrollTo({ top: element.scrollHeight, behavior: 'smooth' });
    // atBottom 故意不入依赖：只在消息变化时触发，避免滚动本身引发循环
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages.length, typing]);

  const handleSend = useCallback(
    (content: string) => {
      void send(content);
    },
    [send],
  );

  const handlePickPrompt = useCallback(
    (prompt: string) => {
      setDraft({ text: prompt, key: Date.now() });
    },
    [send],
  );

  // 最后一条助手消息允许重新生成

  const isEmpty = messages.length === 0 && !loadingHistory && !error;

  return (
    <div className={"chat-page " + (isEmpty ? "is-empty" : "has-messages")}>
      {offline && (
        <OfflineBanner onRetry={onRetryConnection} retrying={retrying} />
      )}

      {demoMode && !offline && <div className="demo-notice"><span>未连接模型 · 本地演示</span><button onClick={onModelSettings}>连接模型 <span>↗</span></button></div>}

      {/* ── 消息区 ──────────────────────────────────────── */}
      <div
        ref={scrollRef}
        className="chat-scroll"
        aria-live="polite"
        aria-busy={loading}
      >
        <div className="chat-content">
          {loadingHistory ? (
            <SkeletonList rows={3} className="pt-4" />
          ) : isEmpty ? (
            <ChatEmptyState bot={bot} onPickPrompt={handlePickPrompt} disabled={loading} />
          ) : (
            <div className="flex flex-col gap-5">
              {messages.map((message) => (
                <MessageBubble bot={bot}
                  key={message.id}
                  message={message}
                  pending={loading}
                  canRegenerate={false}
                />
              ))}

              {/* 请求失败提示：给出可操作的重试入口 */}
              {error && (
                <div
                  role="alert"
                  className="flex items-start gap-3 rounded-md border border-error/30 bg-error/10 px-4 py-3"
                >
                  <div className="min-w-0 flex-1">
                    <p className="text-xs font-medium text-error">这次请求没有成功</p>
                    <p className="mt-1 break-words text-xs leading-5 text-ink-2">{error}</p>
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      // 移除失败的用户消息对应的助手占位后重试
                      const lastFailed = [...messages]
                        .reverse()
                        .find((item) => item.status === 'failed');
                      if (lastFailed) {
                        removeMessage(lastFailed.id);
                        void send(lastFailed.content);
                      } else if (chat.conversationId) {
                        void chat.loadConversation(chat.conversationId);
                      }
                    }}
                  >
                    重试
                  </Button>
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

      {/* ── 输入区 ──────────────────────────────────────── */}
      <ChatInput
        placeholder={bot?.status === 'paused' ? 'Bot 已暂停，请在配置中启用' : bot && bot.id !== 'soren' && !bot.instructions.trim() ? '告诉我，你希望我负责什么…' : `给 ${bot?.name || 'Soren'} 发消息…`}
        onSend={handleSend}
        onStop={stop}
        disabled={bot?.status === 'paused' || offline}
        loading={loading}
        draft={draft}
      />


    </div>
  );
}
