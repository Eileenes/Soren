import axios from 'axios';
import { invoke } from '@tauri-apps/api/core';
import { getApiBaseUrl, normalizeError } from './api';
import { getActiveBotID } from './botScope';
import { keyKindHosts, type KeyKindId } from '../components/connectorKinds';

export interface Connection { id: string; kind: 'gmail' | 'notion' | 'mcp' | KeyKindId; name: string; account: string; endpoint?: string; allowedBots: string[]; enabled: boolean; version: number; tools: {name: string}[]; }
export interface Identity { githubId: number; login: string; name: string; boundAt: string }
export interface DeviceLogin { id: string; userCode: string; url: string; interval: number }
export interface Invocation { connectionName: string; account: string; id: string; runId: string; botId: string; tool: string; arguments: string; status: string; result: string; createdAt: string }
export interface Artifact { id: string; title: string; content: string; botId: string }
export async function connectionRequest<T>(method: string, path: string, data?: unknown): Promise<T> {
  try { return (await axios.request<T>({method, url: getApiBaseUrl() + '/api' + path, data, headers: {'X-Bot-ID': getActiveBotID()}, timeout: 55000})).data; }
  catch (e) { throw normalizeError(e); }
}
export async function openExternal(url: string) {
  const parsed = new URL(url);
  if (parsed.protocol !== 'https:' || !['github.com', 'accounts.google.com', 'developers.notion.com', 'console.cloud.google.com', 'testflight.apple.com', 'apps.apple.com', ...keyKindHosts].includes(parsed.hostname) || parsed.username || parsed.password || parsed.port) throw new Error('不支持的外部地址');
  if ('__TAURI_INTERNALS__' in window) await invoke('open_external', {url});
  else window.open(url, '_blank', 'noopener,noreferrer');
}
