/**
 * Soren 图标集
 *
 * 说明：设计文档推荐 Lucide Icons，但本项目不引入额外依赖（桌面端离线运行时
 * 也无需考虑图标字体/外部 CDN）。这里手写一组 24×24 栅格、1.75 描边的图标，
 * 视觉规格与 Lucide 对齐，尺寸遵循设计文档的 16 / 20 / 24 / 32 规范。
 */

import type { ReactNode, SVGProps } from 'react';

/** 图标统一 props */
export interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'children'> {
  /** 尺寸，默认 20（设计文档"标准图标"） */
  size?: number;
}

/** 生成图标组件，统一 viewBox 与描边属性 */
function createIcon(path: ReactNode, displayName: string) {
  const Icon = ({ size = 20, className, ...rest }: IconProps) => (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {path}
    </svg>
  );
  Icon.displayName = displayName;
  return Icon;
}

/* ── 导航 ───────────────────────────────────────────────── */

export const IconChat = createIcon(
  <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />,
  'IconChat',
);

export const IconBrain = createIcon(
  <>
    <path d="M12 5a3 3 0 0 0-3 3 3 3 0 0 0-2 5.2A3 3 0 0 0 9 19a3 3 0 0 0 3-3z" />
    <path d="M12 5a3 3 0 0 1 3 3 3 3 0 0 1 2 5.2A3 3 0 0 1 15 19a3 3 0 0 1-3-3z" />
  </>,
  'IconBrain',
);

export const IconPlug = createIcon(
  <>
    <path d="M9 2v6" />
    <path d="M15 2v6" />
    <path d="M6 8h12v3a6 6 0 0 1-12 0z" />
    <path d="M12 17v5" />
  </>,
  'IconPlug',
);

export const IconSettings = createIcon(
  <>
    <circle cx="12" cy="12" r="3" />
    <path d="m9.5 3-.6 2.1-1.4.8-2.1-.5L3 9.6l1.5 1.6v1.6L3 14.4l2.4 4.2 2.1-.5 1.4.8.6 2.1h5l.6-2.1 1.4-.8 2.1.5 2.4-4.2-1.5-1.6v-1.6L21 9.6l-2.4-4.2-2.1.5-1.4-.8-.6-2.1Z" />
  </>,
  'IconSettings',
);

/* ── 操作 ───────────────────────────────────────────────── */

export const IconEdit = createIcon(
  <>
    <path d="m16 3 5 5-12 12-6 1 1-6L16 3Z" />
    <path d="m13 6 5 5" />
  </>,
  'IconEdit',
);

export const IconPlus = createIcon(
  <>
    <path d="M12 5v14" />
    <path d="M5 12h14" />
  </>,
  'IconPlus',
);

export const IconSend = createIcon(
  <>
    <path d="M22 2 11 13" />
    <path d="M22 2l-7 20-4-9-9-4z" />
  </>,
  'IconSend',
);

export const IconStop = createIcon(<rect x="6" y="6" width="12" height="12" rx="2" />, 'IconStop');

export const IconCopy = createIcon(
  <>
    <rect x="9" y="9" width="12" height="12" rx="2" />
    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
  </>,
  'IconCopy',
);

export const IconRefresh = createIcon(
  <>
    <path d="M21 12a9 9 0 1 1-2.64-6.36" />
    <path d="M21 3v6h-6" />
  </>,
  'IconRefresh',
);

export const IconTrash = createIcon(
  <>
    <path d="M3 6h18" />
    <path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
    <path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6" />
    <path d="M10 11v6M14 11v6" />
  </>,
  'IconTrash',
);

export const IconSearch = createIcon(
  <>
    <circle cx="11" cy="11" r="7" />
    <path d="m20 20-3.5-3.5" />
  </>,
  'IconSearch',
);

export const IconClose = createIcon(
  <>
    <path d="M18 6 6 18" />
    <path d="m6 6 12 12" />
  </>,
  'IconClose',
);

export const IconCheck = createIcon(<path d="M20 6 9 17l-5-5" />, 'IconCheck');

export const IconPaperclip = createIcon(
  <path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />,
  'IconPaperclip',
);

export const IconArrowDown = createIcon(
  <>
    <path d="M12 5v14" />
    <path d="m19 12-7 7-7-7" />
  </>,
  'IconArrowDown',
);

export const IconChevronRight = createIcon(<path d="m9 18 6-6-6-6" />, 'IconChevronRight');
export const IconChevronDown = createIcon(<path d="m6 9 6 6 6-6" />, 'IconChevronDown');

export const IconFilter = createIcon(
  <path d="M22 3H2l8 9.46V19l4 2v-8.54z" />,
  'IconFilter',
);

export const IconSparkles = createIcon(
  <>
    <path d="M12 3l1.9 4.6L18.5 9.5l-4.6 1.9L12 16l-1.9-4.6L5.5 9.5l4.6-1.9z" />
    <path d="M19 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z" />
  </>,
  'IconSparkles',
);

export const IconZap = createIcon(
  <path d="M13 2 4 14h7l-1 8 9-12h-7z" />,
  'IconZap',
);

