import { ConnectionAppCard } from '../components/ConnectionAppCard';
import { ConnectionAppIcon } from '../components/ConnectionAppIcon';
import { ConnectionsPanel } from '../components/ConnectionsPanel';
import { useState, type ReactNode } from 'react';
import { copyToClipboard } from '../utils/format';
import { OfflineBanner, PageContainer, PageHeading } from '../components/layout/Shell';
import { IconChat, IconCopy, IconPlug, IconRefresh } from '../components/icons';
import { Button, Modal } from '../components/ui';
import type { ChannelStatus } from '../types';

export interface ChannelsPageProps {
  channels: ChannelStatus[];
  loading: boolean;
  error: string | null;
  offline: boolean;
  /** 正在切换状态的渠道名 */
  pendingName: string | null;
  onEnable: (name: string) => Promise<void>;
  onDisable: (name: string) => Promise<void>;
  onReload: () => Promise<void>;
  onRetryConnection?: () => void;
  /** 操作结果通知 */
  onNotify: (type: 'success' | 'error', title: string, description?: string) => void;
}

interface ConnectionApp {
  name: string;
  title: string;
  description: string;
  icon: ReactNode;
  detail: string;
  env?: string;
}

const APPS: ConnectionApp[] = [
  { name: 'feishu', title: '飞书', description: '接收与回复飞书消息。', icon: <ConnectionAppIcon app="feishu" fallback={<IconChat size={23} />} />,
    detail: '在飞书开放平台创建自建应用并开启机器人能力，配置接收消息权限和事件订阅。当前可读取凭据状态、启动或停止接收器；平台授权与消息权限需要在飞书完成。',
    env: 'FEISHU_APP_ID=your-app-id\nFEISHU_APP_SECRET=your-app-secret\nFEISHU_MODE=ws\nFEISHU_ENABLED=false' },
  { name: 'dingtalk', title: '钉钉', description: '接收与回复钉钉消息。', icon: <ConnectionAppIcon app="dingtalk" fallback={<IconChat size={23} />} />,
    detail: '在钉钉开发者后台创建企业内部应用，添加机器人并获取 Client ID 与 Client Secret。Stream 模式适合桌面端；发布机器人并完成平台权限配置后再启用接收器。',
    env: 'DINGTALK_CLIENT_ID=your-client-id\nDINGTALK_CLIENT_SECRET=your-client-secret\nDINGTALK_MODE=ws\nDINGTALK_ENABLED=false' },

];

function stateLabel(channel: ChannelStatus | undefined, offline: boolean, loading: boolean) {
  if (offline) return '等待服务';
  if (loading && !channel) return '读取中';
  if (!channel) return '未注册';
  if (channel.lastError) return '需要处理';
  if (channel.running) return '接收器已启动';
  return channel.configured ? '已配置' : '未配置';
}

export function ChannelsPage({ channels, loading, error, offline, pendingName, onEnable,
  onDisable, onReload, onRetryConnection, onNotify }: ChannelsPageProps) {
  const [selected, setSelected] = useState<ConnectionApp | null>(null);
  const [localPending, setLocalPending] = useState<string | null>(null);
  const selectedChannel = selected ? channels.find((channel) => channel.name === selected.name) : undefined;

  const reload = async () => {
    try { await onReload(); }
    catch (error) { onNotify('error', '无法刷新应用状态', error instanceof Error ? error.message : String(error)); }
  };
  const toggle = async (channel: ChannelStatus) => {
    if (offline || localPending || pendingName || loading) return;
    setLocalPending(channel.name);
    try {
      if (channel.running) await onDisable(channel.name);
      else await onEnable(channel.name);
      onNotify('success', channel.running ? '接收器已停止' : '已请求启动接收器', channel.running ? undefined : '请发送一条平台消息，确认机器人收发正常。');
    } catch (error) {
      onNotify('error', '应用操作未完成', error instanceof Error ? error.message : String(error));
    } finally { setLocalPending(null); }
  };
  const copyTemplate = async () => {
    if (!selected?.env) return;
    const copied = await copyToClipboard(selected.env);
    onNotify(copied ? 'success' : 'error', copied ? '配置模板已复制' : '复制失败', copied ? '请替换模板中的示例值，保存配置后重启 Soren。' : '可以从说明中手动复制。');
  };

  return (
    <PageContainer widthClass="max-w-5xl">
      <PageHeading icon={<IconPlug size={19} />} title="应用连接"
        actions={<Button size="sm" variant="ghost" leadingIcon={<IconRefresh size={15} />} loading={loading} onClick={() => void reload()}>刷新状态</Button>} />
      {offline && <OfflineBanner className="mb-5" onRetry={onRetryConnection} />}
      {error && !offline && <p role="alert" className="mb-5 rounded-xl border border-error/20 bg-error/5 p-4 text-sm text-error">{error}</p>}

      <ConnectionsPanel>{APPS.map(app => <ConnectionAppCard key={app.name} title={app.title} description={app.description} icon={app.icon} detail={channels.find(c => c.name === app.name)?.configured ? '已配置' : undefined} onAdd={() => setSelected(app)} />)}</ConnectionsPanel>
      <Modal open={Boolean(selected)} onClose={() => setSelected(null)} title={selected ? `连接 ${selected.title}` : ''} description={selected?.description}
        footer={<><Button variant="ghost" onClick={() => setSelected(null)}>关闭</Button>{selectedChannel?.configured && <Button loading={localPending === selectedChannel.name || pendingName === selectedChannel.name} disabled={offline || loading || Boolean(localPending || pendingName)} onClick={() => void toggle(selectedChannel)}>{selectedChannel.running ? '停止接收' : '启用接收'}</Button>}{selected?.env && <Button leadingIcon={<IconCopy size={14} />} onClick={() => void copyTemplate()}>复制配置模板</Button>}</>}>
        <div className="space-y-5 text-sm leading-7 text-ink-2">
          <p>{selected?.detail}</p>

            <div className="rounded-xl border border-line bg-surface-3 p-4"><p className="font-medium text-ink-1">当前配置状态：{stateLabel(selectedChannel, offline, loading)}</p><p className="mt-1 text-xs leading-6">{selectedChannel?.running ? '本地接收器已启动。此状态不代表平台已完成连接，请通过实际收发消息确认。' : '先配置应用凭据并重启服务，再回到此页面启用接收器。启停仅作用于本次运行。'}</p>{selectedChannel?.lastError && <p className="mt-2 break-words text-xs text-error">{selectedChannel.lastError}</p>}</div>
            <div><p className="mb-2 font-medium text-ink-1">配置凭据</p><p>当前版本通过本机配置文件读取消息渠道凭据，暂不支持在界面保存。</p><p className="mt-2 text-xs leading-6">桌面端：在设置中打开数据文件夹，将以下配置加入 <code className="rounded bg-surface-3 px-1">.env</code>，保存后退出并重新打开 Soren。开发环境使用 <code className="rounded bg-surface-3 px-1">backend/.env</code>。</p></div>
            <pre className="overflow-x-auto rounded-xl border border-line bg-surface-3 p-4 text-xs leading-6 text-ink-2"><code>{selected?.env}</code></pre>
        </div>
      </Modal>
    </PageContainer>
  );
}
