/**
 * 主题与设置 Hook
 *
 * useSettings 订阅内存中的设置存储；useTheme 负责把主题落到 <html> 上，
 * 保证 Tailwind 的 darkMode: 'class' 能正确生效。
 */

import { useCallback, useEffect, useState } from 'react';
import {
  getSettings,
  resetSettings,
  subscribeSettings,
  updateGeneralSettings,
  updateLLMSettings,
  updateSettings,
  type SettingsPatch,
} from '../services/settingsStore';
import type { AppSettings, GeneralSettings, LLMSettings, ThemeMode } from '../types';

/** 订阅全部设置 */
export function useSettings() {
  const [settings, setSettings] = useState<AppSettings>(() => getSettings());

  useEffect(() => subscribeSettings(setSettings), []);

  const setLLM = useCallback((patch: Partial<LLMSettings>) => {
    updateLLMSettings(patch);
  }, []);

  const setGeneral = useCallback((patch: Partial<GeneralSettings>) => {
    updateGeneralSettings(patch);
  }, []);

  const patch = useCallback((value: SettingsPatch) => {
    updateSettings(value);
  }, []);

  const reset = useCallback(() => {
    resetSettings();
  }, []);

  return { settings, setLLM, setGeneral, patch, reset };
}

/**
 * 主题控制
 *
 * 深色为默认基调；切换时同步 <html class>、color-scheme 与文档背景，
 * 避免窗口边缘出现白色闪边。
 */
export function useTheme(): { theme: 'light' | 'dark'; mode: ThemeMode; toggleTheme: () => void; setTheme: (t: ThemeMode) => void } {
  const { settings, setGeneral } = useSettings();
  const mode = settings.general.theme;
  const [systemDark, setSystemDark] = useState(() => window.matchMedia('(prefers-color-scheme: dark)').matches);
  useEffect(() => {
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const update = () => setSystemDark(media.matches);
    update(); media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);
  const theme = mode === 'system' ? (systemDark ? 'dark' : 'light') : mode;

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle('dark', theme === 'dark');
    root.classList.toggle('light', theme === 'light');
    root.style.colorScheme = theme;
  }, [theme]);

  const setTheme = useCallback(
    (next: ThemeMode) => {
      setGeneral({ theme: next });
    },
    [setGeneral],
  );

  const toggleTheme = useCallback(() => {
    setTheme(theme === 'dark' ? 'light' : 'dark');
  }, [theme, setTheme]);

  return { theme, mode, toggleTheme, setTheme };
}
