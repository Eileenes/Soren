/**
 * 应用设置存储
 *
 * 非敏感偏好存入 localStorage；API Key 始终由桌面私有配置管理。
 */

import type { AppSettings, GeneralSettings, LLMSettings } from '../types';

/** 默认 LLM 配置：与后端 main.go 的 defaultModel 保持一致 */
const DEFAULT_LLM_SETTINGS: LLMSettings = {
  provider: 'openai',
  apiKey: '',
  baseUrl: 'https://api.openai.com/v1',
  model: 'gpt-4o-mini',
  temperature: 0.7,
  maxTokens: 2000,
};

const DEFAULT_GENERAL_SETTINGS: GeneralSettings = {
  theme: 'light',
  language: 'zh-CN',
  showMemoryBadge: true,
};

const DEFAULT_SETTINGS: AppSettings = {
  llm: { ...DEFAULT_LLM_SETTINGS },
  general: { ...DEFAULT_GENERAL_SETTINGS },
};

/**
 * 持久化读取钩子
 *
 * 当前为内存实现，返回 null 表示无历史设置。
 * 接入 Tauri Store 后改为异步读取即可，接口保持不变。
 */
const STORAGE_KEY = 'soren.preferences.v1';

// Only appearance and non-secret model options belong in browser storage.
// Provider credentials are handled by the desktop private config command.
function readPersisted(): AppSettings | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw);
    const llm = value?.llm ?? {};
    const general = value?.general ?? {};
    return {
      llm: {
        ...DEFAULT_LLM_SETTINGS,
        provider: ['openai', 'anthropic', 'custom'].includes(llm.provider) ? llm.provider : DEFAULT_LLM_SETTINGS.provider,
        model: typeof llm.model === 'string' ? llm.model : DEFAULT_LLM_SETTINGS.model,
        baseUrl: typeof llm.baseUrl === 'string' ? llm.baseUrl : DEFAULT_LLM_SETTINGS.baseUrl,
        temperature: typeof llm.temperature === 'number' && llm.temperature >= 0 && llm.temperature <= 2 ? llm.temperature : DEFAULT_LLM_SETTINGS.temperature,
        maxTokens: typeof llm.maxTokens === 'number' && llm.maxTokens > 0 && llm.maxTokens <= 128000 ? llm.maxTokens : DEFAULT_LLM_SETTINGS.maxTokens,
        apiKey: '',
      },
      general: {
        ...DEFAULT_GENERAL_SETTINGS,
        theme: ['light', 'dark', 'system'].includes(general.theme) ? general.theme : DEFAULT_GENERAL_SETTINGS.theme,
        showMemoryBadge: typeof general.showMemoryBadge === 'boolean' ? general.showMemoryBadge : true,
      },
    };
  } catch { return null; }
}

function persist(settings: AppSettings): void {
  const { apiKey: _secret, ...nonSecret } = settings.llm;
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify({ llm: nonSecret, general: settings.general })); }
  catch { /* Storage may be unavailable in private browser contexts. */ }
}

/** 内存中的设置快照 */
let currentSettings: AppSettings = readPersisted() ?? DEFAULT_SETTINGS;

/** 订阅者集合 */
const subscribers = new Set<(settings: AppSettings) => void>();

/** 读取当前设置（同步） */
export function getSettings(): AppSettings {
  return currentSettings;
}

/** 订阅设置变更，返回取消订阅函数 */
export function subscribeSettings(listener: (settings: AppSettings) => void): () => void {
  subscribers.add(listener);
  return () => {
    subscribers.delete(listener);
  };
}

function emit(): void {
  for (const listener of subscribers) {
    listener(currentSettings);
  }
}

/**
 * 设置补丁类型
 *
 * 分组的字段全部可选：调用方通常只改一两个字段。
 * 注意不能直接用 Partial<AppSettings> —— 那样补丁分组本身与完整分组同型，
 * 展开后会丢失"可选"语义（见 updateLLMSettings 的类型报错）。
 */
export interface SettingsPatch {
  llm?: Partial<LLMSettings>;
  general?: Partial<GeneralSettings>;
}

/** 整体替换设置（patch 为深合并，未提供的分组保持原值） */
export function updateSettings(patch: SettingsPatch): AppSettings {
  currentSettings = {
    llm: { ...currentSettings.llm, ...patch.llm },
    general: { ...currentSettings.general, ...patch.general },
  };
  persist(currentSettings);
  emit();
  return currentSettings;
}

/** 局部更新 LLM 设置 */
export function updateLLMSettings(patch: Partial<LLMSettings>): AppSettings {
  return updateSettings({ llm: patch });
}

/** 局部更新通用设置 */
export function updateGeneralSettings(patch: Partial<GeneralSettings>): AppSettings {
  return updateSettings({ general: patch });
}

/** 恢复默认设置 */
export function resetSettings(): AppSettings {
  currentSettings = {
    llm: { ...DEFAULT_LLM_SETTINGS },
    general: { ...DEFAULT_GENERAL_SETTINGS },
  };
  persist(currentSettings);
  emit();
  return currentSettings;
}

export { DEFAULT_SETTINGS, DEFAULT_LLM_SETTINGS, DEFAULT_GENERAL_SETTINGS };
