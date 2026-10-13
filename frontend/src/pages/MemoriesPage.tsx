/**
 * 记忆管理页
 *
 * 这是 Soren 的核心卖点界面：用户能在此看到 AI 究竟记住了什么，
 * 并按类型筛选、搜索、查看重要度、删除记错的内容。
 *
 * 布局：统计概览 → 筛选工具条 → 记忆卡片列表 → 分页。
 */

import { useEffect, useMemo, useState } from 'react';
import { cn, formatCompactNumber, formatRelativeTime } from '../utils/format';
import { MemoryCard } from '../components/memories/MemoryCard';
import { MemoryEditor } from '../components/memories/MemoryEditor';
import { OfflineBanner, PageContainer, PageHeading } from '../components/layout/Shell';
import {
  IconActivity,
  IconBrain,
  IconClock,
  IconDatabase,
  IconFilter,
  IconInbox,
  IconRefresh,
  IconSearch,
  IconTag,
} from '../components/icons';
import {
  Button,
  ConfirmDialog,
  EmptyState,
  IconButton,
  SelectField,
  SkeletonList,
  StatTile,
} from '../components/ui';
import { useDebouncedValue } from '../hooks/useUtilities';
import type { UseMemoriesResult } from '../hooks/useMemories';
import { MEMORY_TYPE_LABELS, MEMORY_TYPES, type MemoryEntry, type MemoryType } from '../types';

export interface MemoriesPageProps {
  memories: UseMemoriesResult;
  /** 后端离线 */
  offline: boolean;
  onRetryConnection?: () => void;
  /** 删除成功后通知外部刷新侧栏徽标 */
  onDeleted?: () => void;
}

/** 类型筛选项：全部 + 四种类型 */
const TYPE_FILTERS: Array<{ value: MemoryType | 'all'; label: string }> = [
  { value: 'all', label: '全部' },
  ...MEMORY_TYPES.map((type) => ({ value: type, label: MEMORY_TYPE_LABELS[type] })),
];

/** 重要度筛选项 */
const IMPORTANCE_OPTIONS = [
  { value: '0', label: '不限' },
  { value: '40', label: '中等以上（≥40）' },
  { value: '70', label: '高重要度（≥70）' },
  { value: '85', label: '极高（≥85）' },
];

