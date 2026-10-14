import { useId, useMemo, useRef, useState, type ReactNode } from 'react';
import { Modal } from './ui';
import { GroupAvatar } from './GroupAvatar';
import { IconSearch, IconClose, IconSettings, IconChat, IconPlug, IconBrain, IconDatabase, IconClock } from './icons';
import { avatarURL, type Bot } from '../services/agents';
import type { BotInbox } from '../hooks/useBotInbox';
import type { GroupView, ViewId } from '../types';

const destinations = [
  { view: 'settings', name: '设置：通用', description: '模型、外观、移动端与同步', Icon: IconSettings },
  { view: 'bots', name: '管理 Bot', description: '创建与配置你的助手', Icon: IconBrain },
  { view: 'history', name: '对话记录', description: '按 Bot 查看聊天记录', Icon: IconChat },
  { view: 'channels', name: '应用连接', description: '连接应用与配置权限', Icon: IconPlug },
  { view: 'memories', name: '我的记忆', description: '偏好与长期记忆', Icon: IconClock },
  { view: 'shared', name: '共享资料', description: '供 Bot 使用的知识与资料', Icon: IconDatabase },
] satisfies { view: ViewId; name: string; description: string; Icon: typeof IconSettings }[];

export function SearchPalette({ bots, inbox, groups, onClose, onSelect, onSelectGroup, onNavigate }: {
  bots: Bot[]; inbox: Record<string, BotInbox>; groups: GroupView[]; onClose: () => void;
  onSelect: (botId: string, conversationId?: string) => void;
  onSelectGroup: (groupId: string) => void;
  onNavigate: (view: ViewId) => void;
}) {
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const list = useRef<HTMLDivElement>(null);
  const id = useId();
  const results = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    const rows: { id: string; title: string; description: string; icon: ReactNode; select: () => void }[] = [];
    for (const bot of bots.filter(b => b.status !== 'archived')) {
      const entry = inbox[bot.id];
      if (!needle || `${bot.name} ${entry?.preview ?? ''}`.toLocaleLowerCase().includes(needle)) {
        rows.push({ id: bot.id, title: bot.name, description: entry?.preview || '开始与这个 Bot 对话', icon: <img src={avatarURL(bot.avatar)} alt="" />, select: () => onSelect(bot.id) });
      }
      if (needle) for (const conversation of entry?.conversations ?? []) {
        if (conversation.title.toLocaleLowerCase().includes(needle)) rows.push({ id: `${bot.id}/${conversation.id}`, title: conversation.title, description: `${bot.name} · 对话记录`, icon: <IconChat size={18} />, select: () => onSelect(bot.id, conversation.id) });
      }
    }
    // 群聊与 Bot 同级：群名或最近一条预览命中即列出
    for (const group of groups) {
      if (needle && !`${group.title} ${group.topic} ${group.lastMessagePreview ?? ''}`.toLocaleLowerCase().includes(needle)) continue;
      rows.push({
        id: `group/${group.id}`,
        title: group.title,
        description: group.lastMessagePreview || '群聊',
        icon: <GroupAvatar members={group.members} />,
        select: () => onSelectGroup(group.id),
      });
    }
    for (const destination of destinations) {
      if (!needle || `${destination.name} ${destination.description}`.toLocaleLowerCase().includes(needle)) rows.push({ id: destination.view, title: destination.name, description: destination.description, icon: <destination.Icon size={18} />, select: () => onNavigate(destination.view) });
    }
    return rows;
  }, [bots, inbox, groups, query, onSelect, onSelectGroup, onNavigate]);
  const selected = Math.min(active, Math.max(0, results.length - 1));
  const choose = (index: number) => { if (results[index]) { onClose(); results[index].select(); } };
  return <Modal open onClose={onClose} title="搜索 Bot、对话和设置" compact size="lg">
    <div className="palette-input"><IconSearch size={19} /><input role="combobox" aria-label="搜索 Bot 和对话" aria-expanded="true" aria-autocomplete="list" aria-controls={`${id}-results`} aria-activedescendant={results.length ? `${id}-${selected}` : undefined} placeholder="搜索 Bot、对话或设置…" value={query} onChange={e => { setQuery(e.target.value); setActive(0); }} onKeyDown={e => {
      if (e.key === 'Enter') { e.preventDefault(); choose(selected); }
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
        e.preventDefault();
        if (!results.length) return;
        const next = (selected + (e.key === 'ArrowDown' ? 1 : -1) + results.length) % results.length;
        setActive(next); list.current?.children[next]?.scrollIntoView?.({ block: 'nearest' });
      }
    }} /><button className="icon-control" aria-label="关闭搜索" onClick={onClose}><IconClose size={17} /></button></div>
    <div className="palette-results" ref={list} role="listbox" id={`${id}-results`} aria-label="搜索结果">
      {results.map((row, index) => <div key={row.id} id={`${id}-${index}`} role="option" aria-selected={index === selected} className="palette-result" onMouseDown={e => e.preventDefault()} onClick={() => choose(index)} onMouseMove={() => setActive(index)}>
        <span className="palette-symbol">{row.icon}</span><span className="palette-copy"><strong>{row.title}</strong><small>{row.description}</small></span>
      </div>)}
    </div>
    {!results.length && <p role="status" className="palette-empty">没有找到结果，试试 Bot 名称或「设置」。</p>}
  </Modal>;
}
