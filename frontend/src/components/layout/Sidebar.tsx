import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { Bot } from '../../services/agents';
import { GroupAvatar } from '../GroupAvatar';
import { BotAvatar } from '../BotAvatar';
import { SidebarContextMenu } from '../SidebarContextMenu';
import type { BotActivity } from '../../hooks/useBotRuns';
import type { BotInbox } from '../../hooks/useBotInbox';
import { IconSettings, IconSearch, IconPlus, IconClose, IconChat, IconSidebarCollapse, IconSidebarExpand, IconRefresh } from '../icons';
import type { GroupView } from '../../types';

export interface SidebarProps {
  bots: Bot[]; activeBot: Bot; onSelectBot: (id: string) => void;
  collapsed: boolean; onCollapsedChange: (collapsed: boolean) => void;
  onPinBot: (id: string, pinned: boolean) => void;
  onRenameBot: (id: string) => void;
  onConfigureBot: (id: string) => void;
  onDeleteBot: (id: string) => void;
  onSearch: () => void;
  onCreateBot: () => void;
  onCreateGroup: () => void;
  groups: GroupView[]; activeGroupId: string | null; onSelectGroup: (id: string) => void;
  /** 右键菜单动作，交给 App 层执行数据变更 */
  onPinGroup: (id: string, pinned: boolean) => void;
  onRenameGroup: (id: string) => void;
  onConfigureGroup: (id: string) => void;
  onDeleteGroup: (id: string) => void;
  groupError: string | null; onRetryGroups: () => void;
  settingsOpen: boolean; onOpenSettings: () => void;
  activity: Record<string, BotActivity>; inbox: Record<string, BotInbox>;
  error: string | null; onRetry: () => void; mobileOpen: boolean; onClose: () => void;
  /** 讨论进行中禁止删除：删除会级联清理与在途讨论冲突 */
  groupSpeaking?: boolean;
}
const activityLabels: Record<BotActivity, string> = {
  running: '正在执行任务…', queued: '任务排队中', waiting_approval: '任务等待确认', paused: '任务已暂停',
};

