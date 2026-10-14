import {
  BotBoundaryFields,
  BotPermissionFields,
  BotVersionHistory,
} from "./BotConfigurationFields";
import { TemplatePublisher } from "./BotTemplates";
import { useState } from "react";
import {
  Button,
  Modal,
  TextField,
  TextAreaField,
  Switch,
  SelectField,
} from "./ui";
import { IconSparkles } from "./icons";
import { AVATARS, avatarURL, agentsAPI, type Bot } from "../services/agents";
import { describeError } from "../services/api";
export const blankBot = (): Bot => ({
  id: "",
  name: "",
  avatar: "iceberg",
  instructions: "",
  model: "",
  memoryEnabled: true,
  collaborationEnabled: true,
  collaborationAll: true,
  collaborators: [],
  status: "active",
  version: 1,
});
export function BotEditor({
  initial,
  bots,
  onClose,
  onSaved,
}: {
  initial: Bot;
  bots: Bot[];
  onClose: () => void;
  onSaved: (bot: Bot) => Promise<void>;
}) {
  const [sharing, setSharing] = useState(false);
  const [draft, setDraft] = useState<Bot>(initial);
  const [saving, setSaving] = useState(false);
  const [localError, setError] = useState("");
  const [generating, setGenerating] = useState(false);
  const [draftNotice, setDraftNotice] = useState("");
  const update = (patch: Partial<Bot>) =>
    setDraft((value) => ({ ...value, ...patch }));
  const save = async () => {
    if (!draft) return;
    setSaving(true);
    setError("");
    try {
      const saved = await agentsAPI.save(draft, !draft.id);
      await onSaved(saved);
    } catch (e) {
      setError(describeError(e));
    } finally {
      setSaving(false);
    }
  };
  const changed = JSON.stringify(initial) !== JSON.stringify(draft);
  if (sharing)
    return (
      <TemplatePublisher botId={initial.id} onClose={() => setSharing(false)} />
    );
  return (
    <Modal
      open
      onClose={() => {
        if (!saving && !generating) onClose();
      }}
      title={draft?.id ? "配置 Bot" : "新建 Bot"}
      size="lg"
      footer={
        <>
          <Button
            variant="ghost"
            disabled={saving || generating}
            onClick={() => onClose()}
          >
            取消
          </Button>
          <Button
            disabled={generating}
            loading={saving}
            onClick={() => void save()}
          >
            {draft.id ? "保存 Bot" : "创建 Bot"}
          </Button>
        </>
      }
    >
      {draft && (
        <div className="space-y-5">
          <div className="avatar-picker" role="group" aria-label="选择头像">
            {AVATARS.map((id) => (
              <button
                key={id}
                aria-label={`头像 ${id}`}
                aria-pressed={draft.avatar === id}
                onClick={() => update({ avatar: id })}
              >
                <img src={avatarURL(id)} alt="" width={48} height={48} />
              </button>
            ))}
          </div>
          <TextField
            label="名称"
            value={draft.name}
            maxLength={40}
            placeholder="给它起个名字"
            onChange={(e) => update({ name: e.target.value })}
          />
          {draft.id || draft.instructions ? (
            <>
              <TextAreaField
                label="职责"
                value={draft.instructions}
                rows={5}
                maxLength={8000}
                placeholder="负责什么、怎么做、交付什么结果…"
                onChange={(e) => update({ instructions: e.target.value })}
              />
              <div className="flex items-center gap-3">
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={!draft.instructions.trim() || saving}
                  loading={generating}
                  onClick={async () => {
                    setGenerating(true);
                    setError("");
                    try {
                      const result = await agentsAPI.draft(
                        draft.name,
                        draft.instructions,
                      );
                      update(result);
                      setDraftNotice("草稿已生成，保存后生效。");
                    } catch (e) {
                      setError(describeError(e));
                    } finally {
                      setGenerating(false);
                    }
                  }}
                >
                  完善职责
                </Button>
                <span className="text-xs text-ink-3">{draftNotice}</span>
              </div>
              {!draft.instructions && (
                <div className="bot-templates">
                  {[
                    {
                      name: "工作助手",
                      duty: "整理工作资料、规划日程、起草邮件。先明确目标与限制，交付简洁可执行的方案。未连接工具时只提供草稿，不声称已发送或已创建日程。",
                    },
                    {
                      name: "创作助手",
                      duty: "负责选题、文案、视频分镜与图像提示词。关注受众、风格与表达，交付可直接使用的创作稿。未连接生成工具时不声称已生成图片或视频。",
                    },
                    {
                      name: "生活助手",
                      duty: "帮助规划日常、整理待办与记录偏好。语气自然，建议具体；不将推测写成用户事实。",
                    },
                  ].map((t) => (
                    <button
                      key={t.name}
                      onClick={() =>
                        update({
                          name: draft.name || t.name,
                          instructions: t.duty,
                        })
                      }
                    >
                      <IconSparkles size={13} />
                      {t.name}
                    </button>
                  ))}
                </div>
              )}
            </>
          ) : (
            <p className="text-sm leading-6 text-ink-2">
              创建后，Bot 会先和你确认自己的职责。
            </p>
          )}
          <BotBoundaryFields
            bot={draft}
            onChange={update}
            disabled={saving || generating}
          />
          <BotPermissionFields
            bot={draft}
            bots={bots}
            onChange={update}
            disabled={saving || generating}
          />
          <div className="bot-option">
            <div>
              <strong>独立记忆</strong>
              <p>仅此 Bot 可召回；关闭后保留已有记录。</p>
            </div>
            <Switch
              label="启用独立记忆"
              checked={draft.memoryEnabled}
              onChange={(memoryEnabled) => update({ memoryEnabled })}
            />
          </div>
          {draft.id && draft.id !== "soren" && (
            <SelectField
              label="状态"
              aria-label="Bot 状态"
              value={draft.status}
              onChange={(status) => update({ status })}
              options={[
                { value: "active", label: "可用" },
                { value: "paused", label: "暂停" },
                { value: "archived", label: "归档" },
              ]}
            />
          )}
          {draft.id && (
            <>
              <BotVersionHistory
                bot={initial}
                disabled={saving || generating || changed}
                onSaved={onSaved}
              />
              <div className="space-y-3 border-t border-line pt-5">
                <Button
                  disabled={
                    saving ||
                    generating ||
                    changed ||
                    !initial.instructions.trim()
                  }
                  onClick={() => setSharing(true)}
                >
                  创建 Bot 模板
                </Button>
                <p className="text-xs text-ink-3">
                  {changed
                    ? "先保存修改，再创建模板或恢复历史版本。"
                    : "分享当前已保存的配置，让别人导入自己的 Soren。"}
                </p>
                {initial.templateSource && (
                  <p className="break-all text-xs text-ink-3">
                    导入来源：{initial.templateSource.service} · v
                    {initial.templateSource.revision}
                  </p>
                )}
              </div>
            </>
          )}
          {localError && (
            <p role="alert" className="bot-error">
              {localError}
            </p>
          )}
        </div>
      )}
    </Modal>
  );
}
