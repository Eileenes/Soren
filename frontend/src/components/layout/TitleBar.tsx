import { IconHelp, IconList, IconChevronRight } from '../icons';

export interface TitleBarProps {
  title: string;
  settingsOpen: boolean;
  onOpenGuide: () => void;
  onOpenMenu: () => void;
  workspaceLabel?: string;
  onOpenWorkspace?: () => void;
}

export function TitleBar({ title, settingsOpen, onOpenGuide, onOpenMenu, workspaceLabel, onOpenWorkspace }: TitleBarProps) {
  return <header className="app-titlebar" data-tauri-drag-region>
    <div className="titlebar-breadcrumb" data-tauri-drag-region>
      <button className="mobile-menu icon-control" aria-label="打开导航" onClick={onOpenMenu}><IconList size={20} /></button>
      {settingsOpen && <><span className="breadcrumb-home" data-tauri-drag-region>设置</span><IconChevronRight size={12} /></>}
      <span className="titlebar-current" data-tauri-drag-region>{title}</span>
    </div>
    <div className="titlebar-actions">
      {onOpenWorkspace && <button className="max-w-56 truncate rounded-lg px-3 py-2 text-xs text-ink-2 transition-colors hover:bg-surface-3" title={workspaceLabel} aria-label={`切换使用空间，当前${workspaceLabel}`} onClick={onOpenWorkspace}>{workspaceLabel}</button>}
      <button className="icon-control" aria-label="使用引导" title="使用引导" onClick={onOpenGuide}><IconHelp size={20} /></button>
    </div>
  </header>;
}
