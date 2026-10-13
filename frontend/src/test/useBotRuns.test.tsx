import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { agentsAPI, DEFAULT_BOT, type Run } from '../services/agents';
import { runActivity, useBotRuns } from '../hooks/useBotRuns';

const run = (status: string, stepStatus = status): Run => ({ id: 'r', ownerBotId: 'soren', goal: 'Test', status, steps: [{ botId: 'peer', name: 'Peer', avatar: 'robo', status: stepStatus, output: '' }], result: '', error: '', demo: true, createdAt: '' });
afterEach(() => vi.restoreAllMocks());

describe('Bot 任务状态', () => {
  it('同时标记任务发起者和执行中的协作 Bot', () => {
    expect(runActivity([run('running')])).toEqual({ soren: 'running', peer: 'running' });
    expect(runActivity([run('queued')])).toEqual({ soren: 'queued' });
  });
  it('暂停、待确认和终态不保留旧步骤的运行动效', () => {
    expect(runActivity([run('paused', 'running')])).toEqual({ soren: 'paused' });
    expect(runActivity([run('waiting_approval')])).toEqual({ soren: 'waiting_approval', peer: 'waiting_approval' });
    for (const status of ['succeeded', 'failed', 'cancelled']) expect(runActivity([run(status, 'running')])).toEqual({});
  });
  it('多个任务时优先展示需要用户处理的状态', () => {
    expect(runActivity([run('running'), run('waiting_approval')]).soren).toBe('waiting_approval');
  });
  it('刷新完成和请求失败后清除旧动效；服务离线时忽略迟到响应', async () => {
    const mock = vi.spyOn(agentsAPI, 'runs').mockResolvedValue([run('running')]);
    const { result, rerender } = renderHook(({ online }) => useBotRuns([DEFAULT_BOT], online), { initialProps: { online: true } });
    await waitFor(() => expect(result.current.activity.soren).toBe('running'));
    mock.mockResolvedValue([run('succeeded', 'running')]);
    await act(async () => { await result.current.reload(); });
    expect(result.current.activity).toEqual({});
    mock.mockRejectedValue(new Error('连接失败'));
    await act(async () => { await result.current.reload(); });
    expect(result.current.errors.soren).toBe('连接失败');
    let resolve!: (value: Run[]) => void;
    mock.mockImplementation(() => new Promise(done => { resolve = done; }));
    let pending!: Promise<void>;
    act(() => { pending = result.current.reload(); });
    rerender({ online: false });
    await act(async () => { resolve([run('running')]); await pending; });
    expect(result.current.activity).toEqual({});
  });
});
