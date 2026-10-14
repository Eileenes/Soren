/**
 * 创建群聊弹窗
 *
 * 复用既有外壳与控件：Modal（焦点陷阱、Esc 关闭）、TextField（.field）、
 * .avatar-picker 成员网格、.bot-filter 分段控件。不新增任何视觉体系。
 *
 * 提交可用性由数据决定：群名为空或成员不足 2 个时按钮禁用，不做点击后报错；
 * 服务端 400 落回对应字段，非字段类错误（500、网络）由调用方弹 Toast。
 */

import { useEffect, useState } from 'react';
import { avatarURL, type Bot } from '../services/agents';
import { IconCheck } from './icons';
import { Button, Modal, TextField } from './ui';
import {
  GROUP_DEFAULT_ROUNDS,
  GROUP_MAX_MEMBERS,
  GROUP_MAX_ROUNDS,
  GROUP_MIN_MEMBERS,
} from '../hooks/useGroupChat';
import type { GroupCreateInput, GroupMode } from '../types';

/** 后端上限是 100 字群名 / 2000 字主题，这里留出更紧的前置约束，避免侧栏与弹窗被长文本撑开 */
const TITLE_MAX = 50;
const TOPIC_MAX = 200;

export function CreateGroupDialog({ open, bots, onClose, onCreate }: {
  open: boolean;
  /** 已过滤 archived 的 Bot 列表 */
  bots: Bot[];
  onClose: () => void;
  onCreate: (input: GroupCreateInput) => Promise<void>;
}) {
  const [title, setTitle] = useState('');
  const [topic, setTopic] = useState('');
  const [selected, setSelected] = useState<string[]>([]);
  const [mode, setMode] = useState<GroupMode>('mention');
  const [maxRounds, setMaxRounds] = useState(GROUP_DEFAULT_ROUNDS);
  const [titleError, setTitleError] = useState('');
  const [topicError, setTopicError] = useState('');
  const [memberError, setMemberError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) return;
    // 不预填任何字段：创建群聊是明确动作，预填等于替用户做决定
    setTitle(''); setTopic(''); setSelected([]); setMode('mention');
    setMaxRounds(GROUP_DEFAULT_ROUNDS); setTitleError(''); setTopicError(''); setMemberError(''); setSubmitting(false);
  }, [open]);

  const full = selected.length >= GROUP_MAX_MEMBERS;
  const selectable = bots.filter(bot => bot.status === 'active');
  /** 只按「能不能建」禁用；超长在提交时就地报错，不静默截断用户输入 */
  const canSubmit = title.trim().length > 0 && selected.length >= GROUP_MIN_MEMBERS && selectable.length > 0;

  const toggle = (id: string) => {
    setMemberError('');
    setSelected(previous => previous.includes(id)
      ? previous.filter(item => item !== id)
      : full ? previous : [...previous, id]);
  };

  const submit = async () => {
    const name = title.trim();
    const subject = topic.trim();
    if (!name) { setTitleError('请填写群名'); return; }
    if (name.length > TITLE_MAX) { setTitleError(`群名最多 ${TITLE_MAX} 字`); return; }
    if (subject.length > TOPIC_MAX) { setTopicError(`群主题最多 ${TOPIC_MAX} 字`); return; }
    if (selected.length < GROUP_MIN_MEMBERS) { setMemberError(`至少选择 ${GROUP_MIN_MEMBERS} 个成员`); return; }
    setTitleError(''); setTopicError(''); setMemberError(''); setSubmitting(true);
    try {
      await onCreate({ title: name, topic: subject, botIds: selected, mode, maxRounds });
    } finally {
      setSubmitting(false);
    }
  };

  return <Modal open={open} onClose={onClose} size="md" title="创建群聊" footer={<>
    <Button variant="ghost" onClick={onClose}>取消</Button>
    <Button variant="primary" onClick={() => void submit()} disabled={!canSubmit} loading={submitting}>创建群聊</Button>
  </>}>
    <div className="flex flex-col gap-4">
      {/* 必填星号会混进 label 的可访问名，用 aria-label 给出干净的名字。
          长度不在输入端截断：超长时提交被拦下并就地报错，用户能看见自己写超了多少 */}
      <TextField label="群名" required aria-label="群名" value={title} error={titleError}
        placeholder="例如：产品评审"
        onChange={event => { setTitle(event.target.value); if (titleError) setTitleError(''); }} />

      <TextField label="群主题" aria-label="群主题" value={topic} error={topicError} placeholder="这群 Bot 一起讨论什么"
        onChange={event => { setTopic(event.target.value); if (topicError) setTopicError(''); }} />

      <div>
        <div className="mb-1.5 flex items-center gap-3">
          <span className="text-sm font-medium text-ink-1">成员</span>
          <span className="text-sm text-ink-1">{selected.length}<span className="text-ink-3">/{GROUP_MAX_MEMBERS}</span></span>
          {selected.length < GROUP_MIN_MEMBERS && memberError && <span className="text-xs text-error">{memberError}</span>}
        </div>
        {selectable.length === 0
          ? <p className="text-sm text-ink-3">没有可用的 Bot</p>
          : <div className="avatar-picker">
            {selectable.map(bot => {
              const on = selected.includes(bot.id);
              const blocked = !on && full;
              return <button key={bot.id} type="button" aria-pressed={on} aria-disabled={blocked || undefined} title={bot.name}
                onClick={() => { if (!blocked) toggle(bot.id); }}
                className={'relative gap-1 ' + (blocked ? 'cursor-not-allowed opacity-40' : '')}>
                <img src={avatarURL(bot.avatar)} alt="" />
                <span className={'w-full truncate text-center text-xs ' + (on ? 'text-ink-1' : 'text-ink-2')}>{bot.name}</span>
                {on && <span className="absolute right-1 top-1 grid h-[18px] w-[18px] place-items-center rounded bg-[var(--card)] text-[var(--accent)]"><IconCheck size={14} /></span>}
              </button>;
            })}
          </div>}
      </div>

      <div>
        <span className="mb-1.5 block text-sm font-medium text-ink-1">模式</span>
        <div className="bot-filter !mb-0">
          <button aria-pressed={mode === 'mention'} onClick={() => setMode('mention')}>提及</button>
          <button aria-pressed={mode === 'auto'} onClick={() => setMode('auto')}>自主</button>
        </div>
        <p className="mt-2 text-xs text-ink-3">{mode === 'mention' ? '你 @ 谁，谁回答' : '群内 Bot 依次自主发言'}</p>
      </div>

      {/* mention 模式忽略轮次上限，露出这段就是无效控件，所以整段卸载 */}
      {mode === 'auto' && <div>
        <div className="mb-1.5 flex items-center justify-between gap-3">
          <span className="text-sm font-medium text-ink-1">轮次上限</span>
          <span className="text-xs text-ink-3">最多 {selected.length || GROUP_MIN_MEMBERS} 位成员 × {maxRounds} 轮 = {(selected.length || GROUP_MIN_MEMBERS) * maxRounds} 次模型调用</span>
        </div>
        <div className="bot-filter !mb-0">
          {Array.from({ length: GROUP_MAX_ROUNDS }, (_, index) => index + 1).map(rounds => (
            <button key={rounds} aria-pressed={maxRounds === rounds} onClick={() => setMaxRounds(rounds)}>{rounds}</button>
          ))}
        </div>
      </div>}
    </div>
  </Modal>;
}