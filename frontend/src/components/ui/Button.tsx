/**
 * 基础交互组件：按钮、图标按钮、加载指示器
 */

import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { cn } from '../../utils/format';

/** 按钮视觉变体 */
export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

/** 兼容已有调用；所有尺寸统一使用界面的标准操作尺寸。 */
export type ButtonSize = 'sm' | 'md' | 'lg';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** 前置图标 */
  leadingIcon?: ReactNode;
  /** 后置图标 */
  trailingIcon?: ReactNode;
  /** 加载中：显示旋转指示器并禁用交互 */
  loading?: boolean;
  /** 占满父容器宽度 */
  block?: boolean;
}

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  ghost: 'btn-ghost',
  danger: 'btn-danger',
};

const SIZE_CLASS: Record<ButtonSize, string> = {
  sm: 'control-button',
  md: 'control-button',
  lg: 'control-button',
};

/** 通用按钮 */
export function Button({
  variant = 'secondary',
  size = 'md',
  leadingIcon,
  trailingIcon,
  loading = false,
  block = false,
  disabled,
  className,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(
        VARIANT_CLASS[variant],
        SIZE_CLASS[size],
        block && 'w-full',
        className,
      )}
      {...rest}
    >
      {loading ? <Spinner size={16} /> : leadingIcon}
      {children}
      {!loading && trailingIcon}
    </button>
  );
}

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** 无障碍标签（必填，图标按钮无可见文本） */
  label: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
  /** 是否处于激活/选中态 */
  active?: boolean;
}

const ICON_BOX_CLASS: Record<ButtonSize, string> = {
  sm: 'control-icon',
  md: 'control-icon',
  lg: 'control-icon',
};

/**
 * 图标按钮
 *
 * label 为必填项：图标按钮没有可见文本，必须提供 aria-label 才能真正可用。
 */
export function IconButton({
  label,
  variant = 'ghost',
  size = 'md',
  active = false,
  className,
  children,
  type = 'button',
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      title={label}
      aria-pressed={active || undefined}
      className={cn(
        VARIANT_CLASS[variant],
        ICON_BOX_CLASS[size],
        '!px-0 shrink-0',
        active && 'text-primary-start bg-surface-3',
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
}

/** 旋转加载指示器 */
export function Spinner({
  size = 16,
  className,
}: {
  size?: number;
  className?: string;
}) {
  return (
    <span
      role="status"
      aria-label="加载中"
      className={cn(
        'inline-block shrink-0 animate-spin rounded-full border-2 border-current border-r-transparent align-[-0.125em]',
        className,
      )}
      style={{ width: size, height: size }}
    />
  );
}

/** 三点跳动指示器，用于"AI 正在思考" */
export function TypingDots({ className }: { className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1', className)} aria-label="正在输入">
      {[0, 1, 2].map((index) => (
        <span
          key={index}
          className="h-1.5 w-1.5 rounded-full bg-current animate-bounce-dot"
          style={{ animationDelay: `${index * 0.16}s` }}
        />
      ))}
    </span>
  );
}
