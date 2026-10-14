import { MobileDevices } from '../components/MobileDevices';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { invoke } from '@tauri-apps/api/core';
import { cn, formatDateTime } from '../utils/format';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { IconCheck, IconDatabase, IconKey, IconMoon, IconRefresh, IconSettings, IconSun } from '../components/icons';
import { Button, PasswordField, SelectField, Switch, TextField } from '../components/ui';
import type { ConnectionState, HealthStatus, LanguageCode, LLMProviderId, ThemeMode } from '../types';
import { getApiBaseUrl, setApiBaseUrl } from '../services/api';

interface DesktopConfig {
  provider: 'openai' | 'anthropic';
  baseURL: string;
  configured: boolean;
  model: string;
  dataDir: string;
  backendReady: boolean;
  startupError: string | null;
}

const PROVIDERS = [
  { id: 'openai' as const, label: 'OpenAI', hint: 'GPT 系列模型', baseURL: 'https://api.openai.com/v1' },
  { id: 'anthropic' as const, label: 'Anthropic', hint: 'Claude 系列模型', baseURL: 'https://api.anthropic.com' },
  { id: 'custom' as const, label: '兼容接口', hint: '接入你习惯的模型', baseURL: '' },
];
const normalizeURL = (url: string) => url.trim().replace(/\/+$/, '');
const errorText = (error: unknown) => error instanceof Error ? error.message : String(error);

export interface SettingsPageProps {
  /** 后端连接状态 */
  connection: ConnectionState;
  health: HealthStatus | null;
  healthError: string | null;
  lastCheckedAt: number | null;
  onRefreshHealth: () => Promise<void>;
  /** 主题 */
  theme: ThemeMode;
  onSetTheme: (theme: ThemeMode) => void;
  /** 语言 */
  language: LanguageCode;
  onSetLanguage: (language: LanguageCode) => void;
  /** 记忆徽标开关 */
  showMemoryBadge: boolean;
  onSetShowMemoryBadge: (value: boolean) => void;
  /** LLM 设置 */
  llm: {
    provider: LLMProviderId;
    apiKey: string;
    baseUrl: string;
    model: string;
    temperature: number;
    maxTokens: number;
  };
  onSetLLM: (patch: Partial<SettingsPageProps['llm']>) => void;
  /** 通知 */
  onNotify: (type: 'success' | 'error', title: string, description?: string) => void;
}

function SettingsSection({ title, description, number, children }: {
  title: string; description?: string; number: string; children: ReactNode;
}) {
  return (
    <section className="settings-section rounded-2xl border border-line bg-surface-2 p-5 sm:p-7">
      <header className="mb-6 flex items-start gap-3">
        <span className="pt-0.5 font-mono text-xs text-ink-3">{number}</span>
        <div><h2 className="text-base font-semibold text-ink-1">{title}</h2>{description && <p className="mt-1 text-sm leading-6 text-ink-2">{description}</p>}</div>
      </header>
      {children}
    </section>
  );
}

