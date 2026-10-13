import { useEffect, useRef, useState } from 'react';
import { MemoryEditor } from './MemoryEditor';
import {
  cn, copyToClipboard, formatDateTime, formatRelativeTime, importanceLevel,
  MEMORY_TYPE_STYLE, MEMORY_TYPE_TEXT, TAG_TYPE_TEXT, truncate,
} from '../../utils/format';
import { IconCheck, IconChevronRight, IconCopy, IconTrash } from '../icons';
import { IconButton, Modal, ProgressBar, Tooltip } from '../ui';
import type { MemoryEntry } from '../../types';

export interface MemoryCardProps {
  onEdited?: () => void;
  memory: MemoryEntry;
  deleting?: boolean;
  onDelete: (memory: MemoryEntry) => void;
}

/** Compact list row; full content and metadata are available in its detail dialog. */
export function MemoryCard({ onEdited, memory, deleting = false, onDelete }: MemoryCardProps) {
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const copyTimer = useRef<ReturnType<typeof setTimeout>>();
  const style = MEMORY_TYPE_STYLE[memory.contentType];
  const level = importanceLevel(memory.importanceScore);
  const tags = memory.tags ?? [];
  const summary = (memory.summary?.trim() || memory.content.trim()).replace(/\s+/g, ' ');
  useEffect(() => () => clearTimeout(copyTimer.current), []);

  const handleCopy = async () => {
    if (!await copyToClipboard(memory.content)) return;
    setCopied(true);
    clearTimeout(copyTimer.current);
    copyTimer.current = setTimeout(() => setCopied(false), 1600);
  };

  return <>
    <article className={cn('memory-row', deleting && 'opacity-60')}>
      <span className={cn('memory-row-accent', style.bar)} aria-hidden="true" />
      <button type="button" className="memory-row-open" aria-haspopup="dialog"
        aria-label={`查看记忆详情：${truncate(summary, 40)}`} onClick={() => setDetailsOpen(true)}>
        <span className="memory-row-copy">
          <strong className="memory-row-summary">{truncate(summary, 120)}</strong>
          <span className="memory-row-meta">
            <span className={cn('badge border', style.badge)}>{MEMORY_TYPE_TEXT[memory.contentType]}</span>
            <time dateTime={memory.createdAt} title={`创建于 ${formatDateTime(memory.createdAt)}`}>{formatRelativeTime(memory.createdAt)}</time>
            <span className="memory-row-importance">重要度 {memory.importanceScore}</span>
          </span>
        </span>
        <IconChevronRight size={16} className="memory-row-chevron" />
      </button>
      <div className="memory-row-actions">
        <Tooltip content={copied ? '已复制' : '复制内容'}>
          <IconButton label="复制记忆内容" onClick={() => void handleCopy()}>
            {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
          </IconButton>
        </Tooltip>
        {onEdited && <MemoryEditor id={memory.id} content={memory.content} onSaved={onEdited} disabled={deleting} />}
        <Tooltip content="删除这条记忆">
          <IconButton label="删除记忆" disabled={deleting} onClick={() => onDelete(memory)} className="hover:text-error">
            <IconTrash size={16} />
          </IconButton>
        </Tooltip>
      </div>
      <span className="sr-only" role="status">{copied ? '记忆内容已复制' : ''}</span>
    </article>

    <Modal open={detailsOpen} onClose={() => setDetailsOpen(false)} title="记忆详情" size="lg">
      <div className="memory-detail">
        <div className="memory-detail-heading">
          <span className={cn('badge border', style.badge)}>{MEMORY_TYPE_TEXT[memory.contentType]}</span>
          {memory.isExplicitlyStated && <span className="badge border-line bg-surface-3 text-ink-3">明确陈述</span>}
          {memory.conversationTurn > 0 && <span className="text-xs text-ink-3">第 {memory.conversationTurn} 轮</span>}
        </div>
        {memory.summary?.trim() && summary !== memory.content.trim().replace(/\s+/g, ' ') && <h3 className="memory-detail-summary">{memory.summary}</h3>}
        <p className="memory-detail-content">{memory.content}</p>
        {tags.length > 0 && <div className="memory-detail-tags" aria-label="记忆标签">
          {tags.map(tag => <span key={tag.id} title={`${TAG_TYPE_TEXT[tag.tagType] ?? tag.tagType}标签`}>{tag.name}</span>)}
        </div>}
        <div className="memory-detail-score">
          <span>重要度 <strong className={level.textClass}>{memory.importanceScore}/100 · {level.label}</strong></span>
          <ProgressBar value={memory.importanceScore} barClass={level.barClass} label={`重要度 ${memory.importanceScore}`} />
        </div>
        <dl className="memory-detail-metadata">
          <div><dt>创建时间</dt><dd>{formatDateTime(memory.createdAt)}</dd></div>
          <div><dt>更新时间</dt><dd>{formatDateTime(memory.updatedAt)}</dd></div>
          <div><dt>访问次数</dt><dd>访问 {memory.accessCount} 次</dd></div>
          <div><dt>最近使用</dt><dd>{memory.lastAccessedAt ? formatDateTime(memory.lastAccessedAt) : '尚未使用'}</dd></div>
          {memory.expiresAt && <div><dt>过期时间</dt><dd>{formatDateTime(memory.expiresAt)}</dd></div>}
        </dl>
      </div>
    </Modal>
  </>;
}
