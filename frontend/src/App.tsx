/**
 * Soren 应用根组件
 *
 * 职责：
 * 1. 提供通知上下文与主题
 * 2. 头像侧栏切换 Bot，其他工具集中在设置内导航
 * 3. 组装侧边栏、顶部栏与当前页面
 *
 * 数据流：各页面的数据由对应的 Hook 在 App 层统一持有，
 * Bot 运行动效与任务页共享状态，对话和记忆也随页面切换保留。
 */

import { useCallback, useEffect, useState, useRef } from 'react';
import { GettingStarted, useGettingStarted } from './components/GettingStarted';
import { SettingsNav } from './components/layout/SettingsNav';
import { ConversationHistoryPage } from './pages/ConversationHistoryPage';
import { BotEditor, blankBot } from './components/BotEditor';
import { useBotInbox, type BotInbox } from './hooks/useBotInbox';
import { useBotRuns } from './hooks/useBotRuns';
import { SearchPalette } from './components/SearchPalette';
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
  settings: '通用设置', history: '对话记录',
};

function AppContent({ bots, activeBot, onSelectBot, onReloadBots, botError, inbox, onReloadInbox, initialConversation, onBotCreated }: { bots: Bot[]; activeBot: Bot; onSelectBot: (id: string, conversationId?: string) => void; onReloadBots: () => Promise<void>; botError: string | null; inbox: Record<string, BotInbox>; onReloadInbox: () => Promise<void>; initialConversation: string | null; onBotCreated: (bot: Bot) => void }) {
  const toast = useToast();
  const { settings, setLLM, setGeneral } = useSettings();
  useTheme();

  const [searchOpen, setSearchOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const closeSidebar = useCallback(() => setMobileOpen(false), []);
  const [newBot, setNewBot] = useState<Bot | null>(null);
  const restoredConversation = useRef(false);
  const [view, setView] = useState<ViewId>('chat');
  const guide = useGettingStarted();
  const [exampleTask, setExampleTask] = useState(false);
  const navigateGuide = (next: ViewId, example = false) => { setExampleTask(example); setView(next); };
  const isDesktop = '__TAURI_INTERNALS__' in window;

  // ── 数据源 ────────────────────────────────────────────
  const health = useHealth();
  const conversations = useConversations();
  const memories = useMemories();
  const channels = useChannels();
  const runs = useBotRuns(bots, health.state === 'online');

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
  useEffect(() => {
    if (restoredConversation.current || conversations.loading || conversations.error) return;
    restoredConversation.current = true;
    const id = initialConversation || conversations.conversations[0]?.id;
    if (id) void chat.loadConversation(id);
  }, [initialConversation, conversations.loading, conversations.error, conversations.conversations, chat.loadConversation]);


  /** 在设置导航中展示记忆数量 */
  const memoryCount = settings.general.showMemoryBadge
    ? memories.stats?.total ?? undefined
    : undefined;

  // ── 全局快捷键 ────────────────────────────────────────
  useHotkey('k', useCallback(() => { setMobileOpen(false); setSearchOpen(true); }, []), { meta: true });

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
      void onReloadInbox();
    }
    previousLoading.current = chat.loading;
  }, [chat.loading, chat.error, conversations.reload, memories.reload, onReloadInbox]);

  // ── 交互处理 ──────────────────────────────────────────
  const handleSelectConversation = useCallback(
    (id: string) => {
      restoredConversation.current = true;
      setView('chat');
      void chat.loadConversation(id);
    },
    [chat],
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
        bots={bots}
        activeBot={activeBot}
        onSelectBot={id => { if (id !== activeBot.id) onSelectBot(id); else if (!chat.loading && inbox[id]?.latestId && chat.conversationId !== inbox[id].latestId) handleSelectConversation(inbox[id].latestId!); setView('chat'); }}
        onSearch={() => setSearchOpen(true)}
        onCreateBot={() => setNewBot(blankBot())}
        inbox={inbox}
        error={botError}
        onRetry={() => { void onReloadBots(); void onReloadInbox(); }}
        mobileOpen={mobileOpen}
        onClose={closeSidebar}
        settingsOpen={view !== 'chat'}
        onOpenSettings={() => setView('settings')}
        activity={chat.loading ? { ...runs.activity, [activeBot.id]: runs.activity[activeBot.id] === 'waiting_approval' ? 'waiting_approval' : 'running' } : runs.activity}
      />

      {searchOpen && <SearchPalette bots={bots} inbox={inbox} onClose={() => setSearchOpen(false)} onSelect={(botId, id) => { if (botId !== activeBot.id) onSelectBot(botId, id); else { const target = id || inbox[botId]?.latestId; if (target && target !== chat.conversationId) handleSelectConversation(target); setView('chat'); } }} onNavigate={setView} />}

      {newBot && <BotEditor initial={newBot} bots={bots} onClose={() => setNewBot(null)} onSaved={async bot => { setNewBot(null); onBotCreated(bot); setView('chat'); }} />}

      {/* ── 主区域 ──────────────────────────────────────── */}
      <div className="app-main" role="main">
        <TitleBar
          onOpenGuide={() => guide.setOpen(true)}
          title={view === 'chat' ? activeBot.name : VIEW_TITLES[view]}
          connection={health.state}
          settingsOpen={view !== 'chat'}
          onOpenMenu={() => setMobileOpen(true)}
        />
        {view !== 'chat' && <SettingsNav view={view} onNavigate={setView} memoryCount={memoryCount} />}

        <GettingStarted guide={guide} modelConfigured={health.state === 'online' && health.health?.demoMode === false} onNavigate={navigateGuide} />

        {view === 'chat' && (
          <ChatPage
            chat={chat}
            bot={activeBot}
            offline={health.state === 'offline'}
            onRetryConnection={() => void health.refresh()}
            retrying={health.state === 'connecting'}
            demoMode={health.health?.demoMode}
            onModelSettings={() => setView('settings')}
          />
        )}

        {view === 'bots' && <BotsPage bots={bots} active={activeBot.id} onSelect={id => { onSelectBot(id); setView('chat'); }} onReload={onReloadBots} error={botError} />}
        {view === 'history' && <ConversationHistoryPage bots={bots} inbox={inbox} onReload={onReloadInbox} onSelect={(botId, id) => { if (botId === activeBot.id) { if (id) handleSelectConversation(id); setView('chat'); } else onSelectBot(botId, id); }} />}
        {view === 'tasks' && <TasksPage runs={runs.byBot[activeBot.id] ?? []} runsError={runs.errors[activeBot.id]} onReload={runs.reload} example={exampleTask} onExampleUsed={() => setExampleTask(false)} bots={bots} bot={activeBot} demo={health.health?.demoMode} />}
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
  const [initialConversation, setInitialConversation] = useState<string | null>(null);
  const botInbox = useBotInbox(bots);
  const [botError, setBotError] = useState<string | null>(null);
  const restored = useRef(false);
  const choose = useCallback((id: string, conversationId?: string) => {
    setInitialConversation(conversationId ?? null);
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
  return <ToastProvider><AppContent key={activeID} bots={bots} activeBot={activeBot} onSelectBot={choose} onReloadBots={reload} botError={botError} inbox={botInbox.inbox} onReloadInbox={botInbox.reload} initialConversation={initialConversation} onBotCreated={bot => { setBots(previous => [...previous.filter(item => item.id !== bot.id), bot]); choose(bot.id); void reload(); }} /></ToastProvider>;
}
