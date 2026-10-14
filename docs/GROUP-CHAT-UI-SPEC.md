# 群聊功能 · 视觉与交互规范

> 面向前端施工（Agent C）。基线 commit `4061c7e`。
> 上游规格：`docs/GROUP-CHAT-PLAN.md` §8 前端设计。
> 本文档**不新增任何色彩体系**，全部沿用 `frontend/tailwind.config.js` + `frontend/src/styles/index.css` 现有 token。

---

## 0. 核对基线

本文档中每个 token、类名、尺寸都已对照下列文件核对，文中以 `文件:行号` 标注。

| 核对对象 | 位置 |
| --- | --- |
| Tailwind token | `frontend/tailwind.config.js:11-135` |
| CSS 变量与共享控件尺寸 | `frontend/src/styles/index.css:11-15` |
| 主题变量（accent / card 等） | `frontend/src/styles/index.css:452-453` |
| 侧栏 | `frontend/src/components/layout/Sidebar.tsx`、样式 `index.css:813-845` |
| 消息气泡 | `frontend/src/components/chat/MessageBubble.tsx`、样式 `index.css:244-253, 568-569` |
| 输入框 | `frontend/src/components/chat/ChatInput.tsx`、样式 `index.css:551-563` |
| 空状态 | `frontend/src/components/chat/ChatEmptyState.tsx`、样式 `index.css:645` |
| 浮层菜单 | `frontend/src/components/ui/Select.tsx`、样式 `index.css:792-798` |
| 模态 | `frontend/src/components/ui/Modal.tsx` |
| 分段控件先例 | `frontend/src/components/BotEditor.tsx`、样式 `index.css:681-683, 700-703` |

### 0.1 施工前必读的三个坑

**坑 1：`.inbox-tool` 的圆角不是圆形。**
`index.css:818` 写了 `border-radius: 50%`，但 `index.css:932-942` 的共享控件块把同一批选择器统一设成 `border-radius: var(--control-radius)`（10px）。两条规则特异度相同（单类），**后出现者胜**，所以实际渲染是 **10px 圆角、36×36**。
同样情况：`.bot-filter button` 的 `20px` 胶囊圆角（`index.css:682`）也被覆盖成 10px。**不要照抄 `border-radius:50%` 那行。**

**坑 2：`primary-start` 不随主题切换。**
`tailwind.config.js:14` 里 `primary.start` 是写死的 `#368773`，而 `surface-*` / `ink-*` / `line-*` 都是 `rgb(var(--soren-*) / <alpha-value>)` 变量驱动。**`primary-start` 用在会随主题换底色的元素上会失控**（见 §5.2 的对比度实测）。

**坑 3：`Button` 的三个尺寸是同一个尺寸。**
`Button.tsx:34-38` 里 `sm` / `md` / `lg` 全部映射到 `control-button`，`IconButton` 同理全部映射到 `control-icon`（`Button.tsx:83-87`）。注释写明「所有尺寸统一使用界面的标准操作尺寸」。
→ **本功能不存在「大号按钮」。任何需要更大视觉重量的设计都是错的**，详见 §6.2。

---

## 1. 视觉基调

### 1.1 色彩：不新增，沿用

群聊**不引入任何新颜色**。以下是本功能允许使用的全部颜色 token：

**表面 / 文本 / 描边（语义层，主题自动切换，首选）**

| Token | 类名 | 浅色 | 深色 | 用途 |
| --- | --- | --- | --- | --- |
| `surface-1` | `bg-surface-1` | `250 249 246` | `20 24 29` | 消息区底、气泡、浮层底 |
| `surface-2` | `bg-surface-2` | `244 244 239` | `25 30 36` | 输入框底（`.field`） |
| `surface-3` | `bg-surface-3` | `237 240 233` | `32 38 45` | 菜单项选中、system pill |
| `surface-4` | `bg-surface-4` | `227 231 222` | `42 50 58` | 行悬停底 |
| `ink-1` | `text-ink-1` | `39 56 48` | `231 237 239` | 主文本（群名、发言人名、消息正文） |
| `ink-2` | `text-ink-2` | `104 118 108` | `170 183 189` | 次文本、菜单项 |
| `ink-3` | `text-ink-3` | `130 142 132` | `137 153 163` | 预览、计数等辅助文本 |
| `line-subtle` | `border-line-subtle` | `227 231 222` | `40 48 56` | 分组分割线、弱描边 |
| `line` | `border-line` | `219 225 215` | `49 59 68` | 浮层边框、输入框边框 |
| `line-strong` | `border-line-strong` | `194 207 193` | `76 92 100` | 输入框聚焦前边框 |

**兼容层**（`tailwind.config.js:42-59`，与上表同源，**仅在必须对齐既有旧代码时使用，新代码不要用**）：
`bg-primary/secondary/tertiary`、`text-primary/secondary/tertiary`、`border-light/medium/dark`。

**强调色**

| Token | 值 | 说明 |
| --- | --- | --- |
| `primary-start` | `#368773` | 单一强调色。**不随主题变**，见 §0.1 坑 2 |
| `--accent`（CSS 变量） | 浅 `#347b67` / 深 `#9bd8c6` | **随主题切换**，侧栏 hover、菜单选中态已在用 |
| `accent-soft` | 浅 `#e5eee3` / 深 `#253d3b` | 强调色的低强度底 |
| `accent.pink` / `accent.coral` | `#bc9384` / `#cf8575` | 群聊功能**不使用** |

**语义色**（仅用于状态，不用于装饰）：`success #368773`、`warning #b38b3d`、`error #d06b63`、`info #589aa6`。
中止按钮的警示语义用 `error`；讨论进行中**不使用** `warning`（不是警告，是进度）。

**禁用**：`gradient-primary`、`gradient-accent`、`gradient-brand`、`aurora`、`grid`、`skeleton`、`shadow-glow`、`shadow-glow-lg`、`backdrop-blur-glass`。其中 `gradient-brand` 含紫色（`#a06cd5`），属红线。

### 1.2 字体

`font-sans`（`tailwind.config.js:69`）：`-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'PingFang SC', 'Microsoft YaHei', sans-serif`。
**禁止**引入 Inter 或任何新字体（`docs/color-preview.html` 里出现 Inter 是那份预览自己的选择，不是应用字体）。

### 1.3 四级字号（群聊只用这四级）

| 级别 | 类名 | 尺寸 / 行高 | 群聊中的用途 |
| --- | --- | --- | --- |
| L1 | `text-xs` | 12 / 16 | 发言人名、讨论状态计数、system pill、`+N` |
| L2 | `text-sm` | 14 / 20 | 群聊页副信息、成员名、气泡内行内标记 |
| L3 | `text-base` | 16 / 24 | 群名、弹窗标题（`Modal` 已用 `text-base font-semibold`） |
| L4 | `text-lg` | 18 / 28 | 空状态群名 |

**气泡正文是例外**：`MessageBubble.tsx:91` 已写死 `text-[15px] leading-relaxed`，属于既有决策，群聊不改。

侧栏群列表项的 14px/13px（`index.css:888-889`）与 Bot 项完全一致，见 §4.1。

### 1.4 字重与圆角

字重（`tailwind.config.js:83-88`）：`normal` = `medium` = **500**，`semibold` = 600，`bold` = 700。
→ **正文与按钮都是 500**。不要用 `font-normal`(400) 制造层次，用颜色和字号做层次。

圆角：`sm` 4 / `DEFAULT` 8 / `md` 12 / `lg` 16 / `xl` 20 / `full`。
共享控件圆角统一 `var(--control-radius)` = **10px**（不是 tailwind 里的任何一档，需用任意值 `rounded-[10px]` 或复用既有类）。

### 1.5 文案原则

界面内每一句中文必须承载功能信息。可直接使用的文案见各节「文案」小节。禁止出现：「欢迎来到群聊」「让智能体一起探讨」「输入 @ 开始对话」这类解释性与抒情表述。

---

## 2. 加号下拉菜单

改造点：`Sidebar.tsx:51`。现状是直接 `onCreateBot`，改为 `aria-haspopup="menu"` + 下拉两项。

### 2.1 复用 `inbox-tool` 的触发按钮规格

触发按钮**类名与尺寸完全不动**，只加 ARIA 与状态：

```
class="inbox-tool"   aria-haspopup="menu"  aria-expanded={open}  aria-controls="inbox-create-menu"
```

实际渲染值（`index.css:818-819` + `932-951`）：

| 属性 | 值 | 来源 |
| --- | --- | --- |
| 尺寸 | 36 × 36（`--control-height`） | `932-951` 覆盖 `818` 的 34px |
| 圆角 | 10px（`--control-radius`） | `938` 覆盖 `818` 的 `50%` |
| 底色 | `var(--card)` = `#ffffff` / `#20272e` | `818` |
| 描边 | `1px solid rgb(var(--soren-line-subtle))` | `818` |
| 文字色 | `rgb(var(--soren-ink-2))` | `818` |
| 悬停 | `background: var(--accent-soft)`；`color: var(--accent)` | `819` |
| 过渡 | `background .15s` | `818` |

图标沿用 `IconPlus size={20}`（`Sidebar.tsx:51` 现状不变）。

**展开态**：`aria-expanded="true"` 时不改底色。加号是菜单触发器不是开关，翻转成 `IconClose` 会让用户以为「取消」。保持 `IconPlus` 旋转 45° 亦可——**但旋转属于新增动效，建议不加**，直接保持静态，靠菜单本身表达状态。

### 2.2 菜单容器：复用 `.app-select-menu`

**不要新写浮层样式。** 直接复用 `Select.tsx` 的浮层类，它是本项目唯一的浮层菜单体系，且是不透明底（符合红线）：

```
class="app-select-menu"  role="menu"  id="inbox-create-menu"
```

| 属性 | 值 | 来源 |
| --- | --- | --- |
| 定位 | `position: fixed; z-index: 100` | `index.css:792` |
| 内边距 | `6px` | `792` |
| 圆角 | `12px` | `792` |
| 边框 | `1px solid rgb(var(--soren-line))` | `792` |
| 底色 | `rgb(var(--soren-surface-1))` | `792` |
| 阴影 | `0 12px 40px -8px #0003` | `792` |
| 宽度 | `184px` | 本规范新增，见下 |
| 定位算法 | 复用 `Select.tsx:20-31` 的 `useLayoutEffect` + `place()` | — |

