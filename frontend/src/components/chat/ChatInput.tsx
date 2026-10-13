import { useEffect, useRef, useState } from 'react';
import { IconArrowDown, IconStop } from '../icons';
export interface ChatInputProps { onSend: (content: string) => void; onStop?: () => void; disabled?: boolean; loading?: boolean; placeholder?: string; draft?: { text: string; key: number }; }
export function ChatInput({ onSend, onStop, disabled = false, loading = false, placeholder = '给 Soren 发消息…', draft }: ChatInputProps) {
  const [value, setValue] = useState('');
  const textarea = useRef<HTMLTextAreaElement>(null);
  useEffect(() => { if (draft) { setValue(draft.text); textarea.current?.focus(); } }, [draft]);
  useEffect(() => { const element = textarea.current; if (element) { element.style.height = 'auto'; element.style.height = Math.min(170, Math.max(62, element.scrollHeight)) + 'px'; } }, [value]);
  const send = () => { if (!value.trim() || disabled || loading) return; onSend(value.trim()); setValue(''); };
  return <div className="composer-wrap"><div className={'composer ' + (disabled ? 'composer-disabled' : '')}>
    <textarea ref={textarea} aria-label="消息输入框" value={value} onChange={event => setValue(event.target.value)} rows={2} placeholder={placeholder} onKeyDown={event => { if (event.nativeEvent.isComposing || event.keyCode === 229) return; if (event.key === 'Enter' && !event.shiftKey && (window.matchMedia('(pointer: fine)').matches || event.metaKey || event.ctrlKey)) { event.preventDefault(); send(); } }} />
    <div className="composer-toolbar"><div className="composer-send-group">{loading && <span>正在思考…</span>}{loading ? <button className="send-button" aria-label="停止生成" onClick={onStop}><IconStop size={15} /></button> : <button className="send-button" aria-label="发送消息" onClick={send} disabled={disabled || !value.trim()}><IconArrowDown size={20} style={{ transform: 'rotate(180deg)' }} /></button>}</div></div>
  </div></div>;
}
