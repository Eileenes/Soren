import { useCallback, useEffect, useState } from 'react';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { Button, Modal, TextField, TextAreaField, ConfirmDialog } from '../components/ui';
import { agentsAPI, avatarURL, type Bot, type SharedNote } from '../services/agents';
import { describeError } from '../services/api';
export function SharedNotesPage({ bots }: { bots: Bot[] }) {
 const [notes, setNotes] = useState<SharedNote[]>([]); const [draft, setDraft] = useState<SharedNote | null>(null); const [error, setError] = useState(''); const [busy, setBusy] = useState(false); const [remove, setRemove] = useState<SharedNote | null>(null);
 const reload = useCallback(async () => { setNotes(await agentsAPI.notes()); }, []);
 useEffect(() => { let alive = true; agentsAPI.notes().then(data => { if (alive) setNotes(data); }).catch(e => { if (alive) setError(describeError(e)); }); return () => { alive = false; }; }, []);
 const save = async () => { if (!draft) return; setBusy(true); setError(''); try { await agentsAPI.saveNote(draft); await reload(); setDraft(null); } catch (e) { setError(describeError(e)); } finally { setBusy(false); } };
 return <PageContainer><PageHeading title="共享资料" actions={<Button onClick={() => { setError(''); setDraft({ id: '', title: '', content: '', allowedBots: [], version: 1 }); }}>添加资料</Button>} />
 <p className="text-sm text-ink-2 mb-6">仅授权的 Bot 可使用；协作时取所有参与者的共同授权。</p>
 {error && !draft && <p role="alert" className="bot-error">{error}</p>}
 <div className="bot-grid">{notes.map(note => <article className="shared-card" key={note.id}><h2>{note.title}</h2><p>{note.content}</p><div className="shared-avatars">{note.allowedBots.map(id => { const b = bots.find(bot => bot.id === id); return b ? <img key={id} src={avatarURL(b.avatar)} alt={b.name} title={b.name} /> : null; })}{note.allowedBots.length === 0 && <span>未授权给任何 Bot</span>}</div><div className="bot-card-actions"><Button variant="ghost" onClick={() => { setError(''); setDraft({ ...note, allowedBots: [...note.allowedBots] }); }}>编辑</Button><Button variant="ghost" onClick={() => setRemove(note)}>删除</Button></div></article>)}</div>
 {notes.length === 0 && !error && <p className="py-16 text-center text-sm text-ink-3">暂无共享资料</p>}
 <Modal open={Boolean(draft)} onClose={() => { if (!busy) setDraft(null); }} title={draft?.id ? '编辑资料' : '添加资料'} footer={<Button loading={busy} onClick={() => void save()}>保存资料</Button>}>
 {draft && <div className="space-y-5"><TextField label="标题" maxLength={100} value={draft.title} onChange={e => setDraft({ ...draft, title: e.target.value })} /><TextAreaField label="内容" rows={6} maxLength={8000} value={draft.content} onChange={e => setDraft({ ...draft, content: e.target.value })} /><div className="bot-peer-list"><span className="text-sm text-ink-2">允许使用的 Bot</span>{bots.filter(b => b.status !== 'archived' || draft.allowedBots.includes(b.id)).map(b => <label key={b.id}><input type="checkbox" checked={draft.allowedBots.includes(b.id)} onChange={e => setDraft({ ...draft, allowedBots: e.target.checked ? [...draft.allowedBots, b.id] : draft.allowedBots.filter(id => id !== b.id) })} /><img src={avatarURL(b.avatar)} alt="" />{b.name}</label>)}</div>{error && <p role="alert" className="bot-error">{error}</p>}</div>}
 </Modal><ConfirmDialog open={Boolean(remove)} title="删除共享资料？" message="删除后不再用于新的对话和任务。已有任务产物保留。" danger loading={busy} onCancel={() => setRemove(null)} onConfirm={async () => { if (!remove) return; setBusy(true); try { await agentsAPI.deleteNote(remove.id); setRemove(null); await reload(); } catch (e) { setError(describeError(e)); setRemove(null); } finally { setBusy(false); } }} /></PageContainer>;
}
