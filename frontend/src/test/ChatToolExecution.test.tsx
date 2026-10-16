import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
const mocks = vi.hoisted(() => ({ request: vi.fn(), download: vi.fn() }));
vi.mock('../services/connections', () => ({ connectionRequest: mocks.request, downloadAttachment: mocks.download }));
vi.mock('../services/api', () => ({ describeError: (e: Error) => e.message }));
vi.mock('../components/chat/MarkdownView', () => ({ MarkdownView: ({ content }: {content: string}) => <p>{content}</p> }));
import { ChatToolExecution } from '../components/chat/ChatToolExecution';
beforeEach(() => { vi.clearAllMocks(); });
describe('聊天工具执行', () => {
  it('在消息所属 Bot 下确认并继续执行', async () => {
    mocks.request.mockImplementation(async (method: string, path: string) => {
      if (method === 'POST') return;
      if (path.endsWith('/tools')) return [{ id: 'inv-1', status: 'pending', tool: 'generate_music', arguments: '{"title":"test"}', connectionName: '音乐', account: 'test account' }];
      if (path.endsWith('/artifacts') || path.endsWith('/files')) return [];
      return { id: 'run-1', status: 'waiting_approval', result: '' };
    });
    render(<ChatToolExecution runId="run-1" botId="speaker-bot" initialContent="" />);
    fireEvent.click(await screen.findByRole('button', { name: '批准此操作' }));
    await waitFor(() => expect(mocks.request).toHaveBeenCalledWith('POST', '/runs/run-1/resume', {}, 'speaker-bot'));
    expect(mocks.request).toHaveBeenCalledWith('POST', '/tool-invocations/inv-1/decision', { approve: true }, 'speaker-bot');
    for (const call of mocks.request.mock.calls) expect(call[3]).toBe('speaker-bot');
  });
  it('从执行记录恢复结果并提供当前 Bot 的成果下载', async () => {
    const file = { id: 'file', runId: 'run-1', name: 'song.mp3', botId: 'speaker-bot', size: 10 };
    mocks.request.mockImplementation(async (_: string, path: string) => {
      if (path.endsWith('/files')) return [file];
      if (path.endsWith('/tools') || path.endsWith('/artifacts')) return [];
      return { id: 'run-1', status: 'succeeded', result: '音乐已生成' };
    });
    mocks.download.mockResolvedValue(undefined);
    render(<ChatToolExecution runId="run-1" botId="speaker-bot" initialContent="" />);
    expect(await screen.findByText('音乐已生成')).toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: '下载文件' }));
    await waitFor(() => expect(mocks.download).toHaveBeenCalledWith(file, 'speaker-bot'));
    expect(screen.queryByRole('button', { name: '取消执行' })).not.toBeInTheDocument();
  });
});
