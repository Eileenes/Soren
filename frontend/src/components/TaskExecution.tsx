import { invoke } from '@tauri-apps/api/core';
import { useEffect, useState } from 'react';
import { connectionRequest as request, type Artifact, type Invocation } from '../services/connections';
import { describeError } from '../services/api';
import { Button } from './ui';
import { MarkdownView } from './chat/MarkdownView';
const states: Record<string, string> = {pending: '等待确认', approved: '已批准 · 继续任务后执行', denied: '已拒绝', running: '执行中', succeeded: '已完成', failed: '失败', unknown: '结果待核实'};
export function TaskExecution({runId}: {runId: string}) {
  const [calls, setCalls] = useState<Invocation[]>([]); const [artifacts, setArtifacts] = useState<Artifact[]>([]);
  const [error, setError] = useState(''); const [busy, setBusy] = useState<string | null>(null);
  useEffect(() => { let active = true; let timer: ReturnType<typeof setTimeout>; const poll = async () => { try { const [a, b] = await Promise.all([request<Invocation[]>('GET', `/runs/${runId}/tools`), request<Artifact[]>('GET', `/runs/${runId}/artifacts`)]); if (active) { setCalls(a); setArtifacts(b); } } catch (e) { if (active) setError(describeError(e)); } if (active) timer = setTimeout(poll, 2000); }; void poll(); return () => { active = false; clearTimeout(timer); }; }, [runId]);
  const decide = async (id: string, approve: boolean) => { setBusy(id); setError(''); try { await request('POST', `/tool-invocations/${id}/decision`, {approve}); setCalls(prev => prev.map(c => c.id === id ? {...c, status: approve ? 'approved' : 'denied'} : c)); } catch (e) { setError(describeError(e)); } finally { setBusy(null); } };
  const download = async (a: Artifact) => { if ('__TAURI_INTERNALS__' in window) { try { await invoke('export_markdown', {title: a.title, content: a.content}); } catch (e) { setError(describeError(e)); } return; } const url = URL.createObjectURL(new Blob([a.content], {type: 'text/markdown;charset=utf-8'})); const link = document.createElement('a'); link.href = url; link.download = a.title.replace(/[\\/:*?"<>|]/g, '_') + '.md'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); };
  return <div className="task-execution">{calls.map(call => <details key={call.id} className="tool-record" open={call.status === 'pending'}><summary><span className="truncate font-mono text-xs">{call.tool.replace(/^c_[a-f0-9]+_/, '')}</span><span className="shrink-0 text-xs text-ink-3">{states[call.status] || call.status}</span></summary>{call.account && <p className="mt-3 break-all text-xs text-ink-2">{call.connectionName} · {call.account}</p>}<pre className="tool-payload">{pretty(call.arguments)}</pre>{call.result && <pre className="tool-payload">{pretty(call.result)}</pre>}{call.status === 'pending' && <div className="mt-3 flex justify-end gap-2"><Button size="sm" variant="ghost" disabled={busy !== null} onClick={() => void decide(call.id, false)}>拒绝</Button><Button size="sm" loading={busy === call.id} disabled={busy !== null} onClick={() => void decide(call.id, true)}>批准此操作</Button></div>}</details>)}
    {artifacts.map(a => <details className="artifact-record" key={a.id}><summary><strong className="text-sm">{a.title}</strong><span className="text-xs text-primary-start">本地成果</span></summary><div className="mt-4"><MarkdownView content={a.content} /><div className="mt-4 flex justify-end"><Button size="sm" variant="ghost" onClick={() => void download(a)}>下载 Markdown</Button></div></div></details>)}
    {error && <p role="alert" className="bot-error">{error}</p>}
  </div>;
}
function pretty(text: string) { try { return JSON.stringify(JSON.parse(text), null, 2); } catch { return text; } }
