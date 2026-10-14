import { useEffect, useState, type FormEvent } from 'react';
import { Button, PasswordField, SelectField, TextField } from './ui';
import { IconKey } from './icons';
import {
  canKeepOAuthSecret, DEFAULT_OAUTH_SETTINGS, oauthConfigAPI,
  type ClientAuthMethod, type OAuthConfig, type OAuthEnvironment, type OAuthSettings,
} from '../services/oauth';

interface Props {
  desktop: boolean;
  onNotify: (type: 'success' | 'error', title: string, description?: string) => void;
}

const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);

export function OAuthSettingsForm({ desktop, onNotify }: Props) {
  const [saved, setSaved] = useState<OAuthConfig | null>(null);
  const [settings, setSettings] = useState<OAuthSettings>({ ...DEFAULT_OAUTH_SETTINGS });
  const [secret, setSecret] = useState('');
  const [loading, setLoading] = useState(desktop);
  const [saving, setSaving] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [saveError, setSaveError] = useState('');
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    if (!desktop) return;
    let active = true;
    setLoading(true);
    setLoadError('');
    void oauthConfigAPI.get().then(config => {
      if (!active) return;
      setSaved(config);
      setSettings(config.settings);
      setSecret('');
      setSaveError('');
    }).catch(error => {
      if (active) setLoadError(errorText(error));
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [desktop, revision]);

  const disabled = !desktop || loading || saving || Boolean(loadError);
  const canKeepSecret = canKeepOAuthSecret(saved, settings);
  const dirty = Boolean(saved && (JSON.stringify(settings) !== JSON.stringify(saved.settings) || secret));
  const patch = <K extends keyof OAuthSettings>(key: K, value: OAuthSettings[K]) => {
    setSettings(current => ({ ...current, [key]: value }));
    setSaveError('');
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (disabled) return;
    if (settings.clientAuthMethod !== 'none' && !secret && !canKeepSecret) {
      setSaveError('请填写 Client Secret；更换环境、Client ID、接口地址或认证方式后需要重新填写。');
      return;
    }
    setSaving(true);
    setSaveError('');
    try {
      const config = await oauthConfigAPI.save(settings, settings.clientAuthMethod === 'none' ? undefined : secret || undefined);
      setSaved(config);
      setSettings(config.settings);
      setSecret('');
      onNotify('success', 'OAuth 配置已保存', '后续登录将使用这组配置；当前尚未发起授权。');
    } catch (error) {
      setSaveError(errorText(error));
    } finally { setSaving(false); }
  };

  return <form onSubmit={event => void save(event)} aria-label="OAuth 登录配置" className="space-y-5">
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-surface-3 px-4 py-3 text-sm" role="status" aria-live="polite">
      <span className="flex items-center gap-2 text-ink-2"><IconKey size={16} />
        {!desktop ? '请在桌面端保存 OAuth 配置' : loading ? '正在读取 OAuth 配置…' : loadError ? '配置读取失败' : dirty ? '有未保存的更改' : saved?.configured ? '配置已保存 · 尚未登录' : '待配置 · 内部 OAuth2 IdP'}
      </span>
      <span className="text-xs text-ink-3">OAuth 2.0 / OpenID Connect</span>
    </div>

    {loadError && <div className="flex flex-wrap items-center gap-3">
      <p role="alert" className="text-sm text-error">{loadError}</p>
      <Button size="sm" type="button" onClick={() => setRevision(value => value + 1)}>重新读取 OAuth 配置</Button>
    </div>}

    <fieldset disabled={disabled} className="min-w-0 space-y-5">
      <div className="grid gap-5 lg:grid-cols-2">
        <SelectField<OAuthEnvironment> label="使用环境" value={settings.environment} disabled={disabled} onChange={value => { patch('environment', value); setSecret(''); }} options={[
          { value: 'development', label: '本地开发', hint: '允许本机 HTTP 回调' },
          { value: 'test', label: '测试环境', hint: '使用测试客户端与 HTTPS' },
          { value: 'production', label: '生产环境', hint: '使用生产客户端与 HTTPS' },
        ]} helper="当前保存一组配置；不同环境请使用分别申请的客户端。" />
        <SelectField<ClientAuthMethod> label="客户端认证方式" value={settings.clientAuthMethod} disabled={disabled} onChange={value => { patch('clientAuthMethod', value); setSecret(''); }} options={[
          { value: 'client_secret_basic', label: 'client_secret_basic', hint: '文档推荐 · HTTP Basic 认证' },
          { value: 'client_secret_post', label: 'client_secret_post', hint: '在令牌请求体中发送密钥' },
          { value: 'none', label: 'none', hint: '公开客户端 · 无密钥' },
        ]} helper="必须与 IdP 中注册的客户端认证方式一致。" />
      </div>
      <div className="grid gap-5 lg:grid-cols-2">
        <TextField label="Client ID" value={settings.clientId} required maxLength={512} autoComplete="off" spellCheck={false}
          placeholder="填写申请获得的客户端 ID" onChange={event => { patch('clientId', event.target.value); setSecret(''); }} />
        {settings.clientAuthMethod === 'none' ? <div className="rounded-xl border border-line px-4 py-3 text-xs leading-6 text-ink-2">
          公开客户端不使用 Client Secret。保存后会移除此前的密钥；请确认 IdP 已将该客户端注册为 none。
        </div> : <PasswordField label="Client Secret" value={secret} maxLength={8192} disabled={disabled} autoComplete="new-password"
          placeholder={canKeepSecret ? '已保存 · 留空保留当前密钥' : '填写客户端密钥'} onChange={event => { setSecret(event.target.value); setSaveError(''); }}
          helper={canKeepSecret ? '填写新值可替换密钥，已保存的密钥不会回显。' : '仅存入本机私有配置，不写入浏览器缓存。'} />}
      </div>
      <div className="grid gap-5 lg:grid-cols-2">
        <TextField label="回调地址（Redirect URI）" value={settings.redirectUri} type="url" required maxLength={2048} spellCheck={false}
          placeholder="填写已在 IdP 注册的回调地址" onChange={event => patch('redirectUri', event.target.value)}
          helper="须与申请时完全一致，包括协议、域名、端口、路径和查询参数。" />
        <TextField label="权限范围（Scope）" value={settings.scope} required maxLength={1024} spellCheck={false}
          placeholder="openid profile email" onChange={event => patch('scope', event.target.value)} helper="用空格分隔，必须包含 openid；profile 和 email 按需申请。" />
      </div>
      <div className="space-y-4 border-t border-line pt-5">
        <div><h3 className="text-sm font-medium text-ink-1">身份服务地址</h3><p className="mt-1 text-xs leading-5 text-ink-3">已预填接入文档中的内部 IdP，可按实际环境修改。</p></div>
        <TextField label="授权地址（Authorization Endpoint）" type="url" required maxLength={2048} spellCheck={false} value={settings.authorizationEndpoint}
          onChange={event => { patch('authorizationEndpoint', event.target.value); setSecret(''); }} />
        <div className="grid gap-5 lg:grid-cols-2">
          <TextField label="令牌地址（Token Endpoint）" type="url" required maxLength={2048} spellCheck={false} value={settings.tokenEndpoint}
            onChange={event => { patch('tokenEndpoint', event.target.value); setSecret(''); }} />
          <TextField label="用户信息地址（UserInfo Endpoint）" type="url" required maxLength={2048} spellCheck={false} value={settings.userInfoEndpoint}
            onChange={event => { patch('userInfoEndpoint', event.target.value); setSecret(''); }} />
        </div>
      </div>
    </fieldset>

    {saveError && <p role="alert" className="rounded-lg border border-error/20 bg-error/5 p-3 text-sm leading-6 text-error">{saveError}</p>}
    <div className="flex flex-wrap items-center gap-3">
      <Button type="submit" variant="primary" loading={saving} disabled={disabled || (!dirty && Boolean(saved?.configured))}>保存 OAuth 配置</Button>
      {dirty && <Button type="button" variant="ghost" disabled={disabled} onClick={() => {
        if (saved) setSettings({ ...saved.settings });
        setSecret(''); setSaveError('');
      }}>撤销更改</Button>}
      <span className="text-xs text-ink-3">保存时校验配置格式，不验证登录凭据。</span>
    </div>
    <p className="text-xs leading-6 text-ink-3">此处为后续统一登录准备配置。授权回调、登录状态与退出登录尚未接入，当前仍可直接使用本地空间。</p>
  </form>;
}
