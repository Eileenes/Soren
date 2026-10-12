import { avatarURL, type Bot } from '../../services/agents';
import { IconBrain, IconSun, IconSparkles, IconMessageSquare, IconChevronRight } from '../icons';
const examples = [
  { tag: '安排今天', Icon: IconSun, tone: 'peach', prompt: '帮我规划今天的安排。先问问我今天需要完成什么、可用时间和精力，再一起排出轻重缓急。' },
  { tag: '构思创意', Icon: IconSparkles, tone: 'sage', prompt: '我想做一个有个人风格的短视频。请先帮我梳理主题、受众和风格，再一起写分镜脚本与画面提示词。' },
  { tag: '起草邮件', Icon: IconMessageSquare, tone: 'lavender', prompt: '请帮我起草一封自然、得体的邮件。先问我收件人、目的和关键信息，完成后给我可以复制的草稿。' },
  { tag: '记录偏好', Icon: IconBrain, tone: 'sand', prompt: '我想让你记住我的一些偏好。请先问我希望你用什么称呼、喜欢什么沟通方式，以及最近在关注什么。' },
];
export function ChatEmptyState({ onPickPrompt, disabled = false, bot }: { bot?: Bot; onPickPrompt: (prompt: string) => void; disabled?: boolean }) {
  return <div className="welcome-scene">
    <div className="welcome-intro"><div className="hero-mascot"><div className="mascot-halo" /><img src={avatarURL(bot?.avatar || 'soren')} alt="" width={112} height={112} /></div><h1><span>{bot && bot.id !== 'soren' ? bot.name : '今天，想做点什么？'}</span></h1></div>
    <div className="suggestion-grid">{examples.map(({ tag, Icon, tone, prompt }) => <button key={tag} className={'suggestion-card ' + tone} disabled={disabled} onClick={() => onPickPrompt(prompt)}><span className="suggestion-top"><span className="suggestion-icon"><Icon size={19} /></span><span>{tag}</span><IconChevronRight size={14} /></span></button>)}</div>
  </div>;
}
