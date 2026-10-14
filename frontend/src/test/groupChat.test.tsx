/**
 * 群聊功能测试
 *
 * 覆盖 PLAN §10 的前端验收项中可自动化的一部分：
 * 发言人逐条解析、切换不串状态、@ 补全（鼠标 + 键盘）、群选中项持久化、
 * 发言后按契约整段替换消息。
 *
 * 「行高/内边距一致」「与发送按钮同高同圆角」在 jsdom 下无法断言计算样式
 * （Tailwind 是类名不是内联样式），因此改为类名断言，真��渲染需人工目检。
 */

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within, fireEvent, act, renderHook } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

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
  return {
    FakeApiError,
    offlineError: () => new FakeApiError('无法连接到 Soren 后端服务。', 0, 'ECONNREFUSED', true),
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
    groupList: vi.fn(),
    groupDetail: vi.fn(),
    groupCreate: vi.fn(),
    groupUpdate: vi.fn(),
    groupRemove: vi.fn(),
    groupSpeak: vi.fn(),
    groupCancel: vi.fn(),
    getApiBaseUrl: vi.fn(() => 'http://localhost:8080'),
    setApiBaseUrl: vi.fn(),
  };
});

vi.mock('../services/api', () => {
  const { FakeApiError } = apiMock;
  return {
    ApiError: FakeApiError,
    normalizeError: (error: unknown) => (error instanceof FakeApiError ? error : new FakeApiError(String(error), 0, String(error), false)),
    describeError: (error: unknown) => (error instanceof Error ? error.message : String(error)),
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
    memoryAPI: { list: apiMock.memoryList, search: apiMock.memorySearch, stats: apiMock.memoryStats, remove: apiMock.memoryRemove },
    channelAPI: { list: apiMock.channelList, get: vi.fn(), enable: apiMock.channelEnable, disable: apiMock.channelDisable },
  };
});

vi.mock('../services/agents', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../services/agents')>();
  return { ...actual, agentsAPI: { ...actual.agentsAPI, list: apiMock.botList, runs: apiMock.runs, save: apiMock.botSave } };
});

vi.mock('../services/groups', () => ({
  groupsAPI: {
    list: apiMock.groupList,
    detail: apiMock.groupDetail,
    create: apiMock.groupCreate,
    remove: apiMock.groupRemove,
    update: apiMock.groupUpdate,
    speak: apiMock.groupSpeak,
    cancel: apiMock.groupCancel,
  },
}));

vi.mock('../services/connections', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../services/connections')>();
  return { ...actual, connectionRequest: vi.fn().mockResolvedValue({ available: false, enabled: false, online: false, devices: [], pending: null }) };
});

import App from '../App';
import { useGroupChat } from '../hooks/useGroupChat';
import { resetSettings } from '../services/settingsStore';

/** 三个成员，与后端 2–5 人的校验一致 */
const SOREN = { id: 'soren', name: 'Soren', avatar: 'soren', instructions: '', model: '', memoryEnabled: true, collaborationEnabled: false, collaborators: [], status: 'active', version: 1 };
const ARCHITECT = { ...SOREN, id: 'arch', name: '架构师', avatar: 'robo' };
const CRITIC = { ...SOREN, id: 'critic', name: '评审员', avatar: 'rex' };

const MEMBERS = [
  { botId: 'soren', name: 'Soren', avatar: 'soren', status: 'active' },
  { botId: 'arch', name: '架构师', avatar: 'robo', status: 'active' },
  { botId: 'critic', name: '评审员', avatar: 'rex', status: 'active' },
];

/** 群视图的最小可用形状，字段名对齐后端 GroupView */
function groupView(overrides: Record<string, unknown> = {}) {
  return {
    id: 'g1', title: '产品评审', topic: '新版本的取舍', mode: 'mention', maxRounds: 3,
    lastMessageAt: '2026-10-14T03:00:00Z', lastMessagePreview: '架构师：先看瓶颈在哪',
    members: MEMBERS, createdAt: '2026-10-14T02:00:00Z', updatedAt: '2026-10-14T03:00:00Z',
    ...overrides,
  };
}

function message(overrides: Record<string, unknown>) {
  return {
    id: 'm', conversationId: 'c1', role: 'assistant', content: '内容',
    timestamp: '2026-10-14T03:00:00Z', createdAt: '2026-10-14T03:00:00Z', updatedAt: '2026-10-14T03:00:00Z',
    ...overrides,
  };
}

/**
 * 等导航列表里的某一行出现
 *
 * 群项并入 Bot 主列表后，<nav> 本身是立即渲染的，只有「导航里出现目标行」
 * 才代表 Bot / 群数据已到达。旧的独立「群聊导航」曾经隐含了这个等待，
 * 合并后必须显式等行，否则会在数据到位前就去查询。
 */
function waitNavRow(name: RegExp | string) {
  return within(screen.getByRole('navigation', { name: 'Bot 导航' })).findByRole('button', { name });
}

function mockOnline(): void {
  apiMock.healthCheck.mockResolvedValue({ status: 'ok', memoryEnabled: true, demoMode: false });
  apiMock.getConversations.mockResolvedValue([]);
  apiMock.channelList.mockResolvedValue([]);
  apiMock.memoryList.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 12, enabled: true });
  apiMock.memoryStats.mockResolvedValue({ enabled: true, total: 0, byType: {}, highImportance: 0, accessedRecently: 0, totalTags: 0 });
}

beforeEach(() => {
  vi.clearAllMocks();
  resetSettings();
  localStorage.removeItem('soren.active-bot');
  localStorage.removeItem('soren.active-group');
  localStorage.setItem('soren.getting-started.v1', JSON.stringify({ seen: true, active: false, step: 0 }));
  document.documentElement.className = 'light';
  mockOnline();
  apiMock.botList.mockResolvedValue([SOREN, ARCHITECT, CRITIC]);
  apiMock.runs.mockResolvedValue([]);
  apiMock.groupList.mockResolvedValue([]);
  apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
  apiMock.groupUpdate.mockImplementation(async (_id: string, input: Record<string, unknown>) => groupView(input as Record<string, unknown>));
});

/**
 * 打开群项的右键菜单
 *
 * 删除与重命名的入口现在只在这里，菜单挂在 document.body 上（portal），
 * 因此不能在导航里 scope 查询。
 */
async function openGroupMenu(name: RegExp | string) {
  fireEvent.contextMenu(await waitNavRow(name));
  return screen.findByRole('menu', { name: '群聊操作' });
}

/** 群列表当前渲染顺序（按后端返回顺序） */
function navGroupTitles() {
  return within(screen.getByRole('navigation', { name: 'Bot 导航' }))
    .getAllByRole('button')
    .map(node => node.querySelector('.inbox-copy strong')?.textContent)
    .filter(title => title !== 'Soren' && title !== '架构师' && title !== '评审员');
}

