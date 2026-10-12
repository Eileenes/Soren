/**
 * 轻量通知（Toast）
 *
 * 通过 useToast() 拿到 push 方法；ToastViewport 挂在 App 根部渲染。
 * 用于「已复制」「已删除」「渠道启动失败」等即时反馈，避免滥用模态框。
 */

import { useEffect, type ReactNode } from 'react';
import { cn } from '../../utils/format';
import { IconAlert, IconCheck, IconClose } from '../icons';

/** 通知类型 */
export type ToastVariant = 'success' | 'error' | 'info';

/** 一条通知 */
export interface ToastItem {
  id: string;
  variant: ToastVariant;
  title: string;
  description?: string;
  /** 毫秒，0 表示不自动关闭 */
  duration: number;
}

const VARIANT_STYLE: Record<ToastVariant, { icon: ReactNode; accent: string }> = {
  success: {
    icon: <IconCheck size={14} />,
    accent: 'bg-success/15 text-success border-success/30',
  },
  error: {
    icon: <IconAlert size={14} />,
    accent: 'bg-error/15 text-error border-error/30',
  },
  info: {
    icon: <IconAlert size={14} />,
    accent: 'bg-info/15 text-info border-info/30',
  },
};

/** 单条通知 */
export function Toast({
  item,
  onDismiss,
}: {
  item: ToastItem;
  onDismiss: (id: string) => void;
}) {
  const { id, variant, title, description, duration } = item;
  const style = VARIANT_STYLE[variant];

  // 定时自动关闭；duration 为 0 时保持常驻
  useEffect(() => {
    if (duration <= 0) return;
    const timer = window.setTimeout(() => onDismiss(id), duration);
    return () => window.clearTimeout(timer);
  }, [id, duration, onDismiss]);

  return (
    <div
      role={variant === 'error' ? 'alert' : 'status'}
      className={cn(
        'pointer-events-auto flex w-[340px] items-start gap-3',
        'glass-panel-strong rounded-lg px-4 py-3 shadow-lg ring-highlight',
        'animate-slide-up',
      )}
    >
      <span
        className={cn(
          'mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border',
          style.accent,
        )}
      >
        {style.icon}
      </span>

      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium text-ink-1">{title}</p>
        {description && (
          <p className="mt-1 break-words text-xs leading-5 text-ink-2">{description}</p>
        )}
      </div>

      <button
        type="button"
        onClick={() => onDismiss(id)}
        aria-label="关闭通知"
        className="mt-0.5 rounded p-1 text-ink-3 transition-colors duration-fast hover:text-ink-1"
      >
        <IconClose size={14} />
      </button>
    </div>
  );
}

/** 通知容器：固定在右下角，新通知从下方滑入 */
export function ToastViewport({
  toasts,
  onDismiss,
}: {
  toasts: ToastItem[];
  onDismiss: (id: string) => void;
}) {
  return (
    <div
      className="pointer-events-none fixed bottom-6 right-6 z-[60] flex flex-col gap-2"
      aria-live="polite"
      aria-atomic="false"
    >
      {toasts.map((item) => (
        <Toast key={item.id} item={item} onDismiss={onDismiss} />
      ))}
    </div>
  );
}
