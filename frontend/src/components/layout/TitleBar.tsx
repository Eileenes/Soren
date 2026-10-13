import { IconHelp, IconList, IconChevronRight, IconWifi, IconWifiOff, IconRefresh } from '../icons';
import type { ConnectionState } from '../../types';

export interface TitleBarProps {
  title: string;
  connection: ConnectionState;
  settingsOpen: boolean;
  onOpenGuide: () => void;
  onOpenMenu: () => void;
}

export function TitleBar({ title, connection, settingsOpen, onOpenGuide, onOpenMenu }: TitleBarProps) {
  const label = { online: '服务已连接', connecting: '正在连接', offline: '服务未连接', unknown: '正在准备' }[connection];
  return <header className="app-titlebar" data-tauri-drag-region>
    <div className="titlebar-breadcrumb" data-tauri-drag-region>
      <button className="mobile-menu icon-control" aria-label="打开导航" onClick={onOpenMenu}><IconList size={20} /></button>
      {settingsOpen && <><span className="breadcrumb-home" data-tauri-drag-region>设置</span><IconChevronRight size={12} /></>}
      <span className="titlebar-current" data-tauri-drag-region>{title}</span>
    </div>
    <div className="titlebar-actions">
      <span role="img" aria-label={label} title={label} className={'connection-icon ' + connection}>{connection === 'online' ? <IconWifi size={17} /> : connection === 'offline' ? <IconWifiOff size={17} /> : <IconRefresh size={16} />}</span>
      <button className="icon-control" aria-label="使用引导" title="使用引导" onClick={onOpenGuide}><IconHelp size={20} /></button>
    </div>
  </header>;
}
