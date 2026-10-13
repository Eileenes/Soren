/**
 * 模态框与确认对话框
 *
 * 无障碍要点：Escape 关闭、遮罩点击关闭、打开时焦点移入对话框、
 * Tab 在对话框内循环（简易焦点陷阱）、关闭后焦点归还触发元素。
 */

import { useCallback, useEffect, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { cn } from '../../utils/format';
import { Button, IconButton } from './Button';
import { IconAlert, IconClose } from '../icons';

export interface ModalProps {
  open: boolean;
  compact?: boolean;
  onClose: () => void;
  title?: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  /** 底部操作区 */
  footer?: ReactNode;
  /** 宽度尺寸 */
  size?: 'sm' | 'md' | 'lg';
  /** 是否允许点击遮罩关闭，默认允许 */
  closeOnBackdrop?: boolean;
}

const MODAL_WIDTH: Record<NonNullable<ModalProps['size']>, string> = {
  sm: 'max-w-sm',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
};

/** 通用模态框 */
export function Modal({
  open,
  compact = false,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
  closeOnBackdrop = true,
}: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  // 打开时记录触发元素，关闭后归还焦点
  useEffect(() => {
    if (!open) return;
    previousFocusRef.current = document.activeElement as HTMLElement | null;
    return () => {
      previousFocusRef.current?.focus?.();
    };
  }, [open]);

  // Escape 关闭 + Tab 焦点循环
  useEffect(() => {
    if (!open) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation();
        onClose();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;

      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;

      if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [open, onClose]);

  // 打开后把焦点移入对话框
  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => {
      const target = dialogRef.current?.querySelector<HTMLElement>(
        'input:not([disabled]), textarea:not([disabled]), button:not([disabled])',
      );
      (target ?? dialogRef.current)?.focus();
    }, 40);
    return () => window.clearTimeout(timer);
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-6"
      role="presentation"
    >
      {/* 遮罩 */}
      <div
        className="absolute inset-0 bg-black/55 backdrop-blur-sm animate-fade-in"
        onClick={closeOnBackdrop ? onClose : undefined}
        aria-hidden="true"
      />

      {/* 对话框 */}
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={typeof title === 'string' ? title : undefined}
        tabIndex={-1}
        className={cn(
          'relative w-full glass-panel-strong rounded-xl shadow-xl animate-scale',
          'ring-highlight flex flex-col max-h-[90dvh] overflow-hidden',
          MODAL_WIDTH[size],
          compact && 'search-palette',
        )}
      >
        {/* 顶部渐变高光线，呼应品牌 */}
        <div className="h-px shrink-0 w-full bg-gradient-to-r from-transparent via-primary-start/70 to-transparent" />

        {!compact && (title || description) && (
          <div className="flex shrink-0 items-start justify-between gap-4 px-6 pt-5 pb-1">
            <div className="min-w-0">
              {title && (
                <h2 className="text-base font-semibold tracking-tight text-ink-1">{title}</h2>
              )}
              {description && (
                <p className="mt-1 text-xs leading-5 text-ink-2">{description}</p>
              )}
            </div>
            <IconButton label="关闭" onClick={onClose} size="sm">
              <IconClose size={16} />
            </IconButton>
          </div>
        )}

        {children && <div className={cn("min-h-0 overflow-y-auto", compact ? "p-2" : "px-6 py-4")}>{children}</div>}

        {footer && (
          <div className="flex shrink-0 items-center justify-end gap-2 border-t border-line-subtle bg-surface-3/40 px-6 py-4">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}

export interface ConfirmDialogProps {
  open: boolean;
  title: string;
  message: ReactNode;
  confirmText?: string;
  cancelText?: string;
  /** 危险操作使用红色确认按钮 */
  danger?: boolean;
  /** 确认按钮加载态 */
  loading?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * 确认对话框
 *
 * 替代 window.confirm：原生 confirm 在 Tauri 中会阻塞渲染进程且样式无法定制。
 */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmText = '确认',
  cancelText = '取消',
  danger = false,
  loading = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const handleCancel = useCallback(() => {
    if (!loading) onCancel();
  }, [loading, onCancel]);

  return (
    <Modal
      open={open}
      onClose={handleCancel}
      size="sm"
      closeOnBackdrop={!loading}
      footer={
        <>
          <Button variant="ghost" onClick={handleCancel} disabled={loading}>
            {cancelText}
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            onClick={onConfirm}
            loading={loading}
          >
            {confirmText}
          </Button>
        </>
      }
    >
      <div className="flex gap-3">
        <div
          className={cn(
            'mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full',
            danger ? 'bg-error/15 text-error' : 'bg-primary-start/15 text-primary-start',
          )}
        >
          <IconAlert size={18} />
        </div>
        <div className="min-w-0 flex-1">
          <h3 className="text-sm font-semibold text-ink-1">{title}</h3>
          <div className="mt-1.5 text-xs leading-6 text-ink-2">{message}</div>
        </div>
      </div>
    </Modal>
  );
}
