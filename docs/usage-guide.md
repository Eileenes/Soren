# Soren 设计系统使用指南

## 快速开始

### 1. 安装依赖

```bash
npm install tailwindcss react react-dom
npm install -D @tailwindcss/forms
```

### 2. 配置 Tailwind

将提供的 `tailwind.config.js` 复制到项目根目录。

### 3. 导入样式

在主入口文件中导入全局样式：

```jsx
import './globals.css';
```

### 4. 使用组件

```jsx
import { Button, ChatInput, TaskCard } from './components';

function App() {
  return (
    <div className="p-6">
      <Button variant="primary">开始对话</Button>
    </div>
  );
}
```

---

## 设计原则

### 1. 可爱而不幼稚
- 使用柔和的渐变色和圆角
- 保持专业的排版和间距
- 添加微妙的动画增强体验

### 2. 精致的科技感
- 清晰的视觉层次
- 适度的阴影和边框
- 简洁的图标和符号

### 3. 友好的交互
- 明确的状态反馈
- 流畅的过渡动画
- 直观的操作提示

---

## 常用模式

### 渐变背景

```jsx
<div className="bg-gradient-primary">
  内容
</div>

// 或使用内联样式
<div style={{ backgroundImage: 'var(--gradient-primary)' }}>
  内容
</div>
```

### 对话气泡

```jsx
// 用户消息
<div className="message-bubble-user">
  你的消息内容
</div>

// AI 回复
<div className="message-bubble-ai">
  AI 的回复内容
</div>
```

### 卡片布局

```jsx
<div className="card">
  <h3 className="text-lg font-semibold mb-2">卡片标题</h3>
  <p className="text-sm text-text-secondary">卡片内容</p>
</div>
```

### 输入框

```jsx
<input
  type="text"
  className="input-field"
  placeholder="输入内容..."
/>
```

### 按钮

```jsx
// 主按钮
<button className="btn-primary">
  主要操作
</button>

// 次要按钮
<button className="btn-secondary">
  次要操作
</button>
```

---

## 响应式设计

使用 Tailwind 的响应式前缀：

```jsx
<div className="w-full md:w-1/2 lg:w-1/3">
  响应式宽度
</div>

<p className="text-sm md:text-base lg:text-lg">
  响应式字体
</p>
```

---

## 动画使用

### 预定义动画

```jsx
// 淡入
<div className="animate-fade-in">内容</div>

// 滑入
<div className="animate-slide-up">内容</div>

// 缩放
<div className="animate-scale">内容</div>

// 脉冲
<div className="animate-pulse">加载中...</div>
```

### 悬停效果

```jsx
<button className="hover:shadow-md hover:scale-105 transition-all duration-base">
  悬停我
</button>
```

### 自定义过渡

```jsx
<div className="transition-all duration-base ease-out">
  平滑过渡
</div>
```

---

## 无障碍支持

### 键盘导航

所有交互元素都应支持键盘访问：

```jsx
<button
  className="focus:outline-none focus:ring-2 focus:ring-primary-start focus:ring-offset-2"
  onKeyPress={(e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      handleClick();
    }
  }}
>
  可访问按钮
</button>
```

### 屏幕阅读器

为视觉元素添加文本说明：

```jsx
<button aria-label="发送消息">
  <svg>...</svg>
</button>

<div role="status" aria-live="polite">
  {isLoading && <span className="sr-only">正在加载...</span>}
</div>
```

---

## 性能优化

### 图片优化

1. 使用 SVG 格式的图标和 logo
2. 为不同屏幕密度提供不同尺寸的位图

### CSS 优化

1. 使用 Tailwind 的 purge 功能移除未使用的样式
2. 避免过度嵌套的选择器

```js
// tailwind.config.js
export default {
  content: [
    "./src/**/*.{js,jsx,ts,tsx}",
  ],
  // ...
}
```

### 动画性能

优先使用 transform 和 opacity：

```jsx
// 好的做法
<div className="transform hover:scale-105 transition-transform">
  内容
</div>

// 避免
<div className="hover:w-full transition-all">
  内容
</div>
```

---

## 常见问题

### Q: 如何更改主题色？

A: 在 `tailwind.config.js` 中修改 `colors.primary` 的值。

### Q: 如何添加深色模式？

A: 使用 Tailwind 的 `dark:` 前缀：

```jsx
<div className="bg-white dark:bg-bg-dark text-text-primary dark:text-white">
  支持深色模式的内容
</div>
```

### Q: 如何自定义字体？

A: 在 `tailwind.config.js` 中修改 `fontFamily`，并在 CSS 中导入字体文件。

### Q: Logo 最小使用尺寸是多少？

A: 建议最小不低于 32px × 32px，以保持清晰度和识别性。

---

## 资源文件清单

```
Soren/
├── assets/
│   ├── logo.svg              # 主 Logo（彩色）
│   ├── logo-dark.svg         # 深色版 Logo
│   ├── icon.svg              # 应用图标
│   └── icon-mono.svg         # 单色图标（系统托盘）
├── docs/
│   ├── design-system.md      # 设计系统文档
│   └── usage-guide.md        # 使用指南（本文件）
└── frontend/
    ├── tailwind.config.js    # Tailwind 配置
    ├── globals.css           # 全局样式
    ├── components.jsx        # 基础组件
    └── examples.jsx          # 示例界面
```

---

## 进一步阅读

- [Tailwind CSS 官方文档](https://tailwindcss.com/docs)
- [React 无障碍指南](https://react.dev/learn/accessibility)
- [Web 动画最佳实践](https://web.dev/animations/)
- [色彩对比度工具](https://webaim.org/resources/contrastchecker/)

---

## 支持与反馈

如有问题或建议，请通过以下方式联系：

- 项目 GitHub Issue
- 设计团队邮箱
- 产品反馈渠道

---

**版本**: 1.0.0
**最后更新**: 2026-10-10
