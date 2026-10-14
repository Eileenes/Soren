import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { OAuthSettingsForm } from '../components/OAuthSettingsForm';
import { DEFAULT_OAUTH_SETTINGS, type OAuthConfig, type OAuthSettings } from '../services/oauth';

const { invoke } = vi.hoisted(() => ({ invoke: vi.fn() }));
vi.mock('@tauri-apps/api/core', () => ({ invoke }));
const notify = vi.fn();
let stored: OAuthConfig;

beforeEach(() => {
  stored = { settings: { ...DEFAULT_OAUTH_SETTINGS }, configured: false, hasClientSecret: false };
  invoke.mockReset(); notify.mockReset();
  invoke.mockImplementation(async (command: string, args?: { config: { settings: OAuthSettings; clientSecret?: string } }) => {
    if (command === 'get_oauth_config') return stored;
    if (command === 'save_oauth_config' && args) {
      stored = { settings: args.config.settings, configured: true, hasClientSecret: args.config.settings.clientAuthMethod !== 'none' };
      return stored;
    }
    throw new Error('Unexpected command');
  });
});

const change = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label, { exact: false }), { target: { value } });
async function ready() {
  const view = render(<OAuthSettingsForm desktop onNotify={notify} />);
  await waitFor(() => expect(screen.getByRole('button', { name: '保存 OAuth 配置' })).toBeEnabled());
  return view;
}
function savedConfig() {
  stored = { settings: { ...DEFAULT_OAUTH_SETTINGS, clientId: 'registered-client', redirectUri: 'http://localhost:8080/oauth/callback' }, configured: true, hasClientSecret: true };
}

describe('OAuth configuration', () => {
  it('saves via native IPC, clears the secret, and restores public settings on reopening', async () => {
    const browserStorage = vi.spyOn(Storage.prototype, 'setItem');
    const view = await ready();
    expect(screen.getByLabelText('授权地址', { exact: false })).toHaveValue('https://auth.chaitin.net/oauth2/auth');
    change('Client ID', 'registered-client');
    change('Client Secret', 'test-only-secret');
    change('回调地址', 'http://localhost:8080/oauth/callback');
    fireEvent.click(screen.getByRole('button', { name: '保存 OAuth 配置' }));
    await waitFor(() => expect(notify).toHaveBeenCalledWith('success', 'OAuth 配置已保存', expect.any(String)));
    expect(invoke).toHaveBeenCalledWith('save_oauth_config', { config: { settings: expect.objectContaining({ clientId: 'registered-client', clientAuthMethod: 'client_secret_basic' }), clientSecret: 'test-only-secret' } });
    expect(screen.getByLabelText('Client Secret')).toHaveValue('');
    expect(browserStorage).not.toHaveBeenCalled();
    view.unmount();
    render(<OAuthSettingsForm desktop onNotify={notify} />);
    expect(await screen.findByText('配置已保存 · 尚未登录')).toBeInTheDocument();
    expect(screen.getByLabelText('Client ID', { exact: false })).toHaveValue('registered-client');
    expect(screen.getByLabelText('Client Secret')).toHaveValue('');
  });

  it('keeps a saved secret for scope changes but requires a new one for a changed endpoint', async () => {
    savedConfig();
    render(<OAuthSettingsForm desktop onNotify={notify} />);
    await screen.findByText('配置已保存 · 尚未登录');
    change('权限范围', 'openid email');
    fireEvent.click(screen.getByRole('button', { name: '保存 OAuth 配置' }));
    await waitFor(() => expect(notify).toHaveBeenCalled());
    expect(invoke).toHaveBeenLastCalledWith('save_oauth_config', { config: { settings: expect.objectContaining({ scope: 'openid email' }) } });
    invoke.mockClear();
    change('令牌地址', 'https://other.example/token');
    fireEvent.click(screen.getByRole('button', { name: '保存 OAuth 配置' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('请填写 Client Secret');
    expect(invoke).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '撤销更改' }));
    expect(screen.getByLabelText('令牌地址', { exact: false })).toHaveValue(DEFAULT_OAUTH_SETTINGS.tokenEndpoint);
  });

  it('does not submit a stale secret after choosing a public client', async () => {
    const user = userEvent.setup();
    await ready();
    change('Client ID', 'public-client');
    change('回调地址', 'http://localhost:8080/oauth/callback');
    change('Client Secret', 'do-not-send');
    await user.click(screen.getByRole('combobox', { name: '客户端认证方式' }));
    await user.click(screen.getByRole('option', { name: /none/ }));
    expect(screen.queryByLabelText('Client Secret')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '保存 OAuth 配置' }));
    await waitFor(() => expect(notify).toHaveBeenCalled());
    expect(invoke).toHaveBeenLastCalledWith('save_oauth_config', { config: { settings: expect.objectContaining({ clientAuthMethod: 'none' }) } });
  });

  it('keeps unsaved fields and reports server validation failures without claiming success', async () => {
    await ready();
    change('Client ID', 'client'); change('Client Secret', 'secret');
    change('回调地址', 'http://remote.example/callback');
    invoke.mockRejectedValueOnce('回调地址须使用 HTTPS');
    fireEvent.click(screen.getByRole('button', { name: '保存 OAuth 配置' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('回调地址须使用 HTTPS');
    expect(notify).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Client ID', { exact: false })).toHaveValue('client');
    expect(screen.getByRole('button', { name: '保存 OAuth 配置' })).toBeEnabled();
  });

  it('blocks writes after a failed read and supports retry', async () => {
    invoke.mockRejectedValueOnce('无法读取本地 OAuth 配置');
    render(<OAuthSettingsForm desktop onNotify={notify} />);
    expect(await screen.findByRole('alert')).toHaveTextContent('无法读取本地 OAuth 配置');
    expect(screen.getByRole('button', { name: '保存 OAuth 配置' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: '重新读取 OAuth 配置' }));
    await waitFor(() => expect(screen.getByRole('button', { name: '保存 OAuth 配置' })).toBeEnabled());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('shows a read-only preview in the browser without native calls', () => {
    render(<OAuthSettingsForm desktop={false} onNotify={notify} />);
    expect(screen.getByText('请在桌面端保存 OAuth 配置')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '保存 OAuth 配置' })).toBeDisabled();
    expect(screen.getByLabelText('Client ID', { exact: false })).toBeDisabled();
    expect(invoke).not.toHaveBeenCalled();
  });
});
