import { IconBrain, IconChat, IconPlug, IconSettings, IconClock, IconDatabase } from '../icons';
import type { ViewId } from '../../types';

const sections = [
  { id: 'settings', label: '通用设置', Icon: IconSettings },
  { id: 'bots', label: '我的 Bot', Icon: IconBrain },
  { id: 'history', label: '对话记录', Icon: IconChat },
  { id: 'memories', label: '我的记忆', Icon: IconClock },
  { id: 'shared', label: '共享资料', Icon: IconDatabase },
  { id: 'channels', label: '应用连接', Icon: IconPlug },
] as const;

export function SettingsNav({ view, onNavigate, memoryCount }: { view: ViewId; onNavigate: (view: ViewId) => void; memoryCount?: number }) {
  return <nav className="settings-nav" aria-label="设置功能">
    {sections.map(({ id, label, Icon }) => <button key={id} aria-current={view === id ? 'page' : undefined} onClick={() => onNavigate(id)}>
      <Icon size={16} /><span>{label}</span>{id === 'memories' && Boolean(memoryCount) && <small>{memoryCount}</small>}
    </button>)}
  </nav>;
}
