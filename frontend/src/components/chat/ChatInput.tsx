/**
 * 消息输入框
 *
 * 默认行为是单聊：Enter 发送、Shift+Enter 换行、loading 时发送按钮变停止按钮。
 * 传入 mentionCandidates 才启用 @ 补全；不传时补全逻辑整体不存在。
 */

import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { IconArrowDown, IconStop } from '../icons';
import { avatarURL } from '../../services/agents';
import { activeMentionQuery, type MentionQuery } from '../../hooks/useGroupChat';

/** @ 补全候选：只需要身份与头像，Bot 与群成员摘要都满足 */
export interface MentionCandidate { id: string; name: string; avatar: string; }

export interface ChatInputProps {
  onSend: (content: string) => void;
  onStop?: () => void;
  disabled?: boolean;
  loading?: boolean;
  placeholder?: string;
  draft?: { text: string; key: number };
  /** 传入则启用 @ 补全，候选为群成员 */
  mentionCandidates?: MentionCandidate[];
  /** 输入框上方的状态区（讨论状态条），与输入框同宽 */
  above?: ReactNode;
}

export function ChatInput({ onSend, onStop, disabled = false, loading = false, placeholder = '给 Soren 发消息…', draft, mentionCandidates, above }: ChatInputProps) {
  const [value, setValue] = useState('');
  const textarea = useRef<HTMLTextAreaElement>(null);
  const [query, setQuery] = useState<MentionQuery | null>(null);
  const [active, setActive] = useState(0);
  const [position, setPosition] = useState({ left: 0, top: 0 });
  const listId = `${useId()}-mentions`;

  useEffect(() => { if (draft) { setValue(draft.text); textarea.current?.focus(); } }, [draft]);
  useEffect(() => { const element = textarea.current; if (element) { element.style.height = 'auto'; element.style.height = Math.min(170, Math.max(62, element.scrollHeight)) + 'px'; } }, [value]);

  const candidates = mentionCandidates ?? [];
  /** 中文名用 includes 过滤：startsWith 在中文里命中率太低 */
  const matches = query
    ? candidates.filter(bot => !query.query || bot.name.toLocaleLowerCase().includes(query.query.toLocaleLowerCase()))
    : [];
  const open = Boolean(query) && matches.length > 0;

  // 浮层浮在输入框正上方：左边缘对齐文字起始位置（composer 17px + textarea 4px）
  useLayoutEffect(() => {
    if (!open || !textarea.current) return;
    const place = () => {
      const r = textarea.current!.getBoundingClientRect();
      setPosition({ left: Math.max(12, r.left + 4), top: Math.max(8, r.top - 8 - Math.min(196, matches.length * 38 + 12)) });
    };
    place();
    window.addEventListener('resize', place);
    window.addEventListener('scroll', place, true);
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true); };
  }, [open, matches.length]);

  useEffect(() => { if (query) setActive(0); }, [query]);

  const syncQuery = (next: string, caret: number) => {
    if (!candidates.length) { setQuery(null); return; }
    setQuery(activeMentionQuery(next, caret));
  };

  const accept = (candidate: MentionCandidate) => {
    if (!query) return;
    const next = `${value.slice(0, query.start)}@${candidate.name} ${value.slice(query.end)}`;
    setValue(next);
    setQuery(null);
    const element = textarea.current;
    const caret = query.start + candidate.name.length + 2;
    window.requestAnimationFrame(() => { element?.focus(); element?.setSelectionRange(caret, caret); });
  };

  const send = () => { if (!value.trim() || disabled || loading) return; onSend(value.trim()); setValue(''); setQuery(null); };

  const keyboard = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    // IME 守卫必须保留：中文输入法确认候选词时的 keydown 不能触发补全选中
    if (event.nativeEvent.isComposing || event.keyCode === 229) return;
    if (open && ['ArrowDown', 'ArrowUp', 'Enter', 'Tab', 'Escape'].includes(event.key)) {
      if (event.key === 'Escape') { event.preventDefault(); setQuery(null); return; }
      event.preventDefault();
      if (event.key === 'Enter' || event.key === 'Tab') { accept(matches[active]); return; }
      setActive(previous => (previous + (event.key === 'ArrowDown' ? 1 : -1) + matches.length) % matches.length);
      return;
    }
    if (event.key === 'Enter' && !event.shiftKey && (window.matchMedia('(pointer: fine)').matches || event.metaKey || event.ctrlKey)) { event.preventDefault(); send(); }
  };

  return <div className="composer-wrap">{above}<div className={'composer ' + (disabled ? 'composer-disabled' : '')}>
    <textarea ref={textarea} aria-label="消息输入框" aria-expanded={open} aria-controls={open ? listId : undefined} aria-activedescendant={open ? `${listId}-${active}` : undefined} value={value} onChange={event => { setValue(event.target.value); syncQuery(event.target.value, event.target.selectionStart ?? event.target.value.length); }} rows={2} placeholder={placeholder} onKeyDown={keyboard} />
    <div className="composer-toolbar"><div className="composer-send-group">{loading && <span>正在思考…</span>}{loading ? <button className="send-button" aria-label="停止生成" onClick={onStop}><IconStop size={15} /></button> : <button className="send-button" aria-label="发送消息" onClick={send} disabled={disabled || !value.trim()}><IconArrowDown size={20} style={{ transform: 'rotate(180deg)' }} /></button>}</div></div>
  </div>
  {open && createPortal(<div id={listId} role="listbox" aria-label="选择要提及的成员" className="app-select-menu" style={{ ...position, width: 280, maxHeight: 196 }} onMouseDown={e => e.preventDefault()}>
    {matches.map((candidate, index) => <div key={candidate.id} id={`${listId}-${index}`} role="option" aria-selected={index === active} className={index === active ? 'app-select-option is-active' : 'app-select-option'} onPointerMove={() => setActive(index)} onClick={() => accept(candidate)}>
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <span className="shrink-0 overflow-hidden rounded-md ring-1 ring-line/70"><img src={avatarURL(candidate.avatar)} alt="" width={28} height={28} /></span>
        <span className="truncate">{candidate.name}</span>
      </span>
    </div>)}
  </div>, document.body)}
  </div>;
}