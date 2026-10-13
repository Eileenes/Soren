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
    botList: vi.fn(),
    botSave: vi.fn(),
    runs: vi.fn(),
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

vi.mock('../services/agents', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../services/agents')>();
  return { ...actual, agentsAPI: { ...actual.agentsAPI, list: apiMock.botList, runs: apiMock.runs, save: apiMock.botSave } };
});
vi.mock('../services/connections', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../services/connections')>();
  return { ...actual, connectionRequest: vi.fn().mockImplementation(async (_method: string, path: string) => path === '/mobile' ? { available: false, enabled: false, online: false, devices: [], pending: null } : []) };
});

import { DEFAULT_BOT } from '../services/agents';
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
  localStorage.removeItem('soren.active-bot');
  apiMock.botList.mockResolvedValue([DEFAULT_BOT]);
  apiMock.runs.mockResolvedValue([]);
  localStorage.setItem('soren.getting-started.v1', JSON.stringify({seen: true, active: false, step: 0}));
  document.documentElement.className = 'light';
});

describe('应用外壳', () => {
  it('在没有任何后端的情况下也能渲染出完整的首屏，不白屏', async () => {
    mockOffline();
    render(<App />);

    // 品牌与导航
    expect((await screen.findAllByText('Soren')).length).toBeGreaterThan(0);
    expect(screen.queryByRole('button', { name: /新建对话|新的对话/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '当前 Bot 的对话记录' })).not.toBeInTheDocument();

    const nav = screen.getByRole('navigation', { name: 'Bot 导航' });
    expect(within(nav).getByRole('button', { name: 'Soren' })).toBeInTheDocument();
    expect(within(nav).getAllByRole('button')).toHaveLength(1);
    expect(screen.queryByRole('navigation', { name: '设置功能' })).not.toBeInTheDocument();
    expect(screen.getByRole('complementary')).not.toHaveTextContent('⌘');
    expect(screen.getByRole('button', { name: '使用引导' }).textContent).toBe('');

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

    await user.click(screen.getByRole('button', { name: '设置' }));
    const nav = screen.getByRole('navigation', { name: '设置功能' });

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
    expect(screen.getByRole('button', { name: '添加 飞书' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '添加 Gmail' })).toBeInTheDocument();
    expect(within(nav).queryByRole('button', { name: '任务' })).not.toBeInTheDocument();

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
    await user.click(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: 'Soren' }));
    expect(await screen.findByRole('heading', { name: '今天，想做点什么？' })).toBeInTheDocument();
  });

  it('主题切换会改动 html 的 class（深色 ⇄ 浅色）', async () => {
    mockOnlineEmpty();
    const user = userEvent.setup();
    render(<App />);

    await screen.findAllByText('Soren');
    expect(document.documentElement.classList.contains('light')).toBe(true);
    expect(screen.queryByRole('button', { name: '切换到深色模式' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(screen.getByRole('button', { name: '深色主题' }));
    await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(true));
    expect(JSON.parse(localStorage.getItem('soren.preferences.v1')!).general.theme).toBe('dark');
    await user.click(screen.getByRole('button', { name: '浅色主题' }));
    await waitFor(() => expect(document.documentElement.classList.contains('light')).toBe(true));
  });

  it('记忆数量显示在设置功能中，头像侧栏不展示统计', async () => {
    mockOnlineEmpty();
    render(<App />);

    await userEvent.click(screen.getByRole('button', { name: '设置' }));
    const nav = screen.getByRole('navigation', { name: '设置功能' });
    const memoryButton = within(nav).getByRole('button', { name: /记忆/ });
    // total = 12
    expect(within(memoryButton).getByText('12')).toBeInTheDocument();
  });

  it('对话记录按 Bot 分组，打开后继续聊天而不提供新建按钮', async () => {
    mockOnlineEmpty();
    apiMock.getConversations.mockResolvedValue([{ id: 'past', title: '之前的计划', createdAt: '2026-10-13', updatedAt: '2026-10-13' }]);
    apiMock.getConversation.mockResolvedValue({ id: 'past', messages: [{ id: 'm1', role: 'user', content: '历史内容', createdAt: '2026-10-13' }] });
    const user = userEvent.setup(); render(<App />);
    await user.click(screen.getByRole('button', { name: '设置' }));
    await user.click(screen.getByRole('button', { name: '对话记录' }));
    const history = screen.getByRole('main');
    expect(within(history).getByRole('button', { name: /Soren/ })).toBeInTheDocument();
    await user.click(within(history).getByText('查看聊天记录'));
    await user.click(await screen.findByRole('button', { name: '之前的计划' }));
    expect(await within(screen.getByRole('main')).findByText('历史内容')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /新建对话|新的对话/ })).not.toBeInTheDocument();
  });

  it('正在执行任务的其他 Bot 显示动效，点击头像切换到对应对话', async () => {
    mockOnlineEmpty();
    apiMock.botList.mockResolvedValue([DEFAULT_BOT, { ...DEFAULT_BOT, id: 'helper', name: '小助手', avatar: 'robo' }]);
    apiMock.runs.mockImplementation(async (id: string) => id === 'helper' ? [{ id: 'r1', ownerBotId: 'helper', status: 'running', steps: [] }] : []);
    const user = userEvent.setup(); render(<App />);
    const helper = await screen.findByRole('button', { name: '小助手 · 正在执行任务' });
    expect(helper).toHaveClass('is-running');
    expect(screen.getByRole('button', { name: 'Soren' })).not.toHaveClass('is-running');
    await user.click(helper);
    await waitFor(() => expect(screen.getByRole('button', { name: /小助手/ })).toHaveAttribute('aria-current', 'page'));
    expect(localStorage.getItem('soren.active-bot')).toBe('helper');
    expect(apiMock.runs).toHaveBeenCalledWith('helper');
  });

  it('搜索其他 Bot 的对话标题并打开对应聊天，Bot 之间不会混淆', async () => {
    mockOnlineEmpty();
    const helper = { ...DEFAULT_BOT, id: 'helper', name: '创作助手', avatar: 'robo' };
    apiMock.botList.mockResolvedValue([DEFAULT_BOT, helper]);
    apiMock.getConversations.mockImplementation(async (botId?: string) => botId === 'helper' || (!botId && localStorage.getItem('soren.active-bot') === 'helper') ? [{ id: 'helper-chat', title: '海报文案', updatedAt: '2026-10-13', messages: [] }] : []);
    apiMock.getConversation.mockResolvedValue({ id: 'helper-chat', messages: [{ id: 'reply', role: 'assistant', content: '这是海报的第一版', createdAt: '2026-10-13' }] });
    const user = userEvent.setup(); render(<App />);
    const nav = screen.getByRole('navigation', { name: 'Bot 导航' });
    expect(await within(nav).findByText('这是海报的第一版')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '搜索 Bot 和对话' }));
    await user.type(screen.getByRole('combobox', { name: '搜索 Bot 和对话' }), '海报');
    const palette = screen.getByRole('dialog', { name: '搜索 Bot、对话和设置' });
    expect(within(palette).queryByRole('option', { name: /^Soren/ })).not.toBeInTheDocument();
    await user.click(within(palette).getByRole('option', { name: /海报文案/ }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(await within(screen.getByRole('main')).findByText('这是海报的第一版')).toBeInTheDocument();
    expect(localStorage.getItem('soren.active-bot')).toBe('helper');
    expect(apiMock.getConversation).toHaveBeenCalledWith('helper-chat', undefined, 'helper');
    await user.click(screen.getByRole('button', { name: 'Soren' }));
    expect(within(screen.getByRole('main')).queryByText('这是海报的第一版')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '创作助手' }));
    expect(await within(screen.getByRole('main')).findByText('这是海报的第一版')).toBeInTheDocument();
  });

  it('搜索浮层支持无结果、键盘选择设置和 Escape 返回', async () => {
    mockOnlineEmpty();
    const user = userEvent.setup(); render(<App />);
    const trigger = screen.getByRole('button', { name: '搜索 Bot 和对话' });
    await user.click(trigger);
    const search = screen.getByRole('combobox', { name: '搜索 Bot 和对话' });
    await user.type(search, '不存在的结果');
    expect(screen.getByText('没有找到结果，试试 Bot 名称或「设置」。')).toBeInTheDocument();
    await user.clear(search);
    await user.type(search, '设置');
    await user.keyboard('{ArrowDown}{ArrowUp}{Enter}');
    expect(await screen.findByRole('heading', { name: '模型' })).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await user.click(trigger);
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it('侧栏加号直接创建 Bot，保存后出现在列表并进入它的聊天', async () => {
    mockOnlineEmpty();
    const created = { ...DEFAULT_BOT, id: 'writer', name: '写作助手', instructions: '帮助写作', avatar: 'iceberg' };
    apiMock.botSave.mockImplementation(async () => { apiMock.botList.mockResolvedValue([DEFAULT_BOT, created]); return created; });
    const user = userEvent.setup(); render(<App />);
    await user.click(screen.getByRole('button', { name: '创建 Bot' }));
    const editor = screen.getByRole('dialog', { name: '新建 Bot' });
    // Wait for the modal's initial focus before typing; its delayed focus can
    // otherwise interrupt userEvent midway through a field on a fast test run.
    await waitFor(() => expect(within(editor).getByRole('button', { name: '关闭' })).toHaveFocus());
    await user.type(within(editor).getByRole('textbox', { name: '名称' }), '写作助手');
    await user.type(within(editor).getByRole('textbox', { name: '职责' }), '帮助写作');
    await user.click(within(editor).getByRole('button', { name: '保存 Bot' }));
    expect(await screen.findByRole('button', { name: '写作助手' })).toHaveAttribute('aria-current', 'page');
    expect(apiMock.botSave).toHaveBeenCalledWith(expect.objectContaining({ name: '写作助手', instructions: '帮助写作' }), true);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(localStorage.getItem('soren.active-bot')).toBe('writer');
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
