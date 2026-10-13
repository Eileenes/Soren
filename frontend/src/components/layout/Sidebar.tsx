import { useEffect, useRef } from 'react';
import type { Bot } from '../../services/agents';
import { BotAvatar } from '../BotAvatar';
import type { BotActivity } from '../../hooks/useBotRuns';
import type { BotInbox } from '../../hooks/useBotInbox';
import { IconSettings, IconSearch, IconPlus, IconClose } from '../icons';

export interface SidebarProps {
  bots: Bot[]; activeBot: Bot; onSelectBot: (id: string) => void;
  onSearch: () => void;
  onCreateBot: () => void; settingsOpen: boolean; onOpenSettings: () => void;
  activity: Record<string, BotActivity>; inbox: Record<string, BotInbox>;
  error: string | null; onRetry: () => void; mobileOpen: boolean; onClose: () => void;
}
const activityLabels: Record<BotActivity, string> = {
  running: '正在执行任务…', queued: '任务排队中', waiting_approval: '任务等待确认', paused: '任务已暂停',
};

export function Sidebar({ bots, activeBot, onSelectBot, onSearch, onCreateBot, settingsOpen, onOpenSettings, activity, inbox, error, onRetry, mobileOpen, onClose }: SidebarProps) {
  const panel = useRef<HTMLElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
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
  const rows = bots.filter(bot => bot.status !== 'archived').map(bot => ({ bot, entry: inbox[bot.id] }));
  const navigate = (action: () => void) => { action(); onClose(); };
  return <>
    {mobileOpen && <button className="sidebar-backdrop" aria-label="关闭导航遮罩" onClick={onClose} />}
    <aside ref={panel} className={'app-sidebar bot-inbox' + (mobileOpen ? ' is-open' : '')} aria-label="Bot 侧栏" role={mobileOpen ? 'dialog' : undefined} aria-modal={mobileOpen || undefined}>
      <div className="inbox-toolbar" data-tauri-drag-region>
        <button ref={closeButton} className="inbox-close icon-control" aria-label="关闭导航" onClick={onClose}><IconClose size={18} /></button>
        <button className="inbox-tool" aria-label="搜索 Bot 和对话" title="搜索 Bot 和对话" aria-haspopup="dialog" onClick={() => navigate(onSearch)}><IconSearch size={18} /></button>
        <button className="inbox-tool" aria-label="创建 Bot" title="创建 Bot" onClick={() => navigate(onCreateBot)}><IconPlus size={20} /></button>
      </div>
      <div className="inbox-heading"><span>Bot</span><span>{bots.filter(bot => bot.status !== 'archived').length}</span></div>
      {error && <div className="inbox-error" role="alert"><span>暂时无法刷新 Bot</span><button onClick={onRetry}>重试</button></div>}
      <nav className="bot-inbox-list" aria-label="Bot 导航">
        {rows.map(({ bot, entry }) => {
          const state = activity[bot.id];
          const preview = state ? activityLabels[state] : bot.status === 'paused' ? '已暂停' : entry?.failed ? '对话暂时无法加载' : entry ? entry.preview || '还没有对话，打个招呼吧' : '正在加载对话…';
          return <div key={bot.id}>
            <button className={'bot-inbox-row' + (state ? ` is-${state}` : '')} aria-label={bot.name + (state ? ` · ${activityLabels[state].replace('…', '')}` : '')} aria-current={activeBot.id === bot.id && !settingsOpen ? 'page' : undefined} onClick={() => navigate(() => onSelectBot(bot.id))}>
              <BotAvatar avatar={bot.avatar} running={state === 'running'} />
              <span className="inbox-copy"><strong>{bot.name}</strong><small title={preview}>{preview}</small></span>
              {state && <span className="inbox-state-dot" aria-hidden="true" />}
            </button>
          </div>;
        })}
        {!rows.length && <p className="inbox-empty">没有找到相关 Bot 或对话</p>}
      </nav>
      <button className="inbox-settings" aria-label="设置" title="设置" aria-current={settingsOpen ? 'page' : undefined} onClick={() => navigate(onOpenSettings)}><IconSettings size={20} /><span>设置</span></button>
    </aside>
  </>;
}