**宽度 184px** 的依据：`inbox-tool` 是 36px，菜单比触发器宽 5 倍会显得菜单是主角。184px 恰好容纳「创建群聊」4 字 × 14px + 16px 图标 + 左右 12px 内边距。
**右对齐**：菜单右边缘与加号右边缘对齐（`left = trigger.right - 184`）。加号位于侧栏工具栏最右（`.inbox-toolbar` 为 `justify-content: flex-end`，`index.css:816`），所以菜单**向左展开**，不超出视窗。
**垂直间距**：菜单顶边 = 触发器底边 + 6px（与 `Select.tsx:27` 的 `r.bottom + 6` 一致）。
**空间不足时向上翻转**：复用 `Select.tsx:24-27` 判定——下方可用 < 180px 且上方更大时翻到上方。

### 2.3 菜单项：复用 `.app-select-option`

两项**完全同构**，不加视觉差异（它们是同级动作，不是主次关系）：

```
class="app-select-option"  role="menuitem"
```

| 状态 | 样式 | 来源 |
| --- | --- | --- |
| 默认 | `min-height: 38px; padding: 9px 11px; border-radius: 7px; font-size: 13px; color: rgb(var(--soren-ink-1))` | `793` |
| 悬停 | `background: rgb(var(--soren-surface-3))` | 复用 `.is-active` 规则 `797` |
| 键盘当前项 | 同悬停：`background: rgb(var(--soren-surface-3))` | `797` |
| 焦点（`:focus-visible`） | `outline: 2px solid var(--accent); outline-offset: 2px` | `953-956` |

**悬停与键盘当前项视觉必须一致**——用同一个类名（`.is-active`），不要写两套。`Select.tsx:57-58` 已实现这个模式（`onPointerMove` 同步 `active` 索引），照抄即可。

### 2.4 菜单项内容

```
[IconPlus 16px]  创建 Bot
[IconUsers/IconChat 16px]  创建群聊
```

- 图标容器：`width:16px; flex-shrink:0`，`color: rgb(var(--soren-ink-2))`。不套 `.palette-symbol`（那个是 34px 方块，属于搜索结果行，菜单项用不上）。
- 文字：`13px`（继承 `.app-select-option`）。
- **不带快捷键后缀、不带分隔线**。两项是同级动作，加分隔线会暗示层级关系。

图标选择：`IconPlus` 已有（`icons.tsx:85`）用于「创建 Bot」。「创建群聊」需要多 Bot 语义，现有图标里 `IconChat`（`icons.tsx:44`）最贴近，**不要为此新增渐变或彩色图标**。

### 2.5 键盘与关闭

复用 `Select.tsx:34-56` 的实现模式：

| 按键 | 行为 |
| --- | --- |
| `↓` / `↑` | 在两项间循环移动，移动即应用 `.is-active` |
| `Enter` / `Space` | 激活当前项，关闭菜单 |
| `Home` / `End` | 跳到首 / 末项 |
| `Esc` | 关闭，**焦点归还加号** |
| `Tab` | 关闭（不阻止焦点离开） |
| 单字符 | 不需要（两项无共同首字母，跳过 `Select.tsx:50-55` 的 typeahead） |

- **点击外部关闭**：`pointerdown` 监听，判定「不在 trigger 内且不在 menu 内」（`Select.tsx:34-36`）。用 `pointerdown` 而非 `click`，避免菜单先收起再触发外部点击。
- **焦点管理**：菜单用 `createPortal` 挂 `document.body`（同 `Select.tsx:58`），因此 DOM 上不与加号相邻。关闭后必须 `trigger.current?.focus()`（`Select.tsx:40` 的既有做法）。
- 菜单打开时**不锁背景滚动**，不设焦点陷阱——菜单只有两项，Esc 与 Tab 足够。

### 2.6 无障碍标注

- 加号：`aria-haspopup="menu"` `aria-expanded` `aria-controls`；`aria-label` 从「创建 Bot」改为「**创建**」，`title` 同步。
- 菜单：`role="menu"`，每项 `role="menuitem"`。
- 当前项用 `aria-activedescendant` 指向选项 id（`Select.tsx:57` 同款），焦点始终留在加号上。

---

## 3. 创建群聊弹窗 `CreateGroupDialog`

### 3.1 容器

复用 `Modal`，**不新写弹窗外壳**：

```
<Modal open onClose size="md" title="创建群聊"
       footer={<><Button variant="ghost" onClick={close}>取消</Button>
                 <Button variant="primary" ...>创建群聊</Button></>}>
```

`Modal` 已提供：焦点陷阱、`Esc` 关闭、遮罩点击关闭、关闭后焦点归还触发元素（`Modal.tsx:59-103`）。
`size="md"` = `max-w-lg` = 512px（`Modal.tsx:31`）。

**标题栏下方的渐变高光线**（`Modal.tsx:134`，`from-primary-start/70`）是既有 `Modal` 的一部分，**沿用，不删也不加**——它 1px 高、不构成紫蓝渐变或发光边框。

**分区**：弹窗内容区（`px-6 py-4`，`Modal.tsx:152`）内自上而下 4 段，用 `gap-4`（16px）分隔：

```
① 群名（TextField，required）
② 群主题（TextField，optional）
③ 成员（.avatar-picker 网格 + 已选计数）
④ 模式（.bot-filter 分段控件）
   └ 自主模式时露出：轮次上限（.bot-filter 分段控件）
```

段落之间**不画分割线**，靠 `gap-4` + 标签层级区分。加横线会让 512px 宽的弹窗变成表单堆叠。

### 3.2 群名与主题

复用 `TextField`（`Field.tsx:117`），自带 `.field` 类（`index.css:211-226`）：

| 状态 | 表现 | 来源 |
| --- | --- | --- |
| 默认 | `bg-surface-2`、`border-line`、`px-3 py-2`、14px | `211-215` |
| 悬停 | `border-line-strong` | `217-219` |
| 聚焦 | `border-transparent` + `box-shadow: 0 0 0 2px rgb(54 135 115/.85), 0 0 18px -4px rgb(54 135 115/.6)` | `220-223` |
| 禁用 | `opacity-50 cursor-not-allowed` | `224-226` |
| 错误 | `border-error/60`（`Field.tsx:157`）+ 下方 `text-xs text-error` 文案（`Field.tsx:86-91`） | — |

> **缺口提示**：`.field:focus` 的第二层阴影是一圈 `18px` 的绿色辉光，与红线「禁止发光边框」冲突。但这是**全站既有焦点语言**（设置页、BotEditor、Select 全部共用）。本规范**建议沿用**，不在群聊弹窗里单独发明另一套焦点样式——统一性优先于单点完美。若产品决定收紧红线，应全局改 `.field:focus`，而不是只改群聊。

| 字段 | 标签 | 必填 | 占位符 | 校验 |
| --- | --- | --- | --- | --- |
| 群名 | `群名` | 是（`text-error` 星号） | `例如：产品评审` | 1–50 字；为空时提交按钮禁用 |
| 群主题 | `群主题` | 否 | `这群 Bot 一起讨论什么` | ≤200 字 |

辅助文案（`helper`，`text-xs text-ink-3`，`Field.tsx:53-54`）：
- 群名无 helper。
- 群主题 helper：**留空**。「一句话说明这群 Bot 讨论什么」属于解释性文案，违反 §1.5。

### 3.3 成员多选

复用 `BotEditor` 的 `.avatar-picker` 几何（`index.css:700-703`），**每个格子加 Bot 名**（`.avatar-picker` 原样只有头像，因为 BotEditor 里选的是「头像」本身；群聊选的是「人」，必须能认出是谁）：

| 属性 | 值 | 来源 |
| --- | --- | --- |
| 网格 | `grid-template-columns: repeat(6, minmax(0,1fr)); gap: 8px` | `700` |
| 格子 | `padding: 8px; border-radius: 13px; border: 1px solid transparent; background: rgb(var(--soren-surface-3)/.5)` | `701` |
| 头像 | `48 × 48`, `object-fit: contain` | `703` |
| 名字 | 新增，`text-xs`(12/16)、`text-ink-2`、`truncate`、居中 | 本规范新增 |
| <760px | `grid-template-columns: repeat(4, minmax(0,1fr))` | `749` |

6 列下每格约 70px：48px 头像 + 8px×2 内边距 = 64px，剩余 6px 给边框，**不溢出**。11 个内置头像排 2 行。

**Bot 选择态三态**：

| 状态 | 视觉 | 来源 / 实现 |
| --- | --- | --- |
| **未选** | `bg: rgb(var(--soren-surface-3)/.5)`，`border: 1px solid transparent`，名字 `text-ink-2` | `701` |
| **选中** | `border-color: var(--accent)`；`background: var(--accent-soft)`；名字 `text-ink-1` | `702` + 名字提色 |
| **禁用** | `opacity: 40%`（与 `MessageBubble.tsx:53` 的 `disabled:opacity-40` 一致），`cursor: not-allowed`，`aria-disabled="true"` | 本规范新增 |

- 选中态用 `aria-pressed="true"` 驱动（`.avatar-picker button[aria-pressed=true]`，`index.css:702`）。**不要用 `aria-selected`**——这是多选按钮组，不是 listbox。
- **禁用条件**：`bot.status !== 'active'` 的 Bot 不可选（服务端会 400，`PLAN §6`）。`archived` Bot 直接**不渲染**，不占位。
- 选中态**额外加一个勾选标记**：`.avatar-picker` 原本靠边框变色表达选中，但边框色 `var(--accent)` 在深色主题下与底色对比偏弱。追加一个 `IconCheck size={14}` 绝对定位于格子右上角，`color: var(--accent)`、`background: var(--card)`、2px 圆角，保证深浅主题都读得出。

**已选成员展示方式**：不另设「已选」区域。
理由：群聊最多 5 人（`PLAN §6` 校验 `2 ≤ n ≤ 5`），选中后原位高亮即可；另设区域会让人在两处信息之间来回对照，反而增加认知成本。已选信息用**计数**表达：

```
成员  3/5
```

`text-sm text-ink-1`（计数）/ `text-ink-3`（上限）。已达 5 人时，未选格子全部转为禁用态并显示 `aria-disabled`，**不禁用提交按钮**——已有 5 人就是合法状态。

**错误态**：成员 < 2 时，在计数行右侧追加 `text-xs text-error` 的「至少选择 2 个成员」，**不逐格标红**。

### 3.4 模式切换：分段控件

复用 `.bot-filter`（`index.css:681-683`），它是项目里已有的分段选择器：

```
class="bot-filter"   （display:flex; gap:10px; margin-bottom:0）
button[aria-pressed="true"] → background: var(--accent-soft); color: var(--accent)
```

