/**
 * 表单控件：文本输入、密码输入、文本域、下拉选择、开关、滑块
 *
 * 统一结构为「标签 + 控件 + 辅助/错误说明」，保证设置页与渠道页的排版一致。
 */

import { useId, useState } from 'react';
import { Select, type SelectProps } from './Select';
import type {
  InputHTMLAttributes,
  ReactNode,
  TextareaHTMLAttributes,
} from 'react';
import { cn } from '../../utils/format';
import { IconAlert, IconCheck } from '../icons';

/** 表单项外壳：负责标签、说明文案与错误提示的排版 */
function FieldShell({
  id,
  label,
  helper,
  error,
  required,
  children,
  layout = 'stacked',
  htmlFor,
}: {
  id: string;
  label?: string;
  helper?: string;
  error?: string;
  required?: boolean;
  children: ReactNode;
  /** stacked=上下布局；inline=设置页常用的左右布局 */
  layout?: 'stacked' | 'inline';
  htmlFor?: string;
}) {
  if (layout === 'inline') {
    return (
      <div className="flex items-start justify-between gap-6 py-4">
        <div className="min-w-0 flex-1">
          {label && (
            <label
              htmlFor={htmlFor ?? id}
              className="block text-sm font-medium text-ink-1"
            >
              {label}
              {required && <span className="ml-1 text-error">*</span>}
            </label>
          )}
          {helper && !error && (
            <p id={`${id}-helper`} className="mt-1 text-xs leading-5 text-ink-3">
              {helper}
            </p>
          )}
          {error && (
            <p
              id={`${id}-error`}
              role="alert"
              className="mt-1 flex items-center gap-1 text-xs text-error"
            >
              <IconAlert size={12} />
              {error}
            </p>
          )}
        </div>
        <div className="w-[260px] shrink-0">{children}</div>
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {label && (
        <label
          htmlFor={htmlFor ?? id}
          className="block text-sm font-medium text-ink-1"
        >
          {label}
          {required && <span className="ml-1 text-error">*</span>}
        </label>
      )}
      {children}
      {error ? (
        <p
          id={`${id}-error`}
          role="alert"
          className="flex items-center gap-1 text-xs text-error"
        >
          <IconAlert size={12} />
          {error}
        </p>
      ) : (
        helper && (
          <p id={`${id}-helper`} className="text-xs leading-5 text-ink-3">
            {helper}
          </p>
        )
      )}
    </div>
  );
}

export interface TextFieldProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'className'> {
  label?: string;
  helper?: string;
  error?: string;
  layout?: 'stacked' | 'inline';
  /** 输入框前置图标 */
  leadingIcon?: ReactNode;
  /** 输入框后置内容（如单位、按钮） */
  trailing?: ReactNode;
}

/** 单行文本输入 */
export function TextField({
  label,
  helper,
  error,
  layout = 'stacked',
  leadingIcon,
  trailing,
  id,
  required,
  ...rest
}: TextFieldProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  const describedBy = error ? `${fieldId}-error` : helper ? `${fieldId}-helper` : undefined;

  return (
    <FieldShell
      id={fieldId}
      label={label}
      helper={helper}
      error={error}
      required={required}
      layout={layout}
      htmlFor={fieldId}
    >
      <div className="relative">
        {leadingIcon && (
          <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-3">
            {leadingIcon}
          </span>
        )}
        <input
          id={fieldId}
          required={required}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn(
            'field h-10',
            leadingIcon && 'pl-9',
            trailing && 'pr-10',
            error && 'border-error/60',
          )}
          {...rest}
        />
        {trailing && (
          <span className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center">
            {trailing}
          </span>
        )}
      </div>
    </FieldShell>
  );
}

/**
 * 密码输入
 *
 * 自带明文切换按钮：API Key 这类长字符串用户需要核对，强制隐藏会造成困扰。
 */
export function PasswordField({
  label,
  helper,
  error,
  layout = 'stacked',
  id,
  required,
  ...rest
}: TextFieldProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  const [visible, setVisible] = useState(false);
  const describedBy = error ? `${fieldId}-error` : helper ? `${fieldId}-helper` : undefined;

  return (
    <FieldShell
      id={fieldId}
      label={label}
      helper={helper}
      error={error}
      required={required}
      layout={layout}
      htmlFor={fieldId}
    >
      <div className="relative">
        <input
          id={fieldId}
          type={visible ? 'text' : 'password'}
          required={required}
          autoComplete="off"
          spellCheck={false}
          aria-invalid={error ? true : undefined}
          aria-describedby={describedBy}
          className={cn('field h-10 pr-12 font-mono text-xs', error && 'border-error/60')}
          {...rest}
        />
        <button
          type="button"
          onClick={() => setVisible((v) => !v)}
          aria-label={visible ? '隐藏密钥' : '显示密钥'}
          title={visible ? '隐藏密钥' : '显示密钥'}
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-xs text-ink-3 transition-colors duration-fast hover:text-ink-1"
        >
          {visible ? '隐藏' : '显示'}
        </button>
      </div>
    </FieldShell>
  );
}

export interface TextAreaFieldProps
  extends Omit<TextareaHTMLAttributes<HTMLTextAreaElement>, 'className'> {
  label?: string;
  helper?: string;
  error?: string;
}

