/**
 * 记忆管理 Hook
 *
 * 统一处理列表分页、类型筛选、关键词搜索与统计。
 * 搜索与列表是后端两个不同端点，这里根据是否有关键词自动切换，
 * 并把两者归一化为同一个 items 数组供界面消费。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { describeError, memoryAPI, normalizeError } from '../services/api';
import type { MemoryEntry, MemoryStats, MemoryType } from '../types';

/** 每页条数 */
export const MEMORY_PAGE_SIZE = 12;

export interface UseMemoriesResult {
  items: MemoryEntry[];
  total: number;
  page: number;
  pageSize: number;
  /** 记忆系统是否启用（后端返回 enabled 字段） */
  enabled: boolean;
  stats: MemoryStats | null;

  loading: boolean;
  loadingStats: boolean;
  error: string | null;
  offline: boolean;
  /** 正在删除的记忆 id 集合，用于按钮加载态 */
  deletingIds: ReadonlySet<number>;

  /** 当前筛选条件 */
  type: MemoryType | 'all';
  keyword: string;
  minImportance: number;

  setType: (type: MemoryType | 'all') => void;
  setKeyword: (keyword: string) => void;
  setMinImportance: (value: number) => void;
  setPage: (page: number) => void;
  reload: () => Promise<void>;
  remove: (id: number) => Promise<void>;
}

export function useMemories(): UseMemoriesResult {
  const [items, setItems] = useState<MemoryEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [enabled, setEnabled] = useState(true);
  const [stats, setStats] = useState<MemoryStats | null>(null);

  const [type, setTypeState] = useState<MemoryType | 'all'>('all');
  const [keyword, setKeywordState] = useState('');
  const [minImportance, setMinImportanceState] = useState(0);

  const [loading, setLoading] = useState(false);
  const [loadingStats, setLoadingStats] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [offline, setOffline] = useState(false);
  const [deletingIds, setDeletingIds] = useState<Set<number>>(new Set());

  const mountedRef = useRef(true);
  /** 请求序号：防止慢请求覆盖快请求的结果 */
  const requestSeqRef = useRef(0);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  /** 拉取当前页数据 */
  const fetchList = useCallback(async () => {
    const seq = requestSeqRef.current + 1;
    requestSeqRef.current = seq;

    setLoading(true);
    try {
      const trimmedKeyword = keyword.trim();

      // 有搜索词时走 search 端点（后端对 content/summary 做匹配）
      if (trimmedKeyword !== '') {
        const result = await memoryAPI.search(trimmedKeyword, 50);
        if (!mountedRef.current || requestSeqRef.current !== seq) return;

        // 本地按类型与重要度做二次过滤，保证筛选条件与搜索结果一致
        const filtered = (result.items ?? []).filter((item) => {
          if (type !== 'all' && item.contentType !== type) return false;
          if (minImportance > 0 && item.importanceScore < minImportance) return false;
          return true;
        });

        setItems(filtered);
        setTotal(filtered.length);
        setEnabled(result.enabled !== false);
      } else {
        const result = await memoryAPI.list({
          page,
          pageSize: MEMORY_PAGE_SIZE,
          type: type === 'all' ? undefined : type,
          minImportance: minImportance > 0 ? minImportance : undefined,
        });
        if (!mountedRef.current || requestSeqRef.current !== seq) return;

        setItems(result.items ?? []);
        setTotal(result.total ?? 0);
        setEnabled(result.enabled !== false);
      }

      setError(null);
      setOffline(false);
    } catch (err) {
      if (!mountedRef.current || requestSeqRef.current !== seq) return;
      const normalized = normalizeError(err);
      setOffline(normalized.isOffline);
      setError(describeError(err));
      setItems([]);
      setTotal(0);
    } finally {
      if (mountedRef.current && requestSeqRef.current === seq) setLoading(false);
    }
  }, [keyword, minImportance, page, type]);

  /** 拉取统计 */
  const fetchStats = useCallback(async () => {
    setLoadingStats(true);
    try {
      const result = await memoryAPI.stats();
      if (!mountedRef.current) return;
      setStats(result);
      setEnabled(result.enabled !== false);
    } catch (err) {
      // 统计失败不影响列表展示，只在离线时同步状态
      if (!mountedRef.current) return;
      const normalized = normalizeError(err);
      if (normalized.isOffline) setOffline(true);
    } finally {
      if (mountedRef.current) setLoadingStats(false);
    }
  }, []);

  useEffect(() => {
    void fetchList();
  }, [fetchList]);

  useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  const reload = useCallback(async () => {
    await Promise.all([fetchList(), fetchStats()]);
  }, [fetchList, fetchStats]);

  /** 删除一条记忆（后端为软删除） */
  const remove = useCallback(
    async (id: number) => {
      setDeletingIds((prev) => new Set(prev).add(id));
      try {
        await memoryAPI.remove(id);
        if (!mountedRef.current) return;

        // 乐观移除，并按分页规则决定是否需要回补一页数据
        setItems((prev) => prev.filter((item) => item.id !== id));
        setTotal((prev) => Math.max(0, prev - 1));
        setStats((prev) => (prev ? { ...prev, total: Math.max(0, prev.total - 1) } : prev));
      } finally {
        if (mountedRef.current) {
          setDeletingIds((prev) => {
            const next = new Set(prev);
            next.delete(id);
            return next;
          });
        }
      }
    },
    [],
  );

  /** 筛选条件变化时回到第一页，避免停留在越界页码 */
  const setType = useCallback((next: MemoryType | 'all') => {
    setTypeState(next);
    setPage(1);
  }, []);

  const setKeyword = useCallback((next: string) => {
    setKeywordState(next);
    setPage(1);
  }, []);

  const setMinImportance = useCallback((next: number) => {
    setMinImportanceState(next);
    setPage(1);
  }, []);

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(total / MEMORY_PAGE_SIZE)),
    [total],
  );

  return {
    items,
    total: keyword.trim() === '' ? total : items.length,
    page,
    pageSize: MEMORY_PAGE_SIZE,
    enabled,
    stats,
    loading,
    loadingStats,
    error,
    offline,
    deletingIds,
    type,
    keyword,
    minImportance,
    setType,
    setKeyword,
    setMinImportance,
    setPage: (next: number) => setPage(Math.min(Math.max(1, next), totalPages)),
    reload,
    remove,
  };
}