describe('群聊发言人解析', () => {
  it('多条不同发言者的消息逐条渲染各自的头像与名称', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      conversationId: 'c1',
      messages: [
        message({ id: 'm1', role: 'user', content: '这个接口怎么设计' }),
        message({ id: 'm2', speakerBotId: 'arch', speakerName: '架构师', content: '先看瓶颈在哪' }),
        message({ id: 'm3', speakerBotId: 'critic', speakerName: '评审员', content: '并发会成为问题' }),
        message({ id: 'm4', speakerBotId: 'arch', speakerName: '架构师', content: '可以用队列削峰' }),
      ],
    }));
    const user = userEvent.setup();
    render(<App />);

    await user.click(await waitNavRow(/产品评审/));
    // 切换选中对象会让 AppContent 重新挂载，main 必须重新取，不能复用旧引用
    const main = screen.getByRole('main');
    await within(main).findByText('可以用队列削峰');

    // 每条 assistant 消息各自解析发言人：N 条不同发言者 = N 个不同头像
    const rowOf = (text: string) => within(main).getByText(text).closest('.bubble-assistant')!.parentElement!.parentElement!;
    const archRow = rowOf('先看瓶颈在哪');
    const criticRow = rowOf('并发会成为问题');
    expect(within(archRow).getByText('架构师')).toBeInTheDocument();
    expect(within(archRow).getByRole('img')).toHaveAttribute('src', '/bot-avatars/robo.png');
    expect(within(criticRow).getByText('评审员')).toBeInTheDocument();
    expect(within(criticRow).getByRole('img')).toHaveAttribute('src', '/bot-avatars/rex.png');

    // 用户消息没有发言人行，也没有头像
    const userBubble = within(main).getByText('这个接口怎么设计').closest<HTMLElement>('.bubble-user')!;
    expect(within(userBubble).queryByRole('img')).not.toBeInTheDocument();
  });

  it('发言 Bot 被删除时用冗余的发言人名，头像回退到品牌标识', async () => {
    apiMock.botList.mockResolvedValue([SOREN]);
    apiMock.groupList.mockResolvedValue([groupView({ members: [MEMBERS[0]] })]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({ id: 'm9', speakerBotId: 'gone', speakerName: '已删除的 Bot', content: '上一轮的观点' })],
    }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));

    const main = screen.getByRole('main');
    expect(await within(main).findByText('已删除的 Bot')).toBeInTheDocument();
    const row = within(main).getByText('上一轮的观点').closest('.bubble-assistant')!.parentElement!.parentElement!;
    expect(within(row).getByRole('img')).toHaveAttribute('src', '/soren-mark.svg');
  });

  it('system 消息渲染为居中 pill，不带头像与发言人名', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [
        message({ id: 'm1', role: 'assistant', speakerBotId: 'arch', speakerName: '架构师', content: '开始讨论' }),
        message({ id: 'm2', role: 'system', content: '讨论已中止' }),
      ],
    }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));

    const main = screen.getByRole('main');
    const notice = await within(main).findByText('讨论已中止');
    expect(notice.tagName).toBe('SPAN');
    expect(notice.className).toContain('rounded-full');
    // system 没有发言人：不渲染头像，也不渲染名称行
    expect(notice.querySelector('img')).toBeNull();
    expect(notice.closest('div')!.textContent).toBe('讨论已中止');
  });
});

describe('群聊状态隔离', () => {
  it('切换群聊后状态不串，切回同一个群时各自独立', async () => {
    apiMock.groupList.mockResolvedValue([groupView(), groupView({ id: 'g2', title: '架构评审', topic: '', lastMessagePreview: '评审员：同意' })]);
    apiMock.groupDetail.mockImplementation(async (id: string) => groupView(
      id === 'g2'
        ? { id: 'g2', title: '架构评审', topic: '', messages: [message({ id: 'g2-m1', role: 'user', content: '第二个群的问题' })] }
        : { conversationId: 'c1', messages: [message({ id: 'g1-m1', role: 'user', content: '第一个群的问题' })] },
    ));
    const user = userEvent.setup();
    render(<App />);
    // 切换选中对象会让 AppContent 重新挂载，侧栏与 main 都是新节点，不能复用旧引用
    // 群项与 Bot 项合并进同一个导航列表（不再是独立的「群聊导航」）
    await waitNavRow(/产品评审/);

    await user.click(await waitNavRow(/产品评审/));
    expect(await within(screen.getByRole('main')).findByText('第一个群的问题')).toBeInTheDocument();
    await user.click(await waitNavRow(/架构评审/));
    const main = screen.getByRole('main');
    expect(await within(main).findByText('第二个群的问题')).toBeInTheDocument();
    // 切换后不残留上一个群的消息
    expect(within(main).queryByText('第一个群的问题')).not.toBeInTheDocument();

    await user.click(await waitNavRow(/产品评审/));
    expect(await within(screen.getByRole('main')).findByText('第一个群的问题')).toBeInTheDocument();
    expect(within(screen.getByRole('main')).queryByText('第二个群的问题')).not.toBeInTheDocument();
  });

  it('群与 Bot 的输入内容互相不污染', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({ id: 'm1', role: 'user', content: '群里的问题' })],
    }));
    const user = userEvent.setup();
    render(<App />);
    // 群项与 Bot 项合并进同一个导航列表，取值时不能再按「群聊导航」定位
    const botNav = () => screen.getByRole('navigation', { name: 'Bot 导航' });
    await botNav();

    await user.click(await waitNavRow(/产品评审/));
    await within(screen.getByRole('main')).findByText('群里的问题');
    await user.type(within(screen.getByRole('main')).getByRole('textbox', { name: '消息输入框' }), '草稿甲');
    expect((within(screen.getByRole('main')).getByRole('textbox', { name: '消息输入框' }) as HTMLTextAreaElement).value).toBe('草稿甲');

    await user.click(await waitNavRow('Soren'));
    const botMain = screen.getByRole('main');
    await waitFor(() => expect(within(botMain).getByRole('heading', { name: '今天，想做点什么？' })).toBeInTheDocument());
    expect((within(botMain).getByRole('textbox', { name: '消息输入框' }) as HTMLTextAreaElement).value).toBe('');

    await user.click(await waitNavRow(/产品评审/));
    await within(screen.getByRole('main')).findByText('群里的问题');
    // 群页持有自己的输入状态，与单聊互不影响：切走再切回也不会把单聊的草稿带进来
    expect((within(screen.getByRole('main')).getByRole('textbox', { name: '消息输入框' }) as HTMLTextAreaElement).value).toBe('');
  });
});