共享控件块覆盖后每项实际为：高度 36px、`padding: 0 12px`、`min-width: 64px`、圆角 10px、13px、默认色 `ink-2`（`index.css:932-946`）。**注意 `margin-bottom: 24px` 来自 `.bot-filter`（`681`），在弹窗内必须覆盖为 0**，否则与其他段落间距冲突。

两项：

| 项 | 值 | 说明文案 |
| --- | --- | --- |
| 提及 | `mention` | 你 @ 谁，谁回答 |
| 自主 | `auto` | 群内 Bot 依次自主发言 |

用 `aria-pressed` 而非 `role="radio"`——与 `.bot-filter` 既有实现保持一致。

### 3.5 轮次上限：分段控件，不是步进器

**结论：用分段控件（1–6 六段），不用 `− / +` 步进器。**

理由：

1. **取值域只有 6 个离散值**（`PLAN §6`：`maxRounds ∈ 1..6`）。分段控件一屏展示全部选项，步进器需要 1–5 次点击才能到达 `6`，是纯粹的操作成本。
2. **这个参数的本质是成本/时长的权衡**。用户在选 `maxRounds` 时需要看到「最多能到 6」这个上限才能判断；步进器把域信息藏起来了，用户会反复点 `+` 去试探最大值。
3. **步进器需要一个「当前值」的大号数字展示**，那会引入本功能不存在的第 5 级字号，与 §1.3 的四级体系冲突，也违反 §0.1 坑 3（「所有尺寸统一」）。
4. **有既有先例**：`.bot-filter` 就是这个形态（`index.css:681-683`），已在 BotEditor 中使用。复用 = 零新增样式。

规格：

```
容器  class="bot-filter"  （margin-bottom 覆盖为 0）
按钮  1  2  3  4  5  6
选中  aria-pressed="true" → bg var(--accent-soft) + color var(--accent)
默认  color rgb(var(--soren-ink-2))
```

- 默认值 **3**（`PLAN §5.2` 的 `MaxRounds int ... default:3`）。默认档位于中段，便于用户向两侧调整。
- 每项只显示数字，**不加「轮」字**。段标签右侧统一放 `text-xs text-ink-3` 的说明：`最多 5 位成员 × 6 轮 = 30 次模型调用`。
  这句是成本信息而非解释性文案——用户需要它才能选。数字取自 `PLAN §6` 推导的不变量 `成员数 × maxRounds ≤ 30`，应随当前值动态计算。
- **仅 `auto` 模式显示**。切到 `mention` 时整段卸载（不是 `disabled`），因为 mention 模式忽略该值，露出一段无效控件就是噪音。

### 3.6 按钮层级

复用 `ConfirmDialog` 的既有层级（`Modal.tsx:207-217`）：

| 按钮 | variant | 类 | 说明 |
| --- | --- | --- | --- |
| 取消 | `ghost` | `btn-ghost` | 左侧 |
| 创建群聊 | `primary` | `btn-primary` | 右侧，DOM 在后 → 视觉在右 |

两者尺寸相同（`control-button`，36px / 10px 圆角 / `min-width: 64px`），**靠色阶分层，不靠尺寸**。这是 §0.1 坑 3 的直接推论。

### 3.7 空提交态

「创建群聊」按钮的可用性**由数据决定，不做点击后报错**：

| 条件 | 按钮状态 | 文案 |
| --- | --- | --- |
| 群名为空 或 成员 < 2 | `disabled` | `创建群聊` |
| 全部合法 | 启用 | `创建群聊` |
| 请求中 | `loading`（`Button.tsx:67` 自动渲染 `Spinner`，`aria-busy`） | `创建群聊` |
| 校验失败（400） | 恢复 `disabled` | `创建群聊` |

- **禁用态样式**：`btn-primary` 的 disabled 表现沿用既有实现（`Button.tsx:57` 只加 `disabled` 属性，视觉由 `.btn-primary` 的 `:disabled` 规则决定）。若该规则缺失，向父级报告，**不要自造一套灰色按钮**。
- 按钮文案在所有状态下**保持「创建群聊」不变**。加载时靠 `Spinner` 表达，不改成「创建中…」——`.btn-primary` 已有 spinner 占位，改文案会导致按钮宽度跳变。
- **400 错误落到字段上**：群名错误 → `TextField` 的 `error`；成员错误 → §3.3 的计数行。不弹 Toast，Toast 用于非字段类错误（500、网络失败）。
- **不预填任何字段**。不预填群名，不默认选中任何 Bot。用户创建群聊是一个明确动作，预填等于替他做决定。

---

## 4. 侧栏群聊分组

改造点：`Sidebar.tsx:55-68` 的 `.bot-inbox-list` 之后、`.inbox-settings`（`Sidebar.tsx:69`）之前插入分组。

### 4.1 群列表项与 Bot 项的同一套尺寸（行高与内边距硬性一致，左槽位为唯一例外）

`PLAN §8.3` 要求「与 Bot 项保持同一套行高与内边距」。`.bot-inbox-row`（`index.css:826-828`）的实际值：

| 属性 | 值 | 群列表项必须相同 |
| --- | --- | --- |
| `min-height` | **62px** | ✅ |
| `padding` | **10px** | ✅ |
| `gap` | **10px** | ✅ |
| `border-radius` | **12px** | ✅ |
| `margin-bottom` | **3px** | ✅ |
| `align-items` | `center` | ✅ |
| 悬停 | `background: rgb(var(--soren-surface-4)/.5)` | ✅ |
| 选中 | `background: rgb(var(--soren-ink-1)/.065)`（`:[aria-current=page]`） | ✅ |
| 过渡 | `background .15s` | ✅ |
| 主副文本规格 | `.inbox-copy`（`14px/600/ink-1` + `13px/ink-3`） | ✅ |
| **左槽位宽度** | **38px**（`.inbox-avatar`） | ❌ **群项为 62px，唯一差异** |

**唯一差异：左侧槽位宽度。**

`PLAN §8.3` 的硬性要求是「同一套**行高与内边距**」，这两项群项与 Bot 项**完全相同**（62px / 10px），由同一个 `bot-inbox-row` 类名保证。
但槽位宽度无法相同：Bot 项 1 个 38px 头像（`index.css:829`），群项要叠放 3 个 26px 头像（`26 + 18 + 18 = 62px`），38px 装不下——强行塞进去只能把头像压到 18px 以下，实测认不出是哪个 Bot（校验渲染见 `docs/previews/group-chat-ui-spec.html` §4）。

**因此明确接受一处差异**：群项左槽位 **62px**，比 Bot 项宽 **24px**，导致**群项的文字左边缘比 Bot 项右移 24px**。
两者被分割线与「群聊 N」标题隔成两个独立列表（§4.2），不会出现同一列表内文字参差的情况。
若产品认为文字必须严格对齐，唯一的替代方案是把 Bot 项槽位也改成 62px（Bot 头像从 38px 缩到 26px）——**那是改动既有单聊侧栏，本功能不擅自决定**，见 §9 Q10。

**实现方式**：群列表项**直接复用 `class="bot-inbox-row"`**，不写新类。只在内部把 `.inbox-avatar` 换成一个 62px 的头像簇容器。

这样「行高与内边距一致」就不是靠人工对齐，而是**同一个类名**——§10 的验收项「行高、内边距一致」可由类名断言直接通过。

**不加的东西**（`PLAN §8.3` 明令）：不加彩色标签、不加特殊圆角、不加「群」字徽章、不加左侧色条。

### 4.2 分组标题

复用 `.inbox-heading`（`index.css:821-822`，被 `890` 覆盖为 13px）：

```
群聊  3
```

| 元素 | 规格 | 来源 |
| --- | --- | --- |
| 容器 | `padding: 12px 10px 10px`；`color: rgb(var(--soren-ink-3))`；`font-size: 13px` | `821` / `890` |
| 计数 | `font-size: 10px; opacity: .7`（`:last-child`） | `822` |
| 文案 | `群聊`（与现有 `Bot` 标题同构，`Sidebar.tsx:53`） | — |

**分割线**：`.inbox-heading` 上方加 `border-top: 1px solid rgb(var(--soren-line-subtle))`。
`index.css:821` 的 `.inbox-heading` 没有上边框，这是群分组专有的一条线。

分割线**只在有群聊时显示**。群聊数为 0 时整段（分割线 + 标题 + 列表）不渲染，避免出现「群聊 0」的空分组。

### 4.3 多头像叠放

**槽位宽度 62px，恰好 3 格。** 这是本文档唯一一处与 Bot 项不同的尺寸（见 §4.1），理由见该节。

```
62px = 26（第 1 张完整）+ 18（第 2 张可见）+ 18（第 3 张可见）
其中每张 26px，后两张 margin-left: -8px
```

重叠量 **8px** 直接取自 `PLAN §8.3`「重叠 8px」，不是自选值。

| 属性 | 值 | 说明 |
| --- | --- | --- |
| 槽位容器 | `width: 62px; flex-shrink: 0` | 作为 `.bot-inbox-row` 的直接子元素，替换 `.inbox-avatar` |
| 单个头像 | `26 × 26` | 占 Bot 头像 36px 的 72%，肉眼可辨 |
| 圆角 | `7px` | 与 26px 尺寸匹配，且与整体 10–13px 圆角语言一致 |
| 重叠量 | **`margin-left: -8px`** | 第 2、3 张各露出 18px |
| 首张 | `margin-left: 0` | 贴齐槽位左边缘（行内 `padding: 10px` 已就位） |
| 描边 | `2px solid` **侧栏实际底色** | 见下 |
| 叠放顺序 | 成员数组顺序，第 1 个在最左 | 不按活跃度排序，避免每次渲染跳动 |

**描边用侧栏底色，不用 `line`，也不用 `surface-1`**：
侧栏的真实底色是写死的 `#f3f4f1`（浅）/ `#191e22`（深）（`index.css:813-814`），
**不是** `var(--sidebar)`（`#f0f2eb` / `#111519`，`index.css:452-453`），也不是 `surface-1`（`#faf9f6` / `#14181d`）。
用错任何一个，描边与底色之间会出现一圈可见的亮/暗边。校验渲染中按正确值渲染，头像之间是干净的「切开」效果。

**`+N` 徽标**：**占用第 3 格**，不是第 4 个元素。

| 成员数 | 三格内容 | 槽位宽 |
| --- | --- | --- |
| 2 | 头像、头像、空 | 62px |
| 3 | 头像、头像、头像 | 62px |
| 4 | 头像、头像、`+2` | 62px |
| 5 | 头像、头像、`+3` | 62px |

