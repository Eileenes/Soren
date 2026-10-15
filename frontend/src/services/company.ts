import axios from 'axios';
import { invoke } from '@tauri-apps/api/core';
import { DEFAULT_API_BASE_URL } from './api';

export interface CompanyConnection {
  active: boolean;
  serverUrl: string;
  organization: string;
  workspaceId: string;
  userId: string;
  userName: string;
}
export interface WorkspaceStatus {
  mode: 'personal' | 'enterprise';
  connection?: CompanyConnection;
  needsLogin?: boolean;
  error?: string;
}
export interface CompanyDiscovery { product: 'soren'; protocolVersion: 1; name: string; serverUrl: string; auth: 'oauth-browser' }
export interface CompanyLogin { id: string; url: string; organization: string; expiresAt: string }

export class CompanyRequestError extends Error {
  constructor(message: string, readonly status?: number) { super(message); }
}

// Always address the local owner bridge, even while company business APIs are selected.
async function request<T>(method: string, path: string, data?: unknown): Promise<T> {
  try {
    return (await axios.request<T>({ method, url: DEFAULT_API_BASE_URL + '/api/enterprise/' + path, data, timeout: 25000 })).data;
  } catch (error) {
    if (axios.isAxiosError(error) && typeof error.response?.data?.error === 'string') throw new CompanyRequestError(error.response.data.error, error.response.status);
    throw new CompanyRequestError('无法连接本地服务，请稍后重试。');
  }
}
export const companyAPI = {
  status: () => request<WorkspaceStatus>('GET', 'status'),
  discover: (serverUrl: string) => request<CompanyDiscovery>('POST', 'discover', { serverUrl }),
  login: (serverUrl: string) => request<CompanyLogin>('POST', 'login', { serverUrl }),
  poll: (id: string) => request<{ status: 'pending' } | { status: 'authorized'; connection: CompanyConnection }>('POST', 'poll', { id }),
  complete: (id: string) => request<WorkspaceStatus>('POST', 'complete', { id }),
  cancel: (id: string) => request<void>('POST', 'cancel', { id }),
  disconnect: () => request<{ revoked: boolean; credentialDeleted: boolean }>('POST', 'disconnect', {}),
};

export async function openCompanyLogin(login: CompanyLogin) {
  if ('__TAURI_INTERNALS__' in window) { await invoke('open_company_login'); return; }
  const url = new URL(login.url);
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.pathname !== '/auth/browser') throw new Error('登录地址无效');
  window.open(url.href, '_blank', 'noopener,noreferrer');
}
