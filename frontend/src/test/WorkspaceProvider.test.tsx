import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { companyAPI, CompanyRequestError, openCompanyLogin, type WorkspaceStatus } from '../services/company';
import { DEFAULT_API_BASE_URL, getApiBaseUrl, http, setApiBaseUrl } from '../services/api';
import { CompanyPanel } from '../components/CompanyPanel';
import { WorkspaceProvider, useWorkspace } from '../components/WorkspaceProvider';

vi.mock('../services/company', async importOriginal => ({
  ...await importOriginal<typeof import('../services/company')>(),
  companyAPI: { status: vi.fn(), discover: vi.fn(), login: vi.fn(), poll: vi.fn(), complete: vi.fn(), cancel: vi.fn(), disconnect: vi.fn() },
  openCompanyLogin: vi.fn(),
}));
const connection = { active: true, serverUrl: 'https://company.example', organization: '测试公司', workspaceId: 'alice-space', userId: 'alice', userName: 'Alice' };
const company: WorkspaceStatus = { mode: 'enterprise', connection };
const login = { id: 'attempt-1', url: 'https://company.example/auth/browser?ticket=attempt-1', expiresAt: new Date(Date.now() + 300000).toISOString(), organization: '测试公司' };
function Content() {
  const { status, reload } = useWorkspace();
  const [draft, setDraft] = useState('');
  return <><p>业务界面：{status.mode}</p><input aria-label="聊天草稿" value={draft} onChange={e => setDraft(e.target.value)} /><button onClick={() => void reload()}>刷新空间</button><CompanyPanel /></>;
}
beforeEach(() => {
  vi.resetAllMocks();
  setApiBaseUrl(DEFAULT_API_BASE_URL);
  vi.mocked(companyAPI.status).mockResolvedValue({ mode: 'personal' });
  vi.mocked(companyAPI.discover).mockResolvedValue({ product: 'soren', protocolVersion: 1, auth: 'oauth-browser', name: '测试公司', serverUrl: connection.serverUrl });
  vi.mocked(companyAPI.login).mockResolvedValue(login);
  vi.mocked(companyAPI.poll).mockResolvedValue({ status: 'authorized', connection });
  vi.mocked(companyAPI.complete).mockResolvedValue(company);
  vi.mocked(companyAPI.cancel).mockResolvedValue(undefined);
  vi.mocked(companyAPI.disconnect).mockResolvedValue({ revoked: true, credentialDeleted: true });
});
afterEach(() => { vi.useRealTimers(); setApiBaseUrl(DEFAULT_API_BASE_URL); });

