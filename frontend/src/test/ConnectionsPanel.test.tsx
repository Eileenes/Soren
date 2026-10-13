import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { keyKinds } from '../components/connectorKinds';

const request = vi.fn();
const openExternal = vi.fn();
vi.mock('../services/connections', () => ({ connectionRequest: (...a: unknown[]) => request(...a), openExternal: (...a: unknown[]) => openExternal(...a) }));
vi.mock('../services/agents', () => ({ agentsAPI: { list: async () => [{ id: 'soren', name: 'Soren', avatar: 'a', status: 'active' }] }, avatarURL: () => '' }));

import { ConnectionsPanel } from '../components/ConnectionsPanel';

describe('ConnectionsPanel API-key connectors', () => {
  beforeEach(() => { request.mockReset(); request.mockImplementation(async (method: string) => method === 'GET' ? [] : {}); openExternal.mockReset(); openExternal.mockResolvedValue(undefined); });

  it('lists every key connector as a card', async () => {
    render(<ConnectionsPanel />);
    for (const k of Object.values(keyKinds)) expect(await screen.findByText(k.title)).toBeTruthy();
  });

  it.each(Object.entries(keyKinds))('saves %s with the entered key', async (id, k) => {
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: `添加 ${k.title}` }));
    fireEvent.change(await screen.findByLabelText(`${k.title} API Key`), { target: { value: 'secret-key' } });
    fireEvent.click(await screen.findByLabelText('Soren'));
    fireEvent.click(screen.getByText('验证并连接'));
    const transport = id === 'agent-toolkit'
      ? { kind: 'mcp', endpoint: 'https://agent-toolkit.app.baizhi.cloud/mcp', name: 'Agent 工具包' }
      : { kind: id };
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ ...transport, token: 'secret-key', allowedBots: ['soren'] })));
  });

  it('provides a key-only Agent Toolkit form and its official access guide', async () => {
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 Agent 工具包' }));
    expect(await screen.findByLabelText('Agent 工具包 API Key')).toHaveAttribute('type', 'password');
    expect(screen.queryByLabelText('HTTPS MCP 地址')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('连接名称')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '获取 API Key' }));
    expect(openExternal).toHaveBeenCalledWith('https://agent-toolkit.app.baizhi.cloud/access-guide');
    fireEvent.change(screen.getByLabelText('Agent 工具包 API Key'), { target: { value: '  ' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    expect(await screen.findAllByText('请填写 Agent 工具包 API Key')).not.toHaveLength(0);
    expect(request.mock.calls.some(([method]) => method === 'POST')).toBe(false);
  });

  it('keeps the form open when MCP discovery rejects the key', async () => {
    request.mockImplementation(async (method: string) => {
      if (method === 'POST') throw new Error('MCP 连接失败；请检查地址及访问令牌');
      return [];
    });
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 Agent 工具包' }));
    fireEvent.change(await screen.findByLabelText('Agent 工具包 API Key'), { target: { value: ' wrong-key ' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    expect(await screen.findAllByText('MCP 连接失败；请检查地址及访问令牌')).not.toHaveLength(0);
    expect(screen.getByRole('dialog', { name: '连接 Agent 工具包' })).toBeInTheDocument();
    expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ token: 'wrong-key' }));
  });

  it('recognizes saved Toolkit MCP connections and preserves their Bot permission controls', async () => {
    const toolkit = { id: 'toolkit', kind: 'mcp', name: '我的工具包', endpoint: 'https://agent-toolkit.app.baizhi.cloud/mcp/', allowedBots: ['soren'], enabled: true, version: 3, tools: [{ name: 'search' }] };
    const generic = { ...toolkit, id: 'other', name: '其他 MCP', endpoint: 'https://example.com/mcp' };
    request.mockImplementation(async (method: string) => method === 'GET' ? [toolkit, generic] : {});
    render(<ConnectionsPanel />);
    await waitFor(() => expect(screen.getAllByText('1 个已连接账号')).toHaveLength(2));
    fireEvent.click(screen.getByRole('button', { name: '添加 Agent 工具包' }));
    const accounts = await screen.findByRole('region', { name: '已连接账号' });
    expect(within(accounts).getByText('我的工具包')).toBeInTheDocument();
    expect(within(accounts).queryByText('其他 MCP')).not.toBeInTheDocument();
    fireEvent.click(within(accounts).getByRole('button', { name: '权限' }));
    expect(screen.queryByLabelText('Agent 工具包 API Key')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Soren')).toBeChecked();
    fireEvent.click(screen.getByLabelText('Soren'));
    fireEvent.click(screen.getByRole('button', { name: '保存' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('PUT', '/connections/toolkit', expect.objectContaining({ kind: 'mcp', allowedBots: [], version: 3 })));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: '添加 Agent 工具包' }));
    fireEvent.click(within(await screen.findByRole('region', { name: '已连接账号' })).getByRole('button', { name: '断开' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('DELETE', '/connections/toolkit'));
  });

  it('continues to support custom MCP endpoints', async () => {
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByRole('button', { name: '添加 MCP' }));
    fireEvent.change(await screen.findByLabelText('连接名称'), { target: { value: 'My tools' } });
    fireEvent.change(screen.getByLabelText('HTTPS MCP 地址'), { target: { value: 'https://example.com/mcp' } });
    fireEvent.click(screen.getByRole('button', { name: '验证并连接' }));
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ kind: 'mcp', endpoint: 'https://example.com/mcp', name: 'My tools', token: '' })));
  });
});
