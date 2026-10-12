/**
 * 通用小工具 Hook 集合
 */

import { useEffect, useState } from 'react';

/**
 * 防抖值
 *
 * 用于搜索框：用户连续输入时只在停顿后触发一次请求，
 * 避免每敲一个字符都打一次后端。
 */
export function useDebouncedValue<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value), delay);
    return () => window.clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}

/**
 * 元素滚动位置监听
 *
 * 用于判断消息列表是否已滚到底部，从而决定是否显示"回到底部"按钮。
 */
export function useScrollPosition<T extends HTMLElement>(
  ref: React.RefObject<T>,
  /** 距底部多少像素内视为"在底部" */
  threshold = 80,
): { atBottom: boolean; scrollToBottom: (behavior?: ScrollBehavior) => void } {
  const [atBottom, setAtBottom] = useState(true);

  useEffect(() => {
    const element = ref.current;
    if (!element) return;

    const handleScroll = () => {
      const distance = element.scrollHeight - element.scrollTop - element.clientHeight;
      setAtBottom(distance <= threshold);
    };

    handleScroll();
    element.addEventListener('scroll', handleScroll, { passive: true });
    return () => element.removeEventListener('scroll', handleScroll);
  }, [ref, threshold]);

  const scrollToBottom = (behavior: ScrollBehavior = 'smooth') => {
    const element = ref.current;
    if (!element) return;
    element.scrollTo({ top: element.scrollHeight, behavior });
  };

  return { atBottom, scrollToBottom };
}

/**
 * 全局键盘快捷键
 *
 * 用于 Cmd/Ctrl+N 新建对话、Cmd/Ctrl+K 聚焦输入框等。
 * 在输入框内输入时不会被拦截（除非显式允许）。
 */
export function useHotkey(
  key: string,
  handler: (event: KeyboardEvent) => void,
  options: { meta?: boolean; shift?: boolean; allowInInput?: boolean } = {},
): void {
  const { meta = false, shift = false, allowInInput = false } = options;

  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      const isEditable =
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target?.isContentEditable === true;

      if (isEditable && !allowInInput) return;
      if (event.key.toLowerCase() !== key.toLowerCase()) return;
      if (meta && !(event.metaKey || event.ctrlKey)) return;
      if (shift && !event.shiftKey) return;

      handler(event);
    };

    window.addEventListener('keydown', listener);
    return () => window.removeEventListener('keydown', listener);
  }, [key, handler, meta, shift, allowInInput]);
}
