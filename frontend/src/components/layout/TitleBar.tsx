import { IconHelp, IconList, IconChevronRight } from '../icons';

export interface TitleBarProps {
  title: string;
  settingsOpen: boolean;
  onOpenGuide: () => void;
  onOpenMenu: () => void;
}

export function TitleBar({ title, settingsOpen, onOpenGuide, onOpenMenu }: TitleBarProps) {
  return <header className="app-titlebar" data-tauri-drag-region>
    <div className="titlebar-breadcrumb" data-tauri-drag-region>
      <button className="mobile-menu icon-control" aria-label="打开导航" onClick={onOpenMenu}><IconList size={20} /></button>
      {settingsOpen && <><span className="breadcrumb-home" data-tauri-drag-region>设置</span><IconChevronRight size={12} /></>}
      <span className="titlebar-current" data-tauri-drag-region>{title}</span>
    </div>
    <div className="titlebar-actions">
      <button className="icon-control" aria-label="使用引导" title="使用引导" onClick={onOpenGuide}><IconHelp size={20} /></button>
    </div>
  </header>;
}