describe('@ 提及补全', () => {
  async function openGroup() {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({ id: 'm1', role: 'user', content: '空消息历史' })],
    }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const main = screen.getByRole('main');
    await within(main).findByText('空消息历史');
    const textarea = within(main).getByRole('textbox', { name: '消息输入框' });
    return { user, main, textarea };
  }

  it('输入 @ 弹出候选，鼠标点击可选中', async () => {
    const { user, textarea } = await openGroup();
    await user.type(textarea, '@');
    const listbox = await screen.findByRole('listbox', { name: '选择要提及的成员' });
    expect(within(listbox).getAllByRole('option').map(option => option.textContent)).toEqual(['Soren', '架构师', '评审员']);

    await user.click(within(listbox).getByRole('option', { name: '架构师' }));
    expect((textarea as HTMLTextAreaElement).value).toBe('@架构师 ');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });

  it('键盘可以移动与回车选中候选', async () => {
    const { user, textarea } = await openGroup();
    await user.type(textarea, '@架');
    const listbox = await screen.findByRole('listbox', { name: '选择要提及的成员' });
    expect(within(listbox).getAllByRole('option')).toHaveLength(1);

    await user.click(textarea);
    await user.keyboard('{ArrowDown}{Enter}');
    expect((textarea as HTMLTextAreaElement).value).toBe('@架构师 ');

    // Tab 与 Enter 等价
    await user.type(textarea, '@评');
    await screen.findByRole('listbox', { name: '选择要提及的成员' });
    await user.keyboard('{Tab}');
    expect((textarea as HTMLTextAreaElement).value).toBe('@架构师 @评审员 ');
  });

  it('Escape 关闭候选但保留已输入的文本', async () => {
    const { user, textarea } = await openGroup();
    await user.type(textarea, '@评审');
    await screen.findByRole('listbox', { name: '选择要提及的成员' });
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect((textarea as HTMLTextAreaElement).value).toBe('@评审');
  });

  it('无匹配成员时不显示浮层', async () => {
    const { user, textarea } = await openGroup();
    await user.type(textarea, '@不存在');
    expect(screen.queryByRole('listbox', { name: '选择要提及的成员' })).not.toBeInTheDocument();
  });

  it('用户消息里的 @成员名 被强调，普通正文与邮箱不受影响', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({
        id: 'm1', role: 'user',
        content: '@架构师 这个接口并发够用吗？抄送 dev@example.com 就行',
      })],
    }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));

    const bubble = (await screen.findByText('@架构师')).closest<HTMLElement>('.bubble-user')!;
    const mention = within(bubble).getByText('@架构师');
    // 纯文字强调：颜色 + 字重，没有描边与背景块
    expect(mention.className).toContain('font-semibold');
    expect(mention.className).toContain('text-[rgb(var(--accent))]');
    expect(mention.className).not.toMatch(/border|rounded/);
    expect(bubble.textContent).toContain('dev@example.com');
    expect(bubble.querySelectorAll('span')).toHaveLength(1);
  });
});

/**
 * 群聊消息区滚动
 *
 * ⚠️ 覆盖范围有限：jsdom 没有布局引擎，scrollHeight / clientHeight 恒为 0，
 * 真实滚动位置无法复现。下面只断言「可观测行为」——离开底部时按钮出现、
 * 点击后调用 scrollTo、新消息是否触发自动滚动。
 * **真正的 bug（DOM 撑高触发的 scroll 事件与自动滚动的竞态）在 jsdom 里无法覆盖**：
 * 布局引发的 scroll 事件由浏览器在 React 提交之后才派发，测试无法把事件插进
 * 「DOM 更新」与「useLayoutEffect」之间。该竞态需要人工目检（发一条触发多 Bot 回复的
 * 消息，确认回复立即出现在视野内，不用手动下拉）。
 */
describe('群聊消息区滚动', () => {
  const SCROLL_HEIGHT = 2000;
  const CLIENT_HEIGHT = 600;

  /** 造一个能测滚动的消息区：jsdom 的尺寸全是 0，这里按几何定义覆盖掉 */
  function scrollBox(main: HTMLElement) {
    const element = main.querySelector<HTMLElement>('.chat-scroll')!;
    for (const [key, value] of [['scrollHeight', SCROLL_HEIGHT], ['clientHeight', CLIENT_HEIGHT], ['scrollTop', 0]] as const) {
      Object.defineProperty(element, key, { configurable: true, value, writable: true });
    }
    return element;
  }

  /** 打开一个已有历史的群，返回主区域与滚动 spy（setup.ts 把 scrollTo 补成了空函数） */
  async function openGroup() {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({ id: 'm1', role: 'user', content: '上一轮的问题' })],
    }));
    apiMock.groupSpeak.mockResolvedValue({
      conversationId: 'c1', status: 'done', error: '',
      messages: [message({ id: 'm2', role: 'user', content: '这次的问题' }),
        message({ id: 'm3', speakerBotId: 'arch', speakerName: '架构师', content: '这次的回复' })],
    });
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const main = screen.getByRole('main');
    await within(main).findByText('上一轮的问题');
    const scrollTo = vi.spyOn(HTMLElement.prototype, 'scrollTo');
    const send = async (text: string) => {
      await user.type(within(main).getByRole('textbox', { name: '消息输入框' }), text);
      await user.click(within(main).getByRole('button', { name: '发送消息' }));
    };
    return { user, main, scrollTo, send, box: scrollBox(main) };
  }

  it('新消息到达且用户仍在底部时，自动滚到最新一条', async () => {
    const { main, scrollTo, send } = await openGroup();
    scrollTo.mockClear();

    await send('这次的问题');

    await within(main).findByText('这次的回复');
    expect(scrollTo).toHaveBeenCalledWith({ top: SCROLL_HEIGHT, behavior: 'smooth' });
  });

  it('用户上翻后新消息不自动跟随，改由「回到底部」按钮接管', async () => {
    const { main, scrollTo, send, box } = await openGroup();
    // 模拟用户滚到距顶部很远的位置：只有这次是真实用户滚动
    box.scrollTop = 0;
    fireEvent.scroll(box);
    await within(main).findByRole('button', { name: '回到底部' });
    scrollTo.mockClear();

    await send('这次的问题');
    await within(main).findByText('这次的回复');

    // 上翻看历史时不该被拽回底部
    expect(scrollTo).not.toHaveBeenCalled();
    expect(within(main).getByRole('button', { name: '回到底部' })).toBeInTheDocument();
  });

  it('「回到底部」按钮与单聊同构，点击后滚到底部', async () => {
    const { main, scrollTo, box } = await openGroup();
    box.scrollTop = 0;
    fireEvent.scroll(box);

    const trigger = await within(main).findByRole('button', { name: '回到底部' });
    expect(trigger.className).toContain('control-icon');
    expect(trigger.className).toContain('glass-panel-strong');
    expect(trigger.className).toContain('animate-fade-in');
    expect(trigger.parentElement!.className).toContain('bottom-[168px]');
    scrollTo.mockClear();

    await userEvent.setup().click(trigger);

    expect(scrollTo).toHaveBeenCalledWith({ top: SCROLL_HEIGHT, behavior: 'smooth' });
  });

  it('贴着底部时不显示「回到底部」按钮', async () => {
    const { main, box } = await openGroup();
    // 距底部在阈值内：读数来自真实滚动事件
    box.scrollTop = SCROLL_HEIGHT - CLIENT_HEIGHT - 10;
    fireEvent.scroll(box);

    await within(main).findByText('上一轮的问题');
    expect(within(main).queryByRole('button', { name: '回到底部' })).not.toBeInTheDocument();
  });

  it('小幅上翻仍算贴着底部：阈值比单聊宽，不弹按钮也不打断跟随', async () => {
    const { main, scrollTo, send, box } = await openGroup();
    // 距底部 150px：群聊阈值是 240px（单聊的 80px 会被一次追加多条长消息的布局位移顶穿）
    box.scrollTop = SCROLL_HEIGHT - CLIENT_HEIGHT - 150;
    fireEvent.scroll(box);
    scrollTo.mockClear();

    expect(within(main).queryByRole('button', { name: '回到底部' })).not.toBeInTheDocument();
    await send('这次的问题');
    await within(main).findByText('这次的回复');

    // 按钮不亮就该继续跟随：两者共用同一个阈值，不会出现「亮着按钮又被滚走」
    expect(scrollTo).toHaveBeenCalledWith({ top: SCROLL_HEIGHT, behavior: 'smooth' });
  });
});

