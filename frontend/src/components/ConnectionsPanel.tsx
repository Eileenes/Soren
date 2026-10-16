import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { ConnectionAppCard } from './ConnectionAppCard';
import { useWorkspace } from './WorkspaceProvider';
import { ConnectionAppIcon } from './ConnectionAppIcon';
import { connectionRequest as request, openExternal, type Connection } from '../services/connections';
import { describeError } from '../services/api';
import { Button, Modal, PasswordField, TextField } from './ui';
import { IconDatabase, IconInbox, IconPlug } from './icons';
import { connectionKind, isKeyKind, keyCredentialLabel, keyKinds, mcpPresetEndpoint, type KeyKindId } from './connectorKinds';

type Kind = 'gmail' | 'notion' | 'mcp' | KeyKindId;
const cards = [
  ['gmail', 'Gmail', '写草稿 · 确认发送', IconInbox], ['notion', 'Notion', '查找资料 · 创建页面', IconDatabase], ['mcp', 'MCP', '连接远程工具', IconPlug],
  ...Object.entries(keyKinds).map(([id, k]) => [id, k.title, k.caption, k.icon] as const),
] as const;
const kindTitle = (kind: Kind | null) => kind === 'gmail' ? 'Gmail' : kind === 'notion' ? 'Notion' : isKeyKind(kind) ? keyKinds[kind].title : 'MCP';

