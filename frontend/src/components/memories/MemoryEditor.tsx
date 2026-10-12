import { useState } from 'react';
import { Button, Modal, TextAreaField } from '../ui';
import { agentsAPI } from '../../services/agents';
import { describeError } from '../../services/api';
export function MemoryEditor({ id, content = '', onSaved }: { id?: number; content?: string; onSaved: () => void }) {
 const [open, setOpen] = useState(false); const [value, setValue] = useState(content); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
 return <><Button size="sm" variant={id ? 'ghost' : 'secondary'} onClick={() => { setValue(content); setError(''); setOpen(true); }}>{id ? '编辑' : '添加记忆'}</Button><Modal open={open} onClose={() => { if (!busy) setOpen(false); }} title={id ? '编辑记忆' : '添加记忆'} footer={<Button loading={busy} disabled={!value.trim()} onClick={async () => { setBusy(true); try { if (id) await agentsAPI.editMemory(id, value); else await agentsAPI.addMemory(value); setOpen(false); onSaved(); } catch (e) { setError(describeError(e)); } finally { setBusy(false); } }}>保存记忆</Button>}><TextAreaField label="内容" value={value} rows={6} maxLength={8000} onChange={e => setValue(e.target.value)} />{error && <p role="alert" className="bot-error">{error}</p>}</Modal></>;
}