徽标规格与叠放头像完全同规格（`26×26`、`7px` 圆角、`2px` 侧栏底色描边），
底色 `rgb(var(--soren-surface-4))`（`index.css:20`，即「悬浮/选中态」档，在浅色下与侧栏底色有可辨差异），
文字 `10px / rgb(var(--soren-ink-1))`，内容为 `成员数 − 2`。

> 校验记录：初版曾把 `+N` 作为第 4 个 flex 子元素放在 38px 槽外，渲染后**与群名文字碰撞**（槽位溢出压住 `.inbox-copy`）。改为「3 格固定 62px，`+N` 占用第 3 格」后碰撞消失，且群项文字左边缘在所有群行之间恒定。

**非图片内容**：成员头像缺失或加载失败时，`img` 走 `onError` 回退到 `/soren-mark.svg`（与 `avatarURL()` 行为一致，`agents.ts:5`），不显示破图占位符。

成员数 < 3 时空出的格子**保持为空**（不画占位圆），槽位仍为 62px 以保证文字对齐。

### 4.4 预览文本的层级与截断

复用 `.inbox-copy`（`index.css:834-837` + `888-889`），**结构与 Bot 项完全相同**：

| 元素 | 规格 | 来源 |
| --- | --- | --- |
| 容器 | `min-width: 0; flex: 1; flex-direction: column; gap: 3px` | `834` |
| 主文本 `strong` | `14px / 21px / 600 / rgb(var(--soren-ink-1))` | `888` |
| 副文本 `small` | `13px / 19px / rgb(var(--soren-ink-3))` | `889` |
| 截断 | `overflow: hidden; text-overflow: ellipsis; white-space: nowrap` | `837` |

**副文本内容**：`lastMessagePreview`，格式为 `发言人名：内容`（`PLAN §5.4`），后端已截断到 200 字。前端**不做二次截断**，靠 CSS 单行省略号处理。

**发言人名不加粗、不变色**：预览已经是 `ink-3`，如果把前缀「讨论架构师：」再染成强调色，侧栏会出现一块彩色文本，与 §4.1「不加彩色标签」冲突。

**空群态**（还没有任何消息）：

| 状态 | 副文本 |
| --- | --- |
| 群已创建但无消息 | `还没有对话` |
| 加载中 | `正在加载…` |

与 `Sidebar.tsx:58` 的 Bot 项三态写法（`正在加载对话…` / `还没有对话，打个招呼吧`）保持同一套措辞结构。

### 4.5 超长内容处理

| 场景 | 处理 |
| --- | --- |
| 超长群名 | `truncate` 单行省略（`.inbox-copy strong` 已带 ellipsis），`title={group.title}` 悬停显示全文 |
| 超长群名 + 计数 | 省略号优先吃掉主文本宽度，计数在最右**始终完整**（`flex-shrink: 0`） |
| 2–5 人群（任意成员数） | 槽位恒为 62px、恒 3 格，**文字左边缘在所有群行之间一致**，不会因成员数变化而跳动 |
| 头像加载失败 | 回退 `/soren-mark.svg`，槽位尺寸不变 |

**3 格算法（已渲染验证，非推测）**：

```
槽位宽 62px = 26 + 18 + 18，其中 26 为头像边长、18 = 26 − 8（重叠 8px）
成员 ≤ 3：第 1–3 格放头像
成员 > 3：第 1–2 格放头像，第 3 格放「+N」徽标（N = 成员数 − 2）
```

**为什么不用「3 头像 + 第 4 格 `+N`」**：4 个 26px 元素按 8px 重叠后总宽 `26 + 18×3 = 80px`，比 62px 多占 18px；
且成员 3 人时占 3 格、成员 5 人时占 4 格，**槽位宽度会随成员数变化**，导致同一列表内群行的文字左边缘参差。
「`+N` 占第 3 格」同时满足「最多 3 个可见头像」（`PLAN §8.3`）与「宽度恒定」两个约束。

**为什么不用 38px 槽位**：38px 内放 3 个可辨识头像需压到 18px 以下，实测无法辨认；
38px 内放 2 个 26px 头像（重叠 14px）则每张只露出 12px，同样不可用。
故槽位必须宽于 38px，该差异已在 §4.1 显式声明。

### 4.6 交互

- 点击：`onSelectGroup(group.id)`，等价于 Bot 项的 `onSelectBot`。
- 移动端：走既有 `navigate()` 模式（`Sidebar.tsx:44`）——执行后调 `onClose()` 关抽屉。
- `aria-current={选中时 'page'}`：与 Bot 项一致（`Sidebar.tsx:60`）。
- `aria-label`：`群名 · 发言人名：预览前 30 字`。
- **不提供群列表项的右键菜单 / 删除入口**。删除群的交互属于功能范围（`PLAN §5.2` 有 `DELETE /api/groups/:id`），但 §12 未决事项已说明入口待定；本规范只预留：删除入口**不写在群列表项上**，若后续需要，放群聊页标题栏。

---

## 5. 群聊页消息流

### 5.1 发言人头像与名称（核心）

**解析规则**（`PLAN §8.4`）：

```
speakerBot = message.speakerBotId ? bots.find(b => b.id === message.speakerBotId) : undefined
显示 Bot   = speakerBot ?? bot            // bot = 单聊既有 prop
```

回退链保证了「群聊里每条 assistant 消息来自不同 Bot，头像与名称必须逐条正确解析」（§10 验收项），且单聊行为完全不变。

**头像**——沿用 `MessageBubble.tsx:134-138` 现状，**只换数据源，不改样式**：

```
<div className="mt-0.5 shrink-0">
  <div className="rounded-md ring-1 ring-line/70 overflow-hidden">
    <img src={avatarURL(speakerBot?.avatar || 'soren')} width={30} height={30} />
  </div>
</div>
```

- 尺寸 30×30、圆角 `rounded-md`(12px)、`ring-1 ring-line/70` 全部不变。
- 用裸 `<img>` 而非 `BotAvatar`：`BotAvatar`（`BotAvatar.tsx:19`）是侧栏用的 36px + 眨眼动效版，气泡里换用它会改变尺寸并引入不需要的动画。**保持现状。**
- `speakerBotId` 存在但 Bot 已被删除 → `avatarURL('soren')` 兜底，名称用 `message.speakerName`（后端冗余存储，`PLAN §5.1`）。

**名称行的精确排版**：

```
位置：在内容列内、气泡上方，与气泡左边缘对齐（不是与头像左边缘对齐）
字号：text-xs（12px / 16px）
颜色：text-ink-2          ← 见下方对比度实测
字重：默认 500（不加粗）
与头像的间距：头像右边缘 → 内容列左边缘 = gap-3 = 12px（MessageBubble.tsx:132 既有）
与气泡的间距：内容列 gap-1.5 = 6px（MessageBubble.tsx:140 既有）
截断：单行 truncate，max-width 跟随内容列（max-w-[82%]）
```

DOM 位置：插在 `MessageBubble.tsx:140` 的内容列 `<div>` 内，**记忆召回提示之前**。顺序是「身份 → 记忆提示 → 正文」，身份在最上。

> **与 `PLAN §8.4` 的偏差（实测依据）**
> 方案写的是 `12px / text-ink-3`。实测对比度（WCAG 2.1，底色为 `bubble-assistant` 的 `surface-1`）：
>
> | 颜色 | 浅色主题 | 深色主题 | 结论 |
> | --- | --- | --- | --- |
> | `text-ink-3` | **3.24:1** | 6.07:1 | 浅色主题 **不达 AA(4.5:1)** |
> | `text-ink-2` | **4.54:1** | 8.60:1 | 两主题均达 AA |
>
> 发言人名是**识别发言者的唯一视觉线索**（头像小、群聊里发言者多），3.24:1 在浅色主题下不可接受。
> **本规范采用 `text-xs text-ink-2`。** 请产品确认接受这一处对方案的修正。

**连续同发言者的处理**：
**不隐藏、不合并**——每条消息都显示头像与名称。
理由：§10 的验收项是「N 条不同发言者 = N 个不同头像」，逐条渲染是唯一无歧义的实现方式；一旦引入「连续同人则隐藏」的规则，测试断言与用户预期都会出现「到底该显示几个头像」的歧义，且在自主模式里发言顺序不可预测，隐藏规则会让身份线索变得不稳定。
代价是 3 个 Bot × 3 轮 = 9 条消息会有 9 组头像。这是信息真实性的成本，接受。

**头像 ring 在深色主题下**：`ring-line/70` 是 `rgb(var(--soren-line) / 0.7)`，随主题变化，无需处理。

### 5.2 @ 提及高亮

**规范：纯文字强调，不加背景色块，不加边框盒。**

实现（作用在用户气泡内容内，行内 `<span>`）：

```
color: rgb(var(--accent));
font-weight: 600;
```

**为什么不用方案写的 `text-primary-start`**（实测）：

`primary-start` 是写死的 `#368773`，而用户气泡底色 `bubble-user` 用的 `var(--accent-soft)` **会随主题切换**。实测：

| 前景 | 浅色底 `#e5eee3` | 深色底 `#253d3b` | 判定 |
| --- | --- | --- | --- |
| `text-primary-start` `#368773` | 3.63:1 | **2.69:1** | 深色主题 **连 3:1 的 UI 最低线都不到** |
| `rgb(var(--accent))` 浅`#347b67`/深`#9bd8c6` | 4.23:1 | 7.20:1 | 深色优，浅色差 0.27 |

**结论：`text-primary-start` 在群聊里不可用。** 深色主题下 @提及几乎不可见，这是硬性缺陷，不是风格偏好问题。

采用 `rgb(var(--accent))`。Tailwind 类名写法（无需改 `tailwind.config.js`）：

```
class="text-[rgb(var(--accent))] font-semibold"
```

浅色主题 4.23:1 略低于 AA 的 4.5（差 6%）。若需严格达标，最小改动是给「用户气泡内的提及」单独用更深一档，见 §8 缺口 G2 的建议值。

**为什么完全不加描边**（对方案「轻描边内联标记」一处偏差）：
行内元素的 `border` 必须在 `display: inline-block` 上才渲染，而 `inline-block` + 垂直 padding 会改变 15px/`leading-relaxed` 正文框内的行高节奏——这正是方案要避免的「切碎消息流」。底部下划线方案（`border-bottom` + `padding-bottom: 1px`）不切碎行，但会在长消息里形成一条横贯的线，仍然读作「装饰」。

