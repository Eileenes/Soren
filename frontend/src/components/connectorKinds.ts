import type { ComponentType } from 'react';
import { IconBrain, IconGlobe, IconPlug, IconSparkle, IconSparkles, IconVideo } from './icons';

// Services connected with a single credential. MCP presets use discovered tools.
export interface KeyKind {
  title: string;
  caption: string;
  hint: string;
  note: string;
  keyURL: string;
  icon: ComponentType<{ size?: number }>;
  mcpEndpoint?: string;
  credentialLabel?: string;
}

export const AGENT_TOOLKIT_MCP_URL = 'https://agent-toolkit.app.baizhi.cloud/mcp';
export const OCTOBUS_MCP_URL = 'https://octobus.chaitin.net/capsets/mcp';

export const keyKinds = {
  music: { title: 'AI 音乐', caption: '创作方案 · 生成音乐', hint: '在百智云 AI 音乐的「接入管理」中新建 API Key 并粘贴到这里。生成的 mp3 保存在本机。', note: '可整理创作方案、查询和下载音乐；生成、重试和删除音乐需要确认。', keyURL: 'https://ai-sound-lab.app.baizhi.cloud/developers', icon: IconSparkles },
  image: { title: 'AI 生图', caption: '文生图 · 图生图', hint: '在百智云 AI 生图的「接入指南」中创建 API Key 并粘贴到这里。生成的图片保存在本机。', note: '可查询任务和价格并保存图片；文生图、图生图和重试会消耗积分，需要确认。', keyURL: 'https://image-generate.app.baizhi.cloud/access-guide', icon: IconSparkle },
  video: { title: '视频 API', caption: '文生视频 · 图生视频', hint: '在 VideoAPI 创建 API Key 并粘贴到这里。可通过文字或公网 HTTPS 原图生成视频，完成后保存到本机。', note: '先查询当前规格和积分价格；生成视频会消耗 Key 所属账号的积分，需要确认。排队与生成中的任务可继续查询，本地原图上传暂不支持。', keyURL: 'https://video-api.app.baizhi.cloud', icon: IconVideo, credentialLabel: 'Key' },
  browser: { title: '云浏览器', caption: '远程浏览器 · 下载文件', hint: '在 CapyBrowser 云浏览器中创建 API Key 并粘贴到这里。下载的文件保存在本机。', note: '可查询运行、档案、价格和用量，生成人工处理链接并保存下载；启动运行（计费）、结束运行和删除档案需要确认。', keyURL: 'https://capybrowser.app.baizhi.cloud/overview', icon: IconGlobe },
  design: { title: '设计提示词', caption: '设计系统 · 视觉风格 · 模板', hint: '在百智云设计提示词的「接入」页面创建 API Key 并粘贴到这里。模板 ZIP 保存在本机。', note: '可搜索和读取设计系统、视觉风格、方向基线与模板，均为只读免费操作。', keyURL: 'https://design-prompt.app.baizhi.cloud/integration', icon: IconBrain },
  'agent-toolkit': { title: 'Agent 工具包', caption: '工具服务 · 按需调用', hint: '在 Agent 工具包的「API Key 管理」中创建 API Key 并粘贴到这里。服务地址已配置，连接时会自动读取可用工具。', note: 'API Key 保存在本机系统凭据库。仅所选 Bot 可使用工具，每次调用前需要确认；可用工具由此 Key 的授权范围决定。', keyURL: 'https://agent-toolkit.app.baizhi.cloud/access-guide', icon: IconPlug, mcpEndpoint: AGENT_TOOLKIT_MCP_URL },
  octobus: { title: '致用 OctoBus', caption: '企业能力 · CRM · JIRA', hint: '在致用平台获取个人 Token 并粘贴到这里，无需填写 Bearer 前缀。服务地址已配置，连接时会自动读取可用工具。', note: '个人 Token 保存在本机系统凭据库。仅所选 Bot 可使用工具，每次调用前需要确认；可用能力由此 Token 的授权范围决定。', keyURL: 'https://octobus.chaitin.net', icon: IconPlug, mcpEndpoint: OCTOBUS_MCP_URL, credentialLabel: '个人 Token' },
} satisfies Record<string, KeyKind>;

export type KeyKindId = keyof typeof keyKinds;
export const isKeyKind = (kind: string | null | undefined): kind is KeyKindId => Boolean(kind && kind in keyKinds);
export const keyKindHosts = Object.values(keyKinds).map(k => new URL(k.keyURL).hostname);

export function keyCredentialLabel(kind: KeyKindId): string {
  const config: KeyKind = keyKinds[kind];
  return config.credentialLabel ?? 'API Key';
}

export function mcpPresetEndpoint(kind: string | null | undefined): string | undefined {
  const config: KeyKind | undefined = isKeyKind(kind) ? keyKinds[kind] : undefined;
  return config?.mcpEndpoint;
}

// Keep the persisted MCP transport while displaying known services in their own cards.
// This also recognizes connections previously added through the generic MCP form.
export function connectionKind(connection: { kind: string; endpoint?: string }): string {
  if (connection.kind === 'mcp') {
    const endpoint = connection.endpoint?.replace(/\/+$/, '');
    for (const kind of Object.keys(keyKinds)) {
      const preset = mcpPresetEndpoint(kind);
      if (preset && endpoint === preset) return kind;
    }
  }
  return connection.kind;
}
