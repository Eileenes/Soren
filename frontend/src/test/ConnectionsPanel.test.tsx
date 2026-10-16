import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { keyCredentialLabel, keyKinds, type KeyKindId } from '../components/connectorKinds';

const request = vi.fn();
const openExternal = vi.fn();
vi.mock('../services/connections', () => ({ connectionRequest: (...a: unknown[]) => request(...a), openExternal: (...a: unknown[]) => openExternal(...a) }));
vi.mock('../services/agents', () => ({ agentsAPI: { list: async () => [{ id: 'soren', name: 'Soren', avatar: 'a', status: 'active' }] }, avatarURL: () => '' }));

import { ConnectionsPanel } from '../components/ConnectionsPanel';

const mcpPresets = [
  { id: 'agent-toolkit', title: 'Agent 工具包', credential: 'API Key', endpoint: 'https://agent-toolkit.app.baizhi.cloud/mcp', keyURL: 'https://agent-toolkit.app.baizhi.cloud/access-guide' },
  { id: 'octobus', title: '致用 OctoBus', credential: '个人 Token', endpoint: 'https://octobus.chaitin.net/capsets/mcp', keyURL: 'https://octobus.chaitin.net' },
];

describe('ConnectionsPanel API-key connectors', () => {
  beforeEach(() => { request.mockReset(); request.mockImplementation(async (method: string) => method === 'GET' ? [] : {}); openExternal.mockReset(); openExternal.mockResolvedValue(undefined); });

  it('lists every key connector as a card', async () => {
    render(<ConnectionsPanel />);
    for (const k of Object.values(keyKinds)) expect(await screen.findByText(k.title)).toBeTruthy();
  });

  it.each(Object.entries(keyKinds))('saves %s with the entered key', async (id, k) => {
    const preset = mcpPresets.find(p => p.id === id);
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: `添加 ${k.title}` }));
    fireEvent.change(await screen.findByLabelText(`${k.title} ${keyCredentialLabel(id as KeyKindId)}`), { target: { value: ' secret-key ' } });
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    fireEvent.click(screen.getByText('验证并连接'));
    const transport = preset
      ? { kind: 'mcp', endpoint: preset.endpoint, name: preset.title }
      : { kind: id };
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ ...transport, token: 'secret-key' })));
    expect(request.mock.calls.find(([method]) => method === 'POST')?.[2]).not.toHaveProperty('allowedBots');
  });

  it.each(mcpPresets)('provides a credential-only $title form and its official link', async preset => {
    const label = `${preset.title} ${preset.credential}`;
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: `添加 ${preset.title}` }));
    expect(await screen.findByLabelText(label)).toHaveAttribute('type', 'password');
    expect(screen.queryByLabelText('HTTPS MCP 地址')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('连接名称')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: `获取 ${preset.credential}` }));
    expect(openExternal).toHaveBeenCalledWith(preset.keyURL);
    fireEvent.change(screen.getByLabelText(label), { target: { value: '  ' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    expect(await screen.findAllByText(`请填写 ${label}`)).not.toHaveLength(0);
    expect(request.mock.calls.some(([method]) => method === 'POST')).toBe(false);
  });

  it.each(mcpPresets)('keeps the $title form open when MCP discovery rejects the credential', async preset => {
    request.mockImplementation(async (method: string) => {
      if (method === 'POST') throw new Error('MCP 连接失败；请检查地址及访问令牌');
      return [];
    });
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: `添加 ${preset.title}` }));
    fireEvent.change(await screen.findByLabelText(`${preset.title} ${preset.credential}`), { target: { value: ' wrong-key ' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    expect(await screen.findAllByText('MCP 连接失败；请检查地址及访问令牌')).not.toHaveLength(0);
    expect(screen.getByRole('dialog', { name: `连接 ${preset.title}` })).toBeInTheDocument();
    expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ token: 'wrong-key' }));
  });

  it.each(mcpPresets)('recognizes legacy $title connections and offers workspace-wide enable/disable', async preset => {
    const saved = { id: preset.id, kind: 'mcp', name: '我的工具包', endpoint: `${preset.endpoint}/`, allowedBots: ['soren'], enabled: true, version: 3, tools: [{ name: 'search' }] };
    const generic = { ...saved, id: 'other', name: '其他 MCP', endpoint: 'https://example.com/mcp' };
    request.mockImplementation(async (method: string) => method === 'GET' ? [saved, generic] : {});
    render(<ConnectionsPanel />);
    await waitFor(() => expect(screen.getAllByText('1 个已连接账号')).toHaveLength(2));
    fireEvent.click(screen.getByRole('button', { name: `添加 ${preset.title}` }));
    const accounts = await screen.findByRole('region', { name: '已连接账号' });
    expect(within(accounts).getByText('我的工具包')).toBeInTheDocument();
    expect(within(accounts).queryByText('其他 MCP')).not.toBeInTheDocument();
    expect(within(accounts).getByText('当前空间所有 Bot 和群聊可用')).toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '权限' })).not.toBeInTheDocument();
    fireEvent.click(within(accounts).getByRole('button', { name: '停用' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('PUT', `/connections/${preset.id}`, { enabled: false, version: 3 }));
    fireEvent.click(within(await screen.findByRole('region', { name: '已连接账号' })).getByRole('button', { name: '断开' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('DELETE', `/connections/${preset.id}`));
  });

  it('continues to support custom MCP endpoints', async () => {
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 MCP' }));
    fireEvent.change(await screen.findByLabelText('连接名称'), { target: { value: 'My tools' } });
    fireEvent.change(screen.getByLabelText('HTTPS MCP 地址'), { target: { value: 'https://example.com/mcp' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ kind: 'mcp', endpoint: 'https://example.com/mcp', name: 'My tools', token: '' })));
  });

  it('connects VideoAPI with only a Key, keeps failed validation visible, and links to the official site', async () => {
    request.mockImplementation(async (method: string) => {
      if (method === 'POST') throw new Error('视频 API Key 无效、已停用或权限不足，请重新连接');
      return [];
    });
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 视频 API' }));
    const dialog = screen.getByRole('dialog', { name: '连接 视频 API' });
    const key = within(dialog).getByLabelText('视频 API Key');
    expect(key).toHaveAttribute('type', 'password');
    expect(within(dialog).queryByLabelText('HTTPS MCP 地址')).not.toBeInTheDocument();
    expect(within(dialog).queryByLabelText('连接名称')).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: '获取 Key' }));
    expect(openExternal).toHaveBeenCalledWith('https://video-api.app.baizhi.cloud');
    fireEvent.click(within(dialog).getByRole('button', { name: '验证并连接' }));
    await waitFor(() => expect(within(dialog).getByRole('alert')).toHaveTextContent('请填写 视频 API Key'));
    expect(request.mock.calls.some(([method]) => method === 'POST')).toBe(false);
    fireEvent.change(key, { target: { value: ' video-key ' } });
    expect(within(dialog).queryByRole('checkbox')).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole('button', { name: '验证并连接' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ kind: 'video', name: '视频 API', token: 'video-key' })));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('请重新连接');
    expect(dialog).toBeInTheDocument();
  });
  it('starts Gmail authorization without a Bot selection', async () => {
    request.mockImplementation(async (method: string) => method === 'GET' ? [] : {url: 'https://accounts.google.com/test'});
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 Gmail' }));
    fireEvent.change(screen.getByLabelText('Google Client ID'), {target: {value: 'client-id'}});
    fireEvent.change(screen.getByLabelText('Google Client Secret'), {target: {value: 'client-secret'}});
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: '前往 Google 授权'}));
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections/gmail/authorize', {clientId: 'client-id', clientSecret: 'client-secret'}));
  });

});
