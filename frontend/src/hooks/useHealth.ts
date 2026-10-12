/**
 * 后端连接状态 Hook
 *
 * 桌面端后端是本机进程，可能在应用启动后才就绪，因此需要轮询并在
 * 窗口重新获得焦点时立即复查，避免用户切回来看到过期的"离线"状态。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { healthAPI, normalizeError } from '../services/api';
import type { ConnectionState, HealthStatus } from '../types';

/** 轮询间隔：在线时放宽，离线时收紧，兼顾"快速发现恢复"与"不做无谓请求" */
const POLL_INTERVAL_ONLINE = 30000;
const POLL_INTERVAL_OFFLINE = 5000;

export interface UseHealthResult {
  state: ConnectionState;
  health: HealthStatus | null;
  /** 最近一次失败原因 */
  error: string | null;
  /** 手动重新检测 */
  refresh: () => Promise<void>;
  /** 最近一次成功检测的时间戳 */
  lastCheckedAt: number | null;
}

export function useHealth(): UseHealthResult {
  const [state, setState] = useState<ConnectionState>('connecting');
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastCheckedAt, setLastCheckedAt] = useState<number | null>(null);

  // 用 ref 保存控制器，组件卸载或重复调用时取消上一次请求
  const abortRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);

  const refresh = useCallback(async () => {
    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setState((prev) => (prev === 'online' ? prev : 'connecting'));

    try {
      const result = await healthAPI.check(controller.signal);
      if (!mountedRef.current || controller.signal.aborted) return;
      setHealth(result);
      setError(null);
      setState(result.status === 'ok' ? 'online' : 'offline');
      setLastCheckedAt(Date.now());
    } catch (err) {
      if (!mountedRef.current || controller.signal.aborted) return;
      const normalized = normalizeError(err);
      setHealth(null);
      setError(normalized.message);
      setState('offline');
      setLastCheckedAt(Date.now());
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    void refresh();

    return () => {
      mountedRef.current = false;
      abortRef.current?.abort();
    };
  }, [refresh]);

  // 自适应轮询
  useEffect(() => {
    const interval = state === 'online' ? POLL_INTERVAL_ONLINE : POLL_INTERVAL_OFFLINE;
    const timer = window.setInterval(() => {
      void refresh();
    }, interval);
    return () => window.clearInterval(timer);
  }, [state, refresh]);

  // 窗口重新聚焦时立即复查
  useEffect(() => {
    const onFocus = () => {
      void refresh();
    };
    window.addEventListener('focus', onFocus);
    return () => window.removeEventListener('focus', onFocus);
  }, [refresh]);

  return { state, health, error, refresh, lastCheckedAt };
}
