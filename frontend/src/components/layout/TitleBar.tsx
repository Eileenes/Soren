import type { ReactNode } from 'react';
import { IconMoon, IconSun, IconList, IconChevronRight, IconWifi, IconWifiOff, IconRefresh } from '../icons';
import type { ConnectionState } from '../../types';
export interface TitleBarProps { title: string; subtitle?: string; connection: ConnectionState; theme: 'dark' | 'light'; onToggleTheme: () => void; actions?: ReactNode; onOpenMenu?: () => void; }
export function TitleBar({ title, connection, theme, onToggleTheme, actions, onOpenMenu }: TitleBarProps) {
  const label = { online: '服务已连接', connecting: '正在连接', offline: '服务未连接', unknown: '正在准备' }[connection];
  return <header className="app-titlebar" data-tauri-drag-region>
    <div className="titlebar-breadcrumb" data-tauri-drag-region><button className="mobile-menu icon-control" aria-label="打开导航" onClick={onOpenMenu}><IconList size={21} /></button><span className="breadcrumb-home" data-tauri-drag-region>个人空间</span><IconChevronRight size={12} /><span data-tauri-drag-region>{title}</span></div>
    <div className="titlebar-actions">{actions}<span role="img" aria-label={label} title={label} className={'connection-icon ' + connection}>{connection === 'online' ? <IconWifi size={17} /> : connection === 'offline' ? <IconWifiOff size={17} /> : <IconRefresh size={16} />}</span><span className="titlebar-divider" /><button className="icon-control theme-control" aria-label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'} title={theme === 'dark' ? '浅色模式' : '深色模式'} onClick={onToggleTheme}>{theme === 'dark' ? <IconSun size={18} /> : <IconMoon size={18} />}</button></div>
  </header>;
}
