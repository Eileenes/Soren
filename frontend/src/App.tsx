/**
 * Soren 应用根组件
 *
 * 职责：
 * 1. 提供通知上下文与主题（深色为默认）
 * 2. 用 state 做视图切换（聊天 / 记忆 / 渠道 / 设置），不引入 react-router
 * 3. 组装侧边栏 + 顶部栏 + 当前页面
 *
 * 数据流：各页面的数据由对应的 Hook 在 App 层统一持有，
 * 这样侧边栏徽标（记忆条数、运行中渠道数）能与其他页面保持同步，
 * 不会出现"切回列表还是旧数据"的问题。
 */

import { useCallback, useEffect, useMemo, useState, useRef } from 'react';
import { GettingStarted, useGettingStarted } from './components/GettingStarted';
import { Button } from './components/ui';
import { Sidebar } from './components/layout/Sidebar';
import { TitleBar } from './components/layout/TitleBar';
import { ToastProvider, useToast } from './hooks/useToast';
import { useChannels } from './hooks/useChannels';
import { useChat } from './hooks/useChat';
import { useConversations } from './hooks/useConversations';
import { useHealth } from './hooks/useHealth';
import { useMemories } from './hooks/useMemories';
import { useSettings, useTheme } from './hooks/useSettings';
import { useHotkey } from './hooks/useUtilities';
import { ChatPage } from './pages/ChatPage';
import { ChannelsPage } from './pages/ChannelsPage';
import { MemoriesPage } from './pages/MemoriesPage';
import { SettingsPage } from './pages/SettingsPage';
import { BotsPage } from './pages/BotsPage';
import { TasksPage } from './pages/TasksPage';
import { SharedNotesPage } from './pages/SharedNotesPage';
import { agentsAPI, DEFAULT_BOT, type Bot } from './services/agents';
import { setActiveBotID } from './services/botScope';
import { describeError } from './services/api';
import type { ViewId } from './types';

/** 视图标题映射 */
const VIEW_TITLES: Record<ViewId, string> = {
  bots: '我的 Bot', tasks: '任务', shared: '共享资料',
  chat: '对话',
  memories: '我的记忆',
  channels: '应用连接',
  settings: '设置',
};

/** 视图副标题（顶部栏展示） */
const VIEW_SUBTITLES: Record<ViewId, string> = {
  bots: '', tasks: '', shared: '',
  chat: '与 Soren 对话',
  memories: '查看与管理 AI 记住的内容',
  channels: '飞书 / 钉钉机器人接入',
  settings: '模型与界面配置',
};