结论：`color + font-weight` 是唯一既满足「一眼可辨」又满足「不切碎消息流」的最小标记。若产品坚持要描边，**一行可切换**：

```
class="text-[rgb(var(--accent))] font-semibold underline decoration-1 underline-offset-2 decoration-[rgb(var(--accent)/0.4)]"
```

**解析范围**：只对 `role === 'user'` 的消息生效（与 `PLAN §8.4` 一致），且只匹配**当前群成员名**。用户气泡用 `whitespace-pre-wrap`（`MessageBubble.tsx:91`）直出文本，要高亮必须改成「按 mention 切分 → 渲染 `<span>`」的结构化渲染，不能继续直接输出字符串。

**截断与换行**：mention 跨行时给 span 加 `box-decoration-break: clone`，避免下划线在换行处断裂。

**不匹配的兜底**：`@` 后跟非成员名（如 `@ someone`、邮件地址、代码片段）**不高亮**，保持普通正文色。误伤代码块和邮箱比漏高亮更糟。

### 5.3 system 消息

**复用 `MessageBubble.tsx:119-128` 的现有分支，不做任何改动。**

> 行号说明：`PLAN §7.4` 与任务描述引用的是 `118-126`，实际代码里该分支是 **119–128**（`120` 是 `if (!isAssistant)`，`121-127` 是 JSX）。施工时以实际代码为准。

现状渲染：

```
<div class="flex justify-center">
  <span class="rounded-full border border-line bg-surface-3 px-3 py-1 text-xs text-ink-3">
    {message.content}
  </span>
</div>
```

视觉确认点：

| 确认项 | 预期 | 依据 |
| --- | --- | --- |
| 位置 | 水平居中，不左对齐也不右对齐 | `flex justify-center` |
| 形态 | 全圆角 pill（`rounded-full`） | 与气泡的 18px 非对称圆角明显区分 |
| 底色 | `bg-surface-3` | 比 `bubble-assistant` 的 `surface-1/.8` 更深一档，视觉后退 |
| 描边 | `1px solid rgb(var(--soren-line))` | 有边框，因此小尺寸下仍有轮廓 |
| 文字 | `text-xs`(12px) + `text-ink-3` | 弱于正文 |
| 无头像 | 不渲染头像、无名称行 | system 没有发言人 |
| 间距 | 由父容器 `flex flex-col gap-5`(20px) 提供 | `ChatPage.tsx` 消息列表 |

**群聊场景下的内容规范**（后端写入，前端只负责展示）：

| 场景 | 文案 | 来源 |
| --- | --- | --- |
| 用户中止 | `讨论已中止` | `PLAN §7.4` |
| 服务重启收敛 | `服务已重启，本次讨论未完成` | `PLAN §7.4` / `§5.2` |
| 讨论自然结束 | 不写 system 消息 | 正常 `done` 不需要说明 |

**约束**：`speakerBotId` 必须留空（`PLAN §7.4` 明确要求）。前端渲染分支已按 `!isUser && !isAssistant` 判定，天然不会走发言人解析路径。

**不要**为了显示中止说明而伪造 assistant 消息（`PLAN §7.4` 明令）。

### 5.4 记忆召回提示与操作栏

操作栏（`MessageBubble.tsx:186-212`）**不改动**。记忆召回提示（`MessageBubble.tsx:142-163`）**改一处颜色**。

现状是 `border-primary-start/25 bg-primary-start/10 text-primary-start`（`MessageBubble.tsx:151`）。
记忆 chip 在群聊里同样会渲染，而 `primary-start` 是写死色，落在 `bubble-assistant`（`surface-1/.8`）上：

| 前景 | 浅色 | 深色 | 判定（文字门槛 4.5:1） |
| --- | --- | --- | --- |
| `text-primary-start` `#368773` | 4.10:1 | 4.13:1 | 两主题均**不达标** |
| `rgb(var(--accent))` 浅`#347b67`/深`#9bd8c6` | **4.78:1** | **11.06:1** | 两主题**均达标** |

改为（透明度档位保持不变，只换色源）：

```
border-[rgb(var(--accent)/0.25)]  bg-[rgb(var(--accent)/0.10)]  text-[rgb(var(--accent))]
```

**影响范围**：记忆 chip 在 `MessageBubble` 内，**单聊与群聊共用同一段代码**，这一改会同时改变单聊外观（深色下由暗青变为薄荷色，可读性提升）。
这是本规范唯一一处触及单聊既有视觉的改动，见 §9 Q11。

**操作栏不动**：`copy` / `regenerate` 用 `text-ink-3` → `hover:text-ink-1`（`MessageBubble.tsx:51-52`），
`ink-1` 在 `surface-1` 上为 11.77:1 / 15.07:1，两主题均达标，无需处理。

### 5.5 `primary-start` 全文审计

`PLAN` 中除 §8.4 的 @ 高亮外，还有多处隐含使用强调色。逐处核对结果：

| 位置 | 现状 | 浅 | 深 | 类型 | 结论 |
| --- | --- | --- | --- | --- | --- |
| §5.2 @提及 | 方案原定 `text-primary-start` | 3.63 | **2.69** | 文本 | ❌ **改** `rgb(var(--accent))`（4.23 / 7.20） |
| §5.1 发言人名 | 方案原定 `text-ink-3` | 3.24 | 6.07 | 文本 | ❌ **改** `text-ink-2`（4.54 / 8.60） |
| §5.4 记忆 chip 文字 | `text-primary-start` | 4.10 | 4.13 | 文本 | ❌ **改** `rgb(var(--accent))`（4.78 / 11.06） |
| §5.4 记忆 chip 边框/底 | `primary-start/25`、`/10` | — | — | 非文本 | ⚠️ 一并换色源，透明度档不变 |
| §7.4 流式光标 | `bg-primary-start`（`MessageBubble.tsx:178`） | 4.10 | 4.13 | **非文本 UI** | ✅ 门槛 3:1，**保持不动** |
| §3.1 Modal 顶部高光线 | `from-primary-start/70`（`Modal.tsx:134`） | 4.10 | 4.13 | **装饰** | ✅ 1px 非文本，**保持不动** |
| §3.4/§3.5 分段控件选中态 | `color: var(--accent)`（`index.css:683`） | 4.23 | 7.20 | 文本 | ✅ 沿用既有类；浅色 4.23 与 @提及 同为已知边际值（Q4） |
| §3.3 成员名（未选） | `text-ink-2`，底 `surface-3/.5` | ≈4.5 | — | 文本 | ✅ 达标 |
| §3.3 成员名（选中） | `text-ink-1`，底 `--accent-soft` | 10.43 | — | 文本 | ✅ 达标（故选中态必须提色到 `ink-1`） |

**规则**：凡是**文字**落在会随主题切换的底色上，一律用 `ink-*` 或 `rgb(var(--accent))`，不用 `primary-start`；
`primary-start` 仅允许用于**非文本**元素（光标、装饰线），门槛为 3:1 且均已达标。

---

## 6. 输入区

### 6.1 @ 补全候选列表

**位置**：**光标正上方**，浮于消息流之上、位于输入框之上。
不放在输入框内部下方——输入框下方是 `composer-caption` 区域，且 Tauri 窗口底部空间紧张。

**容器**：复用 `.app-select-menu`（`index.css:792`），`position: fixed; z-index: 100; padding: 6px; border-radius: 12px; border: 1px solid line; background: rgb(surface-1); box-shadow: 0 12px 40px -8px #0003`。

| 属性 | 值 |
| --- | --- |
| 宽度 | `280px`（比加号菜单宽，因为候选项要放头像 + 名称 + 角色） |
| 最大高度 | `196px`（约 5 行 × 38px + 12px 内边距，超出滚动） |
| 与输入框间距 | **8px**（浮层与锚点留呼吸；`Select.tsx:27` 用 6px，@ 列表更靠近输入区取 8px） |
| 左边缘 | 与 textarea 的**文字起始位置**对齐，即 `composer` 左内边距 `17px` + textarea `padding-left` `4px` = **距 `.composer` 左边缘 21px** |
| 定位 | 复用 `Select.tsx:20-31` 的 `getBoundingClientRect` + `place()` 模式 |

**候选项**：复用 `.app-select-option`（`index.css:793`）

| 属性 | 值 |
| --- | --- |
| 单项 | `min-height: 38px; padding: 9px 11px; border-radius: 7px; font-size: 13px` |
| 头像 | `28 × 28`，`rounded-md`(12px)，`ring-1 ring-line/70`（与 §5.1 气泡头像同语言） |
| 名称 | `13px / ink-1` |
| 键盘当前项 | `background: rgb(var(--soren-surface-3))`（`.is-active`） |
| 悬停 | 同上（同一套） |
| 已发言标记 | 不加 |

**键盘**（复用 `Select.tsx:41-56`）：

| 按键 | 行为 |
| --- | --- |
| `↑` / `↓` | 循环移动候选高亮 |
| `Enter` | 选中候选，替换 `@` 到光标之间的文本 |
| `Tab` | 同 Enter（对话补全的通用预期） |
| `Esc` | 关闭候选，**不阻止**文本继续可编辑 |
| 输入空格 / 中文 / 退格删除完 `@` 前的全部内容 | 关闭候选 |

**鼠标**：`onPointerMove` 同步 `active` 索引（与 `Select.tsx:58` 一致），点击即选中。`onMouseDown` 需 `preventDefault()` 防止 textarea 失焦（`Select.tsx:58` 已这样做）。

**过滤**：输入 `@` 后到光标为止的内容作前缀，对**群成员名**做 `includes` 过滤（不是 `startsWith`，中文里 `startsWith` 命中率太低）。无匹配时**不显示浮层**，不给「无结果」空态——用户只是还没打完。

**不提供数字快捷选择**（`@1` 直接选中第 1 个）。群成员最多 5 人，收益低，且会与「输入数字」冲突。

**IME 保护**：`ChatInput.tsx:11` 已有 `event.nativeEvent.isComposing || event.keyCode === 229` 的守卫，`@` 补全的 keydown **必须复用同一守卫**，否则中文输入法确认候选词时会误触发选中。

### 6.2 讨论状态条

自主模式（`auto`）且讨论进行中时，输入框**上方**出现一条状态条。

**它是一个 strip，不是一个大按钮。** 布局：

```
┌──────────────────────────────────────────────────┐
│ ●●●  讨论中 2/3 轮                    [  ■  ]     │   ← 36px 高
└──────────────────────────────────────────────────┘
```

