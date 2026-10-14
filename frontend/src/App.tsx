/**
 * Soren 应用根组件
 *
 * 职责：
 * 1. 提供通知上下文与主题
 * 2. 头像侧栏切换 Bot 与群聊，其他工具集中在设置内导航
 * 3. 组装侧边栏、顶部栏与当前页面
 *
 * 数据流：各页面的数据由对应的 Hook 在 App 层统一持有，
 * Bot 运行动效与任务页共享状态，对话和记忆也随页面切换保留。
 *
 * 选中对象（sel）是单聊与群聊的统一入口：两者的 key 不同，
 * AppContent 因此各自持有独立状态，切走再切回不互相污染。
 */

import { useCallback, useEffect, useState, useRef } from 'react';
import { GettingStarted, useGettingStarted } from './components/GettingStarted';
import { SettingsNav } from './components/layout/SettingsNav';
import { ConversationHistoryPage } from './pages/ConversationHistoryPage';
import { TemplateImportHost } from './components/BotTemplates';
import { BotEditor, blankBot } from './components/BotEditor';
import { CreateGroupDialog } from './components/CreateGroupDialog';
import { RenameChatDialog } from './components/RenameChatDialog';
import { useBotInbox, type BotInbox } from './hooks/useBotInbox';
import { useBotRuns } from './hooks/useBotRuns';
import { useGroupList } from './hooks/useGroupChat';
import { SearchPalette } from './components/SearchPalette';
import { Sidebar } from './components/layout/Sidebar';
import { TitleBar } from './components/layout/TitleBar';
import { ConfirmDialog } from './components/ui';
import { ToastProvider, useToast } from './hooks/useToast';
import { useChannels } from './hooks/useChannels';
import { useChat } from './hooks/useChat';
import { useConversations } from './hooks/useConversations';
import { useHealth } from './hooks/useHealth';
import { useMemories } from './hooks/useMemories';
import { useSettings, useTheme } from './hooks/useSettings';
import { useHotkey } from './hooks/useUtilities';
import { ChatPage } from './pages/ChatPage';
import { GroupChatRoute } from './pages/GroupChatPage';
import { ChannelsPage } from './pages/ChannelsPage';
import { MemoriesPage } from './pages/MemoriesPage';
import { SettingsPage } from './pages/SettingsPage';
import { BotsPage } from './pages/BotsPage';
import { TasksPage } from './pages/TasksPage';
import { SharedNotesPage } from './pages/SharedNotesPage';
import { agentsAPI, DEFAULT_BOT, type Bot } from './services/agents';
import { groupsAPI } from './services/groups';
import { setActiveBotID } from './services/botScope';
import { describeError, normalizeError } from './services/api';
import type { GroupCreateInput, GroupView, Selection, ViewId } from './types';

/** 视图标题映射 */
const VIEW_TITLES: Record<ViewId, string> = {
  bots: '我的 Bot', tasks: '任务', shared: '共享资料',
  chat: '对话',
  memories: '我的记忆',
  channels: '应用连接',
  settings: '通用设置', history: '对话记录',
};