export function MemoriesPage({
  memories,
  offline,
  onRetryConnection,
  onDeleted,
}: MemoriesPageProps) {
  const {
    items,
    total,
    page,
    pageSize,
    enabled,
    stats,
    loading,
    error,
    deletingIds,
    type,
    keyword,
    minImportance,
    setType,
    setKeyword,
    setMinImportance,
    setPage,
    reload,
    remove,
  } = memories;

  /** 输入框的即时值 + 防抖后的查询值 */
  const [searchInput, setSearchInput] = useState(keyword);
  const debouncedSearch = useDebouncedValue(searchInput, 350);

  /** 待确认删除的记忆 */
  const [pendingDelete, setPendingDelete] = useState<MemoryEntry | null>(null);
  const [deleting, setDeleting] = useState(false);

  // 防抖值变化后同步到 hook（触发查询）
  useEffect(() => {
    setKeyword(debouncedSearch);
  }, [debouncedSearch, setKeyword]);

  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  /** 触发刷新（同时更新统计） */
  const handleRefresh = () => {
    void reload();
  };

  const handleConfirmDelete = async () => {
    if (!pendingDelete) return;
    setDeleting(true);
    try {
      await remove(pendingDelete.id);
      setPendingDelete(null);
      onDeleted?.();
    } catch {
      // 错误详情由 useMemories 的 error 状态呈现
    } finally {
      setDeleting(false);
    }
  };

  /** 统计卡片数据 */
  const statTiles = useMemo(
    () => [
      {
        label: '记忆总数',
        value: formatCompactNumber(stats?.total ?? total),
        hint: stats?.newestCreatedAt
          ? `最近更新 ${formatRelativeTime(stats.newestCreatedAt)}`
          : '暂无记录',
        icon: <IconDatabase size={16} />,
        accentClass: 'text-primary-start bg-primary-start/10',
      },
      {
        label: '高重要度',
        value: formatCompactNumber(stats?.highImportance ?? 0),
        hint: '评分 ≥ 70，优先被召回',
        icon: <IconActivity size={16} />,
        accentClass: 'text-warning bg-warning/10',
      },
      {
        label: '近期使用',
        value: formatCompactNumber(stats?.accessedRecently ?? 0),
        hint: '最近被召回过',
        icon: <IconClock size={16} />,
        accentClass: 'text-info bg-info/10',
      },
      {
        label: '关联标签',
        value: formatCompactNumber(stats?.totalTags ?? 0),
        hint: '用于主题聚类',
        icon: <IconTag size={16} />,
        accentClass: 'text-accent-pink bg-accent-pink/10',
      },
    ],
    [stats, total],
  );

  return (
    <PageContainer widthClass="max-w-6xl" aurora>
      <PageHeading
        icon={<IconBrain size={18} />}
        title="记忆"

        actions={
          <>
            <MemoryEditor onSaved={() => void handleRefresh()} />
            <Button
              size="sm"
              variant="ghost"
              leadingIcon={<IconRefresh size={14} />}
              onClick={handleRefresh}
              disabled={loading}
            >
              刷新
            </Button>
          </>
        }
      />

      {offline && <OfflineBanner onRetry={onRetryConnection} className="mb-5" />}

      {/* ── 记忆系统被禁用 ─────────────────────────────── */}
      {!enabled && (
        <div className="mb-5 rounded-lg border border-warning/30 bg-warning/10 px-4 py-3">
          <p className="text-xs font-medium text-ink-1">记忆系统当前处于关闭状态</p>
          <p className="mt-1 text-xs leading-5 text-ink-2">
            后端启动时未启用长期记忆，因此不会有新的记忆写入。
            可在后端环境变量中设置 <span className="font-mono">MEMORY_ENABLED=true</span> 后重启服务。
          </p>
        </div>
      )}

      {/* ── 统计概览 ───────────────────────────────────── */}
      <div className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        {statTiles.map((tile) => (
          <StatTile
            key={tile.label}
            label={tile.label}
            value={tile.value}
            hint={tile.hint}
            icon={tile.icon}
            accentClass={tile.accentClass}
          />
        ))}
      </div>

      {/* ── 筛选工具条 ─────────────────────────────────── */}
      <div
        className={cn(
          'sticky top-0 z-10 mb-4 rounded-lg border border-line bg-surface-2/85 p-3',
          'ring-highlight backdrop-blur-glass',
        )}
      >
        <div className="flex flex-wrap items-center gap-3">
          {/* 搜索框 */}
          <div className="relative min-w-[220px] flex-1">
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-3">
              <IconSearch size={15} />
            </span>
            <input
              type="search"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="搜索记忆内容或摘要…"
              aria-label="搜索记忆"
              className="field h-9 pl-9 pr-9"
            />
            {searchInput && (
              <IconButton
                label="清空搜索"
                size="sm"
                onClick={() => setSearchInput('')}
                className="absolute right-1.5 top-1/2 -translate-y-1/2"
              >
                <span className="text-sm leading-none">×</span>
              </IconButton>
            )}
          </div>

          {/* 重要度筛选 */}
          <div className="w-[180px] shrink-0">
            <SelectField
              aria-label="按重要度筛选"
              options={IMPORTANCE_OPTIONS}
              value={String(minImportance)}
              onChange={(value) => setMinImportance(Number(value))}
            />
          </div>
        </div>

        {/* 类型筛选 chips */}
        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          <IconFilter size={13} className="mr-0.5 text-ink-3" />
          {TYPE_FILTERS.map((filter) => {
            const active = type === filter.value;
            const count =
              filter.value === 'all'
                ? stats?.total
                : stats?.byType?.[filter.value];

            return (
              <button
                key={filter.value}
                type="button"
                onClick={() => setType(filter.value)}
                aria-pressed={active}
                className={cn(
                  'control-button inline-flex items-center justify-center gap-2 border',
                  'transition-all duration-fast',
                  active
                    ? 'border-primary-start/50 bg-primary-start/15 font-medium text-primary-start'
                    : 'border-line bg-surface-2 text-ink-2 hover:border-line-strong hover:text-ink-1',
                )}
              >
                {filter.label}
                {typeof count === 'number' && count > 0 && (
                  <span className="font-mono text-xs opacity-70">{count}</span>
                )}
              </button>
            );
          })}
        </div>
      </div>

      {/* ── 列表 ───────────────────────────────────────── */}
      {loading ? (
        <SkeletonList rows={4} />
      ) : error && items.length === 0 ? (
        <EmptyState
          icon={<IconInbox size={22} />}
          title="无法加载记忆"
          description={error}
          action={
            <Button variant="secondary" onClick={handleRefresh}>
              重新加载
            </Button>
          }
        />
      ) : items.length === 0 ? (
        <EmptyState
          icon={keyword ? <IconSearch size={22} /> : <IconBrain size={22} />}
          title={keyword ? `没有匹配「${keyword}」的记忆` : '还没有任何记忆'}
          description={
            keyword
              ? '试试换个关键词，或清空搜索条件查看全部记忆。'
              : '当前 Bot 的私有记忆会显示在这里。'
          }
          action={
            keyword ? (
              <Button variant="secondary" onClick={() => setSearchInput('')}>
                清空搜索
              </Button>
            ) : undefined
          }
        />
      ) : (
        <>
          <div className="memory-list">
            {items.map((memory) => (
              <MemoryCard
                key={memory.id}
                memory={memory}
                deleting={deletingIds.has(memory.id)}
                onDelete={setPendingDelete}
                onEdited={() => void handleRefresh()}
              />
            ))}
          </div>

          {/* 分页：搜索模式下后端返回的是完整结果集，不显示分页 */}
          {keyword.trim() === '' && totalPages > 1 && (
            <nav
              className="mt-6 flex items-center justify-center gap-2"
              aria-label="记忆分页"
            >
              <Button
                size="sm"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
              >
                上一页
              </Button>

              <span className="px-2 font-mono text-xs text-ink-2">
                {page} / {totalPages}
              </span>

              <Button
                size="sm"
                variant="secondary"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
              >
                下一页
              </Button>
            </nav>
          )}

          {/* 结果计数 */}
          <p className="mt-4 text-center text-xs text-ink-3">
            共 {total} 条记忆
            {keyword.trim() !== '' && `（搜索「${keyword}」）`}
          </p>
        </>
      )}

      {/* 删除确认：软删除仍然不可逆地"从视图消失"，需要确认 */}
      <ConfirmDialog
        open={pendingDelete !== null}
        danger
        loading={deleting}
        title="删除这条记忆？"
        message={
          <>
            删除后 Soren 将不再回忆起这条内容。
            <span className="mt-2 block rounded border border-line bg-surface-3 px-3 py-2 font-mono text-xs leading-5 text-ink-2">
              {pendingDelete?.content.slice(0, 120) ?? ''}
              {(pendingDelete?.content.length ?? 0) > 120 ? '…' : ''}
            </span>
          </>
        }
        confirmText="删除"
        onConfirm={handleConfirmDelete}
        onCancel={() => setPendingDelete(null)}
      />
    </PageContainer>
  );
}
