/**
 * Bot 与群聊共用的重命名弹窗
 *
 * 复用 CreateGroupDialog 的同一套外壳与控件（Modal + TextField `.field`），
 * 不新增视觉规格。只有一个输入框，所以不做话题、成员等其余字段。
 *
 * 提交交给调用方（App 层）执行，这里只负责表单状态：空名与超长在提交时就地报错，
 * 不静默截断；其余失败也留在弹窗内，界面不丢上下文。
 */

import { useState } from 'react';
import { describeError } from '../services/api';
import { Button, Modal, TextField } from './ui';

export function RenameChatDialog({ open, kind, initialTitle, onClose, onRename }: {
  open: boolean;
  kind: 'bot' | 'group';
  /** 当前群名，打开时预填；用户在此基础上改，不是从空开始 */
  initialTitle: string;
  onClose: () => void;
  onRename: (title: string) => Promise<void>;
}) {
  const [title, setTitle] = useState(initialTitle);
  const [titleError, setTitleError] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const label = kind === 'bot' ? 'Bot 名称' : '群名';
  const maxLength = kind === 'bot' ? 40 : 50;

  const canSubmit = title.trim().length > 0 && !submitting;

  const submit = async () => {
    const name = title.trim();
    if (!name) { setTitleError(`请填写${label}`); return; }
    if ([...name].length > maxLength) { setTitleError(`${label}最多 ${maxLength} 字`); return; }
    if (name === initialTitle) { onClose(); return; }
    setTitleError(''); setError(''); setSubmitting(true);
    try {
      await onRename(name);
    } catch (err) {
      setError(describeError(err));
      setSubmitting(false);
    }
  };

  return <Modal open={open} onClose={submitting ? () => undefined : onClose} size="sm" title={kind === 'bot' ? '重命名 Bot' : '重命名聊天'} footer={<>
    <Button variant="ghost" onClick={onClose} disabled={submitting}>取消</Button>
    <Button variant="primary" onClick={() => void submit()} disabled={!canSubmit} loading={submitting}>保存</Button>
  </>}>
    <div className="flex flex-col gap-2">
      <TextField label={label} required aria-label={label} value={title} error={titleError}
        placeholder={kind === 'bot' ? '例如：写作助手' : '例如：产品评审'}
        onFocus={event => event.currentTarget.select()}
        onChange={event => { setTitle(event.target.value); if (titleError) setTitleError(''); }} />
      {error && <span role="alert" className="text-xs text-error">{error}</span>}
    </div>
  </Modal>;
}
