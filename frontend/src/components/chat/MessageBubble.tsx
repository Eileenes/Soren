/**
 * 消息气泡
 *
 * 用户消息：右对齐，品牌渐变底。
 * 助手消息：左对齐，带头像、Markdown 正文、记忆召回提示与操作栏。
 *
 * 操作栏（复制 / 重新生成）在悬浮时淡入，但通过 focus-within 保证键盘可达。
 */

import { useState, type ReactNode } from 'react';
import { cn, copyToClipboard, formatTime } from '../../utils/format';
import { IconAlert, IconBrain, IconCheck, IconCopy, IconRefresh } from '../icons';
import { avatarURL, type Bot } from '../../services/agents';
import { TypingDots } from '../ui/Button';
import { Tooltip } from '../ui';
import { MarkdownView } from './MarkdownView';
import type { ChatMessage } from '../../hooks/useChat';

export interface MessageBubbleProps {
  bot?: Bot;
  message: ChatMessage;
  /** 是否正在等待响应（最后一条助手消息显示思考动画） */
  pending?: boolean;
  /** 重新生成回调，仅助手消息可用 */
  onRegenerate?: (messageId: string) => void;
  /** 是否为最后一条助手消息（只有最后一条允许重新生成） */
  canRegenerate?: boolean;
}

/** 气泡内的操作按钮 */
function ActionButton({
  label,
  icon,
  onClick,
  disabled,
}: {
  label: string;
  icon: ReactNode;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <Tooltip content={label}>
      <button
        type="button"
        onClick={onClick}
        disabled={disabled}
        aria-label={label}
        className={cn(
          'control-icon inline-flex items-center justify-center',
          'text-ink-3 transition-all duration-fast',
          'hover:bg-surface-4 hover:text-ink-1',
          'disabled:opacity-40 disabled:cursor-not-allowed',
        )}
      >
        {icon}
      </button>
    </Tooltip>
  );
}

export function MessageBubble({
  bot,
  message,
  pending = false,
  onRegenerate,
  canRegenerate = false,
}: MessageBubbleProps) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === 'user';
  const isAssistant = message.role === 'assistant';
  const failed = message.status === 'failed';

  const handleCopy = async () => {
    const ok = await copyToClipboard(message.content);
    if (!ok) return;
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  // 等待中且无内容：显示思考动画
  const showThinking = isAssistant && pending && message.content === '';

  /* ── 用户消息 ─────────────────────────────────────────── */
  if (isUser) {
    return (
      <div className="group/msg flex w-full justify-end gap-3 animate-slide-up">
        <div className="flex max-w-[76%] flex-col items-end gap-1.5">
          <div
            className={cn(
              'bubble-user whitespace-pre-wrap break-words text-[15px] leading-relaxed',
              failed && 'opacity-70',
            )}
          >
            {message.content}
          </div>

          <div className="flex items-center gap-1.5 px-1">
            {failed && (
              <span className="inline-flex items-center gap-1 text-xs text-error">
                <IconAlert size={12} />
                发送失败
              </span>
            )}
            <span className="text-xs text-ink-3 opacity-0 transition-opacity duration-fast group-hover/msg:opacity-100">
              {formatTime(message.timestamp || message.createdAt)}
            </span>
            <ActionButton
              label={copied ? '已复制' : '复制'}
              icon={copied ? <IconCheck size={13} /> : <IconCopy size={13} />}
              onClick={handleCopy}
            />
          </div>
        </div>
      </div>
    );
  }

  // system 消息不渲染为气泡（后端历史中可能出现）
  if (!isAssistant) {
    return (
      <div className="flex justify-center">
        <span className="rounded-full border border-line bg-surface-3 px-3 py-1 text-xs text-ink-3">
          {message.content}
        </span>
      </div>
    );
  }

  /* ── 助手消息 ─────────────────────────────────────────── */
  return (
    <div className="group/msg flex w-full justify-start gap-3 animate-slide-up">
      {/* 头像 */}
      <div className="mt-0.5 shrink-0">
        <div className="rounded-md ring-1 ring-line/70 overflow-hidden">
          <img src={avatarURL(bot?.avatar || 'soren')} alt={bot?.name || 'Soren'} width={30} height={30} />
        </div>
      </div>

      <div className="flex min-w-0 max-w-[82%] flex-col gap-1.5">
        {/* 记忆召回提示：让"AI 记得我"这件事变得可见 */}
        {message.memoriesUsed && message.memoriesUsed.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 pl-1">
            <span className="inline-flex items-center gap-1 text-xs text-primary-start">
              <IconBrain size={12} />
              想起了
            </span>
            {message.memoriesUsed.slice(0, 4).map((item, index) => (
              <span
                key={`${item}-${index}`}
                className="max-w-[220px] truncate rounded-sm border border-primary-start/25 bg-primary-start/10 px-2 py-0.5 text-xs text-primary-start"
                title={item}
              >
                {item}
              </span>
            ))}
            {message.memoriesUsed.length > 4 && (
              <span className="text-xs text-ink-3">
                +{message.memoriesUsed.length - 4}
              </span>
            )}
          </div>
        )}

        <div className="bubble-assistant">
          {showThinking ? (
            <div className="flex items-center gap-2 py-0.5 text-ink-2">
              <TypingDots />
              <span className="text-xs text-ink-3">Soren 正在思考…</span>
            </div>
          ) : (
            <>
              <MarkdownView content={message.content} />

              {/* 打字光标：内容仍在揭示中 */}
              {message.streaming && (
                <span
                  className="ml-0.5 inline-block h-4 w-[2px] translate-y-0.5 animate-caret rounded-full bg-primary-start align-middle"
                  aria-hidden="true"
                />
              )}
            </>
          )}
        </div>

        {/* 操作栏：默认隐藏，悬浮或聚焦时显示 */}
        {!showThinking && (
          <div
            className={cn(
              'flex items-center gap-0.5 pl-1',
              'opacity-0 transition-opacity duration-base',
              'group-hover/msg:opacity-100 focus-within:opacity-100',
            )}
          >
            <ActionButton
              label={copied ? '已复制' : '复制回复'}
              icon={copied ? <IconCheck size={13} /> : <IconCopy size={13} />}
              onClick={handleCopy}
            />
            {onRegenerate && (
              <ActionButton
                label={canRegenerate ? '重新生成' : '仅最后一条回复可重新生成'}
                icon={<IconRefresh size={13} />}
                onClick={() => onRegenerate(message.id)}
                disabled={!canRegenerate}
              />
            )}
            <span className="ml-1 text-xs text-ink-3">
              {formatTime(message.timestamp || message.createdAt)}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