| 元素 | 规格 | 来源 |
| --- | --- | --- |
| 容器 | `display:flex; align-items:center; gap:2`(8px)；`height: 36px`；与 `.composer` **同宽**（同在 `.composer-wrap` 内，`max-width: 890px`）；`margin-bottom: 8px` | — |
| `TypingDots` | 直接复用 `Button.tsx:146`；三点 `6px` 圆点、`gap-1`、`animate-bounce-dot`；**颜色靠 `bg-current`，需在父级设 `color: rgb(var(--soren-ink-2))`** | `Button.tsx:146-158` |
| 计数文字 | `text-xs`(12/16) + `text-ink-2` + `tabular-nums` | — |
| 弹性间隔 | `flex: 1`（`<span class="ml-auto" />` 或容器 `justify-content: space-between`） | — |
| **中止按钮** | 见下 | — |

**中止按钮规格（本节是全文视觉红线最需要被遵守的地方）**：

| 属性 | 值 | 来源 |
| --- | --- | --- |
| 尺寸 | **36 × 36**（`--control-height`） | `index.css:932-951` |
| 圆角 | **10px**（`--control-radius`） | `index.css:938` |
| 类名 | `control-icon`（复用 `IconButton` 的 `ICON_BOX_CLASS`，`Button.tsx:83-87`） | — |
| 底色 | 透明（`btn-ghost` 默认无底） | — |
| 图标色 | `text-error` `#d06b63` | — |
| 图标 | `IconStop size={16}`（已有，`icons.tsx:101`） | — |
| 悬停 | `bg-surface-3` | `MessageBubble.tsx:52` 既有写法 |
| 焦点 | `outline: 2px solid var(--accent); outline-offset: 2px` | `index.css:953-956` |
| 禁用 | 不提供（讨论开始后按钮立即可用） | — |

**这就是 `IconButton` 的标准形态，与 `MessageBubble` 操作栏里的复制/重新生成按钮视觉完全一致。**

**「与输入框同高、同圆角」的解读说明**（重要）：

`PLAN §8.5` 与任务描述都要求中止按钮「与输入框同高、同圆角」。但 `.composer` 是**多行文本区**——`min-height: 62px`、圆角 `20px`（`index.css:552,554`）。若按字面让按钮 62px 高、20px 圆角，那是一个 62×62 的巨型按钮，**正是此前被否决过的「大号专属造型」错误**（麦克风按钮 68px 事件）。

本规范的解读：**「输入框」指输入区的动作按钮，即 `.composer` 内的 `.send-button`**（`index.css:560`，同为 36×36 / 圆角 10px）。
按此解读，中止按钮与发送按钮**严格同高同圆角**，且都是标准控件尺寸，没有任何专属造型或额外视觉重量。
**请产品确认这一解读。** 若确要求 62px，则必须同时下调整个操作区尺寸，属于全局改动，不应由群聊功能单方面引入。

**不做的**：
- 不给中止按钮加红色底、红色描边或发光。
- 不给它动画（无脉冲、无呼吸）。
- 不加大到超过 `--control-height`。
- 不用 `btn-danger` 变体填充实心红色——那是 `ConfirmDialog` 删除确认用的，权重太高。

**轮次进度的表达**：文字 `讨论中 2/3 轮` 用 `text-ink-2`，**不做进度条**。3 位成员 × 6 轮最多 18 条消息，文字足够；进度条是新增控件且在讨论会提前结束（全员弃权，`PLAN §7.3`）时会产生错误的进度暗示。

**状态条的出现/消失**：

| 状态 | 表现 |
| --- | --- |
| `mention` 模式 | **不渲染状态条**（该模式无「讨论中」概念） |
| `auto` 模式，未开始 | 不渲染 |
| `auto` 模式，进行中 | 渲染，`TypingDots` 动 |
| 用户点中止 | 立即消失；**按钮无 loading 态**（中止是瞬时操作，`GroupTurn` 变 `cancelled` 是服务端行为） |
| 单个 Bot 报错跳过 | **不中断**，状态条继续，仅计数不变（`PLAN §7.4`：单 Bot 失败不中断整场） |

**与 `ChatInput` 现有 loading 态的关系**：
`ChatInput.tsx:12` 已有 `{loading && <span>正在思考…</span>}` 的文案，且 `loading` 时发送按钮**变成停止按钮**（`IconStop`，`ChatInput.tsx:12`）。
**群聊 `auto` 模式下这两者会同时出现**——输入框内一个「正在思考…」+ 停止按钮，输入框上方一个「讨论中 2/3 轮」+ 中止按钮，**两个中止按钮功能重复**。

**处置**：群聊页传 `loading={false}` 给 `ChatInput`，让状态条独占「中止」职责。
理由：单聊的停止是「中断一次模型生成」，群聊的中止是「终止整场多轮讨论」，语义不同、文案不同（「停止生成」vs「中止讨论」）。两个按钮同时出现会让用户犹豫点哪个。
若保留 `ChatInput` 的 loading，则**必须**把输入框内文案改成「讨论进行中」并去掉其停止按钮——但这会改动 `ChatInput` 的单聊行为，超出本功能范围。**推荐前者（传 `loading={false}`）。**

### 6.3 群聊空状态

`ChatEmptyState` 的变体（`PLAN §8.4`）。**不新建组件文件结构**，在现有 `ChatEmptyState.tsx:9` 上加 `group` 变体入参。

**不复用 `.welcome-scene` 的大标题 + 吉祥物 + 四张建议卡的布局**（`ChatEmptyState.tsx:10-13`）。理由：

1. 那套布局的主视觉是 `112px` 吉祥物 + `clamp(28px,3vw,38px)` 大标题 + 4 张彩色建议卡。群聊里没有单一「主角 Bot」，放谁的吉祥物都是错的决定。
2. 4 张建议卡（`peach` / `sage` / `lavender` / `sand` 四色，`ChatEmptyState.tsx:5-7`）是**装饰性彩色元素**，踩红线，且群聊场景没有 4 条通用示例可推荐。
3. 侧栏已经有群头像叠放（§4.3），空状态再放一遍成员头像是重复信息。

**群聊变体布局**（`.welcome-scene` 的 `max-width: 590px; margin: 0 auto`，`index.css:645` 保留，其余全部换掉）：

```
        ┌─────────────────────────┐
        │   3 个成员头像横向排列    │   36×36，间距 12px，不叠放
        └─────────────────────────┘

            产品评审                     L4  18px / 600 / ink-1

   把 Bot 拉进同一个群，让它们就同一个问题
   依次给出各自的判断。                          L2  14px / 1.7 / ink-2

        [@提及  谁回答谁]   [自主  依次发言]      ← 两个模式说明，非按钮
```

| 元素 | 规格 |
| --- | --- |
| 成员头像 | 36×36，`rounded-md`(12px)，`ring-1 ring-line/70`，`gap: 12px`，横向居中 |
| 群名 | `text-lg`(18/28) + `font-semibold` + `text-ink-1` |
| 主题 | `text-sm`(14/20) + `leading-[1.7]` + `text-ink-2`；**为空则整段不渲染** |
| 模式说明 | `text-xs` + `text-ink-3`，两项并列，中间 `·` 分隔 |
| 垂直间距 | 头像 → 群名 `16px`；群名 → 主题 `6px`；主题 → 模式说明 `20px` |

**群名超长**：`text-lg` 单行 + `truncate`，容器 `max-width: 590px`。**不改小字号**——空状态是唯一可以放心用一行截断的地方，改字号会让不同群看起来大小不一。

**引导文案**（功能性，可接受）：

| 群类型 | 文案 |
| --- | --- |
| 有主题 | 用主题原文（后端存储） |
| 无主题 | `用 @ 指定要回答的 Bot，或让群内 Bot 依次发言` |

这句承载的是**操作指引**（@ 和自主模式是本功能的两个核心概念），不是抒情装饰。

**不提供示例 prompt 卡片**：群聊示例依赖具体成员组合，编造 4 条通用示例（如单聊那样）会产生与群主题无关的噪音内容。

### 6.4 群删除入口（标题栏右侧）

**位置**：群聊页标题栏 `.titlebar-actions` 内，连接状态图标之后、使用引导按钮之前。

**规格**：复用 `IconButton`（`Button.tsx:94-122`），`size="md"` → `control-icon`：

| 属性 | 值 | 来源 |
| --- | --- | --- |
| 尺寸 | `36 × 36`，圆角 `10px` | `index.css:932-951` |
| 变体 | `ghost`（无底色） | `Button.tsx:99` |
| 图标 | `IconTrash size={20}`（已有，`icons.tsx:119`） | — |
| 默认色 | `rgb(var(--soren-ink-2))` | `.inbox-tool` 同族取值 |
| 悬停 | **不改红**。保持 `text-ink-2`，仅 `hover:bg-surface-3` | 见下 |
| 焦点 | `outline: 2px solid var(--accent); outline-offset: 2px` | `index.css:953-956` |
| `aria-label` | `删除群聊`（必填，`Button.tsx:97`） | — |
| `title` | `删除群聊` | `Button.tsx:100` |

**为什么悬停不变红**：这是常驻在标题栏的操作位，不是用户主动点开的危险确认。
变红会让人每次路过都在被警告。**危险语义由确认弹窗承担**——点击后复用既有 `ConfirmDialog`（`Modal.tsx:184-235`）：

```
variant="danger"  confirmText="删除群聊"  cancelText="取消"
danger  →  确认按钮 btn-danger；标题栏图标同时用 `bg-error/15 text-error` 的圆底告警块（ConfirmDialog.tsx:222-227）
message: 删除后群聊与其中的对话记录将无法恢复，成员的私有记忆会保留。
```

**文案**（功能性，`.text-sm leading-6 ink-2`，`Modal.tsx:231`）：
- 标题：`删除群聊`
- 正文：`删除后群聊与其中的对话记录将无法恢复，成员的私有记忆会保留。`

正文特意写明「私有记忆会保留」——对应 `PLAN §7.4`「删除群时 Bot 的私有记忆一律保留」，这是用户删除前唯一无法从界面推断的副作用。

**进行中禁用**：自主讨论进行中（状态条可见）时，`disabled`，理由见 §7.6。

---

## 7. 状态清单

### 7.1 加号下拉菜单