describe('群聊发言', () => {
  async function openAutoGroup(speak: (id: never, input: Record<string, unknown>) => Promise<unknown>) {
    apiMock.groupList.mockResolvedValue([groupView({ mode: 'auto', lastMessagePreview: '' })]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      mode: 'auto',
      messages: [message({ id: 'm1', role: 'user', content: '上一轮的问题' })],
    }));
    apiMock.groupSpeak.mockImplementation(speak);
    apiMock.groupCancel.mockResolvedValue({ status: 'cancelling' });
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const main = screen.getByRole('main');
    await within(main).findByText('上一轮的问题');
    return { user, main };
  }

  it('按契约整段替换消息：用户那条只出现一次，不做去重', async () => {
    const { user, main } = await openAutoGroup(async (_id, input: Record<string, unknown>) => ({
      conversationId: 'c1',
      status: 'done',
      error: '',
      messages: [
        message({ id: 'server-user', role: 'user', content: input.content as string }),
        message({ id: 'server-a', speakerBotId: 'arch', speakerName: '架构师', content: '架构师的回复' }),
        message({ id: 'server-b', speakerBotId: 'critic', speakerName: '评审员', content: '评审员的回复' }),
      ],
    }));

    const textarea = within(main).getByRole('textbox', { name: '消息输入框' });
    await user.type(textarea, '@架构师 并发够用吗');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));

    await within(main).findByText('架构师的回复');
    // @提及被拆成独立的 span，所以按气泡内容比对整条消息
    const bubbles = [...main.querySelectorAll('.bubble-user')].filter(node => node.textContent === '@架构师 并发够用吗');
    // 乐观插入那条被整段替换掉，同一句话不会渲染两次
    expect(bubbles).toHaveLength(1);
    expect(apiMock.groupSpeak).toHaveBeenCalledTimes(1);
    const [groupId, input] = apiMock.groupSpeak.mock.calls[0];
    expect(groupId).toBe('g1');
    expect(input).toMatchObject({ content: '@架构师 并发够用吗', mentions: ['arch'], mode: 'auto' });
    // requestId 必填，且是每次发言的新 uuid
    expect(String(input.requestId)).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('连续两条内容相同的发言不会互相去重', async () => {
    let call = 0;
    const { user, main } = await openAutoGroup(async (_id, input: Record<string, unknown>) => {
      call += 1;
      return {
        conversationId: 'c1', status: 'done', error: '',
        messages: [message({ id: `turn-${call}-user`, role: 'user', content: input.content as string }),
          message({ id: `turn-${call}-reply`, speakerBotId: 'arch', speakerName: '架构师', content: `第 ${call} 次回复` })],
      };
    });

    const textarea = within(main).getByRole('textbox', { name: '消息输入框' });
    await user.type(textarea, '好');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));
    await within(main).findByText('第 1 次回复');
    await user.type(textarea, '好');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));
    await within(main).findByText('第 2 次回复');

    expect(apiMock.groupSpeak).toHaveBeenCalledTimes(2);
    // 两条同内容的用户消息都在，各自对应一轮
    expect(within(main).getAllByText('好')).toHaveLength(2);
    expect(apiMock.groupSpeak.mock.calls[0][1].requestId).not.toBe(apiMock.groupSpeak.mock.calls[1][1].requestId);
  });

  it('auto 模式讨论中显示状态条与中止按钮，此时右键菜单的删除项禁用', async () => {
    let release: (() => void) | null = null;
    const { user, main } = await openAutoGroup(() => new Promise(resolve => {
      release = () => resolve({
        conversationId: 'c1', status: 'cancelled', error: '',
        messages: [message({ id: 'u1', role: 'user', content: '@架构师 并发够用吗' })],
      });
    }));

    const textarea = within(main).getByRole('textbox', { name: '消息输入框' });
    await user.type(textarea, '@架构师 并发够用吗');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));

    const strip = await within(main).findByRole('button', { name: '中止讨论' });
    expect(strip.className).toContain('control-icon');
    expect(strip).toHaveClass('!text-error');
    expect(within(main).getByText('讨论中 · 最多 3 轮')).toBeInTheDocument();
    // 讨论进行中禁止删除：删除会与在途的讨论级联清理冲突
    const busy = await openGroupMenu(/产品评审/);
    const deleteItem = within(busy).getByRole('menuitem', { name: '删除' });
    expect(deleteItem).toHaveAttribute('aria-disabled', 'true');
    await user.click(deleteItem);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await user.keyboard('{Escape}');

    await user.click(strip);
    expect(apiMock.groupCancel).toHaveBeenCalledWith('g1');
    release!();
    await within(main).findByRole('button', { name: '发送消息' });
    expect(screen.queryByRole('button', { name: '中止讨论' })).not.toBeInTheDocument();
    const idle = await openGroupMenu(/产品评审/);
    expect(within(idle).getByRole('menuitem', { name: '删除' })).not.toHaveAttribute('aria-disabled');
  });

  it('输入区始终用发送按钮，不出现第二个语义不同的停止按钮', async () => {
    let release: (() => void) | null = null;
    const { user, main } = await openAutoGroup(() => new Promise(resolve => { release = () => resolve({ conversationId: 'c1', status: 'done', error: '', messages: [] }); }));

    await user.type(within(main).getByRole('textbox', { name: '消息输入框' }), '开始');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));
    await screen.findByRole('button', { name: '中止讨论' });
    // ChatInput 固定 loading={false}，所以这里是发送按钮而不是「停止生成」
    expect(within(main).getByRole('button', { name: '发送消息' })).toBeInTheDocument();
    expect(within(main).queryByRole('button', { name: '停止生成' })).not.toBeInTheDocument();
    release!();
  });

  it('mention 模式显示各发言人的思考占位，并保留中止入口', async () => {
    let release: (() => void) | null = null;
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({
      messages: [message({ id: 'm1', role: 'user', content: '空消息历史' })],
    }));
    apiMock.groupSpeak.mockImplementation(() => new Promise(resolve => { release = () => resolve({ conversationId: 'c1', status: 'done', error: '', messages: [] }); }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const main = screen.getByRole('main');
    await within(main).findByText('空消息历史');

    await user.type(within(main).getByRole('textbox', { name: '消息输入框' }), '@架构师 怎么看');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));

    expect(await within(main).findByText('架构师 正在思考…')).toBeInTheDocument();
    expect(within(main).getByRole('button', { name: '中止讨论' })).toBeInTheDocument();
    release!();
    await waitFor(() => expect(within(main).queryByText('架构师 正在思考…')).not.toBeInTheDocument());
  });

  it('发言失败时展示后端原因并保留输入框里的内容', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    apiMock.groupSpeak.mockRejectedValue(new apiMock.FakeApiError('该群已有讨论正在进行，请先中止或等待结束', 409));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const main = screen.getByRole('main');
    await user.type(within(main).getByRole('textbox', { name: '消息输入框' }), '@架构师 怎么看');
    await user.click(within(main).getByRole('button', { name: '发送消息' }));

    await within(main).findByRole('button', { name: '架构师的回复' }).catch(() => undefined);
    expect(await within(main).findByRole('alert')).toHaveTextContent('该群已有讨论正在进行，请先中止或等待结束');
    // 失败后回拉群详情，但不能把错误提示抹掉
    expect(apiMock.groupDetail).toHaveBeenCalledTimes(2);
  });
});

