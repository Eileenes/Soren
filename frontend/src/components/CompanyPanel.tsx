import { useCallback, useEffect, useRef, useState } from 'react';
import { companyAPI, CompanyRequestError, openCompanyLogin, type CompanyConnection, type CompanyDiscovery, type CompanyLogin } from '../services/company';
import { useWorkspace } from './WorkspaceProvider';
import { Button, TextField } from './ui';
import { IconCheck, IconDatabase, IconPlug } from './icons';

const message = (e: unknown) => e instanceof Error ? e.message : String(e);
export function CompanyPanel() {
  const { status, reload, apply, notify } = useWorkspace();
  const company = status.mode === 'enterprise';
  const [url, setURL] = useState(status.connection?.serverUrl ?? '');
  const [discovery, setDiscovery] = useState<CompanyDiscovery | null>(null);
  const [login, setLogin] = useState<CompanyLogin | null>(null);
  const [account, setAccount] = useState<CompanyConnection | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const generation = useRef(0);
  const pendingID = useRef<string | null>(null);
  const completing = useRef(false);
  const clearLogin = useCallback((id: string, reason: string) => {
    if (pendingID.current !== id) return;
    generation.current++;
    pendingID.current = null;
    setLogin(null); setAccount(null); setBusy(false); setError(reason);
    void companyAPI.cancel(id).catch(() => {});
  }, []);
  useEffect(() => () => { generation.current++; if (pendingID.current) void companyAPI.cancel(pendingID.current).catch(() => {}); }, []);
  useEffect(() => {
    if (!login) return;
    // Account confirmation has the same deadline as the browser login attempt.
    const timer = setTimeout(() => {
      if (!completing.current) clearLogin(login.id, '登录已过期，请重新登录。');
    }, Math.max(0, Date.parse(login.expiresAt) - Date.now()));
    return () => clearTimeout(timer);
  }, [login, clearLogin]);
  useEffect(() => {
    if (!login || account) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      if (Date.now() >= Date.parse(login.expiresAt)) { clearLogin(login.id, '登录已过期，请重新登录。'); return; }
      try {
        const result = await companyAPI.poll(login.id);
        if (!active || pendingID.current !== login.id) return;
        setError('');
        if (result.status === 'authorized') { setAccount(result.connection); return; }
        timer = setTimeout(poll, 2000);
      } catch (e) {
        if (!active || pendingID.current !== login.id) return;
        if (e instanceof CompanyRequestError && (e.status === 401 || e.status === 403)) {
          clearLogin(login.id, message(e));
        } else {
          setError(message(e));
          timer = setTimeout(poll, 2000);
        }
      }
    };
    timer = setTimeout(poll, 1500);
    return () => { active = false; clearTimeout(timer); };
  }, [login, account, clearLogin]);
  const verify = async () => {
    const id = ++generation.current;
    setBusy(true); setError(''); setDiscovery(null);
    try { const found = await companyAPI.discover(url.trim()); if (id === generation.current) setDiscovery(found); }
    catch (e) { if (id === generation.current) setError(message(e)); }
    finally { if (id === generation.current) setBusy(false); }
  };
  const signIn = async () => {
    if (!discovery) return;
    const id = ++generation.current;
    setBusy(true); setError('');
    try {
      const next = await companyAPI.login(discovery.serverUrl);
      if (id !== generation.current) { await companyAPI.cancel(next.id); return; }
      pendingID.current = next.id; setLogin(next); setAccount(null); await openCompanyLogin(next);
    } catch (e) { if (id === generation.current) setError(message(e)); }
    finally { if (id === generation.current) setBusy(false); }
  };
  const complete = async () => {
    if (!login) return;
    if (Date.now() >= Date.parse(login.expiresAt)) { clearLogin(login.id, '登录已过期，请重新登录。'); return; }
    const id = ++generation.current;
    completing.current = true;
    setBusy(true); setError('');
    try {
      const next = await companyAPI.complete(login.id);
      if (id !== generation.current) return;
      pendingID.current = null; apply(next);
    } catch (e) {
      if (id !== generation.current) return;
      if (e instanceof CompanyRequestError && e.status === 401) clearLogin(login.id, message(e));
      else setError(message(e));
      await reload().catch(() => {});
    } finally {
      completing.current = false;
      if (Date.now() >= Date.parse(login.expiresAt)) clearLogin(login.id, '登录已过期，请重新登录。');
      setBusy(false);
    }
  };
  const disconnect = async () => {
    setBusy(true); setError('');
    try {
      const result = await companyAPI.disconnect();
      if (!result.revoked || !result.credentialDeleted) {
        notify('已切回个人空间。部分退出清理未完成：若服务器离线，远端会话将在到期后失效；系统凭据删除失败时请重新登录后再次退出。');
      }
      apply({ mode: 'personal', connection: status.connection });
    } catch (e) { setError(message(e)); await reload().catch(() => {}); } finally { setBusy(false); }
  };
  const cancel = async () => {
    if (!login) return;
    setBusy(true); setError('');
    try { await companyAPI.cancel(login.id); pendingID.current = null; setLogin(null); setAccount(null); }
    catch (e) { setError(message(e)); } finally { setBusy(false); }
  };
  return <section aria-label="使用空间" className="space-y-5">
    <div className="grid gap-3 sm:grid-cols-2">
      <div className={'rounded-xl border p-4 ' + (!company ? 'border-primary-start bg-primary-start/5' : 'border-line')}>
        <div className="flex items-center gap-2 text-sm font-semibold"><IconDatabase size={17} />个人空间{!company && <IconCheck size={15} className="ml-auto" />}</div>
        <p className="mt-2 text-sm leading-6 text-ink-2">无需登录，Bot 和数据保存在自己的电脑上。</p>
        {company && <Button className="mt-4" disabled={busy || Boolean(login)} onClick={() => void disconnect()}>退出公司，使用个人空间</Button>}
      </div>
      <div className={'rounded-xl border p-4 ' + (company ? 'border-primary-start bg-primary-start/5' : 'border-line')}>
        <div className="flex items-center gap-2 text-sm font-semibold"><IconPlug size={17} />公司空间{company && <IconCheck size={15} className="ml-auto" />}</div>
        <p className="mt-2 text-sm leading-6 text-ink-2">连接公司服务器，登录后使用自己的 Bot。</p>
        {company && <p className="mt-3 break-words text-xs text-ink-2">{status.connection?.organization} · {status.connection?.userName}</p>}
      </div>
    </div>
    {status.needsLogin && <p role="alert" className="bot-error">公司登录已失效，请重新登录。</p>}
    {status.error && <div><p role="alert" className="bot-error">{status.error}</p><Button className="mt-3" onClick={() => void reload().catch(e => setError(message(e)))}>重新连接公司</Button></div>}
    {company && !status.needsLogin && !status.error && <p className="break-all text-xs text-ink-3">已连接 {status.connection?.serverUrl}。公司资料保存在公司服务器，个人资料保留在本机。</p>}
    {(!company || status.needsLogin || status.error) && <div className="space-y-4 border-t border-line pt-5">
      <TextField label="公司服务器地址" placeholder="https://soren.example.com" value={url} disabled={busy || Boolean(login)} onChange={e => { generation.current++; setURL(e.target.value); setDiscovery(null); setError(''); }} helper="向管理员获取地址。OAuth 应用配置由管理员统一管理。" />
      {!discovery && <Button loading={busy} disabled={!url.trim()} onClick={() => void verify()}>识别服务器</Button>}
      {discovery && <div className="rounded-xl bg-surface-3 p-4"><p className="text-sm font-semibold">{discovery.name}</p><p className="mt-1 break-all text-xs text-ink-3">{discovery.serverUrl}</p>
        {login ? <div className="mt-4 space-y-3"><p role="status" className="text-sm text-ink-2">{account ? `已登录：${account.userName}。确认后进入你的公司空间。` : '请在浏览器完成公司登录，再返回此处确认账号。'}</p><div className="flex flex-wrap gap-2">{account ? <Button variant="primary" loading={busy} onClick={() => void complete()}>进入公司空间</Button> : <Button disabled={busy} onClick={() => void openCompanyLogin(login).catch(e => setError(message(e)))}>打开登录页面</Button>}<Button variant="ghost" disabled={busy} onClick={() => void cancel()}>取消登录</Button></div></div>
          : <Button className="mt-4" variant="primary" loading={busy} onClick={() => void signIn()}>使用公司账号登录</Button>}
      </div>}
      <p className="text-xs leading-6 text-ink-3">连接公司时不会自动上传本地 Bot、聊天或记忆。登录后的连接器使用你单独授予的权限。</p>
    </div>}
    {error && <p role="alert" className="bot-error">{error}</p>}
  </section>;
}
