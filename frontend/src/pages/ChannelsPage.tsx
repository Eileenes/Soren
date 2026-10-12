import { ConnectionsPanel } from '../components/ConnectionsPanel';
import { useState, type ReactNode } from 'react';
import { cn, copyToClipboard } from '../utils/format';
import { OfflineBanner, PageContainer, PageHeading } from '../components/layout/Shell';
import { IconClock, IconCopy, IconDatabase, IconInbox, IconPlug, IconRefresh, IconSparkles } from '../components/icons';
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
  category: string;
  description: string;
  icon: ReactNode;
  color: string;
  planned?: boolean;
  detail: string;
  env?: string;
}

function FeishuMark() {
  return <svg width="27" height="27" viewBox="0 0 32 32" fill="none" aria-hidden="true"><path d="m4 10 10 3 8-9 6 3-13 16-9-5Z" fill="#3370FF" /><path d="m4 19 10 3 7-7 6 3-12 11-9-5Z" fill="#15BEB3" /></svg>;
}
function DingTalkMark() {
  return <svg width="28" height="28" viewBox="0 0 32 32" fill="none" aria-hidden="true"><path d="m6 7 21 5-9 7 4 2-11 9 4-12-6-2 9-2L6 11Z" fill="#1685FA" /></svg>;
}

