import { invoke } from '@tauri-apps/api/core';
import { http, normalizeError } from './api';
import { getActiveBotID } from './botScope';
import { keyKindHosts, type KeyKindId } from '../components/connectorKinds';

export interface Connection { id: string; kind: 'gmail' | 'notion' | 'mcp' | KeyKindId; name: string; account: string; endpoint?: string; enabled: boolean; version: number; tools: {name: string}[]; }
export interface Identity { githubId: number; login: string; name: string; boundAt: string }
export interface DeviceLogin { id: string; userCode: string; url: string; interval: number }
export interface Invocation { connectionName: string; account: string; id: string; runId: string; botId: string; tool: string; arguments: string; status: string; result: string; createdAt: string }
export interface Artifact { id: string; title: string; content: string; botId: string }
export async function connectionRequest<T>(method: string, path: string, data?: unknown, botId = getActiveBotID()): Promise<T> {
  try { return (await http.request<T>({method, url: '/api' + path, data, headers: {'X-Bot-ID': botId}, timeout: 55000})).data; }
  catch (e) { throw normalizeError(e); }
}
export async function openExternal(url: string) {
  const parsed = new URL(url);
  if (parsed.protocol !== 'https:' || !['github.com', 'accounts.google.com', 'developers.notion.com', 'console.cloud.google.com', 'testflight.apple.com', 'apps.apple.com', ...keyKindHosts].includes(parsed.hostname) || parsed.username || parsed.password || parsed.port) throw new Error('不支持的外部地址');
  if ('__TAURI_INTERNALS__' in window) await invoke('open_external', {url});
  else window.open(url, '_blank', 'noopener,noreferrer');
}

export interface FileArtifact { id: string; runId: string; botId: string; name: string; size: number; }
export async function downloadAttachment(file: FileArtifact, botId: string): Promise<void> {
  if ('__TAURI_INTERNALS__' in window) {
    await invoke('export_attachment', { runId: file.runId, fileId: file.id, botId, name: file.name, workspace: String(http.defaults.headers.common['X-Soren-Workspace'] || '') });
    return;
  }
  const response = await http.get<Blob>(`/api/runs/${file.runId}/files/${file.id}`, { headers: { 'X-Bot-ID': botId }, responseType: 'blob', timeout: 180000 });
  const url = URL.createObjectURL(response.data);
  const link = document.createElement('a'); link.href = url; link.download = file.name; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