function AppContent({ bots, activeBot, sel, groups, groupsError, onSelectBot, onSelectGroup, onReloadGroups, onReloadBots, botError, inbox, onReloadInbox, initialConversation, onBotCreated, onBotUpdated, onGroupCreated, sidebarCollapsed, onSidebarCollapsedChange }: {
  bots: Bot[]; activeBot: Bot; sel: Selection; groups: GroupView[]; groupsError: string | null;
  onSelectBot: (id: string, conversationId?: string) => void; onSelectGroup: (id: string) => void;
  onReloadGroups: () => Promise<void>; onReloadBots: () => Promise<void>;
  botError: string | null; inbox: Record<string, BotInbox>; onReloadInbox: () => Promise<void>;
  initialConversation: string | null; onBotCreated: (bot: Bot) => void;
  onBotUpdated: (bot: Bot) => void;
  onGroupCreated: (id: string) => void;
  sidebarCollapsed: boolean; onSidebarCollapsedChange: (collapsed: boolean) => void;
}) {
  const toast = useToast();
  const { settings, setLLM, setGeneral } = useSettings();
  useTheme();

  const [searchOpen, setSearchOpen] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const closeSidebar = useCallback(() => setMobileOpen(false), []);
  const [newBot, setNewBot] = useState<Bot | null>(null);
  const [newGroup, setNewGroup] = useState(false);
  const [editBot, setEditBot] = useState<Bot | null>(null);
  const [editGroup, setEditGroup] = useState<GroupView | null>(null);
  const [groupRevision, setGroupRevision] = useState(0);
  const [renameGroup, setRenameGroup] = useState<GroupView | null>(null);
  const [renameBot, setRenameBot] = useState<Bot | null>(null);
  const [deleteBot, setDeleteBot] = useState<Bot | null>(null);
  const [deletingBot, setDeletingBot] = useState(false);
  const [deleteBotError, setDeleteBotError] = useState('');
  const [deleteGroup, setDeleteGroup] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [groupSpeaking, setGroupSpeaking] = useState(false);
  const restoredConversation = useRef(false);
  const [view, setView] = useState<ViewId>('chat');
  const guide = useGettingStarted();
  const [exampleTask, setExampleTask] = useState(false);
  const navigateGuide = (next: ViewId, example = false) => { setExampleTask(example); setView(next); };
  const isDesktop = '__TAURI_INTERNALS__' in window;
  const isGroup = sel.type === 'group';

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
  useHotkey('1', useCallback(() => setView('chat'), []), { meta: true });
  useHotkey('2', useCallback(() => setView('memories'), []), { meta: true });
  useHotkey('3', useCallback(() => setView('channels'), []), { meta: true });
  useHotkey('4', useCallback(() => setView('settings'), []), { meta: true });

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
      void onReloadBots();
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

  const activeGroup = isGroup ? groups.find(group => group.id === sel.id) : undefined;

  const confirmDelete = useCallback(async () => {
    if (!deleteGroup) return;
    setDeleting(true); setDeleteError('');
    try {
      await groupsAPI.remove(deleteGroup);
      await onReloadGroups();
      setDeleteGroup(null);
      // 删除入口搬到了侧栏右键菜单，删的未必是正在看的那个群：
      // 只有删掉的正是当前选中项时才回落到可用 Bot，否则用户正在看的群被静默换掉
      if (isGroup && sel.id === deleteGroup) {
        onSelectBot(bots.find(bot => bot.status !== 'archived')?.id ?? 'soren');
      }
    } catch (err) {
      setDeleteError(describeError(err));
    } finally {
      setDeleting(false);
    }
  }, [deleteGroup, bots, isGroup, sel.id, onReloadGroups, onSelectBot]);

  const handleCreateGroup = useCallback(async (input: GroupCreateInput) => {
    try {
      const group = await groupsAPI.create(input);
      await onReloadGroups();
      setNewGroup(false);
      onGroupCreated(group.id);
    } catch (err) {
      // 字段级错误由弹窗就地展示；其余才是需要打断用户的失败
      if (normalizeError(err).status !== 400) {
        toast.error('创建群聊失败', describeError(err));
      }
      throw err;
    }
  }, [onReloadGroups, onGroupCreated, toast]);

  /**
   * 置顶 / 取消置顶
   *
   * PATCH 的字段全是可选指针，这里只传 pinned；顺序由后端维护（置顶在前），
   * 前端回拉一次列表即可，不自己再排一次。
   */
  const handlePinGroup = useCallback(async (id: string, pinned: boolean) => {
    try {
      await groupsAPI.update(id, { pinned });
      await onReloadGroups();
    } catch (err) {
      toast.error(pinned ? '置顶失败' : '取消置顶失败', describeError(err));
    }
  }, [onReloadGroups, toast]);

  /** 只发 { title }：其余字段是指针，不传就不会被覆盖 */
  const handleRenameGroup = useCallback(async (target: GroupView, title: string) => {
    await groupsAPI.update(target.id, { title });
    await onReloadGroups();
    setRenameGroup(null);
  }, [onReloadGroups]);

  const handlePinBot = async (id: string, pinned: boolean) => {
    const target = bots.find(bot => bot.id === id);
    if (!target) return;
    try {
      onBotUpdated(await agentsAPI.updateNavigation(id, { version: target.version, pinned }));
      await onReloadBots();
    } catch (err) {
      toast.error(pinned ? '置顶失败' : '取消置顶失败', describeError(err));
    }
  };
  const handleRenameBot = async (target: Bot, name: string) => {
    onBotUpdated(await agentsAPI.updateNavigation(target.id, { version: target.version, name }));
    setRenameBot(null);
    await onReloadBots();
    await onReloadGroups();
  };
  const confirmDeleteBot = async () => {
    if (!deleteBot) return;
    setDeletingBot(true); setDeleteBotError('');
    try {
      const archived = await agentsAPI.updateNavigation(deleteBot.id, { version: deleteBot.version, archive: true });
      setDeleteBot(null);
      onBotUpdated(archived);
      await onReloadBots();
      await onReloadGroups();
    } catch (err) {
      setDeleteBotError(describeError(err));
    } finally { setDeletingBot(false); }
  };

  return (
    <div className={'app-shell ' + (isDesktop ? 'desktop-runtime' : '')}>
      {/* ── 侧边栏 ──────────────────────────────────────── */}
      <Sidebar
        bots={bots}
        activeBot={activeBot}
        collapsed={sidebarCollapsed}
        onCollapsedChange={onSidebarCollapsedChange}
        onPinBot={(id, pinned) => { void handlePinBot(id, pinned); }}
        onConfigureBot={id => setEditBot(bots.find(bot => bot.id === id) ?? null)}
        onConfigureGroup={id => setEditGroup(groups.find(group => group.id === id) ?? null)}
        onRenameBot={id => setRenameBot(bots.find(bot => bot.id === id) ?? null)}
        onDeleteBot={id => { setDeleteBotError(''); setDeleteBot(bots.find(bot => bot.id === id) ?? null); }}
        onSelectBot={id => { if (isGroup || id !== activeBot.id) onSelectBot(id); else if (!chat.loading && inbox[id]?.latestId && chat.conversationId !== inbox[id].latestId) handleSelectConversation(inbox[id].latestId!); setView('chat'); }}
        onSearch={() => setSearchOpen(true)}
        onCreateBot={() => setNewBot(blankBot())}
        onCreateGroup={() => setNewGroup(true)}
        groups={groups}
        activeGroupId={isGroup ? sel.id : null}
        onSelectGroup={id => { onSelectGroup(id); setView('chat'); }}
        onPinGroup={(id, pinned) => { void handlePinGroup(id, pinned); }}
        onRenameGroup={id => setRenameGroup(groups.find(group => group.id === id) ?? null)}
        onDeleteGroup={id => { setDeleteError(''); setDeleteGroup(id); }}
        groupError={groupsError}
        onRetryGroups={() => void onReloadGroups()}
        inbox={inbox}
        error={botError}
        onRetry={() => { void onReloadBots(); void onReloadInbox(); }}
        mobileOpen={mobileOpen}
        onClose={closeSidebar}
        settingsOpen={view !== 'chat'}
        onOpenSettings={() => setView('settings')}
        groupSpeaking={groupSpeaking}
        activity={chat.loading ? { ...runs.activity, [activeBot.id]: runs.activity[activeBot.id] === 'waiting_approval' ? 'waiting_approval' : 'running' } : runs.activity}
      />

      {searchOpen && <SearchPalette bots={bots} inbox={inbox} groups={groups} onClose={() => setSearchOpen(false)} onSelect={(botId, id) => { if (isGroup || botId !== activeBot.id) onSelectBot(botId, id); else { const target = id || inbox[botId]?.latestId; if (target && target !== chat.conversationId) handleSelectConversation(target); setView('chat'); } }} onSelectGroup={id => { onSelectGroup(id); setView('chat'); }} onNavigate={setView} />}

      {newBot && <BotEditor initial={newBot} bots={bots} onClose={() => setNewBot(null)} onSaved={async bot => { setNewBot(null); onBotCreated(bot); setView('chat'); }} />}

      {editBot && <BotEditor initial={editBot} bots={bots} onClose={() => setEditBot(null)} onSaved={async bot => { onBotUpdated(bot); setEditBot(null); await Promise.all([onReloadBots(), onReloadGroups()]); }} />}
      {editGroup && <CreateGroupDialog key={editGroup.id} open initial={editGroup} bots={bots.filter(bot => bot.status !== 'archived')} onClose={() => setEditGroup(null)} onCreate={async input => {
        await groupsAPI.update(editGroup.id, { title: input.title, topic: input.topic, botIds: input.botIds });
        await onReloadGroups(); setEditGroup(null); setGroupRevision(value => value + 1);
      }} />}

      {newGroup && <CreateGroupDialog open bots={bots.filter(bot => bot.status !== 'archived')} onClose={() => setNewGroup(false)} onCreate={handleCreateGroup} />}

      {renameGroup && <RenameChatDialog open kind="group" initialTitle={renameGroup.title}
        onClose={() => setRenameGroup(null)}
        onRename={title => handleRenameGroup(renameGroup, title)} />}

      {renameBot && <RenameChatDialog open kind="bot" initialTitle={renameBot.name}
        onClose={() => setRenameBot(null)}
        onRename={name => handleRenameBot(renameBot, name)} />}

      <ConfirmDialog open={Boolean(deleteBot)} title="归档 Bot" danger loading={deletingBot}
        confirmText="归档 Bot" cancelText="取消" onConfirm={() => void confirmDeleteBot()}
        onCancel={() => { if (!deletingBot) { setDeleteBot(null); setDeleteBotError(''); } }}
        message={<>
          将「{deleteBot?.name}」移入已归档并从侧栏移除。对话和记忆会保留，可在「我的 Bot → 已归档」恢复；归档后该 Bot 无法继续执行任务或参与群聊。
          {deleteBotError && <span role="alert" className="mt-2 block text-error">{deleteBotError}</span>}
        </>} />

      <ConfirmDialog
        open={Boolean(deleteGroup)}
        title="删除群聊"
        danger
        loading={deleting}
        confirmText="删除群聊"
        cancelText="取消"
        onConfirm={() => void confirmDelete()}
        onCancel={() => { if (!deleting) { setDeleteGroup(null); setDeleteError(''); } }}
        message={<>
          删除后群聊与其中的对话记录将无法恢复，成员的私有记忆会保留。
          {deleteError && <span role="alert" className="mt-2 block text-error">{deleteError}</span>}
        </>}
      />

      {/* ── 主区域 ──────────────────────────────────────── */}
      <div className="app-main" role="main">
        <TitleBar
          onOpenGuide={() => guide.setOpen(true)}
          title={view === 'chat' ? (isGroup ? activeGroup?.title ?? '群聊' : activeBot.name) : VIEW_TITLES[view]}
          settingsOpen={view !== 'chat'}
          onOpenMenu={() => setMobileOpen(true)}
        />
        {view !== 'chat' && <SettingsNav view={view} onNavigate={setView} memoryCount={memoryCount} />}

        <GettingStarted guide={guide} modelConfigured={health.state === 'online' && health.health?.demoMode === false} onNavigate={navigateGuide} />

        {view === 'chat' && (isGroup
          ? <GroupChatRoute key={`${sel.id}:${groupRevision}`} groupId={sel.id} bots={bots} offline={health.state === 'offline'} onRetryConnection={() => void health.refresh()} retrying={health.state === 'connecting'} onSpeakingChange={setGroupSpeaking} />
          : <ChatPage
            chat={chat}
            bot={activeBot}
            offline={health.state === 'offline'}
            onRetryConnection={() => void health.refresh()}
            retrying={health.state === 'connecting'}
            demoMode={health.health?.demoMode}
            onModelSettings={() => setView('settings')}
          />)}

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

/** localStorage 在隐私模式下会抛错，读写都按可选偏好处理 */
function readStored(key: string): string | null {
  try { return localStorage.getItem(key); } catch { return null; }
}
function writeStored(key: string, value: string): void {
  try { localStorage.setItem(key, value); } catch { /* Optional preference storage. */ }
}
function dropStored(key: string): void {
  try { localStorage.removeItem(key); } catch { /* Optional preference storage. */ }
}

export default function App() {
  // Keep layout outside the keyed conversation subtree so switching Bots never resets it.
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => readStored('soren.sidebar-collapsed') === 'true');
  const changeSidebarCollapsed = useCallback((collapsed: boolean) => {
    setSidebarCollapsed(collapsed);
    writeStored('soren.sidebar-collapsed', String(collapsed));
  }, []);
  const [bots, setBots] = useState<Bot[]>([DEFAULT_BOT]);
  const [botsLoaded, setBotsLoaded] = useState(false);
  const [activeID, setActiveID] = useState('soren');
  const [activeGroupID, setActiveGroupID] = useState<string | null>(null);
  const [initialConversation, setInitialConversation] = useState<string | null>(null);
  const botInbox = useBotInbox(bots);
  const groupList = useGroupList();
  const [botError, setBotError] = useState<string | null>(null);
  const [restored, setRestored] = useState(false);

  const sel: Selection = activeGroupID ? { type: 'group', id: activeGroupID } : { type: 'bot', id: activeID };

  /** 两个选中入口互斥：最后一次选择决定持久化到哪个键，避免恢复出陈旧选择 */
  const chooseBot = useCallback((id: string, conversationId?: string) => {
    setInitialConversation(conversationId ?? null);
    setActiveBotID(id); setActiveID(id); setActiveGroupID(null);
    writeStored('soren.active-bot', id);
    dropStored('soren.active-group');
  }, []);
  const chooseGroup = useCallback((id: string) => {
    setInitialConversation(null);
    setActiveGroupID(id);
    writeStored('soren.active-group', id);
    dropStored('soren.active-bot');
  }, []);

  const reload = useCallback(async () => {
    try {
      const data = await agentsAPI.list(); setBots(data); setBotsLoaded(true); setBotError(null);
    } catch (e) { setBotError(describeError(e)); }
  }, []);
  useEffect(() => { let disposed = false; let timer: ReturnType<typeof setTimeout>; const poll = async () => { if (disposed) return; await reload(); if (!disposed) timer = setTimeout(poll, 10000); }; void poll(); return () => { disposed = true; clearTimeout(timer); }; }, [reload]);

  /** 启动恢复：优先上次打开的群，群已不存在则回落到上次的 Bot */
  useEffect(() => {
    if (restored || !botsLoaded || groupList.loading) return;
    const savedGroup = readStored('soren.active-group');
    // 用 state 而不是 ref：与下面的选中项更新落在同一次渲染里，
    // 自动回退读到的是恢复后的选择，不会用旧值把它覆盖掉
    if (savedGroup && groupList.groups.some(group => group.id === savedGroup)) {
      setRestored(true); chooseGroup(savedGroup); return;
    }
    const savedBot = readStored('soren.active-bot') || 'soren';
    setRestored(true);
    chooseBot(bots.some(b => b.id === savedBot && b.status !== 'archived') ? savedBot : 'soren');
  }, [restored, botsLoaded, groupList.loading, groupList.groups, bots, chooseBot, chooseGroup]);

  // 自动回退只对单聊生效：群选中时把用户弹回 Soren 是静默改写他的选择
  useEffect(() => {
    if (!restored || sel.type !== 'bot') return;
    if (!bots.some(b => b.id === sel.id && b.status !== 'archived')) chooseBot('soren');
  }, [restored, bots, sel.type, sel.id, chooseBot]);

  const activeBot = bots.find(b => b.id === activeID) || DEFAULT_BOT;
  return <ToastProvider><AppContent
    key={sel.type + ':' + sel.id}
    bots={bots}
    activeBot={activeBot}
    sidebarCollapsed={sidebarCollapsed}
    onSidebarCollapsedChange={changeSidebarCollapsed}
    sel={sel}
    groups={groupList.groups}
    groupsError={groupList.error}
    onSelectBot={chooseBot}
    onSelectGroup={id => { chooseGroup(id); }}
    onReloadGroups={groupList.reload}
    onReloadBots={reload}
    botError={botError}
    inbox={botInbox.inbox}
    onReloadInbox={botInbox.reload}
    initialConversation={initialConversation}
    onBotCreated={bot => { setBots(previous => [...previous.filter(item => item.id !== bot.id), bot]); chooseBot(bot.id); void reload(); }}
    onBotUpdated={bot => setBots(previous => previous.map(item => item.id === bot.id ? bot : item))}
    onGroupCreated={chooseGroup}
  /><TemplateImportHost onImported={bot => { setBots(previous => [...previous.filter(item => item.id !== bot.id), bot]); chooseBot(bot.id); void reload(); }} /></ToastProvider>;
}
