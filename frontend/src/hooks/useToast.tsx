/**
 * 通知上下文
 *
 * 把 useToast 的状态提升到 Provider，使任意页面都能推送通知而无需层层传参。
 */

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { ToastViewport, type ToastItem, type ToastVariant } from '../components/ui/Toast';

interface ToastContextValue {
  /** 推送一条通知，返回其 id */
  push: (title: string, options?: { variant?: ToastVariant; description?: string; duration?: number }) => string;
  success: (title: string, description?: string) => string;
  error: (title: string, description?: string) => string;
  info: (title: string, description?: string) => string;
  dismiss: (id: string) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

/** 通知 id 生成器（不依赖 crypto，避免非安全上下文下不可用） */
let toastSeq = 0;
function nextToastId(): string {
  toastSeq += 1;
  return `toast-${Date.now()}-${toastSeq}`;
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: string) => {
    setToasts((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const push = useCallback<ToastContextValue['push']>(
    (title, options) => {
      const id = nextToastId();
      const item: ToastItem = {
        id,
        title,
        variant: options?.variant ?? 'info',
        description: options?.description,
        // 错误信息停留更久，便于阅读
        duration: options?.duration ?? (options?.variant === 'error' ? 6000 : 3200),
      };
      // 最多同时显示 4 条，超出时移除最旧的
      setToasts((prev) => [...prev, item].slice(-4));
      return id;
    },
    [],
  );

  const value = useMemo<ToastContextValue>(
    () => ({
      push,
      dismiss,
      success: (title, description) => push(title, { variant: 'success', description }),
      error: (title, description) => push(title, { variant: 'error', description }),
      info: (title, description) => push(title, { variant: 'info', description }),
    }),
    [push, dismiss],
  );

  return (
    <ToastContext.Provider value={value}>
      {children}
      <ToastViewport toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  );
}

/** 读取通知上下文 */
export function useToast(): ToastContextValue {
  const context = useContext(ToastContext);
  if (!context) {
    throw new Error('useToast 必须在 <ToastProvider> 内部使用');
  }
  return context;
}
