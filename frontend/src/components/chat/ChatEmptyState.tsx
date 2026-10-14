import { avatarURL, type Bot } from '../../services/agents';
import { IconBrain, IconSun, IconSparkles, IconMessageSquare, IconChevronRight } from '../icons';
import type { GroupView } from '../../types';
const examples = [
  { tag: '安排今天', Icon: IconSun, tone: 'peach', prompt: '帮我规划今天的安排。先问问我今天需要完成什么、可用时间和精力，再一起排出轻重缓急。' },
  { tag: '构思创意', Icon: IconSparkles, tone: 'sage', prompt: '我想做一个有个人风格的短视频。请先帮我梳理主题、受众和风格，再一起写分镜脚本与画面提示词。' },
  { tag: '起草邮件', Icon: IconMessageSquare, tone: 'lavender', prompt: '请帮我起草一封自然、得体的邮件。先问我收件人、目的和关键信息，完成后给我可以复制的草稿。' },
  { tag: '记录偏好', Icon: IconBrain, tone: 'sand', prompt: '我想让你记住我的一些偏好。请先问我希望你用什么称呼、喜欢什么沟通方式，以及最近在关注什么。' },
];

/** 成员头像与群名足够识别一个群，不必把整个 Bot 实体传进来 */
export interface MemberPreview { id: string; name: string; avatar: string; }

/**
 * 空状态
 *
 * variant='bot'：单聊，吉祥物 + 建议 prompt 卡片。
 * variant='group'：群聊。群聊没有单一主角 Bot，放谁的吉祥物都是替用户做决定，
 * 所以只给成员头像、群名、主题与两种模式的操作指引。
 */
export function ChatEmptyState({ onPickPrompt, disabled = false, bot, variant = 'bot', group, members = [] }: {
  onPickPrompt?: (prompt: string) => void;
  disabled?: boolean;
  bot?: Bot;
  variant?: 'bot' | 'group';
  group?: GroupView;
  members?: MemberPreview[];
}) {
  if (variant === 'group') {
    const modeHint = '直接提问，大家回复一轮；邀请讨论，最多三轮后等待你决定是否继续。';
    return <div className="welcome-scene flex flex-col items-center py-4 text-center">
      <div className="flex items-center gap-3">
        {members.slice(0, 5).map(item => <span key={item.id} className="overflow-hidden rounded-md ring-1 ring-line/70"><img src={avatarURL(item.avatar)} alt={item.name} width={36} height={36} /></span>)}
      </div>
      <h1 className="mt-4 max-w-[590px] truncate text-lg font-semibold text-ink-1">{group?.title}</h1>
      <p className="mt-1.5 max-w-[590px] text-sm leading-[1.7] text-ink-2">
        {group?.topic || '用 @ 指定 Bot，也可以直接向大家提问或邀请大家讨论。'}
      </p>
      <p className="mt-5 text-xs text-ink-3">{modeHint}</p>
    </div>;
  }
  if (bot && bot.id !== 'soren') {
    const needsResponsibilities = !bot.instructions.trim();
    return <div className="bot-introduction">
      <img src={avatarURL(bot.avatar)} alt="" width={64} height={64} />
      <div>
        <p className="text-sm font-medium text-ink-2">{bot.name}</p>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-ink-1">{needsResponsibilities ? '你好，我的职责是什么？' : '今天，有什么交给我？'}</h1>
        <p className="mt-3 max-w-[480px] whitespace-pre-wrap text-sm leading-7 text-ink-2">{needsResponsibilities ? '你希望我负责哪些事情？可以告诉我工作范围、做事方式，以及你期待的结果。' : bot.instructions}</p>
        {needsResponsibilities && <p className="mt-4 text-xs text-ink-3">在下方回复我，我会记住自己的职责。以后也可以右键打开配置来调整。</p>}
      </div>
    </div>;
  }
  return <div className="welcome-scene">
    <div className="welcome-intro"><div className="hero-mascot"><div className="mascot-halo" /><img src={avatarURL(bot?.avatar || 'soren')} alt="" width={112} height={112} /></div><h1><span>{bot && bot.id !== 'soren' ? bot.name : '今天，想做点什么？'}</span></h1></div>
    <div className="suggestion-grid">{examples.map(({ tag, Icon, tone, prompt }) => <button key={tag} className={'suggestion-card ' + tone} disabled={disabled} onClick={() => onPickPrompt?.(prompt)}><span className="suggestion-top"><span className="suggestion-icon"><Icon size={19} /></span><span>{tag}</span><IconChevronRight size={14} /></span></button>)}</div>
  </div>;
}