async function startLogin() {
  render(<WorkspaceProvider><Content /></WorkspaceProvider>);
  await screen.findByText('业务界面：personal');
  fireEvent.change(screen.getByLabelText('公司服务器地址'), { target: { value: connection.serverUrl } });
  fireEvent.click(screen.getByRole('button', { name: '识别服务器' }));
  await screen.findByRole('button', { name: '使用公司账号登录' });
  vi.useFakeTimers();
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '使用公司账号登录' })); });
  expect(openCompanyLogin).toHaveBeenCalledWith(login);
  await act(async () => { await vi.advanceTimersByTimeAsync(1500); });
}
describe('个人与公司空间', () => {
  it('恢复模式前不挂载业务；个人模式不发起登录', async () => {
    let resolve!: (value: WorkspaceStatus) => void;
    vi.mocked(companyAPI.status).mockReturnValue(new Promise(r => { resolve = r; }));
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    expect(screen.queryByLabelText('聊天草稿')).not.toBeInTheDocument();
    await act(async () => resolve({ mode: 'personal' }));
    expect(screen.getByText('业务界面：personal')).toBeInTheDocument();
    expect(companyAPI.login).not.toHaveBeenCalled();
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL);
  });
  it('恢复公司会话并附带空间约束；过期时停止业务界面', async () => {
    vi.mocked(companyAPI.status).mockResolvedValue(company);
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    await screen.findByText('业务界面：enterprise');
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL + '/enterprise');
    expect(http.defaults.headers.common['X-Soren-Workspace']).toBe('https://company.example:alice-space:alice');
    act(() => window.dispatchEvent(new Event('soren:company-session-expired')));
    expect(screen.queryByLabelText('聊天草稿')).not.toBeInTheDocument();
    expect(screen.getByText('公司登录已失效，请重新登录。')).toBeInTheDocument();
  });
  it('恢复失败时不默认回到个人空间', async () => {
    vi.useFakeTimers();
    vi.mocked(companyAPI.status).mockRejectedValue(new Error('本地服务不可用'));
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    await act(async () => { await vi.advanceTimersByTimeAsync(12000); });
    expect(screen.getByText('本地服务不可用')).toBeInTheDocument();
    expect(screen.queryByLabelText('聊天草稿')).not.toBeInTheDocument();
  });
  it('本地服务延迟启动后自动恢复已保存的公司空间', async () => {
    vi.useFakeTimers();
    vi.mocked(companyAPI.status).mockRejectedValueOnce(new Error('本地服务尚未启动')).mockResolvedValue(company);
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    expect(screen.queryByLabelText('聊天草稿')).not.toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(screen.getByText('业务界面：enterprise')).toBeInTheDocument();
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL + '/enterprise');
    expect(companyAPI.login).not.toHaveBeenCalled();
  });
  it('公司离线保持公司上下文，允许退出回到个人空间', async () => {
    vi.mocked(companyAPI.status).mockResolvedValue({ ...company, error: '公司服务器暂不可达' });
    vi.mocked(companyAPI.disconnect).mockResolvedValue({ revoked: false, credentialDeleted: true });
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    await screen.findByText('公司服务器暂不可达');
    expect(screen.queryByLabelText('聊天草稿')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '退出公司，使用个人空间' }));
    await screen.findByText('业务界面：personal');
    expect(screen.getByText(/部分退出清理未完成/)).toBeInTheDocument();
    expect(http.defaults.headers.common['X-Soren-Workspace']).toBeUndefined();
  });
  it('浏览器授权后等待账号确认；取消不会切换', async () => {
    await startLogin();
    expect(screen.getByText(/已登录：Alice/)).toBeInTheDocument();
    expect(companyAPI.complete).not.toHaveBeenCalled();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '取消登录' })); });
    expect(companyAPI.cancel).toHaveBeenCalledWith(login.id);
    expect(screen.getByText('业务界面：personal')).toBeInTheDocument();
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL);
  });
  it('确认进入公司后清除旧空间的草稿和界面状态', async () => {
    await startLogin();
    fireEvent.change(screen.getByLabelText('聊天草稿'), { target: { value: '个人私密草稿' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '进入公司空间' })); });
    expect(companyAPI.complete).toHaveBeenCalledWith(login.id);
    expect(screen.getByText('业务界面：enterprise')).toBeInTheDocument();
    expect(screen.getByLabelText('聊天草稿')).toHaveValue('');
    expect(companyAPI.cancel).not.toHaveBeenCalled();
  });
  it('账号确认页过期后清理待确认会话并允许重新登录', async () => {
    await startLogin();
    expect(screen.getByText(/已登录：Alice/)).toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(Date.parse(login.expiresAt) - Date.now()); });
    expect(screen.getByText('登录已过期，请重新登录。')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '进入公司空间' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '使用公司账号登录' })).toBeEnabled();
    expect(companyAPI.cancel).toHaveBeenCalledWith(login.id);
    expect(companyAPI.complete).not.toHaveBeenCalled();
  });
  it('短暂轮询失败后继续确认同一次登录', async () => {
    vi.mocked(companyAPI.poll).mockRejectedValueOnce(new CompanyRequestError('服务暂不可用', 503));
    await startLogin();
    expect(screen.getByText('服务暂不可用')).toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
    expect(screen.getByText(/已登录：Alice/)).toBeInTheDocument();
    expect(screen.queryByText('服务暂不可用')).not.toBeInTheDocument();
    expect(companyAPI.login).toHaveBeenCalledTimes(1);
    expect(companyAPI.cancel).not.toHaveBeenCalled();
  });
  it('账号确认被服务器拒绝后允许重新登录', async () => {
    vi.mocked(companyAPI.complete).mockRejectedValue(new CompanyRequestError('公司登录已失效', 401));
    await startLogin();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: '进入公司空间' })); });
    expect(screen.queryByRole('button', { name: '进入公司空间' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '使用公司账号登录' })).toBeEnabled();
    expect(companyAPI.cancel).toHaveBeenCalledWith(login.id);
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL);
  });
  it('识别服务失败不会切换或发起登录', async () => {
    vi.mocked(companyAPI.discover).mockRejectedValue(new Error('服务地址不兼容'));
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    await screen.findByText('业务界面：personal');
    fireEvent.change(screen.getByLabelText('公司服务器地址'), { target: { value: 'https://wrong.example' } });
    fireEvent.click(screen.getByRole('button', { name: '识别服务器' }));
    await screen.findByText('服务地址不兼容');
    expect(companyAPI.login).not.toHaveBeenCalled();
    expect(getApiBaseUrl()).toBe(DEFAULT_API_BASE_URL);
  });
  it('切换至另一员工时重建界面', async () => {
    vi.mocked(companyAPI.status).mockResolvedValue(company);
    render(<WorkspaceProvider><Content /></WorkspaceProvider>);
    await screen.findByText('业务界面：enterprise');
    fireEvent.change(screen.getByLabelText('聊天草稿'), { target: { value: 'Alice 的私密草稿' } });
    vi.mocked(companyAPI.status).mockResolvedValue({ mode: 'enterprise', connection: { ...connection, workspaceId: 'bob-space', userId: 'bob', userName: 'Bob' } });
    fireEvent.click(screen.getByRole('button', { name: '刷新空间' }));
    await waitFor(() => expect(screen.getByLabelText('聊天草稿')).toHaveValue(''));
    expect(http.defaults.headers.common['X-Soren-Workspace']).toBe('https://company.example:bob-space:bob');
  });
});
