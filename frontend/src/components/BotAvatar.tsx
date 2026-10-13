import { useEffect, useId, useRef } from 'react';
import { avatarURL } from '../services/agents';

// Eye bands and face colors match the built-in artwork in public/bot-avatars.
// Only the eye pixels are compressed during a blink; the head keeps its shape.
const faces: Record<string, { color: string; y: number; height: number }> = {
  robo: { color: '#43c2ae', y: 94, height: 48 },
  iceberg: { color: '#5fb8e8', y: 106, height: 32 },
  pudin: { color: '#e9a63f', y: 105, height: 30 },
  rex: { color: '#3f8c3c', y: 110, height: 34 },
  hydrant: { color: '#e24530', y: 99, height: 42 },
  angler: { color: '#2e4a8f', y: 97, height: 56 },
  yurei: { color: '#e4ebe7', y: 89, height: 57 },
  flamingo: { color: '#f87ba6', y: 96, height: 51 },
  saboten: { color: '#9cc63c', y: 107, height: 41 },
  tako: { color: '#7b4bb5', y: 92, height: 54 },
};

export function BotAvatar({ avatar, running = false }: { avatar: string; running?: boolean }) {
  const id = useId();
  const head = useRef<HTMLSpanElement>(null);
  const eyes = useRef<SVGGElement>(null);
  const face = faces[avatar];
  const source = avatarURL(avatar);
  const band = { x: 68, y: face?.y ?? 98, width: 119, height: face?.height ?? 40 };

  useEffect(() => {
    if (!running || !head.current?.animate || !eyes.current?.animate) return;
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const timers = new Set<ReturnType<typeof setTimeout>>();
    const animations = new Set<Animation>();
    const later = (action: () => void, delay: number) => {
      const timer = setTimeout(() => { timers.delete(timer); action(); }, delay);
      timers.add(timer);
    };
    const play = (element: Element | null, frames: Keyframe[], duration: number) => {
      if (!element) return;
      const animation = element.animate(frames, { duration, easing: 'ease-in-out' });
      animations.add(animation);
      animation.onfinish = () => animations.delete(animation);
    };
    const blink = () => {
      const frames = [
        { transform: 'scaleY(1)', offset: 0 },
        { transform: 'scaleY(.08)', offset: .38 },
        { transform: 'scaleY(.08)', offset: .53 },
        { transform: 'scaleY(1)', offset: 1 },
      ];
      play(eyes.current, frames, 150 + Math.random() * 70);
      if (Math.random() < .28) later(() => play(eyes.current, frames, 180), 290);
      later(blink, 1800 + Math.random() * 4300);
    };
    const move = () => {
      const side = Math.random() < .5 ? -1 : 1;
      const tilt = side * (5 + Math.random() * 5);
      const gestures = [
        ['none', `rotate(${tilt}deg) translateY(-1px)`, `rotate(${tilt * .8}deg)`, 'none'],
        ['none', 'translateY(1.5px) rotate(-3deg)', 'translateY(-1px) rotate(2deg)', 'none'],
        ['none', `rotate(${tilt}deg)`, `rotate(${-tilt * .65}deg)`, `rotate(${tilt * .3}deg)`, 'none'],
      ];
      play(head.current, gestures[Math.floor(Math.random() * gestures.length)].map(transform => ({ transform })), 850 + Math.random() * 600);
      later(move, 2500 + Math.random() * 4100);
    };
    const stop = () => {
      timers.forEach(clearTimeout); timers.clear();
      animations.forEach(animation => animation.cancel()); animations.clear();
    };
    const sync = () => {
      stop();
      if (reducedMotion.matches || document.hidden) return;
      later(blink, 350 + Math.random() * 1600);
      later(move, 200 + Math.random() * 1200);
    };
    sync();
    reducedMotion.addEventListener('change', sync);
    document.addEventListener('visibilitychange', sync);
    return () => {
      stop();
      reducedMotion.removeEventListener('change', sync);
      document.removeEventListener('visibilitychange', sync);
    };
  }, [running, avatar]);

  return <span className="inbox-avatar" aria-hidden="true"><span ref={head} className="bot-avatar-head">
    {running ? <svg width="36" height="36" viewBox="0 0 256 256" className="bot-avatar-art">
      <defs>
        <clipPath id={`${id}-eyes`}><rect {...band} /></clipPath>
        {!face && <>
          <linearGradient id={`${id}-face`} x1="73" y1="47" x2="188" y2="205" gradientUnits="userSpaceOnUse">
            <stop stopColor="#B8EFDC" /><stop offset=".48" stopColor="#86DCC3" /><stop offset="1" stopColor="#E7EAB9" />
          </linearGradient>
          <radialGradient id={`${id}-glow`} cx="0" cy="0" r="1" gradientTransform="translate(92 65) rotate(70) scale(116 114)" gradientUnits="userSpaceOnUse">
            <stop stopColor="#F7FFF0" stopOpacity=".68" /><stop offset="1" stopColor="#F7FFF0" stopOpacity="0" />
          </radialGradient>
        </>}
      </defs>
      <image href={source} width="256" height="256" />
      <rect {...band} fill={face?.color ?? `url(#${id}-face)`} />
      {!face && <rect {...band} fill={`url(#${id}-glow)`} />}
      <g ref={eyes} className="bot-avatar-eyes" style={{ transformOrigin: `128px ${band.y + band.height / 2}px` }}>
        <image href={source} width="256" height="256" clipPath={`url(#${id}-eyes)`} />
      </g>
    </svg> : <img src={source} alt="" />}
  </span></span>;
}
