/**
 * 展示类基础组件：卡片、徽标、骨架屏、空状态、统计块、进度条、提示气泡
 */

import type { ReactNode } from 'react';
import { cn } from '../../utils/format';

/* ── 卡片 ───────────────────────────────────────────────── */

export interface CardProps {
  children: ReactNode;
  className?: string;
  /** 可交互卡片：带悬浮抬升效果 */
  interactive?: boolean;
  /** 选中态描边 */
  selected?: boolean;
  /** 点击回调（传入后自动加 role/键盘支持） */
  onClick?: () => void;
  /** 无障碍标签，onClick 存在时建议提供 */
  ariaLabel?: string;
}

export function Card({
  children,
  className,
  interactive = false,
  selected = false,
  onClick,
  ariaLabel,
}: CardProps) {
  const clickable = Boolean(onClick);

  return (
    <div
      className={cn(
        'relative rounded-lg border bg-surface-2/80 ring-highlight',
        'transition-all duration-base ease-ease',
        selected
          ? 'border-primary-start/60 shadow-[0_0_0_1px_rgb(102_126_234/0.35),0_10px_30px_-14px_rgb(102_126_234/0.6)]'
          : 'border-line',
        interactive && 'hover:bg-surface-3/90 hover:border-line-strong hover:-translate-y-0.5',
        className,
      )}
      onClick={onClick}
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      aria-label={ariaLabel}
      onKeyDown={
        clickable
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                onClick?.();
              }
            }
          : undefined
      }
    >
      {children}
    </div>
  );
}

/** 区块标题：小标题 + 可选说明 + 右侧操作 */
export function SectionHeader({
  title,
  description,
  action,
  icon,
  className,
}: {
  title: ReactNode;
  description?: ReactNode;
  action?: ReactNode;
  icon?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex items-start justify-between gap-4', className)}>
      <div className="flex min-w-0 items-start gap-2.5">
        {icon && (
          <span className="mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded bg-primary-start/10 text-primary-start">
            {icon}
          </span>
        )}
        <div className="min-w-0">
          <h2 className="text-sm font-semibold tracking-tight text-ink-1">{title}</h2>
          {description && (
            <p className="mt-1 text-xs leading-5 text-ink-3">{description}</p>
          )}
        </div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

/* ── 徽标 ───────────────────────────────────────────────── */

export interface BadgeProps {
  children: ReactNode;
  className?: string;
  /** 左侧小圆点 */
  dot?: string;
  /** 前置图标 */
  icon?: ReactNode;
}

export function Badge({ children, className, dot, icon }: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-sm border px-2 py-0.5 text-xs font-medium',
        className ?? 'border-line bg-surface-3 text-ink-2',
      )}
    >
      {dot && <span className={cn('h-1.5 w-1.5 rounded-full', dot)} />}
      {icon}
      {children}
    </span>
  );
}

/* ── 进度条 ─────────────────────────────────────────────── */

export interface ProgressBarProps {
  /** 0-100 */
  value: number;
  /** 填充色类名，默认品牌渐变 */
  barClass?: string;
  className?: string;
  /** 无障碍标签 */
  label?: string;
  /** 是否显示动画生长效果 */
  animate?: boolean;
}

export function ProgressBar({
  value,
  barClass = 'bg-gradient-primary',
  className,
  label,
  animate = true,
}: ProgressBarProps) {
  const clamped = Math.max(0, Math.min(100, Math.round(value)));

  return (
    <div
      className={cn('h-1.5 w-full overflow-hidden rounded-full bg-surface-4', className)}
      role="progressbar"
      aria-valuenow={clamped}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
    >
      <div
        className={cn('h-full rounded-full', barClass, animate && 'origin-left animate-grow-width')}
        style={{ width: `${clamped}%` }}
      />
    </div>
  );
}

/* ── 统计块 ─────────────────────────────────────────────── */

export interface StatTileProps {
  label: string;
  value: ReactNode;
  hint?: string;
  icon?: ReactNode;
  /** 强调色（Tailwind 类名），用于图标底色与数值 */
  accentClass?: string;
  className?: string;
}

