/**
 * 代码块与 Markdown 渲染
 *
 * 说明：不引入语法高亮库（如 highlight.js/prismjs）以控制体积与离线依赖，
 * 改为在视觉上做出明确区分：深色代码面板 + 语言标签 + 一键复制。
 */

import { useMemo, useState, type ReactNode } from 'react';
import ReactMarkdown from 'react-markdown';
import { cn, copyToClipboard } from '../../utils/format';
import { IconCheck, IconCopy } from '../icons';

/** 从 react-markdown 传来的 className 中解析语言名，如 "language-ts" → "ts" */
function parseLanguage(className?: string): string {
  if (!className) return '';
  const match = /language-([\w+-]+)/.exec(className);
  return match?.[1] ?? '';
}

/**
 * 判断是否为行内代码
 *
 * react-markdown v9 的 code 组件拿不到 inline 标记，
 * 通用做法是看是否带有 language- 类名或内容是否含换行。
 */
function isInlineCode(className: string | undefined, content: string): boolean {
  if (className && /language-/.test(className)) return false;
  return !content.includes('\n');
}

export interface CodeBlockProps {
  code: string;
  language?: string;
  className?: string;
}

/** 代码块：语言标签 + 复制按钮 + 横向滚动 */
export function CodeBlock({ code, language, className }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    const ok = await copyToClipboard(code);
    if (!ok) return;
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };

  return (
    <div
      className={cn(
        'group/code my-3 overflow-hidden rounded-md border border-line bg-[#0d0e14]',
        'dark:bg-[#0b0c11]',
        className,
      )}
    >
      {/* 顶栏：语言标签 + 复制 */}
      <div className="flex items-center justify-between border-b border-line/60 bg-surface-3/60 px-3 py-1.5">
        <span className="font-mono text-xs uppercase tracking-wider text-ink-3">
          {language || 'code'}
        </span>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={copied ? '已复制代码' : '复制代码'}
          className={cn(
            'inline-flex items-center gap-1.5 rounded-sm px-2 py-1 text-xs',
            'text-ink-3 transition-all duration-fast',
            'hover:bg-surface-4 hover:text-ink-1',
            'opacity-0 group-hover/code:opacity-100 focus-visible:opacity-100',
            copied && 'opacity-100 text-success hover:text-success',
          )}
        >
          {copied ? <IconCheck size={12} /> : <IconCopy size={12} />}
          {copied ? '已复制' : '复制'}
        </button>
      </div>

      <pre className="overflow-x-auto p-3.5">
        <code className="whitespace-pre font-mono text-xs leading-relaxed text-[#d8dbef]">
          {code}
        </code>
      </pre>
    </div>
  );
}

export interface MarkdownViewProps {
  content: string;
  className?: string;
}

/**
 * Markdown 渲染
 *
 * 定制常用元素，让排版与深色主题一致；
 * 链接统一在新窗口打开（桌面端由系统浏览器接管）。
 */
export function MarkdownView({ content, className }: MarkdownViewProps) {
  const components = useMemo(
    () => ({
      // 代码：区分行内与代码块
      code({ className: codeClass, children, ...rest }: {
        className?: string;
        children?: ReactNode;
      }) {
        const raw = String(children ?? '').replace(/\n$/, '');
        if (isInlineCode(codeClass, raw)) {
          return (
            <code className={codeClass} {...rest}>
              {children}
            </code>
          );
        }
        return <CodeBlock code={raw} language={parseLanguage(codeClass)} />;
      },

      // 代码块外层 pre 已由 CodeBlock 自行渲染，这里直接透传
      pre({ children }: { children?: ReactNode }) {
        return <>{children}</>;
      },

      // 链接：外部浏览器打开，并提示来源
      a({ href, children }: { href?: string; children?: ReactNode }) {
        return (
          <a href={href} target="_blank" rel="noreferrer noopener">
            {children}
          </a>
        );
      },

      // 表格：外层加横向滚动容器，窄窗口下不撑破气泡
      table({ children }: { children?: ReactNode }) {
        return (
          <div className="my-3 overflow-x-auto rounded-md border border-line">
            <table>{children}</table>
          </div>
        );
      },
    }),
    [],
  );

  return (
    <div className={cn('markdown-body', className)}>
      <ReactMarkdown components={components}>{content}</ReactMarkdown>
    </div>
  );
}
