/**
 * 应用外壳与导航测试
 *
 * 覆盖：首屏渲染、页面切换、离线提示、主题切换、无障碍基本属性。
 * 所有后端请求都被 mock，因此测试不依赖真实后端进程。
 */

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

// ── mock 后端 API：默认模拟"后端未启动" ──────────────────
const apiMock = vi.hoisted(() => {
  class FakeApiError extends Error {
    status: number;
    detail?: string;
    isOffline: boolean;
    constructor(message: string, status = 0, detail?: string, isOffline = false) {
      super(message);
      this.name = 'ApiError';
      this.status = status;
      this.detail = detail;
      this.isOffline = isOffline;
    }
    static isApiError(value: unknown): value is FakeApiError {
      return value instanceof FakeApiError;
    }
  }

  const offlineError = () =>
    new FakeApiError(
      '无法连接到 Soren 后端服务（http://localhost:8080）。请确认服务已启动后重试。',
      0,
      'ECONNREFUSED',
      true,
    );

  return {
    FakeApiError,
    offlineError,
    healthCheck: vi.fn(),
    getConversations: vi.fn(),
    getConversation: vi.fn(),
    deleteConversation: vi.fn(),
    sendMessage: vi.fn(),
    memoryList: vi.fn(),
    memorySearch: vi.fn(),
    memoryStats: vi.fn(),
    memoryRemove: vi.fn(),
    channelList: vi.fn(),
    channelEnable: vi.fn(),
    channelDisable: vi.fn(),
    getApiBaseUrl: vi.fn(() => 'http://localhost:8080'),
    setApiBaseUrl: vi.fn(),
  };
});

vi.mock('../services/api', () => {
  const { FakeApiError } = apiMock;
  return {
    ApiError: FakeApiError,
    normalizeError: (error: unknown) =>
      error instanceof FakeApiError
        ? error
        : new FakeApiError(String(error), 0, String(error), false),
    describeError: (error: unknown) =>
      error instanceof Error ? error.message : String(error),
    DEFAULT_API_BASE_URL: 'http://localhost:8080',
    getApiBaseUrl: apiMock.getApiBaseUrl,
    setApiBaseUrl: apiMock.setApiBaseUrl,
    pingBackend: vi.fn(),
    healthAPI: { check: apiMock.healthCheck },
    chatAPI: {
      getConversations: apiMock.getConversations,
      getConversation: apiMock.getConversation,
      deleteConversation: apiMock.deleteConversation,
      sendMessage: apiMock.sendMessage,
    },
    memoryAPI: {
      list: apiMock.memoryList,
      search: apiMock.memorySearch,
      stats: apiMock.memoryStats,
      remove: apiMock.memoryRemove,
    },
    channelAPI: {
      list: apiMock.channelList,
      get: vi.fn(),
      enable: apiMock.channelEnable,
      disable: apiMock.channelDisable,
    },
  };
});

import App from '../App';
import { resetSettings } from '../services/settingsStore';

/** 让所有接口都表现为后端离线 */
function mockOffline(): void {
  apiMock.healthCheck.mockRejectedValue(apiMock.offlineError());
  apiMock.getConversations.mockRejectedValue(apiMock.offlineError());
  apiMock.channelList.mockRejectedValue(apiMock.offlineError());
  apiMock.memoryList.mockRejectedValue(apiMock.offlineError());
  apiMock.memoryStats.mockRejectedValue(apiMock.offlineError());
}

/** 让所有接口返回空数据（后端在线但无内容） */
function mockOnlineEmpty(): void {
  apiMock.healthCheck.mockResolvedValue({
    status: 'ok',
    storageDriver: 'sqlite',
    cacheDriver: 'memory',
    memoryEnabled: true,
    cacheHealthy: true,
  });
  apiMock.getConversations.mockResolvedValue([]);
  apiMock.channelList.mockResolvedValue([]);
  apiMock.memoryList.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    pageSize: 12,
    enabled: true,
  });
  apiMock.memoryStats.mockResolvedValue({
    enabled: true,
    total: 12,
    byType: { conversation: 6, preference: 3, task: 2, fact: 1 },
    highImportance: 4,
    accessedRecently: 5,
    totalTags: 7,
    oldestCreatedAt: '2024-09-01T08:00:00Z',
    newestCreatedAt: '2024-10-10T10:30:00Z',
  });
}

beforeEach(() => {
  vi.clearAllMocks();
  resetSettings();
  localStorage.setItem('soren.getting-started.v1', JSON.stringify({seen: true, active: false, step: 0}));
  document.documentElement.className = 'light';
});