| 状态 | 规格 |
| --- | --- |
| 默认 | `bg: var(--card)`，`border: 1px line-subtle`，`color: ink-2`，36×36，r10 |
| 悬停 | `bg: var(--accent-soft)`，`color: var(--accent)`，`transition: background .15s` |
| 聚焦（`:focus-visible`） | `outline: 2px solid var(--accent); outline-offset: -2px`（`index.css:845`，侧栏内是负偏移） |
| 激活（`aria-expanded=true`） | 视觉**不变**，仅 `aria-expanded` 改变 |
| 禁用 | 侧栏加号当前无禁用态；若 Bot 列表加载失败需保留「创建 Bot」，则保持可用 |
| 加载 | 无（点击立即同步打开菜单，不发请求） |
| 错误 | 无（菜单本身不发请求） |
| 空 | 菜单恒为 2 项，无空态 |
| 超长文本 | 两项文案定长，无截断问题 |
| <760px | 侧栏为全屏抽屉（`index.css:582`，`width: min(290px,86vw)`）；菜单宽度取 `min(184px, 86vw - 24px)` |

### 7.2 CreateGroupDialog

| 状态 | 规格 |
| --- | --- |
| 默认 | `max-w-lg`(512px)，标题 `text-base font-semibold ink-1`（`Modal.tsx:140`） |
| 聚焦 | Modal 自动把焦点移入第一个可聚焦元素（`Modal.tsx:94-103`），即群名输入框 |
| 提交禁用 | 群名为空 或 成员 < 2 → `disabled` |
| 加载 | `Button loading` → `Spinner size={16}` + `aria-busy`（`Button.tsx:58,67`） |
| 错误（400） | 字段级：`TextField error` / 计数行 `text-error`；非字段级（500、网络）→ Toast |
| 空 | 群聊数为 0 时，加号菜单仍显示「创建群聊」（不是空态，是正常入口） |
| 成员 0 | 计数 `成员 0/5` + `至少选择 2 个成员` |
| 成员 5（满） | 未选格子全部 `disabled` + `aria-disabled` |
| 无可用 Bot | 网格区渲染 `text-sm text-ink-3`：`没有可用的 Bot`，创建按钮禁用 |
| 超长群名（>50 字） | 输入框内单行滚动；提交时截断拒绝并 `error`：`群名最多 50 字` |
| 超长 Bot 名 | 格子内 `text-xs truncate` + `title` 全名 |
| 超长群主题（>200 字） | 单行滚动，提交时拒绝并 `error` |
| <760px | Modal `p-6` 变窄；`.avatar-picker` 自动变 4 列（`index.css:749`）；`.bot-filter` 6 项可能换行——见 §7.5 |

### 7.3 侧栏群列表项

| 状态 | 规格 |
| --- | --- |
| 默认 | `.bot-inbox-row` 全套（§4.1） |
| 悬停 | `bg: rgb(var(--soren-surface-4)/.5)` |
| 聚焦（`:focus-visible`） | `outline: 2px solid var(--accent); outline-offset: -2px` |
| 选中 | `bg: rgb(var(--soren-ink-1)/.065)`（`:[aria-current=page]`） |
| 激活 | 复用选中态；**无按下态**（`.bot-inbox-row` 无 `:active` 规则，不新增） |
| 禁用 | 不适用（群聊不支持归档态，删除走单独入口） |
| 加载 | 副文本 `正在加载…` |
| 错误 | 群列表加载失败：复用 `.inbox-error` 样式（`index.css:823-824`，`11px ink-3` + `重试` 按钮） |
| 空（0 个群） | **整段不渲染**（无分割线、无「群聊 0」） |
| 成员 2 人 | 头像 2 + 第 3 格留空，槽位仍 62px |
| 成员 3 人 | 头像 3 |
| 成员 4–5 人 | 头像 2 + 第 3 格 `+2` / `+3` |
| 头像簇被压窄 | **不会发生**：槽位 `flex-shrink: 0`，文字由 `.inbox-copy` 的 `min-width: 0` + ellipsis 让位（`index.css:834,837`） |
| 成员头像加载失败 | `onError` → `/soren-mark.svg`，槽位尺寸不变 |
| 超长群名 | `truncate` + `title` |
| 超长预览（200 字） | `truncate` 单行 |
| 20 条消息的长群 | 预览已是最新一条，**不显示消息条数**——侧栏没有第二行可放，且 §4.4 要求与 Bot 项同构 |
| <760px | 抽屉模式（`min(290px,86vw)`），随列表滚动；`.bot-inbox-list` `flex:1; overflow-y:auto`（`index.css:825`） |

### 7.4 群聊页消息流

| 状态 | 规格 |
| --- | --- |
| 默认 | `.bubble-assistant`：底 `rgb(surface-1/.8)`、边 `1px line-subtle`、圆角 `4px 18px 18px 18px`、`px-4 py-3` |
| 用户消息 | `.bubble-user`：底 `var(--accent-soft)`、圆角 `18px 18px 4px 18px`、右对齐、`max-w-[76%]` |
| 发送中（`streaming`） | 光标 `2×16px` 圆角条 `bg-primary-start` + `animate-caret`（`MessageBubble.tsx:178`）。**保持不变**：非文本 UI 元素，门槛 3:1，实测 4.10 / 4.13 达标（见 §5.5） |
| 思考中（`pending` 且空内容） | `TypingDots` + `text-xs text-ink-3` |
| **群聊思考中文案** | **必须改为发言人名**：`{speakerName} 正在思考…`，不能沿用硬编码的「Soren 正在思考…」（`MessageBubble.tsx:169`） |
| 发送失败 | 用户消息 `opacity-70` + `text-xs text-error` 的 `IconAlert` +「发送失败」（`MessageBubble.tsx:92,99-103`） |
| 消息加载中 | `SkeletonList rows={3}`（`ChatPage.tsx:104`） |
| 历史加载失败 | 复用 `ChatPage` 既有错误块 + 重试 |
| 空 | §6.3 群聊空状态 |
| system | §5.3 pill |
| 记忆召回 | 既有 chip，最多 4 条 + `+N`（`MessageBubble.tsx:148-161`） |
| 超长群名 | 标题栏 `.titlebar-current` 已带 `truncate`（`index.css:847`），无需处理 |
| 超长 Bot 名（消息流） | 名称行 `truncate`，`max-width` 跟随内容列 |
| 20 条消息 | 消息列表 `flex flex-col gap-5`(20px)；滚动策略沿用 `ChatPage` 的「仅在用户位于底部时自动跟随」 |
| 发言 Bot 已被删除 | 头像 `avatarURL('soren')`，名称用 `message.speakerName` |
| <760px | `.chat-content` `padding: 5px 22px 15px`（`index.css:592`）；气泡 `max-w-[82%]` 不变 |

**发言人名缺失的兜底**：`speakerBotId` 与 `speakerName` 都为空 → 整行不渲染头像与名称，退化为无头像的 assistant 气泡。**不显示「未知 Bot」**。

### 7.5 输入区

| 状态 | 规格 |
| --- | --- |
| 默认 | `.composer`：底 `var(--card)`、边 `1px rgb(line-strong/.7)`、圆角 **20px**、`px-[18px] py-[17px] pt-[12px]` |
| 聚焦 | `border-color: #7da587` + `box-shadow: 0 7px 35px -12px rgba(51,103,63/.2), 0 0 0 3px #7da58712`（`index.css:553`） |
| 禁用 | 群聊不进入该态——群聊页固定传 `loading={false}`（Q5），讨论中由 §6.2 状态条承担 |
| 发送按钮禁用 | `opacity: .45`（`index.css:561`） |
| 发送按钮悬停 | `transform: translateY(-2px)`（`index.css:562`） |
| 空内容 | 发送按钮 `disabled`（`ChatInput.tsx:12` 的 `!value.trim()`） |
| 自适应高度 | `62px` → `170px` 自动（`ChatInput.tsx:8` 的内联逻辑） |
| 讨论中 | §6.2 状态条；`ChatInput` 传 `loading={false}` |
| @ 补全展开 | §6.1 |
| @ 补全无匹配 | 不显示浮层 |
| 错误 | 发送失败在消息气泡上表达，输入区不变 |
| 超长文本 | textarea 自动增高到 170px 后内部滚动 |
| <760px | `.composer-wrap` `px-[23px]`（`index.css:580`，761–1000px 档）；760px 及以下需确认——见缺口 G4 |

**窄屏整体断点现状**（实测，非推测）：

| 断点 | 现状 |
| --- | --- |
| ≥1400px | `.chat-content` / `.composer-wrap` `max-width: 950px`（`index.css:578`） |
| 761–1000px | 侧栏 216px；`.chat-content` / `.composer-wrap` `px-[23px]`（`index.css:580`） |
| ≤760px | 侧栏变全屏抽屉 `min(290px,86vw)`；`.chat-content` `padding: 5px 22px 15px`（`index.css:581-592`）；`.avatar-picker` 4 列（`749`） |

→ **群聊组件的 760px 行为全部由既有断点兜底**，无需新增媒体查询。760px 及以下 `.composer-wrap` 的水平内边距缺失，记为 §8 缺口 G4（属 CSS 补齐项，不阻塞群聊施工）。

### 7.6 群删除入口（标题栏）

| 状态 | 规格 |
| --- | --- |
| 默认 | `ghost` + `IconTrash size={20}`，`text-ink-2`，36×36 / r10（§6.4） |
| 悬停 | `hover:bg-surface-3`，**不变红**（§6.4） |
| 聚焦 | `outline: 2px solid var(--accent); outline-offset: 2px`（`index.css:953-956`） |
| 激活 | 无按下态（`IconButton` 既有行为，不新增） |
| 禁用 | 自主讨论进行中（§6.2 状态条可见）时 `disabled`；删除会触发 `PLAN §5.2` 的级联清理，与进行中的 `GroupTurn` 冲突 |
| 加载 | 删除请求中：`IconButton` 无 loading 态，用 `ConfirmDialog` 的 `loading` 覆盖（`Modal.tsx:213`），对话框关闭后按钮恢复 |
| 错误 | 删除失败留在 `ConfirmDialog` 内以正文错误行展示，不弹 Toast（操作上下文不能丢） |
| 空 / 超长 | 不适用 |

---

## 8. 现有设计体系缺口（如实记录，未假装存在）

