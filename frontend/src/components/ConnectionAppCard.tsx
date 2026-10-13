import type { ReactNode } from 'react';
import { Button } from './ui';

export function ConnectionAppCard({ title, description, icon, detail, onAdd }: {
  title: string; description: string; icon: ReactNode; detail?: string; onAdd: () => void;
}) {
  return <article className="application-card">
    <span className="application-icon">{icon}</span>
    <div className="application-copy"><h2>{title}</h2><p>{description}</p>{detail && <small>{detail}</small>}</div>
    <Button size="sm" variant="secondary" aria-label={`添加 ${title}`} onClick={onAdd}>添加</Button>
  </article>;
}