/** 统计概览卡片：记忆页与渠道页复用 */
export function StatTile({
  label,
  value,
  hint,
  icon,
  accentClass = 'text-primary-start bg-primary-start/10',
  className,
}: StatTileProps) {
  return (
    <div
      className={cn(
        'group relative overflow-hidden rounded-lg border border-line bg-surface-2/80 p-4 ring-highlight',
        'transition-all duration-base ease-ease hover:border-line-strong hover:bg-surface-3/80',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-medium text-ink-3">{label}</p>
          <p className="mt-1.5 text-2xl font-semibold leading-none tracking-tight text-ink-1">
            {value}
          </p>
          {hint && <p className="mt-1.5 text-xs text-ink-3">{hint}</p>}
        </div>
        {icon && (
          <span
            className={cn(
              'flex h-8 w-8 shrink-0 items-center justify-center rounded transition-transform duration-base ease-ease group-hover:scale-110',
              accentClass,
            )}
          >
            {icon}
          </span>
        )}
      </div>
    </div>
  );
}

/* ── 骨架屏 ─────────────────────────────────────────────── */

export function Skeleton({
  className,
  rounded = 'rounded',
}: {
  className?: string;
  rounded?: string;
}) {
  return (
    <div
      aria-hidden="true"
      className={cn(
        'relative overflow-hidden bg-surface-3',
        rounded,
        'after:absolute after:inset-0 after:animate-shimmer after:bg-skeleton after:bg-[length:200%_100%]',
        className,
      )}
    />
  );
}

/** 列表骨架屏 */
export function SkeletonList({ rows = 4, className }: { rows?: number; className?: string }) {
  return (
    <div className={cn('space-y-3', className)} aria-busy="true" aria-label="加载中">
      {Array.from({ length: rows }).map((_, index) => (
        <div
          key={index}
          className="rounded-lg border border-line bg-surface-2/60 p-4"
          style={{ opacity: 1 - index * 0.12 }}
        >
          <div className="flex items-center gap-2">
            <Skeleton className="h-4 w-16" rounded="rounded-full" />
            <Skeleton className="h-3 w-24" />
          </div>
          <Skeleton className="mt-3 h-3 w-full" />
          <Skeleton className="mt-2 h-3 w-3/5" />
        </div>
      ))}
    </div>
  );
}

/* ── 空状态 ─────────────────────────────────────────────── */

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center px-6 py-14 text-center',
        className,
      )}
    >
      {icon && (
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-xl border border-line bg-surface-3 text-ink-3">
          {icon}
        </div>
      )}
      <h3 className="text-sm font-semibold text-ink-1">{title}</h3>
      {description && (
        <p className="mt-2 max-w-sm text-xs leading-6 text-ink-3">{description}</p>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}

/* ── 提示气泡 ───────────────────────────────────────────── */

export interface TooltipProps {
  content: ReactNode;
  children: ReactNode;
  /** 展示位置 */
  side?: 'top' | 'bottom';
  className?: string;
}

/**
 * 轻量提示气泡
 *
 * 纯 CSS 实现（group-hover / focus-within），不引入定位库；
 * 同时支持键盘聚焦显示，保证可用性。
 */
export function Tooltip({ content, children, side = 'top', className }: TooltipProps) {
  return (
    <span className={cn('group/tip relative inline-flex', className)}>
      {children}
      <span
        role="tooltip"
        className={cn(
          'pointer-events-none absolute left-1/2 z-40 -translate-x-1/2 whitespace-nowrap',
          'rounded-sm border border-line bg-surface-4 px-2 py-1 text-xs text-ink-1',
          'opacity-0 shadow-md transition-all duration-fast ease-ease',
          'group-hover/tip:opacity-100 group-focus-within/tip:opacity-100',
          side === 'top'
            ? 'bottom-full mb-2 translate-y-1 group-hover/tip:translate-y-0'
            : 'top-full mt-2 -translate-y-1 group-hover/tip:translate-y-0',
        )}
      >
        {content}
      </span>
    </span>
  );
}

export { Button, IconButton, Spinner, TypingDots } from './Button';
export type { ButtonProps, IconButtonProps, ButtonVariant, ButtonSize } from './Button';
export {
  TextField,
  PasswordField,
  TextAreaField,
  SelectField,
  Switch,
  RangeField,
  FieldValidMark,
} from './Field';
export type { SelectOption, SwitchProps, RangeFieldProps } from './Field';
export { Modal, ConfirmDialog } from './Modal';
export type { ModalProps, ConfirmDialogProps } from './Modal';
export { Toast, ToastViewport } from './Toast';
export type { ToastItem, ToastVariant } from './Toast';