| # | 缺口 | 影响 | 建议 |
| --- | --- | --- | --- |
| **G1** | **没有随主题切换的 Tailwind 强调色 token。** `primary.start` 是写死的 `#368773`（`tailwind.config.js:14`）；随主题变的只有 CSS 变量 `--accent`（`index.css:452-453`），它**没有对应的 Tailwind 类名** | §5.2 的 @提及、`text-primary-start` 在深色主题下全部失效（实测 2.69:1） | 临时用 `text-[rgb(var(--accent))]`（已验证可行，零配置改动）。根本修法：在 `tailwind.config.js` 加 `accent.DEFAULT: 'rgb(var(--soren-accent) / <alpha-value>)'` 并在 `index.css` 定义 `--soren-accent: 52 123 103 / 155 216 198`（light/dark）。**属产品代码改动，需 coder 执行。** |
| **G2** | 没有满足 AA 的「行内强调标记」token | 浅色主题下 `@提及` 用 `--accent` 是 4.23:1，差 0.27 未达 4.5 | 若需严格 AA：新增 `--soren-mention: 47 110 92 / 155 216 198`（`#2f6e5c` 浅色实测 5.04:1；`#9bd8c6` 深色实测 7.20:1）。**比 G1 的 `--accent` 更暗一档，仅用于行内标记。** |
| **G3** | `.composer-disabled` 类名在 `ChatInput.tsx:10` 被引用，但 `index.css` 中**搜不到该规则** | 单聊若传 `disabled`，禁用态可能无视觉；群聊不受影响 | **本功能不依赖它**：群聊固定传 `loading={false}`（Q5），不进入该态。该缺口留给单聊，是否补 `.composer-disabled{opacity:.5}` 由上游决定 |
| **G4** | `index.css:581-631` 的 `max-width:760px` 段**没有 `.composer-wrap` 的水平内边距规则**（761–1000px 段有 `px-[23px]`） | 窄屏下输入框贴边 | 补 `@media(max-width:760px){.composer-wrap{padding-left:16px;padding-right:16px}}`。**纯 CSS 改动，建议 coder 随群聊一起做。** |
| **G5** | 侧栏副文本 `ink-3` 在浅色主题下仅 3.09:1（实测） | Bot 项**现状**就不达 AA，非群聊引入 | 本规范让群列表项沿用 `ink-3` 以满足 §4.1「与 Bot 项同构」的硬要求。若要修，应全局把 `.inbox-copy small` 改为 `ink-2`（4.33:1，**仍差 4.5**），或再降一档。**不建议在群聊单点修，会破坏一致性。** |
| **G6** | `docs/design-system.md` 与实现**严重脱节**：文档写「主色调：渐变紫色」、`success: #51cf66`、`border-dark: #ced4da`；实现是 `--accent: #347b67`、`success: #368773`（`tailwind.config.js:62`） | 施工者若照文档取色会取到紫色，直接踩红线 | 本规范的所有颜色均以 `tailwind.config.js` + `index.css` 为准。**建议单独提一次文档修正**（不在本功能范围）。`tailwind.config.js:42` 的「兼容层」注释正是为这次历史漂移打的补丁。 |
| **G7** | `Button` 的 `sm`/`md`/`lg` 三档映射到同一个类（`Button.tsx:34-38`） | 想做「大号主按钮」的人会以为 `size="lg"` 有效，实际无效 | 本规范据此规定：群聊**不存在大号按钮**。若产品未来需要大尺寸，须先扩 `Button.tsx` 与 `--control-height`，是全局决策。 |

---

## 9. 决策记录与未决问题

### 9.1 已定决策（照此施工，不再讨论）

| # | 问题 | 结论 | 落地位置 |
| --- | --- | --- | --- |
| **Q1** | §6.2「与输入框同高同圆角」——指多行 `.composer`（62px / r20px）还是其内的 `.send-button`（36px / r10px）？ | **按 `.send-button` 解读**：36×36 / r10px / 标准 `control-icon`。按 `.composer` 字面解读会产出 62px 巨型按钮，即被否决过的「大号专属造型」 | §6.2 |
| **Q2** | 发言人名用 `text-ink-2`（4.54:1）而非 `text-ink-3`（3.24:1） | **`ink-2`**。发言人是群聊唯一身份线索 | §5.1 |
| **Q3** | @提及是否保留方案要求的「轻描边」 | **纯文字强调**：`color + font-weight: 600`，无背景无边框 | §5.2 |
| **Q4** | @提及色用 `--accent` 还是新增 token | **`rgb(var(--accent))`**，不新增 token | §5.2 |
| **Q5** | 群聊页 `ChatInput` 是否传 `loading` | **传 `loading={false}`**，避免与状态条出现两个「停止」 | §6.2 |
| **Q6** | 群聊数为 0 时是否显示空分组 | **整段隐藏**（含分割线与标题） | §4.2 |
| **Q7** | 群删除入口位置 | **群聊页标题栏右侧 `IconButton`**，规格见 §6.4 | §6.4 |
| **Q8** | 成员头像槽位几何 | **62px 恒 3 格**、头像 26px、重叠 8px、`+N` 占第 3 格。已渲染验证无碰撞 | §4.3 / §4.5 |
| **Q9** | 标题栏是否显示模式徽章 | **不显示** | §4.6 |

### 9.2 已于 2026-10-14 由产品负责人拍板（原 Q10 / Q11）

| # | 问题 | 决定 | 理由 |
| --- | --- | --- | --- |
| **Q10** | 群项左槽位 62px 比 Bot 项 38px 宽 24px，导致两类行的文字左边缘不对齐（§4.1） | **接受该差异** | 38px 内放 3 个可辨识头像需压到 18px 以下，实测认不出是哪个 Bot（§4.1）。把 Bot 项头像也缩到 26px 是改动既有单聊侧栏，代价远大于 24px 偏移。两者被分割线与「群聊 N」标题隔成两个独立列表，不会出现同列表内参差。**实现按 §4.1：群项直接复用 `bot-inbox-row` 类名，只替换内部槽位内容。** |
| **Q11** | §5.4 记忆 chip 换色会**同时改变单聊外观**（深色下 4.13:1 → 11.06:1） | **接受换色，单聊与群聊一起改** | 这是与 §5.2 @提及完全同一类的 WCAG AA 缺陷（4.10/4.13 低于小字 4.5:1 标准）。若只修 @提及 不修 chip，等于同一份规范里宽严不一。chip 是小字，可读性收益大于配色变化代价。**按 §5.4 实施，不加按场景分支的 prop。** |

至此本规范无待拍板项。§7 的状态清单与 §9.1 已定决策已全部落地，可直接施工。

---

## 附录 A：可直接复用的既有类名清单

| 类名 | 用途 | 位置 |
| --- | --- | --- |
| `inbox-tool` | 加号触发按钮 | `index.css:818-819` |
| `bot-inbox-row` | 侧栏行（Bot 与群共用） | `index.css:826-828` |
| `inbox-avatar` / `inbox-copy` | 侧栏头像槽 / 文本列 | `index.css:829-837` |
| `inbox-heading` | 分组标题 | `index.css:821-822` |
| `inbox-error` | 侧栏错误行 | `index.css:823-824` |
| `app-select-menu` / `app-select-option` | 浮层菜单 / 选项 | `index.css:792-798` |
| `avatar-picker` + `button[aria-pressed]` | Bot 选择网格 | `index.css:700-703` |
| `bot-filter` + `button[aria-pressed]` | 分段控件 | `index.css:681-683` |
| `field` | 输入框 | `index.css:211-226` |
| `bubble-assistant` / `bubble-user` | 气泡 | `index.css:244-253, 568-569` |
| `composer` / `composer-wrap` / `send-button` | 输入区 | `index.css:551-563` |
| `control-button` / `control-icon` | 按钮标准尺寸 | `index.css:932-951` |
| `btn-primary` / `btn-ghost` | 按钮变体 | `Button.tsx:27-33` |
| `chat-content` / `chat-scroll` | 消息区 | `index.css:519-520` |
| `welcome-scene` | 空状态容器 | `index.css:645` |
| `titlebar-actions` | 群删除入口的落位容器 | `index.css` 标题栏段 |

**本功能需要新增的唯一一处 CSS**（无既有类可复用，因其几何是群聊专属）：

```css
/* 群项头像簇：62px = 26 + 18 + 18，重叠 8px（PLAN §8.3） */
.group-avatars { display:flex; align-items:center; width:62px; flex-shrink:0; }
.group-avatars img,
.group-avatars > span {
  width:26px; height:26px; border-radius:7px;
  border:2px solid #f3f4f1;            /* 侧栏实际底色 */
  object-fit:contain; flex-shrink:0;
  background:rgb(var(--soren-surface-4));
}
.dark .group-avatars img,
.dark .group-avatars > span { border-color:#191e22; }
.group-avatars img + img,
.group-avatars > span { margin-left:-8px; }
.group-avatars > span { display:grid; place-items:center; font-size:10px; color:rgb(var(--soren-ink-1)); }
```

**注意**：描边色不能用 `var(--sidebar)`（`#f0f2eb` / `#111519`）——侧栏真实底色是写死的
`#f3f4f1` / `#191e22`（`index.css:813-814`），与 `--sidebar` 不同值，混用会出现可见色差环。

## 附录 B：组件 API 建议

```tsx
// MessageBubble.tsx —— 最小增量，不破坏既有调用
export interface MessageBubbleProps {
  bot?: Bot;                    // 单聊：当前 Bot（保持）
  bots?: Bot[];                 // 新增：群聊解析池
  message: ChatMessage;         // 已含 speakerBotId / speakerName
  // 其余不变
}
// 解析：const speaker = message.speakerBotId
//         ? bots?.find(b => b.id === message.speakerBotId)
//         : undefined;
// 渲染：(speaker ?? bot)
// 名称行：仅当 speaker 存在时渲染，text-xs text-ink-2（§5.1）
// 思考文案：'Soren 正在思考…' → `${speaker?.name ?? bot?.name} 正在思考…`（§7.4）
// 记忆 chip：primary-start → rgb(var(--accent))（§5.4，唯一影响单聊外观，Q11）
```

```tsx
// ChatEmptyState.tsx —— 变体，不新建文件
export function ChatEmptyState(props:
  | { variant?: 'bot'; bot?: Bot; onPickPrompt: (p: string) => void; disabled?: boolean }
  | { variant: 'group'; group: GroupChat; members: Bot[] }
) { /* ... */ }
```

```tsx
// ChatInput.tsx —— 新增可选能力，默认行为不变
export interface ChatInputProps {
  // 既有全部保持
  mentionCandidates?: Bot[];   // 传入则启用 @ 补全；不传 = 单聊行为不变
}
// 群聊页固定传 loading={false}，由 §6.2 状态条独占中止（Q5）
```

```tsx
// CreateGroupDialog.tsx —— 新建
export function CreateGroupDialog(props: {
  open: boolean;
  bots: Bot[];                  // 已过滤 archived
  onClose: () => void;
  onCreate: (input: { title: string; topic: string; botIds: string[];
                      mode: 'mention' | 'auto'; maxRounds: number }) => Promise<void>;
})
```