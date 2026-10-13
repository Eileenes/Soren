import type { ComponentType } from 'react';
import { IconBrain, IconGlobe, IconSparkle, IconSparkles } from './icons';

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
  browser: { title: '云浏览器', caption: '远程浏览器 · 下载文件', hint: '在 CapyBrowser 云浏览器中创建 API Key 并粘贴到这里。下载的文件保存在本机。', note: '可查询运行、档案、价格和用量，生成人工处理链接并保存下载；启动运行（计费）、结束运行和删除档案需要确认。', keyURL: 'https://capybrowser.app.baizhi.cloud/overview', icon: IconGlobe },
  design: { title: '设计提示词', caption: '设计系统 · 视觉风格 · 模板', hint: '在百智云设计提示词的「接入」页面创建 API Key 并粘贴到这里。模板 ZIP 保存在本机。', note: '可搜索和读取设计系统、视觉风格、方向基线与模板，均为只读免费操作。', keyURL: 'https://design-prompt.app.baizhi.cloud/integration', icon: IconBrain },
} satisfies Record<string, KeyKind>;

export type KeyKindId = keyof typeof keyKinds;
export const isKeyKind = (kind: string | null | undefined): kind is KeyKindId => Boolean(kind && kind in keyKinds);
export const keyKindHosts = Object.values(keyKinds).map(k => new URL(k.keyURL).hostname);
