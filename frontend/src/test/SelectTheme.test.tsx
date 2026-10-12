import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SelectField, Modal } from '../components/ui';
import { useState } from 'react';
import { useTheme } from '../hooks/useSettings';
import { updateGeneralSettings } from '../services/settingsStore';

function Demo() { const [value, setValue] = useState('one'); return <SelectField label="选择模型" value={value} onChange={setValue} options={[{value:'one',label:'Model One'},{value:'two',label:'Model Two'}]} />; }
function ThemeProbe() { const {theme, mode} = useTheme(); return <span>{mode}:{theme}</span>; }

describe('应用内下拉菜单', () => {
  it('支持键盘选择，Escape 只关闭菜单，不关闭外层弹窗', async () => {
    const close = vi.fn(); const user = userEvent.setup();
    render(<Modal open onClose={close} title="配置 Bot"><Demo /></Modal>);
    const select = screen.getByRole('combobox',{name:'选择模型'});
    await user.click(select); await user.keyboard('{ArrowDown}{Enter}');
    expect(select).toHaveTextContent('Model Two');
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    await user.click(select); await user.keyboard('{Escape}');
    expect(close).not.toHaveBeenCalled();
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
    expect(select).toHaveFocus();
  });
  it('点击选项和点击外部都正确关闭菜单', async () => {
    const user=userEvent.setup(); render(<div><Demo /><button>外部</button></div>);
    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option',{name:'Model Two'}));
    expect(screen.getByRole('combobox')).toHaveTextContent('Model Two');
    await user.click(screen.getByRole('combobox')); await user.click(screen.getByRole('button',{name:'外部'}));
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument();
  });
});

it('跟随系统实时变化，手动选择后不被系统覆盖，清理监听器', () => {
  const original=window.matchMedia; let callback: (()=>void)|undefined;
  const media={matches:false,addEventListener:vi.fn((_:string,cb:()=>void)=>{callback=cb;}),removeEventListener:vi.fn()};
  window.matchMedia=vi.fn(()=>media) as unknown as typeof window.matchMedia;
  try {
    updateGeneralSettings({theme:'system'});
    const view=render(<ThemeProbe />);
    expect(screen.getByText('system:light')).toBeInTheDocument();
    act(()=>{media.matches=true; callback?.();});
    expect(document.documentElement).toHaveClass('dark');
    expect(screen.getByText('system:dark')).toBeInTheDocument();
    act(()=>updateGeneralSettings({theme:'light'}));
    act(()=>{media.matches=false;callback?.();media.matches=true;callback?.();});
    expect(document.documentElement).toHaveClass('light');
    view.unmount(); expect(media.removeEventListener).toHaveBeenCalled();
  } finally {window.matchMedia=original; updateGeneralSettings({theme:'system'});}
});
