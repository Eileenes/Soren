import { useState } from 'react';
import { Button, IconButton, Modal, TextAreaField, Tooltip } from '../ui';
import { IconEdit } from '../icons';
import { agentsAPI } from '../../services/agents';
import { describeError } from '../../services/api';
export function MemoryEditor({ id, content = '', onSaved, disabled = false }: { id?: number; content?: string; onSaved: () => void; disabled?: boolean }) {
 const [open, setOpen] = useState(false); const [value, setValue] = useState(content); const [busy, setBusy] = useState(false); const [error, setError] = useState('');
 const start = () => { setValue(content); setError(''); setOpen(true); };
 return <>{id ? <Tooltip content="编辑记忆"><IconButton label="编辑记忆" disabled={disabled} onClick={start}><IconEdit size={16} /></IconButton></Tooltip> : <Button disabled={disabled} onClick={start}>添加记忆</Button>}<Modal open={open} onClose={() => { if (!busy) setOpen(false); }} title={id ? '编辑记忆' : '添加记忆'} footer={<Button loading={busy} disabled={!value.trim()} onClick={async () => { setBusy(true); try { if (id) await agentsAPI.editMemory(id, value); else await agentsAPI.addMemory(value); setOpen(false); onSaved(); } catch (e) { setError(describeError(e)); } finally { setBusy(false); } }}>保存记忆</Button>}><TextAreaField label="内容" value={value} rows={6} maxLength={8000} onChange={e => setValue(e.target.value)} />{error && <p role="alert" className="bot-error">{error}</p>}</Modal></>;
}
