import { TaskExecution } from '../components/TaskExecution';
import { useEffect, useRef, useState } from 'react';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { Button, TextAreaField } from '../components/ui';
import { MarkdownView } from '../components/chat/MarkdownView';
import { agentsAPI, avatarURL, type Bot, type Run } from '../services/agents';
import { describeError } from '../services/api';
const labels: Record<string, string> = { queued: '排队中', running: '进行中', paused: '已暂停', waiting_approval: '待确认', succeeded: '已完成', failed: '失败', cancelled: '已取消' };
export function TasksPage({ bot, bots, demo, example, onExampleUsed, runs, runsError, onReload: reload }: { bot: Bot; bots: Bot[]; demo?: boolean; example?: boolean; onExampleUsed?: () => void; runs: Run[]; runsError?: string; onReload: () => Promise<void> }) {
 const [goal, setGoal] = useState(''); const [peers, setPeers] = useState<string[]>([]); const [busy, setBusy] = useState(false); const [error, setError] = useState(''); const [pending, setPending] = useState<string | null>(null); const [expanded, setExpanded] = useState<string | null>(null);
 useEffect(() => { if (example) { setGoal(value => value || '制定一份三天学习计划，每天包含目标、行动和检查项。调用本地成果工具保存为 Markdown，标题为“三天学习计划”。'); onExampleUsed?.(); } }, [example, onExampleUsed]);
 const alive = useRef(true); const request = useRef<{ fingerprint: string; id: string }>();
 useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
 const eligible = bots.filter(b => b.id !== bot.id && b.status === 'active' && bot.collaborationEnabled && b.collaborationEnabled && bot.collaborators.includes(b.id) && b.collaborators.includes(bot.id));
 const start = async () => { setBusy(true); setError(''); const selected = peers.filter(id => eligible.some(b => b.id === id)); const fingerprint = JSON.stringify([goal, selected]); if (request.current?.fingerprint !== fingerprint) request.current = { fingerprint, id: crypto.randomUUID() }; try { const run = await agentsAPI.start(goal, selected, request.current.id); if (!alive.current) return; setGoal(''); setPeers([]); request.current = undefined; setExpanded(run.id); await reload(); } catch (e) { if (alive.current) setError(describeError(e)); } finally { if (alive.current) setBusy(false); } };
 const action = async (id: string, resume: boolean) => { setPending(id); setError(''); try { if (resume) await agentsAPI.resume(id); else await agentsAPI.cancel(id); await reload(); } catch (e) { if (alive.current) setError(describeError(e)); } finally { if (alive.current) setPending(null); } };
 return <PageContainer><PageHeading title="任务" /><div className="task-composer"><TextAreaField label="任务目标" rows={3} maxLength={8000} value={goal} onChange={e => setGoal(e.target.value)} placeholder={`交给 ${bot.name} 的任务…`} />
 {eligible.length > 0 && <div className="task-peers"><span>邀请协作</span>{eligible.map(b => <label key={b.id}><input type="checkbox" checked={peers.includes(b.id)} disabled={!peers.includes(b.id) && peers.length >= 3} onChange={e => setPeers(prev => e.target.checked ? [...prev, b.id] : prev.filter(id => id !== b.id))} /><img src={avatarURL(b.avatar)} alt="" />{b.name}</label>)}</div>}
 <div className="task-composer-footer"><span>{demo ? '未连接模型 · 任务将运行本地回显示例' : '工具任务 · 按 Bot 授权执行'}</span><Button disabled={!goal.trim() || bot.status !== 'active'} loading={busy} onClick={() => void start()}>开始任务</Button></div></div>
 {(error || runsError) && <p role="alert" className="bot-error my-4">{error || runsError}</p>}
 <div className="run-list">{runs.length === 0 ? <p className="py-14 text-center text-sm text-ink-3">暂无任务</p> : runs.map(run => <article className="run-card" key={run.id}><button className="run-summary" onClick={() => setExpanded(expanded === run.id ? null : run.id)} aria-expanded={expanded === run.id}><div className="run-avatars">{run.steps.map(step => <img key={step.botId} src={avatarURL(step.avatar)} alt={step.name} />)}</div><span className="run-title">{run.goal}</span><span className={'run-state state-' + run.status}>{labels[run.status] || run.status}</span></button>
 {expanded === run.id && <div className="run-detail">{run.demo && <p className="text-xs text-ink-3 mb-4">本地演示记录，未调用真实模型。</p>}{run.steps.map((step, i) => <div className="run-step" key={step.botId}><div><span>{String(i + 1).padStart(2, '0')}</span><img src={avatarURL(step.avatar)} alt="" /><strong>{step.name}</strong><small>{labels[step.status] || step.status}</small></div>{step.output && <MarkdownView content={step.output} />}</div>)}
 <TaskExecution runId={run.id} />{run.error && <p role="alert" className="bot-error">{run.error}</p>}
 <div className="run-footer"><time>{new Date(run.createdAt).toLocaleString()}</time>{['paused', 'waiting_approval'].includes(run.status) && <Button size="sm" loading={pending === run.id} onClick={() => void action(run.id, true)}>继续任务</Button>}{['queued', 'running', 'paused', 'waiting_approval'].includes(run.status) && <Button variant="ghost" size="sm" loading={pending === run.id} onClick={() => void action(run.id, false)}>取消任务</Button>}</div></div>}
 </article>)}</div></PageContainer>;
}