describe('群项右键菜单', () => {
  it('侧栏不再有「Bot」标题行与计数，直接展示列表', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    render(<App />);
    await waitNavRow(/产品评审/);
    expect(screen.queryByText('Bot')).not.toBeInTheDocument();
    expect(document.querySelector('.inbox-heading')).toBeNull();
  });

  it('右键群项弹出三项菜单，左键单击仍然只是打开群', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [message({ id: 'm1', role: 'user', content: '群里的问题' })] }));
    const user = userEvent.setup();
    render(<App />);

    // 左键：只打开群，不弹菜单
    await user.click(await waitNavRow(/产品评审/));
    expect(await within(screen.getByRole('main')).findByText('群里的问题')).toBeInTheDocument();
    expect(screen.queryByRole('menu', { name: '群聊操作' })).not.toBeInTheDocument();

    // 右键：弹出三项，菜单本体挂在 body 上
    const menu = await openGroupMenu(/产品评审/);
    expect(within(menu).getAllByRole('menuitem').map(item => item.textContent)).toEqual(['置顶', '重命名聊天', '删除']);
    expect(menu.className).toContain('app-select-menu');
    expect(within(menu).getAllByRole('menuitem').every(item => item.className.includes('app-select-option'))).toBe(true);
  });

  it('已置顶的群文案切换为「取消置顶」，置顶与取消都只发 { pinned }', async () => {
    apiMock.groupList.mockResolvedValue([
      groupView({ id: 'g1', title: '产品评审' }),
      groupView({ id: 'g2', title: '架构评审', pinned: true }),
    ]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);

    // 未置顶 → 置顶
    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '置顶' }));
    await waitFor(() => expect(apiMock.groupUpdate).toHaveBeenCalledWith('g1', { pinned: true }));
    // PATCH 是全可选指针：不发全量覆盖的请求体
    expect(apiMock.groupUpdate.mock.calls[0][1]).toEqual({ pinned: true });
    expect(screen.queryByRole('menu', { name: '群聊操作' })).not.toBeInTheDocument();

    // 已置顶 → 取消置顶
    const pinnedMenu = await openGroupMenu(/架构评审/);
    expect(within(pinnedMenu).getByRole('menuitem', { name: '取消置顶' })).toBeInTheDocument();
    expect(within(pinnedMenu).queryByRole('menuitem', { name: '置顶' })).not.toBeInTheDocument();
    await user.click(within(pinnedMenu).getByRole('menuitem', { name: '取消置顶' }));
    await waitFor(() => expect(apiMock.groupUpdate).toHaveBeenCalledWith('g2', { pinned: false }));
  });

  it('置顶成功后按后端返回顺序重排群列表', async () => {
    const first = groupView({ id: 'g1', title: '产品评审' });
    const second = groupView({ id: 'g2', title: '架构评审' });
    // 首次未置顶（按 updatedAt 倒序），置顶后后端把 g1 排到前面
    apiMock.groupList.mockResolvedValueOnce([second, first]).mockResolvedValue([first, second]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);
    await waitNavRow(/产品评审/);
    expect(navGroupTitles()).toEqual(['架构评审', '产品评审']);

    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '置顶' }));

    await waitFor(() => expect(apiMock.groupUpdate).toHaveBeenCalled());
    // 前端不自己排序：顺序完全跟着 GET /api/groups 的返回走
    await waitFor(() => expect(navGroupTitles()).toEqual(['产品评审', '架构评审']));
  });

  it('重命名只发 { title }，改完列表标题跟着更新', async () => {
    apiMock.groupList.mockResolvedValueOnce([groupView({ title: '产品评审' })]).mockResolvedValue([groupView({ title: '新群名' })]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);

    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '重命名聊天' }));
    const dialog = await screen.findByRole('dialog', { name: '重命名聊天' });
    // 预填当前群名，用户在此基础上改
    expect((within(dialog).getByRole('textbox', { name: '群名' }) as HTMLInputElement).value).toBe('产品评审');
    await user.clear(within(dialog).getByRole('textbox', { name: '群名' }));
    await user.type(within(dialog).getByRole('textbox', { name: '群名' }), '新群名');
    await user.click(within(dialog).getByRole('button', { name: '保存' }));

    await waitFor(() => expect(apiMock.groupUpdate).toHaveBeenCalledWith('g1', { title: '新群名' }));
    // 话题、模式、成员、轮次都不该被这次改名带上
    expect(apiMock.groupUpdate.mock.calls[0][1]).toEqual({ title: '新群名' });
    await waitFor(() => expect(navGroupTitles()).toEqual(['新群名']));
  });

  it('重命名失败时错误留在弹窗内，群名不变', async () => {
    apiMock.groupList.mockResolvedValue([groupView({ title: '产品评审' })]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    apiMock.groupUpdate.mockRejectedValue(new apiMock.FakeApiError('群名已存在', 400));
    const user = userEvent.setup();
    render(<App />);

    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '重命名聊天' }));
    const dialog = await screen.findByRole('dialog', { name: '重命名聊天' });
    await user.clear(within(dialog).getByRole('textbox', { name: '群名' }));
    await user.type(within(dialog).getByRole('textbox', { name: '群名' }), '重名');
    await user.click(within(dialog).getByRole('button', { name: '保存' }));

    expect(await within(dialog).findByRole('alert')).toHaveTextContent('群名已存在');
    expect(screen.getByRole('dialog', { name: '重命名聊天' })).toBeInTheDocument();
    expect(navGroupTitles()).toEqual(['产品评审']);
  });

  it('键盘可以打开菜单，↑↓ 移动、Enter 确认、Esc 关闭', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);

    // Shift+F10 等价于右键，不用鼠标也能拿到菜单。
    // 不能先点击群项：切换选中会让整个侧栏重新挂载，焦点随之丢失。
    const row = await waitNavRow(/产品评审/);
    row.focus();
    expect(row).toHaveFocus();
    await user.keyboard('{Shift>}{F10}{/Shift}');
    const menu = await screen.findByRole('menu', { name: '群聊操作' });
    const items = within(menu).getAllByRole('menuitem');
    expect(menu).toHaveFocus();
    expect(items[0].className).toContain('is-active');

    await user.keyboard('{ArrowDown}');
    expect(within(menu).getAllByRole('menuitem')[1].className).toContain('is-active');
    await user.keyboard('{Enter}');
    // 当前项是「重命名聊天」，回车即打开重命名弹窗
    expect(await screen.findByRole('dialog', { name: '重命名聊天' })).toBeInTheDocument();

    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '重命名聊天' })).not.toBeInTheDocument());
    await openGroupMenu(/产品评审/);
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu', { name: '群聊操作' })).not.toBeInTheDocument();
  });

  it('点击菜单外部关闭菜单', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);

    await openGroupMenu(/产品评审/);
    await user.click(screen.getByRole('button', { name: '搜索 Bot 和对话' }));
    expect(screen.queryByRole('menu', { name: '群聊操作' })).not.toBeInTheDocument();
  });
});