export function SettingsPage({ connection, health, healthError, lastCheckedAt, onRefreshHealth,
  theme, onSetTheme, language, onSetLanguage, showMemoryBadge, onSetShowMemoryBadge,
  llm, onSetLLM, onNotify }: SettingsPageProps) {
  const [desktop] = useState(() => typeof window !== 'undefined' && '__TAURI_INTERNALS__' in window);
  const [desktopConfig, setDesktopConfig] = useState<DesktopConfig | null>(null);
  const [loadingConfig, setLoadingConfig] = useState(desktop);
  const [configError, setConfigError] = useState<string | null>(null);
  const [provider, setProvider] = useState<LLMProviderId>(llm.provider);
  const [endpoint, setEndpoint] = useState(llm.baseUrl || 'https://api.openai.com/v1');
  // Credentials never enter the persisted application settings or browser storage.
  const [apiKey, setApiKey] = useState('');
  const [model, setModel] = useState(llm.model);
  const [models, setModels] = useState<{id: string; name: string}[]>([]);
  const [modelError, setModelError] = useState('');
  const [modelsLoading, setModelsLoading] = useState(false);
  const [manualModel, setManualModel] = useState(false);
  const modelRequest = useRef(0);
  useEffect(() => {
    modelRequest.current++; setModels([]); setModelError(''); setModelsLoading(false);
    return () => { modelRequest.current++; };
  }, [provider, endpoint, apiKey]);
  const [saving, setSaving] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [serviceURL, setServiceURL] = useState(getApiBaseUrl);

  useEffect(() => {
    if (!desktop) return;
    let active = true;
    void invoke<DesktopConfig>('get_desktop_config').then((config) => {
      if (!active) return;
      setDesktopConfig(config);
      setProvider(config.provider === 'openai' && normalizeURL(config.baseURL) !== 'https://api.openai.com/v1' ? 'custom' : config.provider);
      setEndpoint(config.baseURL);
      if (config.model) { setModel(config.model); onSetLLM({model: config.model}); }
    }).catch((error: unknown) => {
      if (active) setConfigError(errorText(error));
    }).finally(() => { if (active) setLoadingConfig(false); });
    return () => { active = false; };
  }, [desktop]);

  const effectiveProvider = provider === 'anthropic' ? 'anthropic' : 'openai';
  const canKeepKey = Boolean(desktopConfig?.configured && desktopConfig.provider === effectiveProvider && normalizeURL(desktopConfig.baseURL) === normalizeURL(endpoint));

  const fetchModels = async () => {
    const request = ++modelRequest.current;
    setModelsLoading(true); setModelError('');
    try {
      const rows = await invoke<{id: string; name: string}[]>('list_provider_models', {
        config: {provider: effectiveProvider, baseURL: normalizeURL(endpoint), ...(apiKey.trim() ? {apiKey: apiKey.trim()} : {})},
      });
      if (request !== modelRequest.current) return;
      setModels(rows); setManualModel(false);
    } catch (error) { if (request === modelRequest.current) setModelError(errorText(error)); }
    finally { if (request === modelRequest.current) setModelsLoading(false); }
  };

  const refresh = async () => {
    setRefreshing(true);
    try {
      await Promise.all([
        onRefreshHealth(),
        desktop ? invoke<DesktopConfig>('get_desktop_config').then((config) => { setDesktopConfig(config); setConfigError(null); }) : Promise.resolve(),
      ]);
    } catch (error) { onNotify('error', '暂时无法刷新状态', errorText(error)); }
    finally { setRefreshing(false); }
  };

  const save = async () => {
    if (!desktop || saving) return;
    const baseURL = normalizeURL(endpoint);
    try {
      const url = new URL(baseURL);
      if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error();
    } catch { onNotify('error', '请输入有效的模型接口地址', '使用 http 或 https 地址，不包含账号、密码或查询参数。'); return; }
    if (!apiKey.trim() && !canKeepKey) {
      onNotify('error', '请填写 API Key', '首次连接或更换接口地址时，需要填写对应的密钥。');
      return;
    }
    if (!model.trim()) { onNotify('error', '请选择或填写模型名称'); return; }
    setSaving(true);
    setConfigError(null);
    try {
      const config = await invoke<DesktopConfig>('save_desktop_config', {
        config: { provider: effectiveProvider, baseURL, model: model.trim(), ...(apiKey.trim() ? { apiKey: apiKey.trim() } : {}) },
      });
      setDesktopConfig(config);
      setEndpoint(config.baseURL);
      setApiKey('');
      onSetLLM({ provider, baseUrl: config.baseURL, model: model.trim(), apiKey: '' });
      onNotify(config.backendReady ? 'success' : 'error', config.backendReady ? '模型配置已保存' : '配置已保存，服务尚未就绪', config.backendReady ? '本地服务已重新启动，后续对话和新任务将使用此连接与模型。' : config.startupError ?? '请稍后重新检测本地服务。');
      await onRefreshHealth().catch(() => { /* The health hook presents connection errors separately. */ });
    } catch (error) {
      const detail = errorText(error);
      setConfigError(detail);
      onNotify('error', '未能完成模型切换', detail);
      // A restart can fail after the private configuration has already been saved.
      try { setDesktopConfig(await invoke<DesktopConfig>('get_desktop_config')); } catch { /* Retain the last known state. */ }
    } finally { setSaving(false); }
  };

  const openData = async () => {
    try { await invoke('open_data_directory'); }
    catch (error) { onNotify('error', '无法打开数据文件夹', errorText(error)); }
  };

  const applyServiceURL = () => {
    try {
      const url = new URL(serviceURL);
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash) throw new Error();
      setApiBaseUrl(serviceURL);
      setServiceURL(getApiBaseUrl());
      void refresh();
    } catch { onNotify('error', '请输入有效的服务地址'); }
  };

  return (
    <PageContainer widthClass="max-w-5xl">
      <PageHeading icon={<IconSettings size={19} />} title="设置" />
      <div className="space-y-5 pb-8">
        <SettingsSection number="01" title="模型">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl bg-surface-3 px-4 py-3 text-sm">
            <span className="flex items-center gap-2 text-ink-2"><IconKey size={16} />{loadingConfig ? '正在读取本地配置…' : desktopConfig?.configured ? '已保存模型凭据' : desktop ? '添加 API Key，开启第一段对话' : '浏览器预览模式'}</span>
            <span className="text-xs text-ink-3">{desktop ? '凭据仅保存在本机' : '请在桌面端配置模型连接'}</span>
          </div>
          <div className="mb-5 grid grid-cols-1 gap-3 sm:grid-cols-3" aria-label="模型服务商">
            {PROVIDERS.map((item) => (
              <button key={item.id} type="button" aria-pressed={provider === item.id} disabled={!desktop || loadingConfig || saving}
                className={cn('rounded-xl border p-4 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-60', provider === item.id ? 'border-primary-start bg-primary-start/5' : 'border-line hover:bg-surface-3')}
                onClick={() => { setProvider(item.id); setEndpoint(item.baseURL); setApiKey(''); if (item.id === 'anthropic') setModel('claude-sonnet-4-5'); else if (item.id === 'openai') setModel('gpt-4o-mini'); }}>
                <span className="flex items-center justify-between text-sm font-semibold text-ink-1">{item.label}{provider === item.id && <IconCheck size={15} />}</span>
                <span className="mt-1 block text-xs text-ink-2">{item.hint}</span>
              </button>
            ))}
          </div>
          <div className="grid gap-5 sm:grid-cols-2">
            <TextField label="接口地址" placeholder="https://api.openai.com/v1" value={endpoint} disabled={!desktop || loadingConfig || saving} onChange={(event) => setEndpoint(event.target.value)} helper="使用你的服务商提供的模型接口地址。" />
            <PasswordField label="API Key" placeholder={canKeepKey ? '已配置 · 留空保留当前密钥' : '输入服务商提供的密钥'} value={apiKey} disabled={!desktop || loadingConfig || saving} onChange={(event) => setApiKey(event.target.value)} helper={canKeepKey ? '当前接口已有密钥，填写可替换。' : '密钥不会写入浏览器缓存或对话内容。'} />
          </div>
          {configError && <p role="alert" className="mt-4 break-words rounded-lg border border-error/20 bg-error/5 p-3 text-sm text-error">{configError}</p>}
          <div className="mt-6 border-t border-line pt-5">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3"><span className="text-sm font-medium text-ink-1">选择模型</span><Button size="sm" loading={modelsLoading} disabled={!desktop || loadingConfig || saving || !endpoint.trim() || (!apiKey.trim() && !canKeepKey)} onClick={() => void fetchModels()} leadingIcon={<IconRefresh size={14} />}>获取模型</Button></div>
            {models.length > 0 && !manualModel ? <SelectField disabled={saving} label="模型" value={model} onChange={setModel} options={[...(!models.some(m => m.id === model) && model ? [{value: model, label: model, hint: '当前配置 · 不在返回列表中'}] : []), ...models.map(m => ({value: m.id, label: m.name, hint: m.name === m.id ? undefined : m.id}))]} helper="选择支持对话和工具调用的模型，保存后生效。" /> : <TextField disabled={saving} label="模型名称" placeholder="服务商提供的模型 ID" value={model} onChange={event => setModel(event.target.value)} helper="可以获取列表后选择，也可以手动填写。" />}
            {models.length > 0 && <button className="mt-3 text-xs text-ink-2 underline underline-offset-4" onClick={() => setManualModel(value => !value)}>{manualModel ? '从列表选择' : '手动填写'}</button>}
            {modelError && <p role="alert" className="mt-3 text-xs leading-6 text-error">{modelError}</p>}
          <div className="mt-5 flex flex-wrap items-center gap-3">
            <Button variant="primary" loading={saving} disabled={!desktop || loadingConfig || !desktopConfig} onClick={() => void save()}>保存模型连接</Button>
            {configError && <Button variant="ghost" loading={refreshing} onClick={() => void refresh()}>重新读取</Button>}
            <span className="text-xs text-ink-3">{desktop ? '保存时会重启本地服务。' : '请在桌面端保存模型连接。'}</span>
          </div>


            <details className="mt-5 text-sm text-ink-2">
              <summary className="cursor-pointer py-1 font-medium">回复偏好</summary>
              <div className="mt-4 grid gap-5 sm:grid-cols-2">
                <div><label htmlFor="soren-temperature" className="flex justify-between text-sm text-ink-1"><span>创意程度</span><span>{llm.temperature.toFixed(1)}</span></label>
                  <input id="soren-temperature" type="range" min="0" max={provider === 'anthropic' ? 1 : 2} step="0.1" value={llm.temperature} onChange={(event) => onSetLLM({ temperature: Number(event.target.value) })} className="mt-4 w-full accent-primary-start" />

                </div>
                <TextField label="回复长度上限" type="number" min="1" max="128000" value={llm.maxTokens} onChange={(event) => { const value = Number(event.target.value); if (Number.isFinite(value)) onSetLLM({ maxTokens: Math.min(128000, Math.max(1, Math.round(value))) }); }} helper="以 Token 计量，实际限制取决于模型。" />
              </div>
            </details>
          </div>
        </SettingsSection>

        <SettingsSection number="02" title="外观">
          <div className="grid grid-cols-3 gap-3 sm:gap-5">
            {(['light', 'dark', 'system'] as const).map((mode) => (
              <button key={mode} type="button" aria-label={mode === 'system' ? '跟随系统' : mode === 'light' ? '浅色主题' : '深色主题'} aria-pressed={theme === mode} onClick={() => onSetTheme(mode)} className={cn('theme-option overflow-hidden rounded-xl border p-2 text-left transition-colors', theme === mode ? 'border-primary-start' : 'border-line hover:border-line-strong')}>
                <div className="flex h-28 overflow-hidden rounded-lg sm:h-36" style={{ background: mode === 'system' ? 'linear-gradient(115deg, #ffffff 50%, #121212 50%)' : mode === 'light' ? '#ffffff' : '#121212' }} aria-hidden="true">
                  <div className="w-1/4 p-3" style={{ background: mode === 'light' ? '#f5f5f5' : '#1c1c1c' }}><div className="h-3 w-3 rounded-full bg-[#737373]" /><div className="mt-4 h-1 rounded bg-[#8a8a8a]/40" /><div className="mt-2 h-1 rounded bg-[#8a8a8a]/30" /></div>
                  <div className="flex flex-1 flex-col items-center justify-center gap-3 p-3"><img src="/soren-mark.svg" alt="" className="h-10 w-10" /><div className="h-3 w-3/4 rounded-full" style={{ background: mode === 'light' ? '#e5e5e5' : '#333333' }} /><div className="h-1 w-1/2 rounded bg-[#8a8a8a]/30" /></div>
                </div>
                <span className="flex items-center justify-between px-2 pb-1 pt-3 text-sm font-medium text-ink-1"><span className="flex items-center gap-2">{mode === 'light' ? <IconSun size={16} /> : <IconMoon size={16} />}{mode === 'system' ? '跟随系统' : mode === 'light' ? '浅色' : '深色'}</span>{theme === mode && <IconCheck size={15} />}</span>
              </button>
            ))}
          </div>
          <div className="mt-6 flex items-center justify-between gap-4 border-t border-line pt-5"><div><p className="text-sm font-medium text-ink-1">记忆提示</p><p className="mt-1 text-xs text-ink-3">在设置的「我的记忆」入口显示已保存的记忆数量。</p></div><Switch label="显示记忆统计" checked={showMemoryBadge} onChange={onSetShowMemoryBadge} /></div>
          <details className="mt-5 text-sm text-ink-2"><summary className="cursor-pointer py-1">语言偏好</summary><div className="mt-3 max-w-xs"><SelectField label="偏好语言" value={language} onChange={onSetLanguage} options={[{ value: 'zh-CN', label: '简体中文' }, { value: 'en-US', label: 'English' }]} helper="当前界面以中文呈现；完整英文界面正在规划中。" /></div></details>
        </SettingsSection>

        <SettingsSection number="03" title="移动端与同步"><MobileDevices /></SettingsSection>
        <SettingsSection number="04" title="数据">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex items-center gap-3"><span className="rounded-xl bg-primary-start/10 p-3 text-primary-start"><IconDatabase size={21} /></span><div><p className="text-sm font-medium text-ink-1">本地数据空间 · 无需登录</p><p className="mt-1 break-all text-xs text-ink-2">{desktopConfig?.dataDir ?? (desktop ? '正在读取存储位置…' : '在桌面应用中管理本地文件')}</p></div></div>
            <Button disabled={!desktop} onClick={() => void openData()}>打开数据文件夹</Button>
          </div>
          <p className="mt-4 text-xs leading-6 text-ink-3">对话时，相关内容会发送至你配置的模型服务。当前记忆状态：{health?.memoryEnabled === undefined ? '等待连接' : health.memoryEnabled ? '已开启' : '未开启'}。</p>
        </SettingsSection>

        <details className="rounded-xl border border-line px-5 py-4 text-sm text-ink-2">
          <summary className="cursor-pointer"><span className="ml-1">本地服务</span><span className={cn('ml-3 text-xs', connection === 'online' ? 'text-primary-start' : 'text-ink-3')}>{connection === 'online' ? '运行正常' : connection === 'connecting' ? '正在连接' : '等待连接'}</span></summary>
          <div className="mt-4 space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-2"><p className="text-xs text-ink-3">最近检测：{lastCheckedAt ? formatDateTime(new Date(lastCheckedAt).toISOString()) : '尚未检测'}</p><Button size="sm" variant="ghost" loading={refreshing} leadingIcon={<IconRefresh size={14} />} onClick={() => void refresh()}>重新检测</Button></div>
            {(healthError || desktopConfig?.startupError) && <p className="break-words text-xs leading-6 text-error">{desktopConfig?.startupError ?? healthError}</p>}
            <TextField label="后端服务地址" value={serviceURL} onChange={(event) => setServiceURL(event.target.value)} helper="桌面端默认为 http://localhost:8080。" />
            <Button size="sm" onClick={applyServiceURL}>应用服务地址</Button>
          </div>
        </details>
      </div>
    </PageContainer>
  );
}
