import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import { IconCheck, IconChevronDown } from '../icons';
import { cn } from '../../utils/format';

export interface SelectProps<T extends string = string> {
  id?: string; label?: string; value: T; options: ReadonlyArray<{value: T; label: string; hint?: string}>;
  onChange: (value: T) => void; disabled?: boolean; required?: boolean; placeholder?: string;
  className?: string; 'aria-label'?: string; 'aria-describedby'?: string; 'aria-invalid'?: boolean;
}
/** App-owned listbox; focus stays on the trigger, including inside modal focus traps. */
export function Select<T extends string>({id, label, value, options, onChange, disabled, required, placeholder = '请选择', className, ...aria}: SelectProps<T>) {
  const uid = useId(); const listId = uid + '-options';
  const trigger = useRef<HTMLButtonElement>(null); const menu = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false); const [active, setActive] = useState(0);
  const [position, setPosition] = useState({left: 0, top: 0, width: 0, maxHeight: 260});
  const typed = useRef({text: '', time: 0});
  const selected = options.find(option => option.value === value);
  const show = () => { if (!disabled && options.length) { setActive(Math.max(0, options.findIndex(o => o.value === value))); setOpen(true); } };
  useLayoutEffect(() => {
    if (!open) return;
    const place = () => {
      const r = trigger.current!.getBoundingClientRect();
      const below = window.innerHeight - r.bottom - 12; const above = r.top - 12;
      const height = Math.min(280, Math.max(80, below >= 180 || below >= above ? below : above));
      const width = Math.min(Math.max(r.width, 180), window.innerWidth - 24);
      setPosition({left: Math.max(12, Math.min(r.left, window.innerWidth - width - 12)), top: below >= 180 || below >= above ? r.bottom + 6 : Math.max(8, r.top - Math.min(height, options.length * 44 + 12) - 6), width, maxHeight: height});
    };
    place(); window.addEventListener('resize', place); window.addEventListener('scroll', place, true);
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true); };
  }, [open, options.length]);
  useEffect(() => {
    if (!open) return;
    const outside = (e: PointerEvent) => { if (!trigger.current?.contains(e.target as Node) && !menu.current?.contains(e.target as Node)) setOpen(false); };
    document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [open]);
  useEffect(() => { if (disabled) setOpen(false); }, [disabled]);
  useEffect(() => { if (open) document.getElementById(`${listId}-${active}`)?.scrollIntoView({block: 'nearest'}); }, [active, open, listId]);
  const choose = (index: number) => { const option = options[index]; if (option) onChange(option.value); setOpen(false); trigger.current?.focus(); };
  const keyboard = (event: KeyboardEvent<HTMLButtonElement>) => {
    const key = event.key;
    if (key === 'Escape' && open) { event.preventDefault(); event.stopPropagation(); setOpen(false); return; }
    if (key === 'Tab') { setOpen(false); return; }
    if (['ArrowDown','ArrowUp','Home','End','Enter',' '].includes(key)) {
      event.preventDefault(); event.stopPropagation();
      if (!open) { show(); return; }
      if (key === 'Enter' || key === ' ') { choose(active); return; }
      setActive(index => key === 'Home' ? 0 : key === 'End' ? options.length - 1 : (index + (key === 'ArrowDown' ? 1 : -1) + options.length) % options.length);
    } else if (key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey) {
      typed.current.text = Date.now() - typed.current.time > 700 ? key : typed.current.text + key;
      typed.current.time = Date.now();
      const index = options.findIndex(o => o.label.toLocaleLowerCase().startsWith(typed.current.text.toLocaleLowerCase()));
      if (index >= 0) { event.preventDefault(); setOpen(true); setActive(index); }
    }
  };
  return <><button ref={trigger} id={id} type="button" role="combobox" aria-label={aria['aria-label'] || label} aria-describedby={aria['aria-describedby']} aria-invalid={aria['aria-invalid']} aria-required={required} aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? listId : undefined} aria-activedescendant={open ? `${listId}-${active}` : undefined} disabled={disabled || !options.length} className={cn('app-select-trigger field', className)} onKeyDown={keyboard} onBlur={() => setOpen(false)} onClick={() => open ? setOpen(false) : show()}><span className="truncate">{selected?.label || value || placeholder}</span><IconChevronDown size={14} /></button>
    {open && createPortal(<div ref={menu} id={listId} role="listbox" aria-label={aria['aria-label'] || label || '选项'} className="app-select-menu" style={position} onMouseDown={e => e.preventDefault()}>{options.map((option, index) => <div role="option" id={`${listId}-${index}`} key={option.value} aria-selected={option.value === value} className={cn('app-select-option', index === active && 'is-active')} onPointerMove={() => setActive(index)} onClick={() => choose(index)}><div><span>{option.label}</span>{option.hint && <small>{option.hint}</small>}</div>{option.value === value && <IconCheck size={14} />}</div>)}</div>, document.body)}
  </>;
}