describe('群聊选中项持久化与侧栏', () => {
  it('群作为选中项被写入 localStorage，重启后恢复', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    const first = render(<App />);
    await user.click(await waitNavRow(/产品评审/));

    expect(localStorage.getItem('soren.active-group')).toBe('g1');
    // 两个键互斥，避免恢复出一个早已过期的选择
    expect(localStorage.getItem('soren.active-bot')).toBeNull();
    first.unmount();

    render(<App />);
    const nav = await screen.findByRole('navigation', { name: 'Bot 导航' });
    // 恢复选中项会让 AppContent 重新挂载，侧栏是新节点，必须在断言时重新取
    await waitFor(() => expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: /产品评审/ })).toHaveAttribute('aria-current', 'page'));
    expect(screen.getByRole('main')).toHaveTextContent('新版本的取舍');
    // 该群只渲染一行；导航列表现在同时装着 Bot 项，计数按群名收敛
    expect(within(nav).getAllByRole('button', { name: /产品评审/ })).toHaveLength(1);
    // Bot 项同时失去选中态
    expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: 'Soren' })).not.toHaveAttribute('aria-current');
  });

  it('记下的群已不存在时回落到上一个 Bot', async () => {
    localStorage.setItem('soren.active-group', 'deleted-group');
    localStorage.setItem('soren.active-bot', 'critic');
    render(<App />);
    await waitFor(() => expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: '评审员' })).toHaveAttribute('aria-current', 'page'));
    // 群聊并入主列表后，已删除的群不该留下任何一行（旧的「群聊导航」整块已不存在）
    expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).queryByRole('button', { name: /deleted-group/ })).not.toBeInTheDocument();
    expect(document.querySelectorAll('.group-avatars')).toHaveLength(0);
  });

  it('Bot 列表里没有 soren 时，恢复上次 Bot 不会被自动回退覆盖成 soren', async () => {
    // 默认选择是 soren：若恢复结果没和回退判断落在同一次渲染，回退会把用户按回 soren
    apiMock.botList.mockResolvedValue([ARCHITECT, CRITIC]);
    localStorage.setItem('soren.active-bot', 'critic');
    render(<App />);
    await waitFor(() => expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: '评审员' })).toHaveAttribute('aria-current', 'page'));
    expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).queryByRole('button', { name: 'Soren' })).not.toBeInTheDocument();
    expect(localStorage.getItem('soren.active-bot')).toBe('critic');
  });

  it('群列表为空时主列表里不渲染任何群项，也不出现「群聊」标题', async () => {
    render(<App />);
    const nav = await screen.findByRole('navigation', { name: 'Bot 导航' });
    // 群项并入主列表后没有独立的分组块可整段隐藏：空群列表就是一行都不渲染
    expect(nav.querySelectorAll('.group-avatars')).toHaveLength(0);
    expect(nav.querySelectorAll('.bot-inbox-row')).toHaveLength(3);
    expect(screen.queryByText('群聊')).not.toBeInTheDocument();
  });

  /** 两个群：2 人与 5 人（后端上限），另有 7 人群用于覆盖 +N 兜底分支 */
  function twoGroups() {
    apiMock.groupList.mockResolvedValue([
      groupView({ id: 'g1', title: '两人群', members: MEMBERS.slice(0, 2) }),
      groupView({ id: 'g2', title: '五人群', members: [...MEMBERS, { botId: 'x', name: '第四', avatar: 'rex', status: 'active' }, { botId: 'y', name: '第五', avatar: 'robo', status: 'active' }] }),
    ]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
  }

  it('群项与 Bot 项合并进同一个导航列表，行高与内边距复用同一个类名', async () => {
    twoGroups();
    render(<App />);
    const two = await waitNavRow(/两人群/);
    const five = await waitNavRow(/五人群/);
    // 两类行在同一个 <nav> 里：旧的独立「群聊导航」与「群聊 N」标题都不再存在
    expect(await waitNavRow('Soren')).toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: '群聊导航' })).not.toBeInTheDocument();
    expect(screen.queryByText('群聊')).not.toBeInTheDocument();
    // 行高与内边距由同一个类名保证，jsdom 下只能断言类名（实际尺寸需人工目检）
    expect(two).toHaveClass('bot-inbox-row');
    expect(five).toHaveClass('bot-inbox-row');
    expect(await waitNavRow('Soren')).toHaveClass('bot-inbox-row');
    // 主副文本沿用 inbox-copy，没有为群项另写一套规格
    expect(two.querySelector('.inbox-copy strong')!.textContent).toBe('两人群');
    expect(two.querySelector('.inbox-copy small')!.textContent).toBe('架构师：先看瓶颈在哪');
  });

  it('头像槽与 Bot 项同宽：群项复用同一个 inbox-avatar 槽位类名', async () => {
    twoGroups();
    render(<App />);
    const two = await waitNavRow(/两人群/);
    const botRow = await waitNavRow('Soren');
    // 群项的槽位就是 .inbox-avatar（38×38），不是另写一个更宽的头像簇：
    // 同一个类名才能保证文字左边缘一致。jsdom 没有布局，只能断言类名。
    const groupSlot = two.querySelector('.group-avatars')!;
    expect(groupSlot).toHaveClass('inbox-avatar');
    expect(botRow.querySelector('.inbox-avatar')).toHaveClass('inbox-avatar');
    // 两类行的槽位都是行的第一个元素，结构同构
    expect(groupSlot).toBe(two.firstElementChild);
    expect(botRow.querySelector('.inbox-avatar')).toBe(botRow.firstElementChild);
  });

  it('头像簇最多 6 张：成员不超过上限时全部可见且不出现「+N」', async () => {
    twoGroups();
    render(<App />);
    const two = await waitNavRow(/两人群/);
    const five = await waitNavRow(/五人群/);
    expect(two.querySelectorAll('.group-avatars img')).toHaveLength(2);
    expect(two.querySelector('.group-avatars')!.children).toHaveLength(2);
    // 上限 5 人：最多 6 张的上限用不满，全部画出来，不折叠成「+N」
    expect(five.querySelectorAll('.group-avatars img')).toHaveLength(5);
    expect(five.querySelector('.group-avatars')!.children).toHaveLength(5);
    expect(five.querySelector('.group-avatars')!.textContent).toBe('');
  });

  it('超过 6 人时只画前 6 张，不加「+N」，槽位宽度不变', async () => {
    // 后端上限是 5 人，这里是纯前端的兜底分支：超限时只截断，不排第 7 张把槽位撑破
    apiMock.groupList.mockResolvedValue([groupView({
      title: '七人群',
      members: [...MEMBERS,
        { botId: 'x', name: '第四', avatar: 'rex', status: 'active' },
        { botId: 'y', name: '第五', avatar: 'robo', status: 'active' },
        { botId: 'z', name: '第六', avatar: 'rex', status: 'active' },
        { botId: 'w', name: '第七', avatar: 'robo', status: 'active' }],
    })]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    render(<App />);
    const seven = await waitNavRow(/七人群/);

    // 38px 方槽塞不下第 7 张，也不再用「+N」占格——超出的成员直接不展示
    expect(seven.querySelectorAll('.group-avatars img')).toHaveLength(6);
    expect(seven.querySelector('.group-avatars')!.children).toHaveLength(6);
    expect(seven.querySelector('.group-avatars')!.textContent).toBe('');
    // 槽位仍是 Bot 项那一个类，宽度不受成员数影响
    expect(seven.querySelector('.group-avatars')).toHaveClass('inbox-avatar');
  });

  it('搜索可以命中群聊并打开它', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    const user = userEvent.setup();
    render(<App />);
    await waitNavRow(/产品评审/);
    await user.click(screen.getByRole('button', { name: '搜索 Bot 和对话' }));
    const palette = screen.getByRole('dialog', { name: '搜索 Bot、对话和设置' });
    await user.type(within(palette).getByRole('combobox', { name: '搜索 Bot 和对话' }), '产品评审');
    expect(within(palette).getByRole('option', { name: /产品评审/ })).toBeInTheDocument();
    await user.click(within(palette).getByRole('option', { name: /产品评审/ }));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(await within(screen.getByRole('main')).findByText('新版本的取舍')).toBeInTheDocument();
  });
});