export function ConnectionsPanel({ children }: { children?: ReactNode }) {
  const company = useWorkspace().status.mode === 'enterprise';
  const [connections, setConnections] = useState<Connection[]>([]);
  const [kind, setKind] = useState<Kind | null>(null);
  const [name, setName] = useState(''); const [endpoint, setEndpoint] = useState('');
  const [token, setToken] = useState(''); const [clientId, setClientId] = useState('');
  const [busy, setBusy] = useState(false); const [error, setError] = useState(''); const [notice, setNotice] = useState('');
  const reload = useCallback(async () => { setConnections(await request<Connection[]>('GET', '/connections')); }, []);
  useEffect(() => { let disposed = false; let timer: ReturnType<typeof setTimeout>; const poll = async () => { try { const rows = await request<Connection[]>('GET', '/connections'); if (!disposed) setConnections(rows); } catch (e) { if (!disposed) setError(describeError(e)); } if (!disposed) timer = setTimeout(poll, 8000); }; void poll(); return () => { disposed = true; clearTimeout(timer); }; }, []);
  const close = () => { if (busy) return; setKind(null); setToken(''); setClientId(''); };
  const add = (next: Kind) => { setKind(next); setName(next === 'mcp' ? '' : kindTitle(next)); setEndpoint(''); setToken(''); setClientId(''); setError(''); setNotice(''); };
  const save = async () => {
    setBusy(true); setError('');
    try {
      if (kind === 'gmail') { const result = await request<{url: string}>('POST', '/connections/gmail/authorize', {clientId, clientSecret: token}); await openExternal(result.url); setNotice('在浏览器完成 Google 授权后，连接会出现在这里。'); }
      else {
        if (isKeyKind(kind) && !token.trim()) throw new Error(`请填写 ${keyKinds[kind].title} ${keyCredentialLabel(kind)}`);
        const presetEndpoint = mcpPresetEndpoint(kind);
        await request('POST', '/connections', {kind: presetEndpoint ? 'mcp' : kind, name, endpoint: presetEndpoint || endpoint, token: isKeyKind(kind) ? token.trim() : token});
      }
      setToken(''); setKind(null); await reload();
    } catch (e) { setError(describeError(e)); } finally { setBusy(false); }
  };
  const toggle = async (c: Connection) => { setBusy(true); setError(''); try { await request('PUT', `/connections/${c.id}`, {enabled: !c.enabled, version: c.version}); await reload(); } catch (e) { setError(describeError(e)); } finally { setBusy(false); } };
  const disconnect = async (c: Connection) => { setBusy(true); setError(''); try { await request('DELETE', `/connections/${c.id}`); await reload(); } catch (e) { setError(describeError(e)); } finally { setBusy(false); } };
  return <section className="mb-9 space-y-5">
    {company && <p className="text-sm leading-6 text-ink-2">当前空间内你的所有 Bot 和群聊均可使用已启用的连接，凭据由公司服务器加密保存。公司登录不会自动授予第三方平台权限。Gmail 授权暂限个人空间。</p>}
    <div className="application-catalog">{cards.filter(([id]) => !company || id !== 'gmail').map(([id, title, caption, Icon]) => <ConnectionAppCard key={id} title={title} description={caption} icon={<ConnectionAppIcon app={id} fallback={<Icon size={23} />} />} detail={connections.some(c => connectionKind(c) === id) ? `${connections.filter(c => connectionKind(c) === id).length} 个已连接账号` : undefined} onAdd={() => add(id as Kind)} />)}{children}</div>
    {error && <p role="alert" className="bot-error">{error}</p>}{notice && <p role="status" className="text-sm text-ink-2">{notice}</p>}

    <Modal open={Boolean(kind)} onClose={close} title={`连接 ${kindTitle(kind)}`} footer={<><Button variant="ghost" disabled={busy} onClick={close}>取消</Button><Button loading={busy} onClick={() => void save()}>{kind === 'gmail' ? '前往 Google 授权' : '验证并连接'}</Button></>}>
      <div className="space-y-5">
      {connections.some(c => connectionKind(c) === kind) && <section aria-label="已连接账号" className="space-y-3"><h3 className="text-sm font-semibold">已连接账号</h3>{connections.filter(c => connectionKind(c) === kind).map(c => <article className="connector-row" key={c.id}><div className="min-w-0 flex-1"><strong className="text-sm">{c.name}</strong><p className="mt-1 truncate text-xs text-ink-3">{c.account || (c.enabled ? '已连接' : '已停用')}</p><p className="mt-1 text-xs text-ink-3">{c.enabled ? '当前空间所有 Bot 和群聊可用' : '已停用，所有 Bot 均不可调用'}</p></div><div className="flex gap-1"><Button size="sm" variant="ghost" disabled={busy} onClick={() => void toggle(c)}>{c.enabled ? '停用' : '启用'}</Button><Button size="sm" variant="ghost" disabled={busy} onClick={() => void disconnect(c)}>断开</Button></div></article>)}<h3 className="pt-3 text-sm font-semibold">添加连接</h3></section>}
      <>
        {kind === 'gmail' ? <><p className="text-sm leading-6 text-ink-2">使用 Google 桌面 OAuth 客户端。凭据保存在本机系统凭据库。</p><TextField label="Google Client ID" value={clientId} onChange={e => setClientId(e.target.value)} autoComplete="off" /><PasswordField label="Google Client Secret" value={token} onChange={e => setToken(e.target.value)} autoComplete="off" /></> : <>
          {kind === 'mcp' && <><TextField label="连接名称" value={name} onChange={e => setName(e.target.value)} maxLength={100} /><TextField label="HTTPS MCP 地址" placeholder="https://example.com/mcp" value={endpoint} onChange={e => setEndpoint(e.target.value)} /><p className="text-xs leading-5 text-ink-3">支持 Streamable HTTP 和 Bearer 令牌。远程工具逐次确认；尚不支持 MCP OAuth 登录。</p></>}
          {isKeyKind(kind) && <><p className="text-sm leading-6 text-ink-2">{keyKinds[kind].hint}</p><button type="button" className="text-xs text-primary-start underline" onClick={() => void openExternal(keyKinds[kind].keyURL).catch(e => setError(describeError(e)))}>获取 {keyCredentialLabel(kind)}</button></>}
          {kind === 'notion' && <p className="text-sm leading-6 text-ink-2">填写 Notion 内部连接令牌，并在 Notion 中将需要的页面共享给此连接。</p>}
          <PasswordField label={kind === 'notion' ? 'Notion 连接令牌' : isKeyKind(kind) ? `${keyKinds[kind].title} ${keyCredentialLabel(kind)}` : '访问令牌（可选）'} value={token} onChange={e => setToken(e.target.value)} autoComplete="off" />
        </>}
      </>
      <p className="text-sm leading-6 text-ink-2">连接后，当前空间内你的所有 Bot 和群聊均可调用，包括以后新建的 Bot。</p>
      <p className="text-xs text-ink-3">{kind === 'gmail' ? '可创建草稿；发送邮件需要确认。' : isKeyKind(kind) ? keyKinds[kind].note : kind === 'notion' ? '可读取已授权页面；创建页面需要确认。' : '调用任何远程工具前需要确认。'}</p>
      {error && <p role="alert" className="bot-error">{error}</p>}</div>
    </Modal>
  </section>;
}
