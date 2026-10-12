import { cn } from '../utils/format';
export function Logomark({ size = 36, className, interactive = false }: { size?: number; className?: string; interactive?: boolean }) {
  return <img src="/soren-mark.svg" alt="" width={size} height={size} className={cn('soren-mark', interactive && 'mark-interactive', className)} draggable={false} />;
}
export function Logo({ size = 36, showWordmark = true, subtitle, className }: { size?: number; showWordmark?: boolean; subtitle?: string; className?: string; interactive?: boolean }) {
  return <span className={cn('brand', className)}><Logomark size={size} />{showWordmark && <span><span className="brand-wordmark">Soren<span className="brand-period">.</span></span>{subtitle && <small>{subtitle}</small>}</span>}</span>;
}
export function LogoHero({ size = 116, className }: { size?: number; className?: string }) {
  return <div className={cn('hero-mascot', className)}><div className="mascot-halo" /><Logomark size={size} /><span className="mascot-spark spark-one">✧</span><span className="mascot-spark spark-two">✦</span></div>;
}