const APPS: ConnectionApp[] = [
  { name: 'feishu', title: '飞书', category: '消息渠道', description: '接收与回复飞书消息。', icon: <FeishuMark />, color: '#edf5fc',
    detail: '在飞书开放平台创建自建应用并开启机器人能力，配置接收消息权限和事件订阅。当前可读取凭据状态、启动或停止接收器；平台授权与消息权限需要在飞书完成。',
    env: 'FEISHU_APP_ID=your-app-id\nFEISHU_APP_SECRET=your-app-secret\nFEISHU_MODE=ws\nFEISHU_ENABLED=false' },
  { name: 'dingtalk', title: '钉钉', category: '消息渠道', description: '接收与回复钉钉消息。', icon: <DingTalkMark />, color: '#edf5fc',
    detail: '在钉钉开发者后台创建企业内部应用，添加机器人并获取 Client ID 与 Client Secret。Stream 模式适合桌面端；发布机器人并完成平台权限配置后再启用接收器。',
    env: 'DINGTALK_CLIENT_ID=your-client-id\nDINGTALK_CLIENT_SECRET=your-client-secret\nDINGTALK_MODE=ws\nDINGTALK_ENABLED=false' },
  { name: 'gmail', title: 'Gmail', category: '邮件', description: '整理收件箱，协助撰写和发送邮件。', icon: <IconInbox size={25} />, color: '#f5ece7', planned: true,
    detail: '规划通过账号授权读取指定邮件、整理摘要和准备回复。邮件发送将经过明确的收件人、内容和附件确认。目前尚未接入邮箱，也无法读取或发送邮件。' },
  { name: 'calendar', title: '日历', category: '日程', description: '记住安排，也为重要的事留出时间。', icon: <IconClock size={25} />, color: '#eef0fc', planned: true,
    detail: '规划支持日程查询、冲突检测、创建活动和提醒。连接时会说明访问范围，并支持断开授权。目前尚未接入日历服务。' },
  { name: 'notion', title: 'Notion', category: '知识与笔记', description: '让笔记、资料和长期记忆彼此连接。', icon: <IconDatabase size={25} />, color: '#eeeae2', planned: true,
    detail: '规划将授权的笔记作为个人知识来源，并由你控制同步范围、删除与更新。外部资料与私人记忆将保留来源信息。目前尚未接入 Notion。' },
  { name: 'creative', title: '创作工具', category: '图片与视频', description: '从一个想法，到一张图、一段短片。', icon: <IconSparkles size={25} />, color: '#f2eedc', planned: true,
    detail: '规划接入图片和视频生成服务，支持后台任务、进度查询与作品保存。生成前会展示所用服务及可能产生的费用。目前没有连接生成服务，无法实际生成媒体。' },
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
  const [filter, setFilter] = useState<'all' | 'messages' | 'planned'>('all');
  const [selected, setSelected] = useState<ConnectionApp | null>(null);
  const [localPending, setLocalPending] = useState<string | null>(null);
  const selectedChannel = selected ? channels.find((channel) => channel.name === selected.name) : undefined;
  const visible = APPS.filter(app => !['gmail', 'notion'].includes(app.name)).filter((app) => filter === 'all' || (filter === 'planned' ? app.planned : !app.planned));

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

      <ConnectionsPanel /><h2 className="mb-4 text-sm font-semibold text-ink-2">消息渠道与更多应用</h2><div className="mb-5 flex flex-wrap gap-2" aria-label="应用分类">
        {([{ id: 'all', label: '全部应用' }, { id: 'messages', label: '消息渠道' }, { id: 'planned', label: '规划中' }] as const).map((item) => <button key={item.id} type="button" aria-pressed={filter === item.id} onClick={() => setFilter(item.id)} className={cn('rounded-full border px-4 py-2 text-sm transition-colors', filter === item.id ? 'border-primary-start bg-primary-start/10 text-primary-start' : 'border-transparent text-ink-2 hover:bg-surface-3')}>{item.label}</button>)}
      </div>

      <div className="grid gap-4 pb-5 sm:grid-cols-2 xl:grid-cols-3">
        {visible.map((app) => {
          const channel = channels.find((item) => item.name === app.name);
          const status = app.planned ? '规划中' : stateLabel(channel, offline, loading);
          const pending = pendingName === app.name || localPending === app.name;
          return <section key={app.name} className="connection-card flex min-h-[248px] flex-col rounded-2xl border border-line bg-surface-2 p-5 transition-colors hover:border-line-strong">
            <div className="mb-5 flex items-start justify-between gap-3"><span className="flex h-12 w-12 items-center justify-center rounded-xl text-[#35584a]" style={{ backgroundColor: app.color }}>{app.icon}</span><span className={cn('rounded-full px-2.5 py-1 text-xs', app.planned ? 'bg-surface-3 text-ink-3' : channel?.lastError ? 'bg-error/5 text-error' : channel?.configured && !offline ? 'bg-primary-start/10 text-primary-start' : 'bg-surface-3 text-ink-2')}>{status}</span></div>
            <h2 className="text-base font-semibold text-ink-1">{app.title}</h2><p className="mt-2 flex-1 text-sm leading-6 text-ink-2">{app.description}</p>
            {channel?.lastError && !offline && <p className="mt-3 text-xs text-error">接收器需要处理，请查看连接说明。</p>}
            <div className="mt-5 flex items-center justify-between gap-2 border-t border-line-subtle pt-4"><span className="text-xs text-ink-3">{app.category}</span>
              {app.planned ? <Button size="sm" variant="ghost" onClick={() => setSelected(app)} aria-label={`了解${app.title}规划`}>了解规划</Button> : <div className="flex gap-1"><Button size="sm" variant="ghost" onClick={() => setSelected(app)} aria-label={`${app.title}连接说明`}>连接说明</Button>{channel?.configured && <Button size="sm" disabled={offline || loading || Boolean((pendingName || localPending) && !pending)} loading={pending} onClick={() => void toggle(channel)}>{channel.running ? '停止' : '启用'}</Button>}</div>}
            </div>
          </section>;
        })}
      </div>
      <Modal open={Boolean(selected)} onClose={() => setSelected(null)} title={selected ? `${selected.title}${selected.planned ? ' · 规划中' : '连接说明'}` : ''} description={selected?.description}
        footer={<><Button variant="ghost" onClick={() => setSelected(null)}>关闭</Button>{selected?.env && <Button leadingIcon={<IconCopy size={14} />} onClick={() => void copyTemplate()}>复制配置模板</Button>}</>}>
        <div className="space-y-5 text-sm leading-7 text-ink-2">
          <p>{selected?.detail}</p>
          {selected?.planned ? <div className="rounded-xl border border-line bg-surface-3 p-4"><p className="font-medium text-ink-1">还在准备中</p><p className="mt-1">这里展示的是后续接入方向。当前没有连接任何账号，也不会获取你的数据。</p></div> : <>
            <div className="rounded-xl border border-line bg-surface-3 p-4"><p className="font-medium text-ink-1">当前配置状态：{stateLabel(selectedChannel, offline, loading)}</p><p className="mt-1 text-xs leading-6">{selectedChannel?.running ? '本地接收器已启动。此状态不代表平台已完成连接，请通过实际收发消息确认。' : '先配置应用凭据并重启服务，再回到此页面启用接收器。启停仅作用于本次运行。'}</p>{selectedChannel?.lastError && <p className="mt-2 break-words text-xs text-error">{selectedChannel.lastError}</p>}</div>
            <div><p className="mb-2 font-medium text-ink-1">配置凭据</p><p>当前版本通过本机配置文件读取消息渠道凭据，暂不支持在界面保存。</p><p className="mt-2 text-xs leading-6">桌面端：在设置中打开数据文件夹，将以下配置加入 <code className="rounded bg-surface-3 px-1">.env</code>，保存后退出并重新打开 Soren。开发环境使用 <code className="rounded bg-surface-3 px-1">backend/.env</code>。</p></div>
            <pre className="overflow-x-auto rounded-xl border border-line bg-surface-3 p-4 text-xs leading-6 text-ink-2"><code>{selected?.env}</code></pre>
          </>}
        </div>
      </Modal>
    </PageContainer>
  );
}
