import { useEffect, useRef, useState } from 'react';
import { IconArrowDown, IconStop, IconChevronDown, IconPlus, IconClose } from '../icons';
export interface ChatInputProps { onSend: (content: string) => void; onStop?: () => void; disabled?: boolean; loading?: boolean; placeholder?: string; draft?: { text: string; key: number }; model?: string; onModelSettings?: () => void; }
export function ChatInput({ onSend, onStop, disabled = false, loading = false, placeholder = '给 Soren 发消息…', draft, model, onModelSettings }: ChatInputProps) {
  const [value, setValue] = useState('');
  const [showPrompts, setShowPrompts] = useState(false);
  const textarea = useRef<HTMLTextAreaElement>(null);
  useEffect(() => { if (draft) { setValue(draft.text); textarea.current?.focus(); } }, [draft]);
  useEffect(() => { const element = textarea.current; if (element) { element.style.height = 'auto'; element.style.height = Math.min(170, Math.max(62, element.scrollHeight)) + 'px'; } }, [value]);
  const send = () => { if (!value.trim() || disabled || loading) return; onSend(value.trim()); setValue(''); setShowPrompts(false); };
  return <div className="composer-wrap"><div className={'composer ' + (disabled ? 'composer-disabled' : '')}>
    <textarea ref={textarea} aria-label="消息输入框" value={value} onChange={event => setValue(event.target.value)} rows={2} placeholder={placeholder} onKeyDown={event => { if (event.nativeEvent.isComposing || event.keyCode === 229) return; if (event.key === 'Enter' && !event.shiftKey && (window.matchMedia('(pointer: fine)').matches || event.metaKey || event.ctrlKey)) { event.preventDefault(); send(); } }} />
    <div className="composer-toolbar"><div className="composer-tools"><button className="composer-add" aria-label="快捷提示" aria-expanded={showPrompts} onClick={() => setShowPrompts(!showPrompts)}>{showPrompts ? <IconClose size={17} /> : <IconPlus size={17} />}</button><span className="composer-tool-divider" /><button className="model-picker" onClick={onModelSettings} title="配置对话模型"><span className="model-dot" />{model || '选择模型'}<IconChevronDown size={12} /></button></div><div className="composer-send-group"><span>{loading ? '正在思考…' : '↵ 发送'}</span>{loading ? <button className="send-button" aria-label="停止生成" onClick={onStop}><IconStop size={15} /></button> : <button className="send-button" aria-label="发送消息" onClick={send} disabled={disabled || !value.trim()}><IconArrowDown size={20} style={{ transform: 'rotate(180deg)' }} /></button>}</div></div>
    {showPrompts && <div className="composer-prompt-menu">{['帮我理清今天最重要的三件事', '请帮我润色一段文字', '我想告诉你一个我的偏好'].map(text => <button key={text} onClick={() => { setValue(text); setShowPrompts(false); textarea.current?.focus(); }}>{text}</button>)}</div>}
  </div></div>;
}