describe('应用外壳', () => {
  it('在没有任何后端的情况下也能渲染出完整的首屏，不白屏', async () => {
    mockOffline();
    render(<App />);

    // 品牌与导航
    expect((await screen.findAllByText('Soren')).length).toBeGreaterThan(0);
    expect(await screen.findByRole('button', { name: /新建对话/ })).toBeInTheDocument();

    const nav = screen.getByRole('navigation', { name: '主导航' });
    for (const label of ['对话', '记忆', '应用连接']) {
      expect(within(nav).getByRole('button', { name: new RegExp(label) })).toBeInTheDocument();
    }

    // 空状态与输入框
    expect(screen.getByRole('heading', { name: '今天，想做点什么？' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: '消息输入框' })).toBeInTheDocument();
  });

  it('后端离线时展示可操作的重试提示，而不是静默失败', async () => {
    mockOffline();
    render(<App />);

    const alerts = await screen.findAllByRole('alert');
    const text = alerts.map((node) => node.textContent ?? '').join(' ');
    expect(text).toContain('无法连接到 Soren 后端服务');
    expect(text).toContain('在设置中检查服务状态');
    expect(screen.getAllByRole('button', { name: '重试' }).length).toBeGreaterThan(0);
  });

  it('可以在聊天 / 记忆 / 渠道 / 设置之间切换', async () => {
    mockOnlineEmpty();
    const user = userEvent.setup();
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: '主导航' });

    // → 记忆
    await user.click(within(nav).getByRole('button', { name: /记忆/ }));
    expect(await screen.findByRole('heading', { name: '记忆' })).toBeInTheDocument();
    // 统计概览
    expect(screen.getByText('记忆总数')).toBeInTheDocument();
    expect(screen.getByText('高重要度')).toBeInTheDocument();
    // 搜索框与类型筛选
    expect(screen.getByRole('searchbox', { name: '搜索记忆' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^偏好/ })).toBeInTheDocument();

    // → 渠道
    await user.click(within(nav).getByRole('button', { name: /应用连接/ }));
    expect(
      await screen.findByRole('heading', { name: '应用连接' }),
    ).toBeInTheDocument();
    expect(screen.getAllByText('规划中').length).toBeGreaterThan(0);

    // → 设置
    await user.click(screen.getByRole('button', { name: '设置' }));
    expect(await screen.findByRole('heading', { name: '设置' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: '模型' })).toBeInTheDocument();
    // 服务商卡片：OpenAI / Anthropic / 自定义（OpenAI 兼容）
    const providerNames = screen
      .getAllByRole('button')
      .map((node) => node.textContent ?? '');
    expect(providerNames.some((name) => name.startsWith('OpenAI'))).toBe(true);
    expect(providerNames.some((name) => name.startsWith('Anthropic'))).toBe(true);
    expect(providerNames.some((name) => name.includes('兼容接口'))).toBe(true);
    expect(screen.getByRole('button', { name: '保存模型连接' })).toBeDisabled();

    // → 回到聊天
    await user.click(within(nav).getByRole('button', { name: /^对话$/ }));
    expect(await screen.findByRole('heading', { name: '今天，想做点什么？' })).toBeInTheDocument();
  });

  it('主题切换会改动 html 的 class（深色 ⇄ 浅色）', async () => {
    mockOnlineEmpty();
    const user = userEvent.setup();
    render(<App />);

    await screen.findAllByText('Soren');
    expect(document.documentElement.classList.contains('light')).toBe(true);
    await user.click(screen.getByRole('button', { name: '切换到深色模式' }));
    await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(true));
    expect(JSON.parse(localStorage.getItem('soren.preferences.v1')!).general.theme).toBe('dark');
    await user.click(screen.getByRole('button', { name: '切换到浅色模式' }));
    await waitFor(() => expect(document.documentElement.classList.contains('light')).toBe(true));
  });

  it('记忆数量会作为徽标出现在侧栏导航上', async () => {
    mockOnlineEmpty();
    render(<App />);

    const nav = await screen.findByRole('navigation', { name: '主导航' });
    const memoryButton = within(nav).getByRole('button', { name: /记忆/ });
    // total = 12
    expect(within(memoryButton).getByText('12')).toBeInTheDocument();
  });

  it('聊天页示例 prompt 是可点击的按钮并带无障碍语义', async () => {
    mockOnlineEmpty();
    render(<App />);

    const promptButton = await screen.findByRole('button', { name: '记录偏好' });
    expect(promptButton).toBeEnabled();
    await userEvent.click(promptButton);
    expect((screen.getByRole('textbox', { name: '消息输入框' }) as HTMLTextAreaElement).value).toContain('我想让你记住');
    expect(apiMock.sendMessage).not.toHaveBeenCalled();
  });
});


describe('首次使用引导', () => {
  it('首次展示，跳过后不重复弹出，仍可主动重新打开', async () => {
    mockOnlineEmpty();
    localStorage.removeItem('soren.getting-started.v1');
    const user = userEvent.setup();
    const first = render(<App />);
    expect(await screen.findByRole('dialog', { name: '开始使用 Soren' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '跳过引导' }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    first.unmount();
    render(<App />);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '使用引导' }));
    expect(screen.getByRole('dialog', { name: '开始使用 Soren' })).toBeInTheDocument();
  });

  it('跳转实际配置页，恢复进度，并只填写示例任务而不自动执行', async () => {
    mockOnlineEmpty();
    localStorage.removeItem('soren.getting-started.v1');
    const user = userEvent.setup();
    const first = render(<App />);
    await user.click(await screen.findByRole('button', { name: '开始配置' }));
    expect(await screen.findByRole('heading', { name: '模型' })).toBeInTheDocument();
    const strip = screen.getByRole('region', { name: '使用引导步骤' });
    await user.click(within(strip).getByRole('button', { name: '暂时跳过' }));
    expect(await screen.findByRole('heading', { name: '我的 Bot' })).toBeInTheDocument();
    first.unmount();
    render(<App />);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(within(screen.getByRole('region', { name: '使用引导步骤' })).getByText('设置你的 Bot')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '下一步' }));
    expect((await screen.findByRole('textbox', { name: '任务目标' }) as HTMLTextAreaElement).value).toContain('三天学习计划');
    expect(screen.getByText('尚未配置模型，任务将使用演示模式。')).toBeInTheDocument();
    expect(apiMock.sendMessage).not.toHaveBeenCalled();
  });
});
