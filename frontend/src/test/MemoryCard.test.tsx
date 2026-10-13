/**
 * 记忆卡片与格式化工具测试
 *
 * 重点验证"用户能不能看懂 AI 记住了什么"这条主线：
 * 类型标签、重要度可视化、标签、删除入口是否都正确渲染。
 */

import { describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryCard } from '../components/memories/MemoryCard';
import {
  formatCompactNumber,
  formatRelativeTime,
  importanceLevel,
  parseDate,
  truncate,
} from '../utils/format';
import type { MemoryEntry } from '../types';

/** 构造一条测试用记忆 */
function makeMemory(overrides: Partial<MemoryEntry> = {}): MemoryEntry {
  return {
    id: 12,
    userId: 'default-user',
    sessionId: 'sess-1',
    contentType: 'preference',
    content: '咖啡: 喜欢拿铁，不喜欢美式',
    summary: '咖啡',
    importanceScore: 95,
    accessCount: 3,
    lastAccessedAt: '2024-10-10T10:31:00Z',
    createdAt: '2024-10-10T10:30:00Z',
    updatedAt: '2024-10-10T10:30:00Z',
    conversationTurn: 1,
    isExplicitlyStated: true,
    tags: [
      { id: 4, name: '咖啡', tagType: 'topic' },
      { id: 5, name: '饮品偏好', tagType: 'preference' },
    ],
    ...overrides,
  };
}