describe('创建群聊', () => {
  it('选满成员与模式后创建，成功即进入该群', async () => {
    const created = groupView({ id: 'g9', title: '新的群' });
    apiMock.groupList.mockResolvedValue([]);
    apiMock.groupCreate.mockResolvedValue(created);
    const user = userEvent.setup();
    render(<App />);

    await user.click(screen.getByRole('button', { name: '创建' }));
    await user.click(screen.getByRole('menuitem', { name: '创建群聊' }));
    const dialog = screen.getByRole('dialog', { name: '创建群聊' });
    // Modal 的延迟聚焦会抢焦点，先等它落定，避免打断后续输入
    await waitFor(() => expect(within(dialog).getByRole('button', { name: '关闭' })).toHaveFocus());
    const submit = within(dialog).getByRole('button', { name: '创建群聊' });
    const members = () => ['Soren', '架构师', '评审员'].map(name => within(dialog).getByRole('button', { name }));

    // 成员不足 2 个时不可提交；不预填群名，也不默认勾选任何成员
    expect(submit).toBeDisabled();
    expect((within(dialog).getByRole('textbox', { name: '群名' }) as HTMLInputElement).value).toBe('');
    expect(members().filter(node => node.getAttribute('aria-pressed') === 'true')).toHaveLength(0);

    await user.type(within(dialog).getByRole('textbox', { name: '群名' }), '新的群');
    expect(submit).toBeDisabled();
    await user.click(members()[1]);
    expect(submit).toBeDisabled();
    await user.click(members()[2]);
    expect(submit).toBeEnabled();

    // mention 模式不暴露轮次上限（该模式下该值无效）
    expect(within(dialog).queryByText('轮次上限')).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: '自主' }));
    expect(within(dialog).getByText('轮次上限')).toBeInTheDocument();

    await user.click(submit);
    await waitFor(() => expect(apiMock.groupCreate).toHaveBeenCalledWith({
      title: '新的群', topic: '', botIds: ['arch', 'critic'], mode: 'auto', maxRounds: 3,
    }));
    expect(localStorage.getItem('soren.active-group')).toBe('g9');
    await waitFor(() => expect(screen.getByRole('main')).toHaveTextContent('新版本的取舍'));
  });

  it('群名超长时提交被拦下并落到字段错误上', async () => {
    apiMock.groupList.mockResolvedValue([]);
    const user = userEvent.setup();
    render(<App />);
    await user.click(screen.getByRole('button', { name: '创建' }));
    await user.click(screen.getByRole('menuitem', { name: '创建群聊' }));
    const dialog = screen.getByRole('dialog', { name: '创建群聊' });
    // Modal 的延迟聚焦会在点击之后把焦点抢走，先等它落定再操作输入框
    await waitFor(() => expect(within(dialog).getByRole('button', { name: '关闭' })).toHaveFocus());
    const field = within(dialog).getByRole('textbox', { name: '群名' });
    await user.click(within(dialog).getByRole('button', { name: '架构师' }));
    await user.click(within(dialog).getByRole('button', { name: '评审员' }));
    // 输入端不静默截断：超长在提交时拦下并就地报错，用户能看见自己写超了多少
    await user.click(field);
    await user.paste('群'.repeat(60));
    expect((field as HTMLInputElement).value).toHaveLength(60);
    await user.click(within(dialog).getByRole('button', { name: '创建群聊' }));
    expect(within(dialog).getByRole('alert')).toHaveTextContent('群名最多 50 字');
    expect(apiMock.groupCreate).not.toHaveBeenCalled();
  });
});

describe('删除群聊', () => {
  it('确认弹窗写明私有记忆保留，删除后回到可用 Bot', async () => {
    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    apiMock.groupRemove.mockResolvedValue({ message: '群聊已删除' });
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));

    // 删除入口只剩右键菜单一处，标题栏不再常驻删除按钮
    expect(screen.queryByRole('button', { name: '删除群聊' })).not.toBeInTheDocument();
    const menu = await openGroupMenu(/产品评审/);
    // 菜单里的删除项也不做危险色：危险语义由确认弹窗承担
    expect(within(menu).getByRole('menuitem', { name: '删除' }).className).not.toMatch(/text-error|bg-error/);
    await user.click(within(menu).getByRole('menuitem', { name: '删除' }));

    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent('删除后群聊与其中的对话记录将无法恢复，成员的私有记忆会保留');
    await user.click(within(dialog).getByRole('button', { name: '删除群聊' }));

    await waitFor(() => expect(apiMock.groupRemove).toHaveBeenCalledWith('g1'));
    expect(localStorage.getItem('soren.active-group')).toBeNull();
    await waitFor(() => expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: 'Soren' })).toHaveAttribute('aria-current', 'page'));
  });

  it('删掉的不是当前选中的群时，不把用户正在看的群换掉', async () => {
    apiMock.groupList.mockResolvedValue([
      groupView({ id: 'g1', title: '产品评审' }),
      groupView({ id: 'g2', title: '架构评审' }),
    ]);
    apiMock.groupDetail.mockImplementation(async (id: string) => groupView({
      id,
      title: id === 'g1' ? '产品评审' : '架构评审',
      messages: [message({ id: `${id}-m1`, role: 'user', content: '正在看的群' })],
    }));
    apiMock.groupRemove.mockResolvedValue({ message: '群聊已删除' });
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/架构评审/));
    await within(screen.getByRole('main')).findByText('正在看的群');

    // 删除入口在侧栏，可以删掉没打开的那个群
    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '删除' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '删除群聊' }));

    await waitFor(() => expect(apiMock.groupRemove).toHaveBeenCalledWith('g1'));
    await waitNavRow(/架构评审/);
    // 选中的仍是架构评审，没有被静默弹回默认 Bot
    expect(localStorage.getItem('soren.active-group')).toBe('g2');
    expect(within(screen.getByRole('navigation', { name: 'Bot 导航' })).getByRole('button', { name: /架构评审/ })).toHaveAttribute('aria-current', 'page');
  });

  it('删除失败时错误留在确认弹窗内，界面不丢上下文', async () => {    apiMock.groupList.mockResolvedValue([groupView()]);
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    apiMock.groupRemove.mockRejectedValue(new apiMock.FakeApiError('删除群聊失败', 500));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    const menu = await openGroupMenu(/产品评审/);
    await user.click(within(menu).getByRole('menuitem', { name: '删除' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '删除群聊' }));

    expect(await within(screen.getByRole('dialog')).findByRole('alert')).toHaveTextContent('删除群聊失败');
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });
});

