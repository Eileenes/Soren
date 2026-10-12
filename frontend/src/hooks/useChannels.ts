/**
 * 消息渠道 Hook
 *
 * 负责渠道状态列表的加载与启停操作。
 * 后端启停接口会同步返回最新状态，因此操作成功后直接用返回值做本地更新，
 * 避免额外的整表刷新。
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { channelAPI, describeError, normalizeError } from '../services/api';
import type { ChannelStatus } from '../types';

export interface UseChannelsResult {
  channels: ChannelStatus[];
  loading: boolean;
  error: string | null;
  offline: boolean;
  /** 正在切换状态的渠道名，用于开关的加载态 */
  pendingName: string | null;
  reload: () => Promise<void>;
  /** 启用渠道，失败时抛出以便界面提示具体原因 */
  enable: (name: string) => Promise<void>;
  disable: (name: string) => Promise<void>;
}

export function useChannels(): UseChannelsResult {
  const [channels, setChannels] = useState<ChannelStatus[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [pendingName, setPendingName] = useState<string | null>(null);

  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const list = await channelAPI.list();
      if (!mountedRef.current) return;
      setChannels(list);
      setError(null);
      setOffline(false);
    } catch (err) {
      if (!mountedRef.current) return;
      const normalized = normalizeError(err);
      setOffline(normalized.isOffline);
      setError(describeError(err));
      setChannels([]);
    } finally {
      if (mountedRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  /** 更新单个渠道状态 */
  const applyStatus = useCallback((status: ChannelStatus) => {
    setChannels((prev) => {
      const index = prev.findIndex((item) => item.name === status.name);
      if (index === -1) return [...prev, status];
      return prev.map((item) => (item.name === status.name ? status : item));
    });
  }, []);

  const toggle = useCallback(
    async (name: string, shouldEnable: boolean) => {
      setPendingName(name);
      try {
        const status = shouldEnable
          ? await channelAPI.enable(name)
          : await channelAPI.disable(name);
        if (!mountedRef.current) return;
        applyStatus(status);
      } catch (err) {
        if (!mountedRef.current) return;
        const normalized = normalizeError(err);
        // 启动失败时后端会带上 lastError，重新拉取一次让界面显示真实状态
        if (normalized.status === 400) {
          void reload();
        }
        throw err;
      } finally {
        if (mountedRef.current) setPendingName(null);
      }
    },
    [applyStatus, reload],
  );

  const enable = useCallback((name: string) => toggle(name, true), [toggle]);
  const disable = useCallback((name: string) => toggle(name, false), [toggle]);

  return { channels, loading, error, offline, pendingName, reload, enable, disable };
}
