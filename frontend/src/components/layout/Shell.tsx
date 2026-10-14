/**
 * 应用外壳与页面容器
 */

import type { ReactNode } from 'react';
import { cn } from '../../utils/format';
import { IconAlert, IconRefresh } from '../icons';
import { Button } from '../ui';

/**
 * 离线提示条
 *
 * 后端未启动是桌面端最常见的失败场景，这里给出可操作的提示
 * （启动命令 + 重试按钮），而不是让用户面对空白的列表。
 */
export function OfflineBanner({
  message,
  onRetry,
  retrying = false,
  className,
}: {
  message?: string;
  onRetry?: () => void;
  retrying?: boolean;
  className?: string;
}) {
  return (
    <div
      role="alert"
      className={cn(
        'flex items-center gap-3 border border-warning/25 bg-warning/10 px-4 py-2.5 rounded-md',
        'animate-slide-down',
        className,
      )}
    >
      <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-warning/20 text-warning">
        <IconAlert size={14} />
      </span>

      <div className="min-w-0 flex-1">
        <p className="text-xs font-medium text-ink-1">无法连接到 Soren 后端服务</p>
        <p className="mt-0.5 text-xs leading-5 text-ink-2">
          {message ?? '本地服务暂未就绪。请稍后重试，或在设置中检查服务状态。'}

        </p>
      </div>

      {onRetry && (
        <Button
          size="sm"
          variant="secondary"
          loading={retrying}
          onClick={onRetry}
          leadingIcon={retrying ? undefined : <IconRefresh size={13} />}
        >
          重试
        </Button>
      )}
    </div>
  );
}

/**
 * 页面容器
 *
 * 统一各页面的最大宽度、内边距与可选极光背景，
 * 避免每个页面各自定义间距导致视觉节奏不一致。
 */
export function PageContainer({
  children,
  /** 内容最大宽度类名 */
  widthClass = 'max-w-5xl',
  /** 是否渲染底层极光光晕 */
  aurora = false,
  className,
  /** 内容区是否可滚动 */
  scrollable = true,
}: {
  children: ReactNode;
  widthClass?: string;
  aurora?: boolean;
  className?: string;
  scrollable?: boolean;
}) {
  return (
    <div className={cn('page-container relative min-h-0 flex-1', className)}>
      {aurora && (
        <div
          className="pointer-events-none absolute inset-0 bg-aurora opacity-90"
          aria-hidden="true"
        />
      )}

      <div
        className={cn(
          'page-container-inner relative mx-auto w-full px-6 py-6',
          widthClass,
          scrollable && 'h-full overflow-y-auto',
        )}
      >
        {children}
      </div>
    </div>
  );
}

/** 页面级标题区：大标题 + 说明 + 右侧操作 */
export function PageHeading({
  title,
  description,
  actions,
  icon,
}: {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  icon?: ReactNode;
}) {
  return (
    <div className="page-heading mb-6 flex items-start justify-between gap-6">
      <div className="flex min-w-0 items-start gap-3">
        {icon && (
          <span className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-gradient-primary text-white shadow-sm">
            {icon}
          </span>
        )}
        <div className="min-w-0">
          <h1 className="text-xl font-semibold tracking-tight text-ink-1">{title}</h1>
          {description && (
            <p className="mt-1.5 max-w-2xl text-xs leading-6 text-ink-2">{description}</p>
          )}
        </div>
      </div>
      {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
    </div>
  );
}
