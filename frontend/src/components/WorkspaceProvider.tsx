import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { DEFAULT_API_BASE_URL, setApiBaseUrl } from '../services/api';
import { setActiveBotID } from '../services/botScope';
import { companyAPI, type WorkspaceStatus } from '../services/company';
import { CompanyPanel } from './CompanyPanel';
import { WorkspaceWelcome } from './WorkspaceWelcome';
import { Button } from './ui';

type WorkspaceContextValue = {
  status: WorkspaceStatus;
  scope: string;
  reload: () => Promise<void>;
  apply: (status: WorkspaceStatus) => void;
  notify: (message: string) => void;
};
const WorkspaceContext = createContext<WorkspaceContextValue>({ status: { mode: 'personal' }, scope: 'personal', reload: async () => {}, apply: () => {}, notify: () => {} });
export const useWorkspace = () => useContext(WorkspaceContext);
const workspaceScope = (status: WorkspaceStatus) => status.mode === 'enterprise'
  ? `${status.connection?.serverUrl}:${status.connection?.workspaceId}:${status.connection?.userId}` : 'personal';

export function WorkspaceProvider({ children, welcome = false }: { children: ReactNode; welcome?: boolean }) {
  const [status, setStatus] = useState<WorkspaceStatus | null>(null);
  const [error, setError] = useState('');
  const [notice, notify] = useState('');
  const generation = useRef(0);
  const mounted = useRef(false);
  const previousStatus = useRef<WorkspaceStatus | null>(null);
  const apply = useCallback((next: WorkspaceStatus) => {
    if (!mounted.current) return;
    generation.current++;
    setApiBaseUrl(DEFAULT_API_BASE_URL + (next.mode === 'enterprise' ? '/enterprise' : ''), next.mode === 'enterprise' ? workspaceScope(next) : '', true);
    const previous = previousStatus.current;
    if (!previous || previous.needsLogin || previous.error || workspaceScope(previous) !== workspaceScope(next)) setActiveBotID('soren');
    previousStatus.current = next;
    setStatus(next); setError('');
  }, []);
  const reload = useCallback(async () => {
    const id = ++generation.current;
    const next = await companyAPI.status();
    if (mounted.current && id === generation.current) apply(next);
  }, [apply]);
  useEffect(() => {
    mounted.current = true;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    // The native sidecar starts on a background thread. Keep the workspace gate
    // closed while it starts, including when the webview is initially hidden.
    const deadline = Date.now() + 12000;
    const restore = async () => {
      try { await reload(); }
      catch (e) {
        if (!active) return;
        if (Date.now() < deadline) timer = setTimeout(restore, 1000);
        else setError(e instanceof Error ? e.message : String(e));
      }
    };
    void restore();
    return () => { active = false; clearTimeout(timer); mounted.current = false; generation.current++; };
  }, [reload]);
  useEffect(() => {
    const expired = () => {
      generation.current++;
      if (previousStatus.current?.mode === 'enterprise') {
        previousStatus.current = { ...previousStatus.current, needsLogin: true };
        setStatus(previousStatus.current);
      }
    };
    window.addEventListener('soren:company-session-expired', expired);
    return () => window.removeEventListener('soren:company-session-expired', expired);
  }, []);
  if (!status) return <main className="flex min-h-screen items-center justify-center bg-surface-1 p-8 text-ink-1"><div className="max-w-md text-center"><img src="/soren-mark.svg" alt="Soren" className="mx-auto mb-6 h-14 w-14" /><p role="status">{error || '正在恢复使用空间…'}</p>{error && <Button className="mt-5" onClick={() => void reload().catch(e => setError(String(e)))}>重新连接</Button>}</div></main>;
  const scope = workspaceScope(status);
  const content = status.needsLogin || status.error
    ? <main className="min-h-screen bg-surface-1 p-6 pt-20 text-ink-1"><div className="mx-auto max-w-2xl"><CompanyPanel /></div></main>
    : <div key={scope} className="contents">{children}</div>;
  return <WorkspaceContext.Provider value={{ status, scope, reload, apply, notify }}>
    {notice && <div role="status" className="fixed bottom-6 left-1/2 z-[100] w-[min(90vw,36rem)] -translate-x-1/2 rounded-xl border border-line bg-surface-2 p-4 text-sm text-ink-1 shadow-xl"><p>{notice}</p><Button className="mt-2" variant="ghost" size="sm" onClick={() => notify('')}>知道了</Button></div>}
    {welcome ? <WorkspaceWelcome>{content}</WorkspaceWelcome> : content}
  </WorkspaceContext.Provider>;
}
