import { useState } from 'react';
import { Button, Modal } from './ui';
import type { ViewId } from '../types';

const STORAGE_KEY = 'soren.getting-started.v1';
const STEPS: {title: string; detail: string; action: string; view: ViewId; optional?: boolean}[] = [
  {title: '连接模型', detail: '在设置中填写接口地址、API Key 和模型名称，保存后先发一句话验证。', action: '去配置模型', view: 'settings'},
  {title: '设置你的 Bot', detail: '选择头像、填写职责。也可以直接使用默认 Soren，之后再创建更多 Bot。', action: '管理 Bot', view: 'bots', optional: true},
  {title: '尝试第一个任务', detail: '让 Bot 制定三天学习计划并保存为 Markdown。展开任务查看工具记录和成果。', action: '填写示例任务', view: 'tasks'},
  {title: '连接常用应用', detail: '按需连接 Gmail 或 Notion，并分配给 Bot。发送邮件等操作需你确认后继续。', action: '去连接应用', view: 'channels', optional: true},
];
type Progress = {seen: boolean; active: boolean; step: number};
function readProgress(): Progress {
  try {
    const value = JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null');
    if (value && typeof value.seen === 'boolean' && typeof value.active === 'boolean' && Number.isInteger(value.step) && value.step >= 0 && value.step < STEPS.length) return value;
  } catch { /* Optional local preferences. */ }
  return {seen: false, active: false, step: 0};
}

export function useGettingStarted() {
  const [progress, setProgress] = useState(readProgress);
  const [open, setOpen] = useState(!progress.seen);
  const save = (next: Progress) => {
    setProgress(next);
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(next)); } catch { /* Still usable without storage. */ }
  };
  const dismiss = () => { setOpen(false); save({...progress, seen: true, active: false}); };
  return {progress, open, setOpen, save, dismiss};
}

export function GettingStarted({guide, modelConfigured, onNavigate}: {
  guide: ReturnType<typeof useGettingStarted>;
  modelConfigured: boolean;
  onNavigate: (view: ViewId, example?: boolean) => void;
}) {
  const {progress, open, setOpen, save, dismiss} = guide;
  const current = STEPS[progress.step];
  const go = (step: number) => {
    save({seen: true, active: true, step});
    setOpen(false);
    onNavigate(STEPS[step].view, step === 2);
  };
  return <>
    <Modal open={open} onClose={dismiss} title="开始使用 Soren" size="lg" footer={<><Button variant="ghost" onClick={dismiss}>跳过引导</Button><Button variant="primary" onClick={() => go(progress.active ? progress.step : 0)}>{progress.active ? '继续引导' : '开始配置'}</Button></>}>
      <div className="welcome-intro"><img src="/app-icon.svg" alt="" width={72} height={72} /><div><p className="welcome-headline">让它成为你的日常助手。</p><p className="text-sm text-ink-2">先连接模型，再交给它第一件事。</p></div></div>
      <ol className="welcome-steps">{STEPS.map((step, index) => <li key={step.title}><span className="welcome-number">{index + 1}</span><div><strong>{step.title}</strong>{step.optional && <small>可选</small>}<p>{step.detail}</p></div></li>)}</ol>
      <p className="welcome-note">本地使用无需登录；未连接模型时仅为演示。手机扫码关联尚未开放。</p>
    </Modal>
    {progress.active && !open && <section className="setup-guide" aria-label="使用引导步骤">
      <div className="setup-guide-copy"><span className="setup-guide-count">{progress.step + 1} / {STEPS.length}</span><div><strong>{current.title}</strong><p>{progress.step === 0 && modelConfigured ? '模型已配置，请先发一句话验证回复。' : current.detail}</p>{progress.step === 2 && !modelConfigured && <p className="text-warning">尚未配置模型，任务将使用演示模式。</p>}</div></div>
      <div className="setup-guide-actions"><Button size="sm" onClick={() => go(progress.step)}>{current.action}</Button>{progress.step > 0 && <Button size="sm" variant="ghost" onClick={() => go(progress.step - 1)}>上一步</Button>}<Button size="sm" variant="ghost" onClick={() => progress.step === STEPS.length - 1 ? dismiss() : go(progress.step + 1)}>{progress.step === STEPS.length - 1 ? '结束引导' : progress.step === 0 && !modelConfigured ? '暂时跳过' : '下一步'}</Button><button className="setup-guide-dismiss" onClick={dismiss}>跳过引导</button></div>
    </section>}
  </>;
}