/** 多行文本域（渠道说明、备注等） */
export function TextAreaField({
  label,
  helper,
  error,
  id,
  required,
  rows = 3,
  ...rest
}: TextAreaFieldProps) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  const describedBy = error ? `${fieldId}-error` : helper ? `${fieldId}-helper` : undefined;

  return (
    <FieldShell
      id={fieldId}
      label={label}
      helper={helper}
      error={error}
      required={required}
      htmlFor={fieldId}
    >
      <textarea
        id={fieldId}
        rows={rows}
        required={required}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        className={cn('field resize-y leading-6', error && 'border-error/60')}
        {...rest}
      />
    </FieldShell>
  );
}

export interface SelectOption<T extends string = string> {
  value: T;
  label: string;
  /** 可选说明，展示在下拉项右侧 */
  hint?: string;
}

export interface SelectFieldProps<T extends string = string>
  extends Omit<SelectProps<T>, 'className' | 'onChange' | 'value' | 'options'> {
  label?: string;
  helper?: string;
  error?: string;
  layout?: 'stacked' | 'inline';
  options: ReadonlyArray<SelectOption<T>>;
  value: T;
  onChange: (value: T) => void;
}

/** 下拉选择 */
export function SelectField<T extends string = string>({
  label,
  helper,
  error,
  layout = 'stacked',
  options,
  value,
  onChange,
  id,
  required,
  disabled,
  ...rest
}: SelectFieldProps<T>) {
  const autoId = useId();
  const fieldId = id ?? autoId;
  const describedBy = error ? `${fieldId}-error` : helper ? `${fieldId}-helper` : undefined;

  return (
    <FieldShell
      id={fieldId}
      label={label}
      helper={helper}
      error={error}
      required={required}
      layout={layout}
      htmlFor={fieldId}
    >
      <Select id={fieldId} label={label} value={value} options={options} onChange={onChange}
        required={required} disabled={disabled} aria-describedby={describedBy} aria-invalid={Boolean(error)} {...rest} />
    </FieldShell>
  );
}

export interface SwitchProps {
  /** 无障碍标签 */
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  /** 开关右侧的可见文本（若不传则仅渲染开关本体） */
  showText?: boolean;
  /** 处于加载态（如正在调用启停接口） */
  loading?: boolean;
}

/**
 * 开关
 *
 * 规格遵循设计文档：44×24，开启为品牌渐变，关闭为灰色。
 * 用 role="switch" + aria-checked 而不是 checkbox，语义更准确。
 */
export function Switch({
  label,
  checked,
  onChange,
  disabled,
  showText = false,
  loading = false,
}: SwitchProps) {
  return (
    <span className="inline-flex items-center gap-3">
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label}
        disabled={disabled || loading}
        onClick={() => onChange(!checked)}
        className={cn(
          'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full',
          'transition-all duration-base ease-ease',
          'disabled:opacity-40 disabled:cursor-not-allowed',
          checked
            ? 'bg-gradient-primary shadow-[0_2px_12px_-2px_rgb(102_126_234/0.7)]'
            : 'bg-line-strong',
        )}
      >
        <span
          className={cn(
            'inline-block h-4 w-4 rounded-full bg-white shadow-sm',
            'transition-transform duration-base ease-ease',
            checked ? 'translate-x-6' : 'translate-x-1',
          )}
        />
      </button>
      {showText && (
        <span className={cn('text-sm', checked ? 'text-ink-1' : 'text-ink-3')}>
          {loading ? '处理中…' : checked ? '已启用' : '已停用'}
        </span>
      )}
    </span>
  );
}

export interface RangeFieldProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
  /** 值的展示转换，如温度保留一位小数 */
  format?: (value: number) => string;
  helper?: string;
  disabled?: boolean;
}

/** 滑块：用于 temperature 这类连续参数 */
export function RangeField({
  label,
  value,
  min,
  max,
  step = 0.1,
  onChange,
  format = (v) => String(v),
  helper,
  disabled,
}: RangeFieldProps) {
  const id = useId();

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <label htmlFor={id} className="text-sm font-medium text-ink-1">
          {label}
        </label>
        <span className="rounded-sm bg-surface-3 px-2 py-0.5 font-mono text-xs text-primary-start">
          {format(value)}
        </span>
      </div>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(Number(event.target.value))}
        className={cn(
          'h-1.5 w-full cursor-pointer appearance-none rounded-full bg-line',
          'accent-primary-start',
          '[&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:w-4',
          '[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:rounded-full',
          '[&::-webkit-slider-thumb]:bg-gradient-primary',
          '[&::-webkit-slider-thumb]:shadow-[0_0_0_3px_rgb(102_126_234/0.22)]',
          '[&::-webkit-slider-thumb]:transition-transform [&::-webkit-slider-thumb]:duration-fast',
          'hover:[&::-webkit-slider-thumb]:scale-110',
          disabled && 'opacity-40 cursor-not-allowed',
        )}
        style={{
          // 用渐变填充已选区间，形成进度感
          backgroundImage: `linear-gradient(to right, #667eea 0%, #764ba2 ${
            ((value - min) / (max - min)) * 100
          }%, transparent ${((value - min) / (max - min)) * 100}%)`,
        }}
      />
      {helper && <p className="text-xs text-ink-3">{helper}</p>}
    </div>
  );
}

/** 表单校验通过时的小对勾，用于就地反馈 */
export function FieldValidMark({ show, text = '已配置' }: { show: boolean; text?: string }) {
  if (!show) return null;
  return (
    <span className="inline-flex items-center gap-1 text-xs text-success">
      <IconCheck size={12} />
      {text}
    </span>
  );
}
