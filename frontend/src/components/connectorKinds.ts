import type { ComponentType } from 'react';
import { IconSparkle, IconSparkles } from './icons';

// Baizhi Cloud apps connected with a single API key. Capabilities are defined by the backend.
export interface KeyKind {
  title: string;
  caption: string;
  hint: string;
  note: string;
  keyURL: string;
  icon: ComponentType<{ size?: number }>;
}

export const keyKinds = {
  music: { title: 'AI 音乐', caption: '创作方案 · 生成音乐', hint: '在百智云 AI 音乐的「接入管理」中新建 API Key 并粘贴到这里。生成的 mp3 保存在本机。', note: '可整理创作方案、查询和下载音乐；生成、重试和删除音乐需要确认。', keyURL: 'https://ai-sound-lab.app.baizhi.cloud/developers', icon: IconSparkles },
  image: { title: 'AI 生图', caption: '文生图 · 图生图', hint: '在百智云 AI 生图的「接入指南」中创建 API Key 并粘贴到这里。生成的图片保存在本机。', note: '可查询任务和价格并保存图片；文生图、图生图和重试会消耗积分，需要确认。', keyURL: 'https://image-generate.app.baizhi.cloud/access-guide', icon: IconSparkle },
} satisfies Record<string, KeyKind>;

export type KeyKindId = keyof typeof keyKinds;
export const isKeyKind = (kind: string | null | undefined): kind is KeyKindId => Boolean(kind && kind in keyKinds);
export const keyKindHosts = Object.values(keyKinds).map(k => new URL(k.keyURL).hostname);
