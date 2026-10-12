/**
 * 展示层格式化工具
 *
 * 集中在 utils 中，避免各组件重复实现相对时间、数字缩写等逻辑。
 */

import type { MemoryType, TagType } from '../types';

/**
 * 将后端返回的时间字符串安全地转换为 Date
 *
 * 后端使用 RFC3339；这里对空值与解析失败做兜底，避免 Invalid Date 直接渲染到界面。
 */
export function parseDate(value: string | null | undefined): Date | null {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  // Go 的零值时间（0001-01-01）视为"未设置"
  if (date.getFullYear() < 1970) return null;
  return date;
}

/** 相对时间：刚刚 / 5 分钟前 / 3 小时前 / 2 天前 / 具体日期 */
export function formatRelativeTime(value: string | null | undefined): string {
  const date = parseDate(value);
  if (!date) return '—';

  const diffMs = Date.now() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 0) return '刚刚';
  if (diffSec < 60) return '刚刚';
  if (diffSec < 3600) return `${Math.floor(diffSec / 60)} 分钟前`;
  if (diffSec < 86400) return `${Math.floor(diffSec / 3600)} 小时前`;
  if (diffSec < 86400 * 30) return `${Math.floor(diffSec / 86400)} 天前`;

  return formatDate(date);
}

/** 绝对日期：2024-10-10 */
export function formatDate(value: string | Date | null | undefined): string {
  const date = value instanceof Date ? value : parseDate(value);
  if (!date) return '—';
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

/** 日期 + 时分：2024-10-10 10:30 */
export function formatDateTime(value: string | null | undefined): string {
  const date = parseDate(value);
  if (!date) return '—';
  const hh = String(date.getHours()).padStart(2, '0');
  const mm = String(date.getMinutes()).padStart(2, '0');
  return `${formatDate(date)} ${hh}:${mm}`;
}

/** 消息时间戳：10:30 */
export function formatTime(value: string | null | undefined): string {
  const date = parseDate(value);
  if (!date) return '';
  const hh = String(date.getHours()).padStart(2, '0');
  const mm = String(date.getMinutes()).padStart(2, '0');
  return `${hh}:${mm}`;
}

/** 数字缩写：1200 → 1.2k */
export function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (Math.abs(value) < 1000) return String(value);
  if (Math.abs(value) < 1_000_000) {
    return `${(value / 1000).toFixed(value % 1000 === 0 ? 0 : 1)}k`;
  }
  return `${(value / 1_000_000).toFixed(1)}M`;
}

/** 记忆类型 → 中文标签 */
export const MEMORY_TYPE_TEXT: Record<MemoryType, string> = {
  conversation: '对话',
  preference: '偏好',
  task: '任务',
  fact: '事实',
};

/** 记忆类型 → 语义色（Tailwind 类名片段，避免动态拼接导致 JIT 漏扫） */
export const MEMORY_TYPE_STYLE: Record<
  MemoryType,
  { badge: string; dot: string; ring: string; bar: string }
> = {
  conversation: {
    badge: 'bg-info/15 text-info border-info/30',
    dot: 'bg-info',
    ring: 'ring-info/30',
    bar: 'bg-info',
  },
  preference: {
    badge: 'bg-accent-pink/15 text-accent-pink border-accent-pink/30',
    dot: 'bg-accent-pink',
    ring: 'ring-accent-pink/30',
    bar: 'bg-accent-pink',
  },
  task: {
    badge: 'bg-warning/15 text-warning border-warning/30',
    dot: 'bg-warning',
    ring: 'ring-warning/30',
    bar: 'bg-warning',
  },
  fact: {
    badge: 'bg-success/15 text-success border-success/30',
    dot: 'bg-success',
    ring: 'ring-success/30',
    bar: 'bg-success',
  },
};

/** 标签类型 → 中文标签 */
export const TAG_TYPE_TEXT: Record<TagType, string> = {
  preference: '偏好',
  topic: '话题',
  entity: '实体',
  skill: '技能',
};

/**
 * 重要度分级
 *
 * 阈值与后端设计文档一致：>=70 为高重要度（会被优先召回）。
 */
export function importanceLevel(score: number): {
  label: string;
  textClass: string;
  barClass: string;
} {
  if (score >= 85) {
    return { label: '极高', textClass: 'text-accent-coral', barClass: 'bg-gradient-accent' };
  }
  if (score >= 70) {
    return {
      label: '高',
      textClass: 'text-warning',
      barClass: 'bg-gradient-to-r from-warning to-accent-coral',
    };
  }
  if (score >= 40) {
    return {
      label: '中',
      textClass: 'text-info',
      barClass: 'bg-gradient-to-r from-info to-primary-start',
    };
  }
  return { label: '低', textClass: 'text-ink-3', barClass: 'bg-ink-3' };
}

/**
 * 类名合并
 *
 * 接受任意嵌套的条件值并过滤掉 falsy 项，便于条件拼接。
 * 签名与 clsx 的 ClassValue 保持一致的宽松度，避免 `cond && 'class'` 时
 * 出现 `0 | undefined` 之类的类型噪音。
 */
export type ClassValue =
  | string
  | number
  | boolean
  | null
  | undefined
  | ClassValue[];

export function cn(...classes: ClassValue[]): string {
  const out: string[] = [];

  const walk = (value: ClassValue): void => {
    if (!value) return;
    if (Array.isArray(value)) {
      value.forEach(walk);
      return;
    }
    if (typeof value === 'string' || typeof value === 'number') {
      out.push(String(value));
    }
  };

  classes.forEach(walk);
  return out.join(' ');
}

/** 截断长文本，保留指定字符数 */
export function truncate(text: string, max = 80): string {
  const chars = Array.from(text);
  if (chars.length <= max) return text;
  return `${chars.slice(0, max).join('')}…`;
}

/** 复制文本到剪贴板，带降级方案（非安全上下文下 navigator.clipboard 不可用） */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.style.position = 'fixed';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(textarea);
    return ok;
  } catch {
    return false;
  }
}
