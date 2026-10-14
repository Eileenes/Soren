# Soren AI 助手 - 视觉设计系统

## 品牌定位

**Soren** 是一款桌面端 AI 助手应用，以黑白灰的克制界面搭配友好的眨眼角色。通过留白、字体、描边和细微层次区分信息与交互。

---

## 色彩系统

桌面端、网页与移动端默认使用白色背景。已选择的深色或跟随系统偏好继续生效，深色界面同样使用中性灰阶。

| 用途 | 浅色 | 深色 |
| --- | --- | --- |
| 页面背景 | `#FFFFFF` | `#121212` |
| 次级表面 | `#FAFAFA` | `#181818` |
| 选中态 / 输入区层次 | `#F5F5F5` | `#202020` |
| 悬浮表面 | `#EBEBEB` | `#2C2C2C` |
| 主文字 | `#181818` | `#F2F2F2` |
| 次文字 | `#525252` | `#B8B8B8` |
| 辅助文字 | `#737373` | `#969696` |
| 常规边框 | `#DFDFDF` | `#343434` |
| 主操作背景 / 文字 | `#171717` / `#FFFFFF` | `#F0F0F0` / `#171717` |

`surface`、`ink`、`line` 与 `primary` 由 CSS 变量驱动。主操作使用 `--action-bg` / `--action-ink`，使按钮与开关在两个主题中均有清晰对比。旧的彩色光晕、彩色选中描边及装饰背景统一移除。

成功、警告、错误、信息保留语义命名，视觉以灰阶呈现，并用文字、图标、确认弹窗与状态说明表达差异，不单靠颜色。其他 Bot 头像及外部服务品牌图标保留身份色。

---

## 字体系统

### 字体家族

**英文字体**
```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
```

**中文字体**
```css
font-family: 'PingFang SC', 'Microsoft YaHei', 'Noto Sans SC', sans-serif;
```

**等宽字体** (代码显示)
```css
font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
```

### 字号规范

| 名称 | 大小 | 用途 |
|------|------|------|
| `text-xs` | 12px | 辅助说明、时间戳 |
| `text-sm` | 14px | 次要文本、标签 |
| `text-base` | 16px | 正文、对话内容 |
| `text-lg` | 18px | 小标题 |
| `text-xl` | 20px | 标题 |
| `text-2xl` | 24px | 大标题 |
| `text-3xl` | 30px | 主标题 |

### 字重规范

- `font-normal` (400): 正文
- `font-medium` (500): 强调文本
- `font-semibold` (600): 小标题
- `font-bold` (700): 大标题

---

## 间距系统

采用 8px 基准网格系统：

| Token | 值 | 用途 |
|-------|-----|------|
| `spacing-1` | 4px | 极小间距 |
| `spacing-2` | 8px | 小间距 |
| `spacing-3` | 12px | 紧凑间距 |
| `spacing-4` | 16px | 标准间距 |
| `spacing-5` | 20px | 中等间距 |
| `spacing-6` | 24px | 大间距 |
| `spacing-8` | 32px | 区块间距 |
| `spacing-10` | 40px | 大区块间距 |
| `spacing-12` | 48px | 页面级间距 |

---

## 圆角系统

| Token | 值 | 用途 |
|-------|-----|------|
| `rounded-sm` | 4px | 小元素 |
| `rounded` | 8px | 按钮、卡片 |
| `rounded-md` | 12px | 对话气泡 |
| `rounded-lg` | 16px | 大卡片 |
| `rounded-xl` | 20px | 模态框 |
| `rounded-full` | 9999px | 圆形按钮、头像 |

---

## 阴影系统

```css
/* 轻微阴影 - 悬浮元素 */
shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);

/* 标准阴影 - 卡片 */
shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);

/* 中等阴影 - 弹出框 */
shadow-md: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);

/* 大阴影 - 模态框 */
shadow-lg: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);

/* 特大阴影 - 重要弹窗 */
shadow-xl: 0 25px 50px -12px rgba(0, 0, 0, 0.25);
```

---

## 界面组件设计

### 1. 聊天界面

#### 对话气泡 (Message Bubble)

**用户消息**
- 背景：浅灰色 `#F5F5F5`
- 文字：主文字色
- 圆角：`rounded-md` 左上/右上/左下，右下为 `4px`
- 对齐：右侧
- 最大宽度：70%
- 内边距：`12px 16px`

**AI 回复**
- 背景：`#f5f5f5`
- 文字：`text-primary`
- 圆角：`rounded-md` 左上/右上/右下，左上为 `4px`
- 对齐：左侧
- 最大宽度：80%
- 内边距：`12px 16px`

#### 输入框

- 背景：白色
- 边框：`1px solid #ebebeb`
- 圆角：`rounded-lg`
- 内边距：`12px 16px`
- 高度：最小 `48px`，最大 `200px` (可扩展)
- 聚焦状态：边框变为 `primary-start` 颜色，带轻微阴影

#### 发送按钮