function AppContent({ bots, activeBot, onSelectBot, onReloadBots, botError }: { bots: Bot[]; activeBot: Bot; onSelectBot: (id: string) => void; onReloadBots: () => Promise<void>; botError: string | null }) {
  const toast = useToast();
  const { settings, setLLM, setGeneral } = useSettings();
  const { theme, toggleTheme } = useTheme();

  const [view, setView] = useState<ViewId>('chat');
  const guide = useGettingStarted();
  const [exampleTask, setExampleTask] = useState(false);
  const navigateGuide = (next: ViewId, example = false) => { setExampleTask(example); setView(next); };
  const [mobileOpen, setMobileOpen] = useState(false);
  const isDesktop = '__TAURI_INTERNALS__' in window;
  useEffect(() => {
    if (!mobileOpen) return;
    const close = (event: KeyboardEvent) => { if (event.key === 'Escape') setMobileOpen(false); };
    window.addEventListener('keydown', close);
    return () => window.removeEventListener('keydown', close);
  }, [mobileOpen]);

  // ── 数据源 ────────────────────────────────────────────
  const health = useHealth();
  const conversations = useConversations();
  const memories = useMemories();
  const channels = useChannels();

  /** 新建对话后由 useChat 回调写入列表标题 */
  const handleConversationCreated = useCallback(
    (conversationId: string, firstMessage: string) => {
      const now = new Date().toISOString();
      conversations.prependLocal({
        id: conversationId,
        title: firstMessage.length > 50 ? `${firstMessage.slice(0, 50)}…` : firstMessage,
        messages: [],
        createdAt: now,
        updatedAt: now,
      });
    },
    [conversations],
  );

  const chat = useChat({ onConversationCreated: handleConversationCreated });

  /** 记忆首次加载完成后刷新侧栏徽标 */
  const memoryCount = settings.general.showMemoryBadge
    ? memories.stats?.total ?? undefined
    : undefined;

  const activeChannelCount = useMemo(
    () => channels.channels.filter((item) => item.running).length,
    [channels.channels],
  );

  // ── 全局快捷键 ────────────────────────────────────────
  /** Cmd/Ctrl+N：新建对话 */
  useHotkey(
    'n',
    useCallback(() => {
      chat.reset();
      setView('chat');
    }, [chat]),
    { meta: true },
  );

  /** Cmd/Ctrl+,：打开设置 */
  useHotkey(
    ',',
    useCallback(() => setView('settings'), []),
    { meta: true },
  );

  /** Cmd/Ctrl+1/2/3/4：切换视图 */
  useHotkey(
    '1',
    useCallback(() => setView('chat'), []),
    { meta: true },
  );
  useHotkey(
    '2',
    useCallback(() => setView('memories'), []),
    { meta: true },
  );
  useHotkey(
    '3',
    useCallback(() => setView('channels'), []),
    { meta: true },
  );
  useHotkey(
    '4',
    useCallback(() => setView('settings'), []),
    { meta: true },
  );

  // ── 后端恢复在线时自动重载数据 ────────────────────────
  // 用户从"忘了启动后端"到"启动完成"的常见路径：切回窗口 → 焦点触发 health 复查
  // → 状态变 online → 这里把各页数据补齐，避免界面停留在空列表。
  const [wasOffline, setWasOffline] = useState(false);

  useEffect(() => {
    if (health.state === 'offline') {
      setWasOffline(true);
      return;
    }
    if (health.state === 'online' && wasOffline) {
      setWasOffline(false);
      void conversations.reload();
      void memories.reload();
      void channels.reload();
    }
  }, [health.state, wasOffline, conversations, memories, channels]);

  const previousLoading = useRef(false);
  useEffect(() => {
    if (previousLoading.current && !chat.loading && !chat.error) {
      void conversations.reload();
      void memories.reload();
    }
    previousLoading.current = chat.loading;
  }, [chat.loading, chat.error, conversations.reload, memories.reload]);

  // ── 交互处理 ──────────────────────────────────────────
  const handleNewConversation = useCallback(() => {
    chat.reset();
    setView('chat');
  }, [chat]);

  const handleSelectConversation = useCallback(
    (id: string) => {
      setView('chat');
      void chat.loadConversation(id);
    },
    [chat],
  );

  const handleDeleteConversation = useCallback(
    async (id: string) => {
      try {
        await conversations.remove(id);
        // 删除的是当前会话时，清空聊天区
        if (chat.conversationId === id) {
          chat.reset();
        }
        toast.success('对话已删除');
      } catch (err) {
        toast.error(
          '删除失败',
          err instanceof Error ? err.message : '请稍后重试',
        );
        throw err;
      }
    },
    [conversations, chat, toast],
  );

  /** 清除失败态提示 */
  const handleNotify = useCallback(
    (type: 'success' | 'error', title: string, description?: string) => {
      if (type === 'success') toast.success(title, description);
      else toast.error(title, description);
    },
    [toast],
  );

  return (
    <div className={'app-shell ' + (isDesktop ? 'desktop-runtime' : '')}>
      {/* ── 侧边栏 ──────────────────────────────────────── */}
      <Sidebar
        view={view}
        bots={bots}
        activeBot={activeBot}
        onSelectBot={onSelectBot}
        onNavigate={setView}
        conversations={conversations.conversations}
        currentConversationId={chat.conversationId}
        onSelectConversation={handleSelectConversation}
        onDeleteConversation={handleDeleteConversation}
        onNewConversation={handleNewConversation}
        memoryCount={memoryCount}
        activeChannelCount={activeChannelCount}
        loading={conversations.loading}
        mobileOpen={mobileOpen}
        onClose={() => setMobileOpen(false)}
      />

      {/* ── 主区域 ──────────────────────────────────────── */}
      <div className="app-main" role="main">
        <TitleBar
          actions={<Button size="sm" variant="ghost" onClick={() => guide.setOpen(true)}>使用引导</Button>}
          title={view === 'chat' ? activeBot.name : VIEW_TITLES[view]}
          subtitle={VIEW_SUBTITLES[view]}
          connection={health.state}
          theme={theme}
          onToggleTheme={toggleTheme}
          onOpenMenu={() => setMobileOpen(true)}
        />

        <GettingStarted guide={guide} modelConfigured={health.state === 'online' && health.health?.demoMode === false} onNavigate={navigateGuide} />

        {view === 'chat' && (
          <ChatPage
            chat={chat}
            bot={activeBot}
            offline={health.state === 'offline'}
            onRetryConnection={() => void health.refresh()}
            retrying={health.state === 'connecting'}
            model={activeBot.model || settings.llm.model}
            demoMode={health.health?.demoMode}
            onModelSettings={() => setView('settings')}
          />
        )}

        {view === 'bots' && <BotsPage bots={bots} active={activeBot.id} onSelect={id => { onSelectBot(id); setView('chat'); }} onReload={onReloadBots} error={botError} />}
        {view === 'tasks' && <TasksPage example={exampleTask} onExampleUsed={() => setExampleTask(false)} bots={bots} bot={activeBot} demo={health.health?.demoMode} />}
        {view === 'shared' && <SharedNotesPage bots={bots} />}

        {view === 'memories' && (
          <MemoriesPage
            memories={memories}
            offline={health.state === 'offline'}
            onRetryConnection={() => void health.refresh()}
            onDeleted={() => void memories.reload()}
          />
        )}

        {view === 'channels' && (
          <ChannelsPage
            channels={channels.channels}
            loading={channels.loading}
            error={channels.error}
            offline={health.state === 'offline'}
            pendingName={channels.pendingName}
            onEnable={channels.enable}
            onDisable={channels.disable}
            onReload={channels.reload}
            onRetryConnection={() => void health.refresh()}
            onNotify={handleNotify}
          />
        )}

        {view === 'settings' && (
          <SettingsPage
            connection={health.state}
            health={health.health}
            healthError={health.error}
            lastCheckedAt={health.lastCheckedAt}
            onRefreshHealth={health.refresh}
            theme={settings.general.theme}
            onSetTheme={(next) => setGeneral({ theme: next })}
            language={settings.general.language}
            onSetLanguage={(next) => setGeneral({ language: next })}
            showMemoryBadge={settings.general.showMemoryBadge}
            onSetShowMemoryBadge={(value) => setGeneral({ showMemoryBadge: value })}
            llm={{
              provider: settings.llm.provider,
              apiKey: settings.llm.apiKey,
              baseUrl: settings.llm.baseUrl,
              model: settings.llm.model,
              temperature: settings.llm.temperature,
              maxTokens: settings.llm.maxTokens,
            }}
            onSetLLM={setLLM}
            onNotify={handleNotify}
          />
        )}
      </div>
    </div>
  );
}

