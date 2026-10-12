import { SelectField } from '../components/ui';
import { useState } from 'react';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { Button, Modal, TextField, TextAreaField, Switch } from '../components/ui';
import { IconPlus, IconSparkles } from '../components/icons';
import { AVATARS, avatarURL, agentsAPI, type Bot } from '../services/agents';
import { describeError } from '../services/api';
const blank = (): Bot => ({ id: '', name: '', avatar: 'iceberg', instructions: '', model: '', memoryEnabled: true, collaborationEnabled: false, collaborators: [], status: 'active', version: 1 });
export function BotsPage({ bots, active, onSelect, onReload, error }: { bots: Bot[]; active: string; onSelect: (id: string) => void; onReload: () => Promise<void>; error: string | null }) {
 const [draft, setDraft] = useState<Bot | null>(null); const [saving, setSaving] = useState(false); const [localError, setError] = useState(''); const [showArchive, setShowArchive] = useState(false); const [generating, setGenerating] = useState(false); const [draftNotice, setDraftNotice] = useState('');
 const update = (patch: Partial<Bot>) => setDraft(value => value ? { ...value, ...patch } : null);
 const edit = (bot: Bot) => { setError(''); setDraftNotice(''); setDraft({ ...bot, collaborators: [...(bot.collaborators || [])] }); };
 const save = async () => { if (!draft) return; setSaving(true); setError(''); try { await agentsAPI.save(draft, !draft.id); await onReload(); setDraft(null); } catch (e) { setError(describeError(e)); } finally { setSaving(false); } };
 return <PageContainer><PageHeading title="我的 Bot" actions={<Button leadingIcon={<IconPlus size={16} />} onClick={() => edit(blank())}>新建 Bot</Button>} />
 {error && <p role="alert" className="bot-error">{error}<button onClick={() => void onReload()} className="ml-3 underline">重试</button></p>}
 <div className="bot-filter"><button aria-pressed={!showArchive} onClick={() => setShowArchive(false)}>全部 Bot</button><button aria-pressed={showArchive} onClick={() => setShowArchive(true)}>已归档</button></div>
 <div className="bot-grid">{bots.filter(b => showArchive ? b.status === 'archived' : b.status !== 'archived').map(bot => <article className={'bot-card ' + (active === bot.id ? 'is-current' : '')} key={bot.id}>
 <div className="bot-card-top"><img src={avatarURL(bot.avatar)} alt="" width={72} height={72} /><span className="bot-state">{bot.status === 'paused' ? '已暂停' : bot.status === 'archived' ? '已归档' : active === bot.id ? '当前 Bot' : '可用'}</span></div>
 <h2>{bot.name}</h2><p className="bot-duty">{bot.instructions}</p><div className="bot-tags"><span>{bot.memoryEnabled ? '独立记忆' : '记忆关闭'}</span><span>{bot.collaborationEnabled ? '协作已开启' : '独立工作'}</span></div>
 <div className="bot-card-actions"><Button variant="ghost" onClick={() => edit(bot)} aria-label={`配置 ${bot.name}`}>配置</Button><Button variant="ghost" onClick={() => edit({ ...bot, id: '', name: bot.name + ' 副本', collaborators: [], collaborationEnabled: false, version: 1, status: 'active' })} aria-label={`复制 ${bot.name}`}>复制</Button><Button disabled={bot.status !== 'active'} onClick={() => onSelect(bot.id)}>对话</Button></div>
 </article>)}</div>
 {showArchive && !bots.some(b => b.status === 'archived') && <p className="py-20 text-center text-ink-3 text-sm">暂无归档</p>}
 <Modal open={Boolean(draft)} onClose={() => { if (!saving && !generating) setDraft(null); }} title={draft?.id ? '配置 Bot' : '新建 Bot'} size="lg" footer={<><Button variant="ghost" disabled={saving || generating} onClick={() => setDraft(null)}>取消</Button><Button disabled={generating} loading={saving} onClick={() => void save()}>保存 Bot</Button></>}>
 {draft && <div className="space-y-5"><div className="avatar-picker" role="group" aria-label="选择头像">{AVATARS.map(id => <button key={id} aria-label={`头像 ${id}`} aria-pressed={draft.avatar === id} onClick={() => update({ avatar: id })}><img src={avatarURL(id)} alt="" width={48} height={48} /></button>)}</div>
 <TextField label="名称" value={draft.name} maxLength={40} placeholder="给它起个名字" onChange={e => update({ name: e.target.value })} />
 <TextAreaField label="职责" value={draft.instructions} rows={5} maxLength={8000} placeholder="负责什么、怎么做、交付什么结果…" onChange={e => update({ instructions: e.target.value })} />
 <div className="flex items-center gap-3"><Button size="sm" variant="ghost" disabled={!draft.instructions.trim() || saving} loading={generating} onClick={async () => { setGenerating(true); setError(''); try { const result = await agentsAPI.draft(draft.name, draft.instructions); update(result); setDraftNotice('草稿已生成，保存后生效。'); } catch (e) { setError(describeError(e)); } finally { setGenerating(false); } }}>完善职责</Button><span className="text-xs text-ink-3">{draftNotice}</span></div>
 {!draft.instructions && <div className="bot-templates">{[{ name: '工作助手', duty: '整理工作资料、规划日程、起草邮件。先明确目标与限制，交付简洁可执行的方案。未连接工具时只提供草稿，不声称已发送或已创建日程。' }, { name: '创作助手', duty: '负责选题、文案、视频分镜与图像提示词。关注受众、风格与表达，交付可直接使用的创作稿。未连接生成工具时不声称已生成图片或视频。' }, { name: '生活助手', duty: '帮助规划日常、整理待办与记录偏好。语气自然，建议具体；不将推测写成用户事实。' }].map(t => <button key={t.name} onClick={() => update({ name: draft.name || t.name, instructions: t.duty })}><IconSparkles size={13} />{t.name}</button>)}</div>}
 <TextField label="模型" value={draft.model} placeholder="继承全局模型" maxLength={200} onChange={e => update({ model: e.target.value })} helper="使用已连接服务商支持的模型 ID。" />
 <div className="bot-option"><div><strong>独立记忆</strong><p>仅此 Bot 可召回；关闭后保留已有记录。</p></div><Switch label="启用独立记忆" checked={draft.memoryEnabled} onChange={memoryEnabled => update({ memoryEnabled })} /></div>
 <div className="bot-option"><div><strong>允许协作</strong><p>双方授权后，可在任务中交换成果。</p></div><Switch label="允许协作" checked={draft.collaborationEnabled} onChange={collaborationEnabled => update({ collaborationEnabled })} /></div>
 {draft.collaborationEnabled && <div className="bot-peer-list">{bots.filter(b => b.id !== draft.id && b.status !== 'archived').map(b => <label key={b.id}><input type="checkbox" checked={draft.collaborators.includes(b.id)} onChange={e => update({ collaborators: e.target.checked ? [...draft.collaborators, b.id] : draft.collaborators.filter(id => id !== b.id) })} /><img src={avatarURL(b.avatar)} alt="" />{b.name}</label>)}</div>}
 {draft.id && draft.id !== 'soren' && <SelectField label="状态" aria-label="Bot 状态" value={draft.status} onChange={status => update({ status })} options={[{value: 'active', label: '可用'}, {value: 'paused', label: '暂停'}, {value: 'archived', label: '归档'}]} />}
 {localError && <p role="alert" className="bot-error">{localError}</p>}
 </div>}
 </Modal></PageContainer>;
}