- 形状：圆形按钮 `40px × 40px`
- 背景：黑白高对比
- 图标：纸飞机/箭头，白色
- 悬停：缩放 1.05，阴影增强
- 禁用：灰色 `#737373`，不可点击

### 2. 任务列表界面

#### 任务卡片

- 背景：白色
- 边框：`1px solid #ebebeb`
- 圆角：`rounded-lg`
- 阴影：`shadow-sm`，悬停时变为 `shadow`
- 内边距：`16px 20px`
- 间距：卡片间 `12px`

**状态标签**
- 进行中：浅灰背景 `#f5f5f5`，文字 `#181818`
- 已完成：浅灰背景 `#f5f5f5`，文字 `#181818`
- 待处理：灰色背景 `#f5f5f5`，文字 `#737373`

#### 列表项

- 复选框：自定义样式，选中时显示黑白高对比
- 标题：`text-base`，`font-medium`
- 描述：`text-sm`，`text-secondary`
- 时间：`text-xs`，`text-tertiary`

### 3. 设置界面

#### 设置项

- 布局：左右结构，左侧标签，右侧控件
- 分割线：`1px solid #ebebeb`
- 内边距：`16px 0`

**开关按钮**
- 宽度：`44px`
- 高度：`24px`
- 圆角：`rounded-full`
- 开启：黑白高对比背景
- 关闭：灰色背景 `#b9b9b9`
- 滑块：白色圆形，带阴影

**下拉选择**
- 高度：`40px`
- 圆角：`rounded`
- 边框：`1px solid #dfdfdf`
- 聚焦：中性高对比描边

---

## 动效规范

### 过渡时间

- `duration-fast`: 150ms (小元素悬停)
- `duration-base`: 250ms (标准过渡)
- `duration-slow`: 350ms (复杂动画)

### 缓动函数

```css
/* 标准缓动 */
ease: cubic-bezier(0.4, 0.0, 0.2, 1);

/* 入场动画 */
ease-in: cubic-bezier(0.4, 0.0, 1, 1);

/* 出场动画 */
ease-out: cubic-bezier(0.0, 0.0, 0.2, 1);

/* 弹性效果 */
ease-bounce: cubic-bezier(0.68, -0.55, 0.265, 1.55);
```

### 常用动画

**淡入淡出**
```css
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
```

**滑入**
```css
@keyframes slideUp {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
```

**缩放**
```css
@keyframes scale {
  from { transform: scale(0.95); }
  to { transform: scale(1); }
}
```

---

## 图标系统

推荐使用：
- **Lucide Icons** (React 组件)
- **Heroicons** (SVG)

图标大小规范：
- 小图标：16px
- 标准图标：20px
- 大图标：24px
- 特大图标：32px

---

## 响应式设计

### 断点

```javascript
breakpoints: {
  sm: '640px',   // 小屏幕
  md: '768px',   // 平板
  lg: '1024px',  // 笔记本
  xl: '1280px',  // 桌面
  '2xl': '1536px' // 大桌面
}
```

### 窗口尺寸建议

- 最小宽度：800px
- 最小高度：600px
- 推荐宽度：1200px
- 推荐高度：800px

---

## 可访问性

### 对比度

- 正文文本对比度：至少 4.5:1
- 大文本对比度：至少 3:1
- 交互元素：至少 3:1

### 焦点状态

所有交互元素必须有清晰的焦点指示：
```css
focus:outline-none focus:ring-2 focus:ring-primary-start focus:ring-offset-2
```

### 键盘导航

- 支持 Tab 键导航
- 支持 Enter 和 Space 触发按钮
- 支持 Escape 关闭模态框

---

## 实现建议

### 技术栈
- **框架**: React + TypeScript
- **样式**: Tailwind CSS
- **图标**: Lucide React
- **动画**: Framer Motion (可选)
- **状态管理**: Zustand / Context API

### 文件结构
```
frontend/
├── src/
│   ├── assets/
│   │   ├── logo.svg
│   │   └── icon.svg
│   ├── components/
│   │   ├── Chat/
│   │   ├── TaskList/
│   │   └── Settings/
│   ├── styles/
│   │   └── globals.css
│   └── tailwind.config.js
```

---

## 品牌应用示例

### Logo 使用规范

1. **最小尺寸**: 32px × 32px
2. **安全空间**: 周围至少留出 logo 高度 1/4 的空白
3. **背景使用**:
   - 浅色背景：使用黑色轮廓、灰白面部版本
   - 深色背景：使用同一角色，以灰白面部保证识别
4. **禁止**:
   - 拉伸变形
   - 改变颜色
   - 添加边框或阴影

### 应用场景

- **窗口标题栏**: 使用 24px icon
- **启动画面**: 使用 128px logo，带淡入动画
- **关于页面**: 使用 200px logo
- **托盘图标**: 使用系统推荐尺寸

---

## 下一步

1. 在 Figma 中创建完整的设计稿（可选）
2. 实现 React 组件库
3. 编写组件文档和 Storybook
4. 进行用户测试和反馈收集
