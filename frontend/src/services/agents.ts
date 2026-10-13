import axios from 'axios';
import { getApiBaseUrl, normalizeError } from './api';
import { getActiveBotID } from './botScope';
export const AVATARS = ['soren', 'angler', 'flamingo', 'hydrant', 'iceberg', 'pudin', 'rex', 'robo', 'saboten', 'tako', 'yurei'] as const;
export const avatarURL = (id: string) => AVATARS.includes(id as typeof AVATARS[number]) && id !== 'soren' ? `/bot-avatars/${id}.png` : '/soren-mark.svg';
export interface Bot { id: string; name: string; avatar: string; instructions: string; model: string; memoryEnabled: boolean; collaborationEnabled: boolean; collaborators: string[]; status: 'active' | 'paused' | 'archived'; version: number; createdAt?: string; updatedAt?: string; }
export const DEFAULT_BOT: Bot = { id: 'soren', name: 'Soren', avatar: 'soren', instructions: '', model: '', memoryEnabled: true, collaborationEnabled: false, collaborators: [], status: 'active', version: 1 };
export interface SharedNote { id: string; title: string; content: string; allowedBots: string[]; version: number; }
export interface RunStep { botId: string; name: string; avatar: string; status: string; output: string; }
export interface Run { id: string; ownerBotId: string; goal: string; status: string; steps: RunStep[]; result: string; error: string; demo: boolean; createdAt: string; }
async function request<T>(method: string, path: string, data?: unknown, scope = getActiveBotID()): Promise<T> {
 try { return (await axios.request<T>({ method, url: getApiBaseUrl() + '/api' + path, data, headers: { 'X-Bot-ID': scope }, timeout: path === '/bots/draft' ? 65000 : 15000 })).data; }
 catch (error) { throw normalizeError(error); }
}
export const agentsAPI = {
 draft: (name: string, instructions: string) => request<{name: string; instructions: string}>('POST', '/bots/draft', {name, instructions}),
 list: () => request<Bot[]>('GET', '/bots', undefined, 'soren'),
 save: (bot: Bot, create: boolean) => request<Bot>(create ? 'POST' : 'PUT', create ? '/bots' : `/bots/${bot.id}`, bot, 'soren'),
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
