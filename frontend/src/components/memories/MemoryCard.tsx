import { MemoryEditor } from './MemoryEditor';
/**
 * 记忆卡片
 *
 * 承担"让用户看懂 AI 记住了什么"的职责，因此信息层级按重要性排列：
 * 类型 → 摘要 → 正文 → 标签 → 重要度/访问次数/时间。
 */

import { useState } from 'react';
import {
  cn,
  copyToClipboard,
  formatDateTime,
  formatRelativeTime,
  importanceLevel,
  MEMORY_TYPE_STYLE,
  MEMORY_TYPE_TEXT,
  TAG_TYPE_TEXT,
} from '../../utils/format';
import { IconCheck, IconCopy, IconClock, IconTag, IconTrash } from '../icons';
import { IconButton, ProgressBar, Tooltip } from '../ui';
import type { MemoryEntry } from '../../types';

export interface MemoryCardProps {
 onEdited?: () => void;
  memory: MemoryEntry;
  /** 删除中（按钮显示加载态） */
  deleting?: boolean;
  onDelete: (memory: MemoryEntry) => void;
}

export function MemoryCard({ onEdited, memory, deleting = false, onDelete }: MemoryCardProps) {
  const [copied, setCopied] = useState(false);
  const style = MEMORY_TYPE_STYLE[memory.contentType];
  const level = importanceLevel(memory.importanceScore);
  const tags = memory.tags ?? [];

  const handleCopy = async () => {
    const ok = await copyToClipboard(memory.content);
    if (!ok) return;
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  return (
    <article
      className={cn(
        'group/card relative flex flex-col gap-3 rounded-lg border border-line bg-surface-2/80 p-4',
        'ring-highlight transition-all duration-base ease-ease',
        'hover:border-line-strong hover:bg-surface-3/70',
        'hover:shadow-[0_16px_40px_-24px_rgb(0_0_0/0.7)]',
        deleting && 'opacity-60',
      )}
    >
      {/* 左侧类型色条 */}
      <span
        className={cn('absolute left-0 top-4 h-[calc(100%-32px)] w-[2px] rounded-r-full', style.bar)}
        aria-hidden="true"
      />

      {/* ── 头部：类型 + 重要度 ─────────────────────────── */}
      <header className="flex items-start justify-between gap-3 pl-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span className={cn('badge border', style.badge)}>
            <span className={cn('h-1.5 w-1.5 rounded-full', style.dot)} />
            {MEMORY_TYPE_TEXT[memory.contentType]}
          </span>

          {memory.isExplicitlyStated && (
            <Tooltip content="用户明确陈述过，重要度评分更高">
              <span className="badge border-line bg-surface-3 text-ink-3">明确陈述</span>
            </Tooltip>
          )}

          {memory.conversationTurn > 0 && (
            <span className="text-xs text-ink-3">第 {memory.conversationTurn} 轮</span>
          )}
        </div>

        {/* 重要度：数值 + 分级 + 进度条 */}
        <Tooltip content={`重要度 ${memory.importanceScore}/100（${level.label}）`}>
          <div className="flex w-[92px] shrink-0 flex-col items-end gap-1">
            <div className="flex items-baseline gap-1">
              <span className={cn('font-mono text-sm font-semibold', level.textClass)}>
                {memory.importanceScore}
              </span>
              <span className="text-xs text-ink-3">/100</span>
            </div>
            <ProgressBar
              value={memory.importanceScore}
              barClass={level.barClass}
              className="h-1"
              label={`重要度 ${memory.importanceScore}`}
            />
            <span className={cn('text-xs', level.textClass)}>{level.label}</span>
          </div>
        </Tooltip>
      </header>

      {/* ── 摘要与正文 ─────────────────────────────────── */}
      <div className="min-w-0 pl-2">
        {memory.summary && memory.summary !== memory.content && (
          <h3 className="mb-1 truncate text-sm font-semibold tracking-tight text-ink-1">
            {memory.summary}
          </h3>
        )}
        <p className="whitespace-pre-wrap break-words text-sm leading-6 text-ink-2">
          {memory.content}
        </p>
      </div>

      {/* ── 标签 ───────────────────────────────────────── */}
      {tags.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 pl-2">
          <IconTag size={12} className="text-ink-3" />
          {tags.slice(0, 6).map((tag) => (
            <Tooltip key={tag.id} content={`${TAG_TYPE_TEXT[tag.tagType] ?? tag.tagType}标签`}>
              <span className="rounded-sm border border-line bg-surface-3 px-1.5 py-0.5 text-xs text-ink-2">
                {tag.name}
              </span>
            </Tooltip>
          ))}
          {tags.length > 6 && (
            <span className="text-xs text-ink-3">+{tags.length - 6}</span>
          )}
        </div>
      )}

      {/* ── 底部：元信息 + 操作 ─────────────────────────── */}
      <footer className="mt-auto flex items-center justify-between gap-3 border-t border-line-subtle pt-3 pl-2">
        <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-3">
          <Tooltip content={`创建于 ${formatDateTime(memory.createdAt)}`}>
            <span className="inline-flex items-center gap-1">
              <IconClock size={11} />
              {formatRelativeTime(memory.createdAt)}
            </span>
          </Tooltip>

          <span>访问 {memory.accessCount} 次</span>

          {memory.lastAccessedAt && (
            <span className="hidden sm:inline">
              最近使用 {formatRelativeTime(memory.lastAccessedAt)}
            </span>
          )}
        </div>

        {/* 操作：悬浮淡入，键盘聚焦可见 */}
        <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity duration-base group-hover/card:opacity-100 focus-within:opacity-100">
          <Tooltip content={copied ? '已复制' : '复制内容'}>
            <IconButton label="复制记忆内容" size="sm" onClick={handleCopy}>
              {copied ? <IconCheck size={13} /> : <IconCopy size={13} />}
            </IconButton>
          </Tooltip>

          <Tooltip content="删除这条记忆">
            <IconButton
              label="删除记忆"
              size="sm"
              disabled={deleting}
              onClick={() => onDelete(memory)}
              className="hover:text-error"
            >
              <IconTrash size={13} />
            </IconButton>
          </Tooltip>
        </div>
        {onEdited && <MemoryEditor id={memory.id} content={memory.content} onSaved={onEdited} />}
      </footer>
    </article>
  );
}
