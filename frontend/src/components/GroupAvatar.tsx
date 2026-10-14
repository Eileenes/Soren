import { avatarURL } from '../services/agents';
import type { GroupMemberView } from '../types';

/** 与 Bot 头像共用 38px 槽位；按人数排布，最多展示六个成员。 */
export function GroupAvatar({ members }: { members: GroupMemberView[] }) {
  const visible = members.slice(0, 6);
  return <span className="inbox-avatar group-avatars" data-count={visible.length} aria-hidden="true">
    {visible.map(member => <img key={member.botId} src={avatarURL(member.avatar)} alt="" loading="lazy"
      onError={event => { event.currentTarget.onerror = null; event.currentTarget.src = '/soren-mark.svg'; }} />)}
  </span>;
}