export default function App() {
  const [bots, setBots] = useState<Bot[]>([DEFAULT_BOT]);
  const [activeID, setActiveID] = useState('soren');
  const [botError, setBotError] = useState<string | null>(null);
  const restored = useRef(false);
  const choose = useCallback((id: string) => {
    setActiveBotID(id); setActiveID(id);
    try { localStorage.setItem('soren.active-bot', id); } catch { /* Optional preference storage. */ }
  }, []);
  const reload = useCallback(async () => {
    try {
      const data = await agentsAPI.list(); setBots(data); setBotError(null);
      if (!restored.current) {
        restored.current = true;
        let saved = 'soren'; try { saved = localStorage.getItem('soren.active-bot') || 'soren'; } catch { /* private mode */ }
        choose(data.some(b => b.id === saved && b.status !== 'archived') ? saved : 'soren');
      }
    } catch (e) { setBotError(describeError(e)); }
  }, [choose]);
  useEffect(() => { let disposed = false; let timer: ReturnType<typeof setTimeout>; const poll = async () => { if (disposed) return; await reload(); if (!disposed) timer = setTimeout(poll, 10000); }; void poll(); return () => { disposed = true; clearTimeout(timer); }; }, [reload]);
  useEffect(() => { if (!bots.some(b => b.id === activeID && b.status !== 'archived')) choose('soren'); }, [bots, activeID, choose]);
  const activeBot = bots.find(b => b.id === activeID) || DEFAULT_BOT;
  return <ToastProvider><AppContent key={activeID} bots={bots} activeBot={activeBot} onSelectBot={choose} onReloadBots={reload} botError={botError} /></ToastProvider>;
}
