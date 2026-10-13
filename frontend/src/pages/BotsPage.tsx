import { useState } from 'react';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { Button } from '../components/ui';
import { IconPlus } from '../components/icons';
import { avatarURL, type Bot } from '../services/agents';
import { BotEditor, blankBot } from '../components/BotEditor';
export function BotsPage({ bots, active, onSelect, onReload, error }: { bots: Bot[]; active: string; onSelect: (id: string) => void; onReload: () => Promise<void>; error: string | null }) {
 const [draft, setDraft] = useState<Bot | null>(null); const [showArchive, setShowArchive] = useState(false);
 const edit = (bot: Bot) => setDraft({ ...bot, collaborators: [...bot.collaborators] });
 return <PageContainer><PageHeading title="我的 Bot" actions={<Button leadingIcon={<IconPlus size={16} />} onClick={() => edit(blankBot())}>新建 Bot</Button>} />
 {error && <p role="alert" className="bot-error">{error}<button onClick={() => void onReload()} className="ml-3 underline">重试</button></p>}
 <div className="bot-filter"><button aria-pressed={!showArchive} onClick={() => setShowArchive(false)}>全部 Bot</button><button aria-pressed={showArchive} onClick={() => setShowArchive(true)}>已归档</button></div>
 <div className="bot-directory">{bots.filter(b => showArchive ? b.status === 'archived' : b.status !== 'archived').map(bot => <article className={'bot-card ' + (active === bot.id ? 'is-current' : '')} key={bot.id}>
 <img className="bot-card-avatar" src={avatarURL(bot.avatar)} alt="" width={52} height={52} />
 <div className="bot-card-content"><div className="bot-card-heading"><h2>{bot.name}</h2><span className="bot-state">{bot.status === 'paused' ? '已暂停' : bot.status === 'archived' ? '已归档' : active === bot.id ? '当前 Bot' : '可用'}</span><div className="bot-tags"><span>{bot.memoryEnabled ? '独立记忆' : '记忆关闭'}</span><span>{bot.collaborationEnabled ? '协作已开启' : '独立工作'}</span></div></div><p className="bot-duty">{bot.instructions}</p></div>
 <div className="bot-card-actions"><Button variant="ghost" onClick={() => edit(bot)} aria-label={`配置 ${bot.name}`}>配置</Button><Button variant="ghost" onClick={() => edit({ ...bot, id: '', name: bot.name + ' 副本', collaborators: [], collaborationEnabled: false, version: 1, status: 'active' })} aria-label={`复制 ${bot.name}`}>复制</Button><Button disabled={bot.status !== 'active'} onClick={() => onSelect(bot.id)}>对话</Button></div>
 </article>)}</div>
 {showArchive && !bots.some(b => b.status === 'archived') && <p className="py-20 text-center text-ink-3 text-sm">暂无归档</p>}
 {draft && <BotEditor initial={draft} bots={bots} onClose={() => setDraft(null)} onSaved={async () => { await onReload(); setDraft(null); }} />}
 </PageContainer>;
}
