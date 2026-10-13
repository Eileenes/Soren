import { useState, type ReactNode } from 'react';

const logos: Record<string, string> = {
  gmail: 'gmail.svg', notion: 'notion.png', mcp: 'mcp.png',
  feishu: 'feishu.svg', dingtalk: 'dingtalk.png',
  music: 'music.png', image: 'image.png', design: 'design.png',
  'agent-toolkit': 'agent-toolkit.png',
};

export function ConnectionAppIcon({ app, fallback }: { app: string; fallback: ReactNode }) {
  const [failedSource, setFailedSource] = useState<string>();
  const source = logos[app];
  return source && failedSource !== source
    ? <img className="connection-brand-logo" src={`/app-logos/${source}`} alt="" width={28} height={28} onError={() => setFailedSource(source)} />
    : <>{fallback}</>;
}
