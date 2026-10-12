import { avatarURL, type Bot } from '../../services/agents';
import { useMemo, useState, useEffect, useRef } from 'react';
import { IconBrain, IconChat, IconPlus, IconPlug, IconSearch, IconSettings, IconTrash, IconClose } from '../icons';
import { Logo } from '../Logo';
import { Select } from '../ui/Select';
import { ConfirmDialog } from '../ui';
import type { Conversation, ViewId } from '../../types';
export interface SidebarProps {
 bots: Bot[]; activeBot: Bot; onSelectBot: (id: string) => void;
  view: ViewId; onNavigate: (view: ViewId) => void; conversations: Conversation[];
  currentConversationId: string | null; onSelectConversation: (id: string) => void;
  onDeleteConversation: (id: string) => Promise<void>; onNewConversation: () => void;
  memoryCount?: number; activeChannelCount?: number; loading?: boolean; className?: string;
  mobileOpen?: boolean; onClose?: () => void;
}
export function Sidebar(props: SidebarProps) {
  const { view, onNavigate, conversations, currentConversationId, onSelectConversation, onNewConversation, memoryCount, activeChannelCount, mobileOpen, onClose } = props;
  const panel = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!mobileOpen) return;
    const previous = document.activeElement as HTMLElement | null;
    const main = document.querySelector<HTMLElement>('.app-main');
    if (main) main.inert = true;
    panel.current?.querySelector<HTMLButtonElement>('.sidebar-close')?.focus();
    const trap = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return;
      const elements = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not([disabled]),input:not([disabled]),select:not([disabled])') ?? []).filter(element => element.getClientRects().length > 0);
      const first = elements[0]; const last = elements[elements.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    };
    document.addEventListener('keydown', trap);
    return () => { if (main) main.inert = false; document.removeEventListener('keydown', trap); previous?.focus(); };
  }, [mobileOpen]);
  const [search, setSearch] = useState('');
  const [pendingDelete, setPendingDelete] = useState<Conversation | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const filtered = useMemo(() => conversations.filter(c => c.title.toLocaleLowerCase().includes(search.toLocaleLowerCase())), [conversations, search]);
  const navigate = (id: ViewId) => { onNavigate(id); onClose?.(); };
  const items = [
    { id: 'bots' as const, label: '我的 Bot', Icon: IconBrain, count: undefined },
    { id: 'tasks' as const, label: '任务', Icon: IconChat, count: undefined },
    { id: 'shared' as const, label: '共享资料', Icon: IconPlug, count: undefined },
    { id: 'chat' as const, label: '对话', Icon: IconChat, count: undefined },
    { id: 'memories' as const, label: '我的记忆', Icon: IconBrain, count: memoryCount },
    { id: 'channels' as const, label: '应用连接', Icon: IconPlug, count: activeChannelCount },
  ];
  return <>
    {mobileOpen && <button className="sidebar-backdrop" aria-label="关闭导航遮罩" onClick={onClose} />}
    <aside ref={panel} role={mobileOpen ? 'dialog' : undefined} aria-modal={mobileOpen ? true : undefined} aria-label={mobileOpen ? '导航面板' : undefined} className={'app-sidebar ' + (mobileOpen ? 'is-open' : '')}>
      <div className="sidebar-brand"><button aria-label="返回对话" onClick={() => navigate('chat')}><Logo size={39} /></button><button className="sidebar-close icon-control" aria-label="关闭导航" onClick={onClose}><IconClose size={19} /></button></div>
      <div className="bot-switcher"><img src={avatarURL(props.activeBot.avatar)} alt="" /><Select aria-label="切换 Bot" value={props.activeBot.id} onChange={id => { props.onSelectBot(id); onClose?.(); }} options={props.bots.filter(b => b.status !== 'archived').map(b => ({value: b.id, label: b.name + (b.status === 'paused' ? ' · 暂停' : '')}))} /><button aria-label="管理 Bot" onClick={() => navigate('bots')}><IconSettings size={15} /></button></div>
      <button className="new-chat-button" onClick={() => { onNewConversation(); onClose?.(); }}><IconPlus size={18} /><span>新建对话</span><kbd>⌘ N</kbd></button>
      <div className="sidebar-nav-spacer" />
      <nav className="primary-nav" aria-label="主导航">{items.map(({ id, label, Icon, count }) => <button key={id} onClick={() => navigate(id)} aria-current={view === id ? 'page' : undefined}><Icon size={19} /><span>{label}</span>{typeof count === 'number' && count > 0 ? <span className="nav-count">{count}</span> : view === id ? <span className="nav-active-dot" /> : null}</button>)}</nav>
      <div className="history-heading"><span>最近对话</span><IconChat size={13} /></div>
      {conversations.length > 0 && <label className="history-search"><IconSearch size={14} /><input aria-label="搜索对话" placeholder="搜索对话" value={search} onChange={e => setSearch(e.target.value)} /></label>}
      <div className="conversation-history">{filtered.map(c => <div className={'history-item ' + (currentConversationId === c.id ? 'selected' : '')} key={c.id}><button onClick={() => { onSelectConversation(c.id); onClose?.(); }} title={c.title}><IconChat size={14} /><span>{c.title || '新的对话'}</span></button><button className="history-delete" aria-label={'删除对话 ' + c.title} onClick={() => { setDeleteError(''); setPendingDelete(c); }}><IconTrash size={13} /></button></div>)}</div>
      <button className={'sidebar-profile ' + (view === 'settings' ? 'selected' : '')} onClick={() => navigate('settings')} aria-label="设置"><span className="profile-avatar"><img src="/app-icon.svg" alt="" width={30} height={30} /></span><span><strong>设置</strong></span><IconSettings size={18} /></button>
    </aside>
    <ConfirmDialog open={pendingDelete !== null} danger loading={deleting} title="删除这个对话？" message={<>{'「' + (pendingDelete?.title || '新的对话') + '」及其消息将被删除。'}{deleteError && <p role="alert">{deleteError}</p>}</>} confirmText="删除" onConfirm={async () => { if (!pendingDelete) return; setDeleting(true); try { await props.onDeleteConversation(pendingDelete.id); setPendingDelete(null); } catch { setDeleteError('删除失败，请重试。'); } finally { setDeleting(false); } }} onCancel={() => setPendingDelete(null)} />
  </>;
}