/* ── 状态 ───────────────────────────────────────────────── */

export const IconAlert = createIcon(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 8v5" />
    <path d="M12 16.5h.01" />
  </>,
  'IconAlert',
);

export const IconInbox = createIcon(
  <>
    <path d="M3 12h5l2 3h4l2-3h5" />
    <path d="M5.5 5h13l3.5 7v5a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-5z" />
  </>,
  'IconInbox',
);

export const IconMessageSquare = createIcon(
  <>
    <path d="M21 15a2 2 0 0 1-2 2H8l-5 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    <path d="M8 9h8M8 13h5" />
  </>,
  'IconMessageSquare',
);

export const IconUser = createIcon(
  <>
    <circle cx="12" cy="8" r="4" />
    <path d="M4 21a8 8 0 0 1 16 0" />
  </>,
  'IconUser',
);

export const IconTag = createIcon(
  <>
    <path d="M20.6 13.4 12 22l-9-9V3h10l7.6 7.6a2 2 0 0 1 0 2.8z" />
    <circle cx="7.5" cy="7.5" r="1.2" />
  </>,
  'IconTag',
);

export const IconClock = createIcon(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 7v5l3 2" />
  </>,
  'IconClock',
);

export const IconDatabase = createIcon(
  <>
    <ellipse cx="12" cy="5" rx="8" ry="3" />
    <path d="M4 5v14c0 1.7 3.6 3 8 3s8-1.3 8-3V5" />
    <path d="M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3" />
  </>,
  'IconDatabase',
);

export const IconActivity = createIcon(<path d="M22 12h-4l-3 8-4-16-3 8H2" />, 'IconActivity');

export const IconMoon = createIcon(
  <path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />,
  'IconMoon',
);

export const IconSun = createIcon(
  <>
    <circle cx="12" cy="12" r="4" />
    <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
  </>,
  'IconSun',
);

export const IconGlobe = createIcon(
  <>
    <circle cx="12" cy="12" r="9" />
    <path d="M3 12h18" />
    <path d="M12 3a15 15 0 0 1 0 18a15 15 0 0 1 0-18z" />
  </>,
  'IconGlobe',
);

export const IconTerminal = createIcon(
  <>
    <path d="m5 8 4 4-4 4" />
    <path d="M13 16h6" />
    <rect x="2" y="3" width="20" height="18" rx="2" />
  </>,
  'IconTerminal',
);

export const IconRobot = createIcon(
  <>
    <rect x="4" y="8" width="16" height="12" rx="3" />
    <path d="M12 4v4" />
    <circle cx="12" cy="3" r="1.4" />
    <path d="M9 13h.01M15 13h.01" />
    <path d="M9.5 16.5h5" />
  </>,
  'IconRobot',
);

export const IconKey = createIcon(
  <>
    <circle cx="8" cy="15" r="4" />
    <path d="m10.8 12.2 8-8" />
    <path d="m17 4 3 3" />
    <path d="m14.5 6.5 3 3" />
  </>,
  'IconKey',
);

export const IconSparkle = createIcon(
  <path d="M12 3l2.2 5.3L19.5 10.5l-5.3 2.2L12 18l-2.2-5.3L4.5 10.5l5.3-2.2z" />,
  'IconSparkle',
);

export const IconPin = createIcon(
  <>
    <path d="M12 21s7-6.2 7-11a7 7 0 1 0-14 0c0 4.8 7 11 7 11z" />
    <circle cx="12" cy="10" r="2.5" />
  </>,
  'IconPin',
);

export const IconList = createIcon(
  <>
    <path d="M8 6h13M8 12h13M8 18h13" />
    <path d="M3.5 6h.01M3.5 12h.01M3.5 18h.01" />
  </>,
  'IconList',
);

export const IconWifi = createIcon(
  <>
    <path d="M2.5 9a15 15 0 0 1 19 0" />
    <path d="M5.5 12.5a10 10 0 0 1 13 0" />
    <path d="M8.5 16a5 5 0 0 1 7 0" />
    <path d="M12 19.5h.01" />
  </>,
  'IconWifi',
);

export const IconWifiOff = createIcon(
  <>
    <path d="M2 2l20 20" />
    <path d="M8.5 16a5 5 0 0 1 7 0" />
    <path d="M5.5 12.5a10 10 0 0 1 4-2.3" />
    <path d="M2.5 9a15 15 0 0 1 6-3.7" />
    <path d="M12 19.5h.01" />
  </>,
  'IconWifiOff',
);

export const IconWindowMinimize = createIcon(<path d="M5 12h14" />, 'IconWindowMinimize');

export const IconWindowMaximize = createIcon(
  <rect x="5" y="5" width="14" height="14" rx="2" />,
  'IconWindowMaximize',
);

export const IconHelp = createIcon(
  <><circle cx="12" cy="12" r="9" /><path d="M9.5 9a2.5 2.5 0 0 1 5 .5c0 1.5-2.5 2-2.5 3.5" /><path d="M12 16.5h.01" /></>,
  'IconHelp',
);