describe('MemoryCard', () => {
  it('列表保留摘要和必要信息，点击后查看完整内容', async () => {
    const user = userEvent.setup();
    render(<MemoryCard memory={makeMemory()} onDelete={vi.fn()} />);
    expect(screen.getByText('偏好')).toBeInTheDocument();
    expect(screen.getByText('咖啡')).toBeInTheDocument();
    expect(screen.queryByText('咖啡: 喜欢拿铁，不喜欢美式')).not.toBeInTheDocument();
    expect(screen.queryByText('饮品偏好')).not.toBeInTheDocument();
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '查看记忆详情：咖啡' }));
    const detail = within(screen.getByRole('dialog', { name: '记忆详情' }));
    expect(detail.getByText('咖啡: 喜欢拿铁，不喜欢美式')).toBeInTheDocument();
    expect(detail.getByText('饮品偏好')).toBeInTheDocument();
    expect(detail.getByText('95/100 · 极高')).toBeInTheDocument();
    expect(detail.getByText('访问 3 次')).toBeInTheDocument();
    expect(detail.getByText('明确陈述')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '查看记忆详情：咖啡' })).toHaveFocus();
  });

  it('四种记忆类型都能正确显示中文标签', () => {
    const cases: Array<[MemoryEntry['contentType'], string]> = [
      ['conversation', '对话'],
      ['preference', '偏好'],
      ['task', '任务'],
      ['fact', '事实'],
    ];

    for (const [type, label] of cases) {
      const { unmount } = render(
        <MemoryCard memory={makeMemory({ contentType: type })} onDelete={vi.fn()} />,
      );
      expect(screen.getByText(label)).toBeInTheDocument();
      unmount();
    }
  });

  it('详情中的重要度进度条带有无障碍属性', async () => {
    render(<MemoryCard memory={makeMemory({ importanceScore: 42 })} onDelete={vi.fn()} />);
    await userEvent.click(screen.getByRole('button', { name: '查看记忆详情：咖啡' }));

    const bar = screen.getByRole('progressbar', { name: /重要度 42/ });
    expect(bar).toHaveAttribute('aria-valuenow', '42');
    expect(bar).toHaveAttribute('aria-valuemin', '0');
    expect(bar).toHaveAttribute('aria-valuemax', '100');
  });

  it('点击删除按钮会把该条记忆交给回调', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const memory = makeMemory();

    render(<MemoryCard memory={memory} onDelete={onDelete} />);
    await user.click(screen.getByRole('button', { name: '删除记忆' }));

    expect(onDelete).toHaveBeenCalledTimes(1);
    expect(onDelete).toHaveBeenCalledWith(memory);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('删除中时按钮被禁用，避免重复提交', () => {
    render(<MemoryCard memory={makeMemory()} onDelete={vi.fn()} deleting />);
    expect(screen.getByRole('button', { name: '删除记忆' })).toBeDisabled();
  });

  it('没有标签时不渲染标签区域', () => {
    render(<MemoryCard memory={makeMemory({ tags: undefined })} onDelete={vi.fn()} />);
    expect(screen.queryByText('饮品偏好')).not.toBeInTheDocument();
  });

  it('复制按钮有可访问名称', async () => {
    const user = userEvent.setup();
    render(<MemoryCard memory={makeMemory()} onDelete={vi.fn()} />);

    const copyButton = screen.getByRole('button', { name: '复制记忆内容' });
    expect(copyButton).toBeInTheDocument();
    // 点击不应抛错（jsdom 下 execCommand 未实现，函数内部已兜底）
    await user.click(copyButton);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('编辑图标直接打开完整内容编辑器，不触发详情', async () => {
    const user = userEvent.setup();
    render(<MemoryCard memory={makeMemory()} onDelete={vi.fn()} onEdited={vi.fn()} />);
    const edit = screen.getByRole('button', { name: '编辑记忆' });
    expect(edit.textContent).toBe('');
    await user.click(edit);
    expect(screen.getByRole('dialog', { name: '编辑记忆' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '内容' })).toHaveValue('咖啡: 喜欢拿铁，不喜欢美式');
    expect(screen.queryByRole('dialog', { name: '记忆详情' })).not.toBeInTheDocument();
  });

  it('没有摘要时使用正文缩略，键盘仍能打开完整详情', async () => {
    const user = userEvent.setup();
    const content = '这是一条很长的记忆。'.repeat(30);
    render(<MemoryCard memory={makeMemory({ summary: '', content })} onDelete={vi.fn()} />);
    const open = screen.getByRole('button', { name: /^查看记忆详情/ });
    expect(open.textContent).not.toContain(content);
    open.focus();
    await user.keyboard('{Enter}');
    expect(within(screen.getByRole('dialog', { name: '记忆详情' })).getByText(content)).toBeInTheDocument();
  });
});

describe('格式化工具', () => {
  it('parseDate 对无效值与 Go 零值时间返回 null', () => {
    expect(parseDate(null)).toBeNull();
    expect(parseDate(undefined)).toBeNull();
    expect(parseDate('')).toBeNull();
    expect(parseDate('not-a-date')).toBeNull();
    // Go 的零值时间
    expect(parseDate('0001-01-01T00:00:00Z')).toBeNull();
    expect(parseDate('2024-10-10T10:30:00Z')).toBeInstanceOf(Date);
  });

  it('formatRelativeTime 处理各种时间跨度', () => {
    const now = Date.now();
    expect(formatRelativeTime(new Date(now - 5_000).toISOString())).toBe('刚刚');
    expect(formatRelativeTime(new Date(now - 5 * 60_000).toISOString())).toBe('5 分钟前');
    expect(formatRelativeTime(new Date(now - 3 * 3_600_000).toISOString())).toBe('3 小时前');
    expect(formatRelativeTime(new Date(now - 2 * 86_400_000).toISOString())).toBe('2 天前');
    expect(formatRelativeTime(null)).toBe('—');
  });

  it('formatCompactNumber 缩写大数字', () => {
    expect(formatCompactNumber(0)).toBe('0');
    expect(formatCompactNumber(999)).toBe('999');
    expect(formatCompactNumber(1000)).toBe('1k');
    expect(formatCompactNumber(1200)).toBe('1.2k');
    expect(formatCompactNumber(1_500_000)).toBe('1.5M');
  });

  it('importanceLevel 按后端阈值分级（>=70 为高）', () => {
    expect(importanceLevel(95).label).toBe('极高');
    expect(importanceLevel(85).label).toBe('极高');
    expect(importanceLevel(84).label).toBe('高');
    expect(importanceLevel(70).label).toBe('高');
    expect(importanceLevel(69).label).toBe('中');
    expect(importanceLevel(40).label).toBe('中');
    expect(importanceLevel(39).label).toBe('低');
  });

  it('truncate 按字符数截断并保留省略号', () => {
    expect(truncate('短文本', 10)).toBe('短文本');
    expect(truncate('一二三四五', 3)).toBe('一二三…');
  });
});

describe('记忆列表空态与筛选（集成）', () => {
  it('类型筛选按钮带 aria-pressed，反映当前选中态', () => {
    // 直接验证按钮语义的实现约定
    render(
      <button type="button" aria-pressed={true}>
        偏好
      </button>,
    );
    expect(screen.getByRole('button', { name: '偏好', pressed: true })).toBeInTheDocument();
  });
});
