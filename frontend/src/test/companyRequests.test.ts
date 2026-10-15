import { afterEach, describe, expect, it } from 'vitest';
import { AxiosError, type AxiosResponse } from 'axios';
import { DEFAULT_API_BASE_URL, chatAPI, http, setApiBaseUrl } from '../services/api';
import { agentsAPI } from '../services/agents';
import { connectionRequest } from '../services/connections';
import { templatesAPI } from '../services/templates';

const originalAdapter = http.defaults.adapter;
afterEach(() => { http.defaults.adapter = originalAdapter; setApiBaseUrl(DEFAULT_API_BASE_URL); });
describe('企业 API 请求边界', () => {
  it('聊天、Bot、连接器与模板统一使用当前空间代理及身份约束', async () => {
    const requests: string[] = [];
    setApiBaseUrl(DEFAULT_API_BASE_URL + '/enterprise', 'company:alice');
    http.defaults.adapter = async config => {
      expect(config.baseURL).toBe(DEFAULT_API_BASE_URL + '/enterprise');
      expect(config.headers.get('X-Soren-Workspace')).toBe('company:alice');
      expect(config.headers.get('Authorization')).toBeUndefined();
      requests.push(config.url!);
      return { data: [], config, headers: {}, status: 200, statusText: 'OK' };
    };
    await Promise.all([chatAPI.getConversations(), agentsAPI.list(), connectionRequest('GET', '/connections'), templatesAPI.list()]);
    expect(requests.sort()).toEqual(['/api/bot-templates', '/api/bots', '/api/connections', '/api/conversations']);
  });
  it('切换账号后丢弃旧请求的响应，即使调用者自己传入 signal', async () => {
    setApiBaseUrl(DEFAULT_API_BASE_URL + '/enterprise', 'company:alice');
    let finish!: () => void;
    http.defaults.adapter = config => new Promise<AxiosResponse>(resolve => {
      finish = () => resolve({ data: 'Alice private data', config, headers: {}, status: 200, statusText: 'OK' });
    });
    const pending = http.get('/api/conversations', { signal: new AbortController().signal });
    const rejected = expect(pending).rejects.toMatchObject({ status: 409 });
    setApiBaseUrl(DEFAULT_API_BASE_URL + '/enterprise', 'company:bob');
    finish();
    await rejected;
  });
  it.each(['company:bob', 'company:alice'])('旧会话请求的 401 不会使新会话退出（%s）', async nextWorkspace => {
    setApiBaseUrl(DEFAULT_API_BASE_URL + '/enterprise', 'company:alice');
    let fail!: () => void;
    http.defaults.adapter = config => new Promise((_resolve, reject) => {
      fail = () => reject(new AxiosError('expired', 'ERR_BAD_REQUEST', config, {}, { data: {}, config, headers: {}, status: 401, statusText: 'Unauthorized' }));
    });
    let expired = false;
    const onExpired = () => { expired = true; };
    window.addEventListener('soren:company-session-expired', onExpired);
    try {
      const pending = http.get('/api/bots', { signal: new AbortController().signal });
      const rejected = expect(pending).rejects.toMatchObject({ status: 401 });
      setApiBaseUrl(DEFAULT_API_BASE_URL + '/enterprise', nextWorkspace, true);
      fail();
      await rejected;
      expect(expired).toBe(false);
    } finally { window.removeEventListener('soren:company-session-expired', onExpired); }
  });
});
