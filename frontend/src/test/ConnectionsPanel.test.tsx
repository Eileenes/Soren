import { describe, expect, it, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { keyKinds } from '../components/connectorKinds';

const request = vi.fn();
vi.mock('../services/connections', () => ({ connectionRequest: (...a: unknown[]) => request(...a), openExternal: vi.fn() }));
vi.mock('../services/agents', () => ({ agentsAPI: { list: async () => [{ id: 'soren', name: 'Soren', avatar: 'a', status: 'active' }] }, avatarURL: () => '' }));

import { ConnectionsPanel } from '../components/ConnectionsPanel';

describe('ConnectionsPanel API-key connectors', () => {
  beforeEach(() => { request.mockReset(); request.mockImplementation(async (method: string) => method === 'GET' ? [] : {}); });

  it('lists every key connector as a card', async () => {
    render(<ConnectionsPanel />);
    for (const k of Object.values(keyKinds)) expect(await screen.findByText(k.title)).toBeTruthy();
  });

  it.each(Object.entries(keyKinds))('saves %s with the entered key', async (id, k) => {
    render(<ConnectionsPanel />);
    fireEvent.click(await screen.findByText(k.title));
    fireEvent.change(await screen.findByLabelText(`${k.title} API Key`), { target: { value: 'secret-key' } });
    fireEvent.click(await screen.findByLabelText('Soren'));
    fireEvent.click(screen.getByText('验证并连接'));
    await waitFor(() => expect(request).toHaveBeenCalledWith('POST', '/connections', expect.objectContaining({ kind: id, token: 'secret-key', allowedBots: ['soren'] })));
  });
});
