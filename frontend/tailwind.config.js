/** @type {import('tailwindcss').Config} */
export default {
  // 深色模式通过 <html class="dark"> 切换，由 useTheme 统一控制
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // ── 品牌色（docs/design-system.md）─────────────────────────
        primary: {
          start: '#368773',
          end: '#25564b',
          DEFAULT: '#368773',
        },
        accent: {
          pink: '#bc9384',
          coral: '#cf8575',
        },

        // ── 语义化表面 / 文本 / 描边（由 CSS 变量驱动，随主题自动切换）──
        // 深色为默认基调，浅色模式走同一套类名，无需写两遍
        surface: {
          1: 'rgb(var(--soren-surface-1) / <alpha-value>)',
          2: 'rgb(var(--soren-surface-2) / <alpha-value>)',
          3: 'rgb(var(--soren-surface-3) / <alpha-value>)',
          4: 'rgb(var(--soren-surface-4) / <alpha-value>)',
        },
        ink: {
          1: 'rgb(var(--soren-ink-1) / <alpha-value>)',
          2: 'rgb(var(--soren-ink-2) / <alpha-value>)',
          3: 'rgb(var(--soren-ink-3) / <alpha-value>)',
        },
        line: {
          subtle: 'rgb(var(--soren-line-subtle) / <alpha-value>)',
          DEFAULT: 'rgb(var(--soren-line) / <alpha-value>)',
          strong: 'rgb(var(--soren-line-strong) / <alpha-value>)',
        },

        // ── 兼容层：保留设计文档原始 token 名称 ────────────────────
        bg: {
          primary: 'rgb(var(--soren-surface-1) / <alpha-value>)',
          secondary: 'rgb(var(--soren-surface-2) / <alpha-value>)',
          tertiary: 'rgb(var(--soren-surface-3) / <alpha-value>)',
          dark: '#1a1b1e',
        },
        text: {
          primary: 'rgb(var(--soren-ink-1) / <alpha-value>)',
          secondary: 'rgb(var(--soren-ink-2) / <alpha-value>)',
          tertiary: 'rgb(var(--soren-ink-3) / <alpha-value>)',
          inverse: '#ffffff',
        },
        border: {
          light: 'rgb(var(--soren-line-subtle) / <alpha-value>)',
          medium: 'rgb(var(--soren-line) / <alpha-value>)',
          dark: 'rgb(var(--soren-line-strong) / <alpha-value>)',
        },

        // ── 语义色 ────────────────────────────────────────────────
        success: '#368773',
        warning: '#b38b3d',
        error: '#d06b63',
        info: '#589aa6',
      },

      fontFamily: {
        sans: ['Avenir Next', 'Avenir', 'PingFang SC', 'Microsoft YaHei', 'Noto Sans SC', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'SFMono-Regular', 'Consolas', 'monospace'],
      },

      fontSize: {
        xs: ['12px', { lineHeight: '16px' }],
        sm: ['14px', { lineHeight: '20px' }],
        base: ['16px', { lineHeight: '24px' }],
        lg: ['18px', { lineHeight: '28px' }],
        xl: ['20px', { lineHeight: '28px' }],
        '2xl': ['24px', { lineHeight: '32px' }],
        '3xl': ['30px', { lineHeight: '36px' }],
      },

      fontWeight: {
        normal: '400',
        medium: '500',
        semibold: '600',
        bold: '700',
      },

      spacing: {
        '1': '4px',
        '2': '8px',
        '3': '12px',
        '4': '16px',
        '5': '20px',
        '6': '24px',
        '8': '32px',
        '10': '40px',
        '12': '48px',
      },

      borderRadius: {
        'sm': '4px',
        DEFAULT: '8px',
        'md': '12px',
        'lg': '16px',
        'xl': '20px',
        'full': '9999px',
      },

      boxShadow: {
        'sm': '0 1px 2px 0 rgba(0, 0, 0, 0.05)',
        DEFAULT: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)',
        'md': '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)',
        'lg': '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)',
        'xl': '0 25px 50px -12px rgba(0, 0, 0, 0.25)',
        'none': 'none',
        // 科技感光晕
        'glow': '0 0 0 1px rgb(54 135 115 / 0.35), 0 8px 32px -8px rgb(54 135 115 / 0.45)',
        'glow-lg': '0 0 0 1px rgb(54 135 115 / 0.4), 0 18px 60px -12px rgb(80 130 108 / 0.55)',
        'ring-inset': 'inset 0 1px 0 0 rgb(255 255 255 / 0.06)',
      },

      transitionDuration: {
        'fast': '150ms',
        'base': '250ms',
        'slow': '350ms',
      },

      transitionTimingFunction: {
        'ease': 'cubic-bezier(0.4, 0.0, 0.2, 1)',
        'ease-in': 'cubic-bezier(0.4, 0.0, 1, 1)',
        'ease-out': 'cubic-bezier(0.0, 0.0, 0.2, 1)',
        'ease-bounce': 'cubic-bezier(0.68, -0.55, 0.265, 1.55)',
      },

      keyframes: {
        fadeIn: {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        slideUp: {
          from: { opacity: '0', transform: 'translateY(10px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        slideDown: {
          from: { opacity: '0', transform: 'translateY(-8px)' },
          to: { opacity: '1', transform: 'translateY(0)' },
        },
        scale: {
          from: { transform: 'scale(0.95)' },
          to: { transform: 'scale(1)' },
        },
        // 光晕呼吸：用于 logo、连接状态指示灯
        glowPulse: {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%': { opacity: '0.72', transform: 'scale(1.06)' },
        },
        // 漂浮：空状态 logo
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-8px)' },
        },
        // 流光：骨架屏、品牌渐变扫光
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        // 渐变位移动画：主按钮、标题
        gradientX: {
          '0%, 100%': { backgroundPosition: '0% 50%' },
          '50%': { backgroundPosition: '100% 50%' },
        },
        // 打字光标
        caretBlink: {
          '0%, 45%': { opacity: '1' },
          '50%, 95%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        // 三点跳动
        bounceDot: {
          '0%, 60%, 100%': { transform: 'translateY(0)', opacity: '0.45' },
          '30%': { transform: 'translateY(-4px)', opacity: '1' },
        },
        // 旋转光环
        spinSlow: {
          from: { transform: 'rotate(0deg)' },
          to: { transform: 'rotate(360deg)' },
        },
        // 进度条生长
        growWidth: {
          from: { transform: 'scaleX(0)' },
          to: { transform: 'scaleX(1)' },
        },
      },

      animation: {
        'fade-in': 'fadeIn 250ms cubic-bezier(0, 0, 0.2, 1) both',
        'slide-up': 'slideUp 350ms cubic-bezier(0, 0, 0.2, 1) both',
        'slide-down': 'slideDown 200ms cubic-bezier(0, 0, 0.2, 1) both',
        'scale': 'scale 250ms cubic-bezier(0, 0, 0.2, 1) both',
        'glow-pulse': 'glowPulse 3.2s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'float': 'float 5s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'shimmer': 'shimmer 2.2s linear infinite',
        'gradient-x': 'gradientX 6s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'caret': 'caretBlink 1.05s step-end infinite',
        'bounce-dot': 'bounceDot 1.25s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'spin-slow': 'spinSlow 2.6s linear infinite',
        'grow-width': 'growWidth 600ms cubic-bezier(0, 0, 0.2, 1) both',
      },

      backgroundImage: {
        'gradient-primary': 'linear-gradient(135deg, #368773 0%, #25564b 100%)',
        'gradient-accent': 'linear-gradient(135deg, #bc9384 0%, #cf8575 100%)',
        // 品牌三色渐变，用于标题等需要更丰富层次的地方
        'gradient-brand': 'linear-gradient(120deg, #368773 0%, #a06cd5 45%, #bc9384 100%)',
        // 极光背景：页面底层的科技感光晕
        'aurora': 'radial-gradient(60% 55% at 12% 8%, rgb(54 135 115 / 0.20) 0%, transparent 60%), radial-gradient(50% 50% at 88% 4%, rgb(204 176 112 / 0.16) 0%, transparent 62%), radial-gradient(50% 50% at 70% 96%, rgb(80 130 108 / 0.18) 0%, transparent 60%)',
        // 网格纹理
        'grid': 'linear-gradient(to right, rgb(var(--soren-line) / 0.5) 1px, transparent 1px), linear-gradient(to bottom, rgb(var(--soren-line) / 0.5) 1px, transparent 1px)',
        // 骨架屏流光
        'skeleton': 'linear-gradient(90deg, transparent 0%, rgb(255 255 255 / 0.07) 50%, transparent 100%)',
      },

      backgroundSize: {
        'grid': '32px 32px',
      },

      backdropBlur: {
        glass: '20px',
      },

      // 标题字距收紧，正文保持默认，营造细腻的排版层级
      letterSpacing: {
        tighter: '-0.02em',
        tight: '-0.01em',
      },
    },
    screens: {
      'sm': '640px',
      'md': '768px',
      'lg': '1024px',
      'xl': '1280px',
      '2xl': '1536px',
    },
  },
  plugins: [],
}
