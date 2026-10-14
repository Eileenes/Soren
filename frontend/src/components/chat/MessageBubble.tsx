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
import { findMentionHits } from '../../hooks/useGroupChat';
import type { ChatMessage } from '../../hooks/useChat';

export interface MessageBubbleProps {
  bot?: Bot;
  /** 群聊发言人的解析池：按 message.speakerBotId 逐条取人 */
  bots?: Bot[];
  message: ChatMessage;
  /** 是否正在等待响应（最后一条助手消息显示思考动画） */
  pending?: boolean;
  /** 重新生成回调，仅助手消息可用 */
  onRegenerate?: (messageId: string) => void;
  /** 是否为最后一条助手消息（只有最后一条允许重新生成） */
  canRegenerate?: boolean;
  /** 群成员名：用户消息里的 @成员名 会被强调 */
  mentionNames?: string[];
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

/**
 * 用户气泡里的 @提及
 *
 * 纯文字强调（颜色 + 字重），不加背景块也不加描边：行内 border 需要
 * inline-block，会改变 15px/leading-relaxed 正文框内的行高节奏，把消息流切碎。
 * 颜色用随主题切换的 rgb(var(--accent)) 而不是写死的 primary-start。
 */
function UserContent({ content, mentionNames }: { content: string; mentionNames?: string[] }) {
  const hits = mentionNames?.length ? findMentionHits(content, mentionNames) : [];
  if (!hits.length) return <>{content}</>;
  const parts: ReactNode[] = [];
  let cursor = 0;
  hits.forEach((hit, index) => {
    if (hit.start > cursor) parts.push(content.slice(cursor, hit.start));
    parts.push(
      <span key={`${hit.start}-${index}`} className="font-semibold text-[rgb(var(--accent))]">
        {content.slice(hit.start, hit.end)}
      </span>,
    );
    cursor = hit.end;
  });
  if (cursor < content.length) parts.push(content.slice(cursor));
  return <>{parts}</>;
}

export function MessageBubble({
  bot,
  bots,
  message,
  pending = false,
  onRegenerate,
  canRegenerate = false,
  mentionNames,
}: MessageBubbleProps) {
  const [copied, setCopied] = useState(false);
  const isUser = message.role === 'user';
  const isAssistant = message.role === 'assistant';
  const failed = message.status === 'failed';

  // 群聊里一条发言对应一个 Bot：先按 speakerBotId 解析，解析不到（Bot 已删除
  // 或被移出群）回退到单聊的 bot prop，名称用后端冗余的 speakerName。
  const speaker = message.speakerBotId
    ? bots?.find(item => item.id === message.speakerBotId)
    : undefined;
  const display = speaker ?? bot;
  const speakerLabel = message.speakerName || display?.name || 'Soren';
  const hasIdentity = Boolean(message.speakerBotId || message.speakerName);
  // 群聊里若两个字段都为空，退化为无头像的无名气泡，不显示「未知 Bot」；
  // 单聊的身份来自 bot prop，这两个字段本来就没有，行为保持不变。
  const showAvatar = bots === undefined || hasIdentity;

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
            <UserContent content={message.content} mentionNames={mentionNames} />
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
      {showAvatar && (
        <div className="mt-0.5 shrink-0">
          <div className="rounded-md ring-1 ring-line/70 overflow-hidden">
            <img src={avatarURL(display?.avatar || 'soren')} alt={display?.name || 'Soren'} width={30} height={30} />
          </div>
        </div>
      )}

      <div className="flex min-w-0 max-w-[82%] flex-col gap-1.5">
        {/* 发言人名：群聊里识别发言者的唯一线索，ink-2 是浅色主题下达 AA 的最浅一档 */}
        {hasIdentity && (
          <span className="truncate pl-1 text-xs text-ink-2" title={speakerLabel}>{speakerLabel}</span>
        )}

        {/* 记忆召回提示：让"AI 记得我"这件事变得可见 */}
        {message.memoriesUsed && message.memoriesUsed.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 pl-1">
            <span className="inline-flex items-center gap-1 text-xs text-[rgb(var(--accent))]">
              <IconBrain size={12} />
              想起了
            </span>
            {message.memoriesUsed.slice(0, 4).map((item, index) => (
              <span
                key={`${item}-${index}`}
                className="max-w-[220px] truncate rounded-sm border border-[rgb(var(--accent)/0.25)] bg-[rgb(var(--accent)/0.1)] px-2 py-0.5 text-xs text-[rgb(var(--accent))]"
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
              <span className="text-xs text-ink-3">{speakerLabel} 正在思考…</span>
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
