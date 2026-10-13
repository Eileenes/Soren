import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { agentsAPI, type Bot, type Run } from '../services/agents';
import { describeError } from '../services/api';

export type BotActivity = 'running' | 'queued' | 'waiting_approval' | 'paused';
const priority: Record<BotActivity, number> = { paused: 1, queued: 2, running: 3, waiting_approval: 4 };

export function runActivity(runs: Run[]): Record<string, BotActivity> {
  const activity: Record<string, BotActivity> = {};
  const mark = (id: string, state: BotActivity) => {
    if (!activity[id] || priority[state] > priority[activity[id]]) activity[id] = state;
  };
  for (const run of runs) {
    if (!Object.prototype.hasOwnProperty.call(priority, run.status)) continue;
    mark(run.ownerBotId, run.status as BotActivity);
    for (const step of run.steps) {
      // Completed/cancelled runs and paused steps must never retain an active animation.
      if (run.status === 'running' && step.status === 'running') mark(step.botId, 'running');
      if (run.status === 'waiting_approval' && step.status === 'waiting_approval') mark(step.botId, 'waiting_approval');
    }
  }
  return activity;
}

/** One shared poll feeds task history and every Bot avatar, with an explicit Bot scope. */
export function useBotRuns(bots: Bot[], enabled: boolean) {
  const idsKey = JSON.stringify(bots.filter(bot => bot.status !== 'archived').map(bot => bot.id).sort());
  const ids = useMemo<string[]>(() => JSON.parse(idsKey), [idsKey]);
  const [byBot, setByBot] = useState<Record<string, Run[]>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const generation = useRef(0);
  const reload = useCallback(async () => {
    if (!enabled) return;
    const current = ++generation.current;
    const results = await Promise.allSettled(ids.map(id => agentsAPI.runs(id)));
    if (current !== generation.current) return;
    const next: Record<string, Run[]> = {};
    const failures: Record<string, string> = {};
    results.forEach((result, index) => {
      if (result.status === 'fulfilled') next[ids[index]] = result.value;
      else failures[ids[index]] = describeError(result.reason);
    });
    setByBot(next); setErrors(failures);
  }, [enabled, ids]);
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    if (!enabled) { setByBot({}); setErrors({}); return; }
    const poll = async () => { await reload(); if (!disposed) timer = setTimeout(poll, 2500); };
    void poll();
    return () => { disposed = true; generation.current++; clearTimeout(timer); };
  }, [reload, enabled]);
  const activity = useMemo(() => runActivity(Object.values(byBot).flat()), [byBot]);
  return { byBot, errors, activity, reload };
}