describe('群聊审查回归', () => {
  async function openGroup() {
    apiMock.groupList.mockResolvedValue([groupView({ mode: 'auto' })]);
    apiMock.groupDetail.mockResolvedValue(groupView({ mode: 'auto', messages: [] }));
    const user = userEvent.setup();
    render(<App />);
    await user.click(await waitNavRow(/产品评审/));
    await screen.findByPlaceholderText('发消息到 产品评审，@ 可以指定 Bot');
    return user;
  }

  it('讨论中点击发送或按快捷键都保留第二条草稿', async () => {
    let release!: (value: unknown) => void;
    apiMock.groupSpeak.mockImplementation(() => new Promise(resolve => { release = resolve; }));
    const user = await openGroup();
    const input = screen.getByRole('textbox', { name: '消息输入框' });
    await user.type(input, '第一条');
    await user.click(screen.getByRole('button', { name: '发送消息' }));
    await screen.findByRole('button', { name: '中止讨论' });
    await user.type(input, '下一条草稿');
    expect(screen.getByRole('button', { name: '发送消息' })).toBeDisabled();
    await user.click(screen.getByRole('button', { name: '发送消息' }));
    fireEvent.keyDown(input, { key: 'Enter', ctrlKey: true });
    expect(apiMock.groupSpeak).toHaveBeenCalledTimes(1);
    expect(input).toHaveValue('下一条草稿');
    await act(async () => { release({ conversationId: 'c1', status: 'done', messages: [], error: '' }); });
    expect(input).toHaveValue('下一条草稿');
    expect(screen.getByRole('button', { name: '发送消息' })).toBeEnabled();
  });

  it('网络失败回拉历史后，幂等重试不重复追加相同消息 ID', async () => {
    const persisted = [message({ id: 'server-user', role: 'user', content: '问题' }),
      message({ id: 'server-answer', speakerBotId: 'arch', content: '回答' })];
    apiMock.groupDetail.mockResolvedValueOnce(groupView({ messages: [] }))
      .mockResolvedValue(groupView({ messages: persisted }));
    apiMock.groupSpeak.mockRejectedValueOnce(new apiMock.FakeApiError('connection lost', 0))
      .mockResolvedValueOnce({ conversationId: 'c1', status: 'done', error: '', messages: persisted });
    const { result } = renderHook(() => useGroupChat('g1'));
    await waitFor(() => expect(result.current.loadingHistory).toBe(false));
    await act(async () => { await result.current.send('问题'); });
    expect(result.current.messages).toHaveLength(2);
    await act(async () => { await result.current.send('问题'); });
    expect(apiMock.groupSpeak.mock.calls[0][1]).toEqual(apiMock.groupSpeak.mock.calls[1][1]);
    expect(result.current.messages.map(item => item.id)).toEqual(['server-user', 'server-answer']);
  });

  it('讨论中切走再返回后可以中止，并拉回完整结束消息', async () => {
    let release!: (value: unknown) => void;
    const user = await openGroup();
    let key = '';
    apiMock.groupSpeak.mockImplementation((_id: string, request: { requestId: string }) => {
      key = request.requestId;
      apiMock.groupDetail.mockResolvedValue(groupView({ mode: 'auto', messages: [message({ id: 'u1', role: 'user', content: '问题' })], latestTurn: { requestId: key, status: 'running', mode: 'auto' } }));
      return new Promise(resolve => { release = resolve; });
    });
    await user.type(screen.getByRole('textbox', { name: '消息输入框' }), '问题');
    await user.click(screen.getByRole('button', { name: '发送消息' }));
    await screen.findByRole('button', { name: '中止讨论' });
    await user.click(await waitNavRow('Soren'));
    await user.click(await waitNavRow(/产品评审/));
    const cancel = await screen.findByRole('button', { name: '中止讨论' });
    const messages = [message({ id: 'u1', role: 'user', content: '问题' }), message({ id: 'notice', role: 'system', content: '讨论已中止' })];
    apiMock.groupCancel.mockImplementation(async () => {
      apiMock.groupDetail.mockResolvedValue(groupView({ mode: 'auto', messages, latestTurn: { requestId: key, status: 'cancelled', mode: 'auto' } }));
      release({ conversationId: 'c1', status: 'cancelled', messages, error: '' });
      return { status: 'cancelling' };
    });
    await user.click(cancel);
    expect(apiMock.groupCancel).toHaveBeenCalledWith('g1');
    await screen.findByText('讨论已中止');
    expect(screen.queryByRole('button', { name: '中止讨论' })).not.toBeInTheDocument();
  });

  it('running 重放持续轮询直到终态，不重复调用模型，之后相同正文可发新轮次', async () => {
    apiMock.groupDetail.mockResolvedValue(groupView({ messages: [] }));
    apiMock.groupSpeak.mockRejectedValueOnce(new apiMock.FakeApiError('connection lost', 0))
      .mockResolvedValueOnce({ conversationId: 'c1', status: 'running', error: '', messages: [] });
    const { result } = renderHook(() => useGroupChat('g1'));
    await waitFor(() => expect(result.current.loadingHistory).toBe(false));
    await act(async () => { await result.current.send('问题'); });
    const key = apiMock.groupSpeak.mock.calls[0][1].requestId;
    vi.useFakeTimers();
    try {
      await act(async () => { await result.current.send('问题'); });
      expect(result.current.speaking).toBe(true);
      expect(apiMock.groupSpeak.mock.calls[1][1].requestId).toBe(key);
      apiMock.groupDetail.mockResolvedValue(groupView({ messages: [], latestTurn: { requestId: key, status: 'running', mode: 'mention' } }));
      await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
      expect(result.current.speaking).toBe(true);
      const messages = [message({ id: 'u1', role: 'user', content: '问题' }), message({ id: 'a1', content: '回答' })];
      apiMock.groupDetail.mockResolvedValue(groupView({ messages, latestTurn: { requestId: key, status: 'done', mode: 'mention' } }));
      await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
      expect(result.current.speaking).toBe(false);
      expect(result.current.messages.map(item => item.id)).toEqual(['u1', 'a1']);
      expect(apiMock.groupSpeak).toHaveBeenCalledTimes(2);
      apiMock.groupSpeak.mockResolvedValue({ conversationId: 'c1', status: 'done', error: '', messages: [] });
      await act(async () => { await result.current.send('问题'); });
      expect(apiMock.groupSpeak.mock.calls[2][1].requestId).not.toBe(key);
    } finally { vi.useRealTimers(); }
  });

  it('明确失败后重发同一句话使用新请求，允许模型恢复后再次执行', async () => {
    apiMock.groupSpeak.mockRejectedValueOnce(new apiMock.FakeApiError('模型暂不可用', 500))
      .mockResolvedValueOnce({ conversationId: 'c1', status: 'done', error: '', messages: [] });
    const { result } = renderHook(() => useGroupChat('g1'));
    await waitFor(() => expect(result.current.loadingHistory).toBe(false));
    await act(async () => { await result.current.send('问题'); });
    await act(async () => { await result.current.send('问题'); });
    expect(apiMock.groupSpeak.mock.calls[0][1].requestId).not.toBe(apiMock.groupSpeak.mock.calls[1][1].requestId);
  });
});