export function Sidebar({ bots, activeBot, collapsed, onCollapsedChange, onSelectBot, onPinBot, onRenameBot, onConfigureBot, onDeleteBot, onSearch, onCreateBot, onCreateGroup, groups, activeGroupId, onSelectGroup, onPinGroup, onRenameGroup, onConfigureGroup, onDeleteGroup, groupError, onRetryGroups, settingsOpen, onOpenSettings, activity, inbox, error, onRetry, mobileOpen, onClose, groupSpeaking = false }: SidebarProps) {
  const panel = useRef<HTMLElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const createTrigger = useRef<HTMLButtonElement>(null);
  const createMenu = useRef<HTMLDivElement>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [createActive, setCreateActive] = useState(0);
  const [createPosition, setCreatePosition] = useState({ left: 0, top: 0 });
  const [contextMenu, setContextMenu] = useState<{ kind: 'bot' | 'group'; id: string; anchor: { x: number; y: number } } | null>(null);
  const menuOwner = useRef<HTMLButtonElement | null>(null);
  const uid = useId();
  const menuId = `${uid}-inbox-create`;
  const navId = `${uid}-bot-navigation`;
  const items = [
    { label: '创建 Bot', Icon: IconPlus, run: onCreateBot },
    { label: '创建群聊', Icon: IconChat, run: onCreateGroup },
  ];

  useEffect(() => {
    if (!mobileOpen) return;
    const desktop = window.matchMedia('(min-width: 761px)');
    if (desktop.matches) { onClose(); return; }
    const resize = () => { if (desktop.matches) onClose(); };
    desktop.addEventListener('change', resize);
    const previous = document.activeElement as HTMLElement | null;
    const main = document.querySelector<HTMLElement>('.app-main');
    if (main) main.inert = true;
    closeButton.current?.focus();
    const trap = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { onClose(); return; }
      if (event.key !== 'Tab') return;
      const controls = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not([disabled]), input') ?? []).filter(el => el.getClientRects().length);
      const first = controls[0], last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    document.addEventListener('keydown', trap);
    return () => { if (main) main.inert = false; desktop.removeEventListener('change', resize); document.removeEventListener('keydown', trap); previous?.focus(); };
  }, [mobileOpen, onClose]);

  // 加号菜单：右对齐触发器，空间不足时向上翻转
  useLayoutEffect(() => {
    if (!createOpen) return;
    const place = () => {
      const r = createTrigger.current!.getBoundingClientRect();
      const width = Math.min(184, window.innerWidth - 24);
      const below = window.innerHeight - r.bottom - 12;
      const above = r.top - 12;
      setCreatePosition({
        left: Math.max(12, Math.min(r.right - width, window.innerWidth - width - 12)),
        top: below >= 180 || below >= above ? r.bottom + 6 : Math.max(8, r.top - 100 - 6),
      });
    };
    place(); window.addEventListener('resize', place);
    return () => window.removeEventListener('resize', place);
  }, [createOpen]);

  useEffect(() => {
    if (!createOpen) return;
    const outside = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!createTrigger.current?.contains(target) && !createMenu.current?.contains(target)) setCreateOpen(false);
    };
    document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [createOpen]);

  const runItem = (index: number) => {
    const item = items[index];
    setCreateOpen(false);
    createTrigger.current?.focus();
    item.run();
    onClose();
  };

  const createKeyboard = (event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (!createOpen && ['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(event.key)) {
      event.preventDefault(); setCreateOpen(true); return;
    }
    if (!createOpen) return;
    if (event.key === 'Escape' || event.key === 'Tab') { setCreateOpen(false); return; }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter', ' '].includes(event.key)) {
      event.preventDefault();
      if (event.key === 'Home') { setCreateActive(0); return; }
      if (event.key === 'End') { setCreateActive(items.length - 1); return; }
      if (event.key === 'Enter' || event.key === ' ') { runItem(createActive); return; }
      setCreateActive(previous => (previous + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length);
    }
  };

  const rows = bots.filter(bot => bot.status !== 'archived').map(bot => ({ bot, entry: inbox[bot.id] }));
  const navigate = (action: () => void) => { action(); onClose(); };

  /** 菜单关闭后将焦点还给原来的头像行。 */
  const closeContextMenu = () => {
    setContextMenu(null);
    menuOwner.current?.focus();
  };
  const openContextMenu = (kind: 'bot' | 'group', id: string, owner: HTMLButtonElement, point: { clientX: number; clientY: number }) => {
    setCreateOpen(false);
    menuOwner.current = owner;
    setContextMenu({ kind, id, anchor: { x: point.clientX, y: point.clientY } });
  };
  /** 键盘打开：ContextMenu 键或 Shift+F10，锚点取群行自身位置 */
  const openContextMenuFromRow = (kind: 'bot' | 'group', id: string, event: React.KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return;
    event.preventDefault();
    const rect = event.currentTarget.getBoundingClientRect();
    openContextMenu(kind, id, event.currentTarget, { clientX: Math.round(rect.left + rect.width / 2), clientY: Math.round(rect.bottom - 8) });
  };
  // Resolve against current data so polling cannot leave deleted or stale targets actionable.
  const menuBot = contextMenu?.kind === 'bot' ? bots.find(bot => bot.id === contextMenu.id && bot.status !== 'archived') : undefined;
  const menuGroup = contextMenu?.kind === 'group' ? groups.find(group => group.id === contextMenu.id) : undefined;
  return <>
    {mobileOpen && <button className="sidebar-backdrop" aria-label="关闭导航遮罩" onClick={onClose} />}
    <aside ref={panel} className={'app-sidebar bot-inbox' + (collapsed ? ' is-collapsed' : '') + (mobileOpen ? ' is-open' : '')} aria-label="Bot 侧栏" role={mobileOpen ? 'dialog' : undefined} aria-modal={mobileOpen || undefined}>
      <div className="inbox-toolbar" data-tauri-drag-region>
        <button ref={closeButton} className="inbox-close icon-control" aria-label="关闭导航" onClick={onClose}><IconClose size={18} /></button>
        <button className="inbox-tool inbox-collapse" aria-label={collapsed ? '展开侧栏' : '收起侧栏'} title={collapsed ? '展开侧栏' : '收起侧栏'} aria-expanded={!collapsed} aria-controls={navId} onClick={() => { setCreateOpen(false); setContextMenu(null); onCollapsedChange(!collapsed); }}>
          {collapsed ? <IconSidebarExpand size={20} /> : <IconSidebarCollapse size={20} />}
        </button>
        <button className="inbox-tool inbox-expanded-tool" aria-label="搜索 Bot 和对话" title="搜索 Bot 和对话" aria-haspopup="dialog" onClick={() => navigate(onSearch)}><IconSearch size={20} /></button>
        <button ref={createTrigger} className="inbox-tool inbox-expanded-tool" aria-label="创建" title="创建" aria-haspopup="menu" aria-expanded={createOpen} aria-controls={createOpen ? menuId : undefined} aria-activedescendant={createOpen ? `${menuId}-${createActive}` : undefined} onClick={() => setCreateOpen(open => !open)} onKeyDown={createKeyboard}><IconPlus size={20} /></button>
      </div>
      {error && <div className="inbox-error" role="alert"><span>暂时无法刷新 Bot</span><button onClick={onRetry} title="暂时无法刷新 Bot，点击重试" aria-label="重试"><span>重试</span><IconRefresh className="inbox-compact-retry" size={16} /></button></div>}
      <nav id={navId} className="bot-inbox-list" aria-label="Bot 导航">
        {rows.map(({ bot, entry }) => {
          const state = activity[bot.id];
          const preview = state ? activityLabels[state] : bot.status === 'paused' ? '已暂停' : entry?.failed ? '对话暂时无法加载' : entry ? entry.preview || '还没有对话，打个招呼吧' : '正在加载对话…';
          return <div key={bot.id}>
            <button className={'bot-inbox-row' + (state ? ` is-${state}` : '')} title={`${bot.name} · ${preview}`} aria-label={bot.name + (state ? ` · ${activityLabels[state].replace('…', '')}` : '')} aria-current={!activeGroupId && activeBot.id === bot.id && !settingsOpen ? 'page' : undefined} onClick={() => navigate(() => onSelectBot(bot.id))}
              onContextMenu={event => { event.preventDefault(); openContextMenu('bot', bot.id, event.currentTarget, event); }}
              onKeyDown={event => openContextMenuFromRow('bot', bot.id, event)}>
              <BotAvatar avatar={bot.avatar} running={state === 'running'} />
              <span className="inbox-copy"><strong>{bot.name}</strong><small title={preview}>{preview}</small></span>
              {state && <span className="inbox-state-dot" aria-hidden="true" />}
            </button>
          </div>;
        })}
        {/* 群聊排在 Bot 项之后、同一个列表：共用 bot-inbox-row 与 inbox-copy，
            不再有独立的「群聊 N」标题与分割线。群列表为空时 map 出空数组，无内容可渲染。 */}
        {groups.map(group => {
          const preview = group.lastMessagePreview || '还没有对话';
          return <button key={group.id} className="bot-inbox-row"
            title={`${group.title} · ${preview}`}
            aria-label={`${group.title} · ${preview.slice(0, 30)}`}
            aria-current={activeGroupId === group.id && !settingsOpen ? 'page' : undefined}
            onClick={() => navigate(() => onSelectGroup(group.id))}
            onContextMenu={event => { event.preventDefault(); openContextMenu('group', group.id, event.currentTarget, event); }}
            onKeyDown={event => openContextMenuFromRow('group', group.id, event)}>
            <GroupAvatar members={group.members} />
            <span className="inbox-copy"><strong title={group.title}>{group.title}</strong><small title={preview}>{preview}</small></span>
          </button>;
        })}
        {!rows.length && <p className="inbox-empty">没有找到相关 Bot 或对话</p>}
      </nav>

      {groupError && <div className="inbox-error" role="alert"><span>暂时无法刷新群聊</span><button onClick={onRetryGroups} title="暂时无法刷新群聊，点击重试" aria-label="重试"><span>重试</span><IconRefresh className="inbox-compact-retry" size={16} /></button></div>}

      <button className="inbox-settings" aria-label="设置" title="设置" aria-current={settingsOpen ? 'page' : undefined} onClick={() => navigate(onOpenSettings)}><IconSettings size={20} /><span>设置</span></button>
    </aside>

    {createOpen && createPortal(<div ref={createMenu} id={menuId} role="menu" aria-label="创建" className="app-select-menu" style={{ ...createPosition, width: Math.min(184, window.innerWidth - 24) }} onMouseDown={event => event.preventDefault()}>
      {items.map(({ label, Icon }, index) => <div key={label} id={`${menuId}-${index}`} role="menuitem" className={index === createActive ? 'app-select-option is-active' : 'app-select-option'} onPointerMove={() => setCreateActive(index)} onClick={() => runItem(index)}>
        <span className="flex min-w-0 flex-1 items-center gap-2">
          <span className="w-4 shrink-0 text-ink-2"><Icon size={16} /></span>
          <span className="truncate">{label}</span>
        </span>
      </div>)}
    </div>, document.body)}

    {contextMenu && menuGroup && <SidebarContextMenu key={`group:${menuGroup.id}`} kind="group" pinned={menuGroup.pinned} anchor={contextMenu.anchor}
      deleteDisabled={groupSpeaking} deleteDisabledReason="讨论进行中，请结束后再删除" onClose={closeContextMenu}
      onTogglePin={() => { onPinGroup(menuGroup.id, !menuGroup.pinned); onClose(); }}
      onConfigure={() => { onConfigureGroup(menuGroup.id); onClose(); }}
      onRename={() => { onRenameGroup(menuGroup.id); onClose(); }}
      onDelete={() => { onDeleteGroup(menuGroup.id); onClose(); }} />}
    {contextMenu && menuBot && <SidebarContextMenu key={`bot:${menuBot.id}`} kind="bot" pinned={Boolean(menuBot.pinned)} anchor={contextMenu.anchor}
      deleteDisabled={menuBot.id === 'soren'} deleteDisabledReason="默认 Soren 需保持可用" onClose={closeContextMenu}
      onTogglePin={() => { onPinBot(menuBot.id, !menuBot.pinned); onClose(); }}
      onConfigure={() => { onConfigureBot(menuBot.id); onClose(); }}
      onRename={() => { onRenameBot(menuBot.id); onClose(); }}
      onDelete={() => { onDeleteBot(menuBot.id); onClose(); }} />}
  </>;
}
