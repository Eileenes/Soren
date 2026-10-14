/**
 * Bot 与群聊共用的侧栏右键菜单
 *
 * 复用既有浮层样式体系（`.app-select-menu` / `.app-select-option`，与加号下拉菜单、
 * Select.tsx 同构），不新增任何视觉规格：这些是操作区的常规动作，
 * 与「创建 Bot / 创建群聊」同级，不给专属尺寸、专属底色或危险色——
 * 危险语义由 App 层确认弹窗承担。
 *
 * 键盘模型与加号菜单一致：↑↓ 移动、Home/End 跳到首尾、Enter/空格确认、Esc 关闭。
 * 菜单挂在 document 上监听按键，右键打开时焦点不一定落在菜单内，
 * 但打开后会把焦点移进菜单并用 aria-activedescendant 指向当前项。
 */

import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { IconEdit, IconPin, IconTrash, IconSettings } from './icons';

export interface SidebarContextMenuProps {
  kind: 'bot' | 'group';
  pinned: boolean;
  /** 右键点位的视口坐标 */
  anchor: { x: number; y: number };
  /** 讨论进行中禁止删除：删除会级联清理与在途讨论冲突 */
  deleteDisabled?: boolean;
  deleteDisabledReason?: string;
  onClose: () => void;
  onTogglePin: () => void;
  onRename: () => void;
  onConfigure: () => void;
  onDelete: () => void;
}

/** 与加号菜单同一档宽度；「重命名聊天」5 字 × 13px + 图标 + 内边距实测约 123px */
const MENU_WIDTH = 160;
/** 6px 内边距 + 4 项 × 38px，用于判断上下翻转 */
const MENU_HEIGHT = 164;

export function SidebarContextMenu({ kind, pinned, anchor, deleteDisabled = false, deleteDisabledReason, onClose, onTogglePin, onRename, onConfigure, onDelete }: SidebarContextMenuProps) {
  const menu = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);
  const [position, setPosition] = useState({ left: anchor.x, top: anchor.y });
  const uid = useId();
  const menuId = `${uid}-sidebar-menu`;
  const items = [
    { id: 'pin', label: pinned ? '取消置顶' : '置顶', Icon: IconPin, disabled: false, run: onTogglePin },
    { id: 'configure', label: kind === 'bot' ? '配置 Bot' : '配置群聊', Icon: IconSettings, disabled: false, run: onConfigure },
    { id: 'rename', label: kind === 'bot' ? '重命名 Bot' : '重命名聊天', Icon: IconEdit, disabled: false, run: onRename },
    { id: 'delete', label: '删除', Icon: IconTrash, disabled: deleteDisabled, run: onDelete },
  ];

  // 贴着右键点位展开，越界时夹回视口；下方放不下且上方更宽裕时翻到上方
  useLayoutEffect(() => {
    const width = Math.min(MENU_WIDTH, window.innerWidth - 24);
    const below = window.innerHeight - anchor.y - 12;
    const above = anchor.y - 12;
    setPosition({
      left: Math.max(12, Math.min(anchor.x, window.innerWidth - width - 12)),
      top: below >= MENU_HEIGHT || below >= above ? anchor.y + 6 : Math.max(8, anchor.y - MENU_HEIGHT - 6),
    });
  }, [anchor]);

  useEffect(() => {
    menu.current?.focus();
  }, []);

  useEffect(() => {
    const outside = (event: PointerEvent) => {
      if (!menu.current?.contains(event.target as Node)) onClose();
    };
    document.addEventListener('pointerdown', outside);
    return () => document.removeEventListener('pointerdown', outside);
  }, [onClose]);

  useEffect(() => {
    const step = (delta: number) => setActive(previous => {
      for (let offset = 1; offset <= items.length; offset++) {
        const next = (previous + delta * offset + items.length) % items.length;
        if (!items[next].disabled) return next;
      }
      return previous;
    });
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape' || event.key === 'Tab') { onClose(); return; }
      if (event.key === 'ArrowDown') { event.preventDefault(); step(1); return; }
      if (event.key === 'ArrowUp') { event.preventDefault(); step(-1); return; }
      if (event.key === 'Home') { event.preventDefault(); setActive(0); return; }
      if (event.key === 'End') { event.preventDefault(); setActive(items.length - 1 - [...items].reverse().findIndex(item => !item.disabled)); return; }
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      if (!items[active].disabled) { onClose(); items[active].run(); }
    };
    document.addEventListener('keydown', keyboard);
    return () => document.removeEventListener('keydown', keyboard);
  }, [active, items, onClose]);

  const run = (index: number) => {
    if (items[index].disabled) return;
    onClose();
    items[index].run();
  };

  return createPortal(
    <div ref={menu} id={menuId} role="menu" aria-label={kind === 'bot' ? 'Bot 操作' : '群聊操作'} tabIndex={-1}
      className="app-select-menu" style={{ ...position, width: Math.min(MENU_WIDTH, window.innerWidth - 24) }}
      aria-activedescendant={`${menuId}-${active}`}
      onMouseDown={event => event.preventDefault()}
      onContextMenu={event => event.preventDefault()}>
      {items.map((item, index) => {
        const { Icon } = item;
        return <div key={item.id} id={`${menuId}-${index}`} role="menuitem" aria-disabled={item.disabled || undefined}
          title={item.disabled ? deleteDisabledReason : undefined}
          className={'app-select-option' + (index === active ? ' is-active' : '') + (item.disabled ? ' opacity-40' : '')}
          onPointerMove={() => { if (!item.disabled) setActive(index); }}
          onClick={() => run(index)}>
          <span className="flex min-w-0 flex-1 items-center gap-2">
            <span className="w-4 shrink-0 text-ink-2"><Icon size={16} /></span>
            <span className="truncate">{item.label}</span>
          </span>
        </div>;
      })}
    </div>,
    document.body,
  );
}
