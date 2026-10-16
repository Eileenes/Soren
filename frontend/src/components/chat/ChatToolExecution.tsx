import { useCallback, useEffect, useRef, useState } from 'react';
import { connectionRequest } from '../../services/connections';
import { describeError } from '../../services/api';
import type { Run } from '../../services/agents';
import { TaskExecution } from '../TaskExecution';
import { Button } from '../ui';
import { MarkdownView } from './MarkdownView';

const labels: Record<string, string> = { queued: '正在排队', running: '正在处理', waiting_approval: '需要你确认操作', paused: '执行已暂停，可继续或取消', succeeded: '已完成', failed: '执行未完成', cancelled: '已取消' };
const live = new Set(['queued', 'running', 'waiting_approval', 'paused']);

/** The scope is fixed to the message's Bot, never the sidebar's current selection. */
export function ChatToolExecution({ runId, botId, initialContent }: { runId: string; botId: string; initialContent: string }) {
  const [run, setRun] = useState<Run>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [revision, setRevision] = useState(0);
  const alive = useRef(true);
  useEffect(() => {
    let disposed = false;
    alive.current = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      let again = true;
      try {
        const next = await connectionRequest<Run>('GET', `/runs/${runId}`, undefined, botId);
        if (disposed) return;
        setRun(next); setError(''); again = live.has(next.status);
      } catch (e) { if (!disposed) setError(describeError(e)); }
      if (!disposed && again) timer = setTimeout(poll, 1500);
    };
    void poll();
    return () => { disposed = true; alive.current = false; clearTimeout(timer); };
  }, [runId, botId, revision]);

  const action = useCallback(async (verb: 'resume' | 'cancel') => {
    setBusy(true); setError('');
    try {
      await connectionRequest('POST', `/runs/${runId}/${verb}`, {}, botId);
      if (alive.current) setRevision(n => n + 1);
    } catch (e) { if (alive.current) setError(describeError(e)); }
    finally { if (alive.current) setBusy(false); }
  }, [runId, botId]);
  const content = run?.status === 'succeeded' ? run.result : initialContent;
  return <div className="space-y-3" aria-label="聊天执行">
    {content ? <MarkdownView content={content} /> : null}
    {(!run || run.status !== 'succeeded') && <p role="status" className="text-sm text-ink-2">{run ? labels[run.status] || run.status : '正在读取执行状态…'}</p>}
    {run?.error && <p role="alert" className="text-sm text-error">{run.error}</p>}
    <TaskExecution runId={runId} botId={botId} active={!run || live.has(run.status)} onDecision={() => action('resume')} />
    {run && live.has(run.status) && <div className="flex flex-wrap gap-2">
      {['paused', 'waiting_approval'].includes(run.status) && <Button size="sm" loading={busy} onClick={() => void action('resume')}>继续执行</Button>}
      <Button size="sm" variant="ghost" disabled={busy} onClick={() => void action('cancel')}>取消执行</Button>
    </div>}
    {error && <p role="alert" className="text-sm text-error">{error}</p>}
  </div>;
}
