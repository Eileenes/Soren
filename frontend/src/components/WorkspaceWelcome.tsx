import { useState, type ReactNode } from 'react';
import { CompanyPanel } from './CompanyPanel';
import { Modal } from './ui';

const STORAGE_KEY = 'soren.workspace-confirmed.v2';
function hasSelected() {
  try { return localStorage.getItem(STORAGE_KEY) === 'true'; }
  catch { return false; }
}
function rememberSelection() {
  try {
    localStorage.setItem(STORAGE_KEY, 'true');
    // Keep the optional personal guide available from the title bar.
    localStorage.setItem('soren.getting-started.v1', JSON.stringify({ seen: true, active: false, step: 0 }));
  } catch { /* The current session remains usable when storage is unavailable. */ }
}

export function WorkspaceWelcome({ children }: { children: ReactNode }) {
  const [selected, setSelected] = useState(hasSelected);
  if (selected) return <>{children}</>;
  return <main className="min-h-screen bg-surface-1 text-ink-1">
    <Modal open dismissible={false} onClose={() => {}} title="欢迎使用 Soren" description="选择你的使用空间，开始与 Bot 协作。" size="lg">
      <CompanyPanel onSelected={() => { rememberSelection(); setSelected(true); }} />
    </Modal>
  </main>;
}
