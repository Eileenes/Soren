import { useState } from 'react';
import { PageContainer, PageHeading } from '../components/layout/Shell';
import { Button } from '../components/ui';
import { IconSearch, IconChevronRight } from '../components/icons';
import { avatarURL, type Bot } from '../services/agents';
import type { BotInbox } from '../hooks/useBotInbox';

export function ConversationHistoryPage({ bots, inbox, onSelect, onReload }: {
  bots: Bot[]; inbox: Record<string, BotInbox>; onSelect: (botId: string, conversationId?: string) => void; onReload: () => Promise<void>;
}) {
  const [search, setSearch] = useState('');
  const rows = bots.filter(bot => bot.status !== 'archived' && `${bot.name} ${inbox[bot.id]?.preview ?? ''}`.toLocaleLowerCase().includes(search.trim().toLocaleLowerCase()));
  return <PageContainer>
    <PageHeading title="对话记录" description="每个 Bot，一个对话入口。继续上次聊天，也可以展开查看以往记录。" />
    <label className="conversation-search"><IconSearch size={17} /><input aria-label="搜索 Bot 记录" placeholder="搜索 Bot" value={search} onChange={e => setSearch(e.target.value)} /></label>
    <div className="bot-history-list">{rows.map(bot => {
      const entry = inbox[bot.id];
      return <article className="bot-history" key={bot.id}>
        <button className="bot-history-open" onClick={() => onSelect(bot.id, entry?.latestId ?? undefined)}><img src={avatarURL(bot.avatar)} alt="" /><span><strong>{bot.name}</strong><small>{entry?.failed ? '暂时无法加载对话' : !entry ? '正在加载…' : entry.preview || '还没有对话'}</small></span><IconChevronRight size={17} /></button>
        {entry?.failed && <Button variant="ghost" size="sm" onClick={() => void onReload()}>重试</Button>}
        {Boolean(entry?.conversations.length) && <details><summary>查看聊天记录</summary><div className="bot-history-sessions">{entry.conversations.map(c => <button key={c.id} onClick={() => onSelect(bot.id, c.id)}><span>{c.title || '未命名对话'}</span><IconChevronRight size={14} /></button>)}</div></details>}
      </article>;
    })}</div>
    {!rows.length && <p className="history-page-empty">没有找到相关 Bot。</p>}
  </PageContainer>;
}
