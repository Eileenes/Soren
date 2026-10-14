import axios from 'axios';
import { getApiBaseUrl, normalizeError } from './api';
import { getActiveBotID } from './botScope';
export const AVATARS = ['soren', 'angler', 'flamingo', 'hydrant', 'iceberg', 'pudin', 'rex', 'robo', 'saboten', 'tako', 'yurei'] as const;
export const avatarURL = (id: string) => AVATARS.includes(id as typeof AVATARS[number]) && id !== 'soren' ? `/bot-avatars/${id}.png` : '/soren-mark.svg';
export interface BotDefinition { canDo: string[]; cannotDo: string[]; workflow: string; output: string; }
export interface BotPolicy { actions: Record<string, 'deny' | 'confirm' | 'allow'>; }
export interface BotVersion { version: number; createdAt: string; bot: Bot; }
export interface BotAction { key: string; name: string; approval: boolean; }
export interface Bot { definition?: BotDefinition; policy?: BotPolicy | null; templateSource?: { service: string; id: string; revision: number; digest: string }; id: string; name: string; avatar: string; instructions: string; model: string; memoryEnabled: boolean; collaborationEnabled: boolean; collaborationAll?: boolean; collaborators: string[]; status: 'active' | 'paused' | 'archived'; version: number; pinned?: boolean; pinnedAt?: string; createdAt?: string; updatedAt?: string; }
export const DEFAULT_BOT: Bot = { id: 'soren', name: 'Soren', avatar: 'soren', instructions: '', model: '', memoryEnabled: true, collaborationEnabled: false, collaborators: [], status: 'active', version: 1 };
export interface SharedNote { id: string; title: string; content: string; allowedBots: string[]; version: number; }
export interface RunStep { botId: string; name: string; avatar: string; status: string; output: string; }
export interface Run { id: string; ownerBotId: string; goal: string; status: string; steps: RunStep[]; result: string; error: string; demo: boolean; createdAt: string; }
async function request<T>(method: string, path: string, data?: unknown, scope = getActiveBotID()): Promise<T> {
 try { return (await axios.request<T>({ method, url: getApiBaseUrl() + '/api' + path, data, headers: { 'X-Bot-ID': scope }, timeout: path === '/bots/draft' ? 65000 : 15000 })).data; }
 catch (error) { throw normalizeError(error); }
}
export const agentsAPI = {
 actions: (id: string) => request<BotAction[]>('GET', `/bots/${id}/actions`, undefined, 'soren'),
 versions: (id: string) => request<BotVersion[]>('GET', `/bots/${id}/versions`, undefined, 'soren'),
 restore: (id: string, target: number, version: number) => request<Bot>('POST', `/bots/${id}/versions/${target}/restore`, { version }, 'soren'),
 draft: (name: string, instructions: string) => request<{name: string; instructions: string}>('POST', '/bots/draft', {name, instructions}),
 list: () => request<Bot[]>('GET', '/bots', undefined, 'soren'),
 save: (bot: Bot, create: boolean) => request<Bot>(create ? 'POST' : 'PUT', create ? '/bots' : `/bots/${bot.id}`, bot, 'soren'),
 updateNavigation: (id: string, patch: { version: number; name?: string; pinned?: boolean; archive?: boolean }) => request<Bot>('PATCH', `/bots/${id}`, patch, 'soren'),
 notes: () => request<SharedNote[]>('GET', '/shared-notes'),
 saveNote: (note: SharedNote) => request<SharedNote>(note.id ? 'PUT' : 'POST', note.id ? `/shared-notes/${note.id}` : '/shared-notes', note),
 deleteNote: (id: string) => request<void>('DELETE', `/shared-notes/${id}`),
 runs: (botId = getActiveBotID()) => request<Run[]>('GET', '/runs', undefined, botId),
 start: (goal: string, participants: string[], requestId: string) => request<Run>('POST', '/runs', { goal, participants, requestId }),
 cancel: (id: string) => request<void>('POST', `/runs/${id}/cancel`),
 resume: (id: string) => request<void>('POST', `/runs/${id}/resume`),
 addMemory: (content: string) => request<void>('POST', '/memories', { content }),
 editMemory: (id: number, content: string) => request<void>('PUT', `/memories/${id}`, { content }),
};

export function actionLabel(key: string): string {
 const labels: Record<string,string> = { workspace_create: '保存本地成果', workspace_list: '查看本地成果列表', workspace_read: '读取本地成果', 'gmail.draft': '创建邮件草稿', 'gmail.send': '发送邮件', 'notion.search': '搜索 Notion 页面', 'notion.read': '读取 Notion 页面', 'notion.create': '创建 Notion 页面' };
 return labels[key] || key;
}
