import { useEffect, useState } from "react";
import {
  agentsAPI,
  actionLabel,
  type Bot,
  type BotAction,
  type BotDefinition,
  type BotVersion,
} from "../services/agents";
import { describeError } from "../services/api";
import { Button, SelectField, Switch, TextAreaField, TextField } from "./ui";

const emptyDefinition = (): BotDefinition => ({
  canDo: [],
  cannotDo: [],
  workflow: "",
  output: "",
});
export function BotBoundaryFields({
  bot,
  onChange,
  disabled,
}: {
  bot: Bot;
  onChange: (patch: Partial<Bot>) => void;
  disabled: boolean;
}) {
  const definition = { ...emptyDefinition(), ...bot.definition };
  const update = (patch: Partial<BotDefinition>) =>
    onChange({ definition: { ...definition, ...patch } });
  return (
    <div className="space-y-4 border-t border-line pt-5">
      <h3 className="text-sm font-semibold">职责与边界</h3>
      {(["canDo", "cannotDo"] as const).map((key) => (
        <div key={key} className="space-y-2">
          <p className="text-sm text-ink-2">
            {key === "canDo" ? "负责事项" : "不处理的事项"}
          </p>
          {(definition[key] || []).map((row, i) => (
            <div key={i} className="flex items-end gap-2">
              <div className="min-w-0 flex-1">
                <TextField
                  aria-label={`${key === "canDo" ? "负责事项" : "不处理的事项"} ${i + 1}`}
                  value={row}
                  maxLength={500}
                  disabled={disabled}
                  onChange={(e) =>
                    update({
                      [key]: definition[key].map((item, j) =>
                        j === i ? e.target.value : item,
                      ),
                    })
                  }
                />
              </div>
              <Button
                variant="ghost"
                disabled={disabled}
                aria-label={`删除${key === "canDo" ? "负责事项" : "边界"} ${i + 1}`}
                onClick={() =>
                  update({ [key]: definition[key].filter((_, j) => j !== i) })
                }
              >
                移除
              </Button>
            </div>
          ))}
          <Button
            size="sm"
            variant="ghost"
            disabled={disabled || (definition[key]?.length || 0) >= 30}
            onClick={() => update({ [key]: [...(definition[key] || []), ""] })}
          >
            ＋ 添加{key === "canDo" ? "负责事项" : "边界"}
          </Button>
        </div>
      ))}
      <TextAreaField
        label="工作方式"
        value={definition.workflow}
        rows={2}
        maxLength={4000}
        disabled={disabled}
        onChange={(e) => update({ workflow: e.target.value })}
        placeholder="例如：先确认需求，给出方案，经确认后执行"
      />
      <TextAreaField
        label="输出标准"
        value={definition.output}
        rows={2}
        maxLength={4000}
        disabled={disabled}
        onChange={(e) => update({ output: e.target.value })}
        placeholder="例如：结论在前，注明来源，列出待确认事项"
      />
      <p className="text-xs leading-6 text-ink-3">
        文字规则指导回答；工具访问由下面的动作权限约束。移除文字规则不会自动放开权限。
      </p>
    </div>
  );
}

export function BotPermissionFields({
  bot,
  bots,
  onChange,
  disabled,
}: {
  bot: Bot;
  bots: Bot[];
  onChange: (patch: Partial<Bot>) => void;
  disabled: boolean;
}) {
  const [actions, setActions] = useState<BotAction[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    let alive = true;
    if (!bot.id || bot.status !== "active") return;
    setLoading(true);
    void agentsAPI
      .actions(bot.id)
      .then((rows) => {
        if (alive) {
          setActions(rows);
          setError("");
        }
      })
      .catch((e) => {
        if (alive) setError(describeError(e));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [bot.id, bot.status]);
  const keys = [
    ...new Set([
      ...actions.map((a) => a.key),
      ...Object.keys(bot.policy?.actions || {}),
    ]),
  ].sort();
  return (
    <div className="space-y-4 border-t border-line pt-5">
      <h3 className="text-sm font-semibold">工具与协作</h3>
      {!bot.policy ? (
        <div className="space-y-3">
          <p className="text-xs leading-6 text-ink-3">
            当前沿用应用连接中的授权。开始逐项管理后，新出现的动作默认禁用。
          </p>
          <Button
            size="sm"
            disabled={
              disabled ||
              loading ||
              Boolean(error) ||
              !bot.id ||
              bot.status !== "active"
            }
            onClick={() =>
              onChange({
                policy: {
                  actions: Object.fromEntries(
                    actions.map((a) => [a.key, "allow" as const]),
                  ),
                },
              })
            }
          >
            开始逐项管理工具
          </Button>
        </div>
      ) : (
        <>
          <p className="text-xs leading-6 text-ink-3">
            当前空间已启用的应用工具可供所有 Bot 使用。「允许」仍保留应用固定的审批要求；新增动作默认禁用。
          </p>
          {keys.map((key) => (
            <SelectField
              key={key}
              label={actionLabel(key)}
              helper={
                actions.find((a) => a.key === key)?.name ||
                "待连接：在当前空间连接应用后可用"
              }
              value={bot.policy?.actions[key] || "deny"}
              disabled={disabled}
              onChange={(value) =>
                onChange({
                  policy: { actions: { ...bot.policy?.actions, [key]: value } },
                })
              }
              options={[
                { value: "deny", label: "禁用" },
                { value: "confirm", label: "每次确认" },
                { value: "allow", label: "允许" },
              ]}
            />
          ))}
          {!keys.length && (
            <p className="text-sm text-ink-3">
              暂无可用工具。文字对话仍可使用。
            </p>
          )}
        </>
      )}
      {loading && <p className="text-xs text-ink-3">正在加载动作…</p>}
      {error && (
        <p role="alert" className="bot-error">
          {error}
        </p>
      )}
      <Switch
        label="允许与其他 Bot 协作"
        checked={bot.collaborationEnabled}
        disabled={disabled}
        onChange={(value) =>
          onChange({
            collaborationEnabled: value,
            collaborationAll: value ? bot.collaborationAll : false,
          })
        }
      />
      {bot.collaborationEnabled && (
        <>
          <Switch
            label="允许与所有 Bot 协作"
            checked={Boolean(bot.collaborationAll)}
            disabled={disabled}
            onChange={(value) => onChange({ collaborationAll: value })}
          />
          {!bot.collaborationAll && (
            <div className="space-y-2">
              {bots
                .filter((b) => b.id !== bot.id && b.status === "active")
                .map((other) => (
                  <label
                    key={other.id}
                    className="flex items-center gap-2 text-sm text-ink-2"
                  >
                    <input
                      type="checkbox"
                      disabled={disabled}
                      checked={bot.collaborators.includes(other.id)}
                      onChange={(e) =>
                        onChange({
                          collaborators: e.target.checked
                            ? [...bot.collaborators, other.id]
                            : bot.collaborators.filter((id) => id !== other.id),
                        })
                      }
                    />
                    {other.name}
                  </label>
                ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}

export function BotVersionHistory({
  bot,
  disabled,
  onSaved,
}: {
  bot: Bot;
  disabled: boolean;
  onSaved: (bot: Bot) => Promise<void>;
}) {
  const [rows, setRows] = useState<BotVersion[] | null>(null);
  const [selected, setSelected] = useState<BotVersion | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      setRows(await agentsAPI.versions(bot.id));
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-3 border-t border-line pt-5">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">版本记录 · v{bot.version}</h3>
        <Button
          variant="ghost"
          disabled={disabled}
          loading={busy}
          onClick={() => void load()}
        >
          查看版本
        </Button>
      </div>
      {rows && (
        <SelectField
          label="历史版本"
          value={selected ? String(selected.version) : ""}
          onChange={(value) =>
            setSelected(
              rows.find((row) => String(row.version) === value) || null,
            )
          }
          options={[
            { value: "", label: "选择版本查看职责" },
            ...rows.map((row) => ({
              value: String(row.version),
              label: `v${row.version} · ${new Date(row.createdAt).toLocaleString()}`,
            })),
          ]}
        />
      )}
      {selected && (
        <div className="space-y-3 rounded-xl bg-surface-2 p-4">
          <p className="text-sm font-medium">{selected.bot.name}</p>
          <p className="whitespace-pre-wrap break-words text-sm text-ink-2">
            {selected.bot.instructions}
          </p>
          <div className="text-xs text-ink-2">
            {selected.bot.definition?.canDo?.map((row, i) => (
              <p key={`yes${i}`}>负责：{row}</p>
            ))}
            {selected.bot.definition?.cannotDo?.map((row, i) => (
              <p key={`no${i}`}>边界：{row}</p>
            ))}
            <p className="whitespace-pre-wrap">
              {selected.bot.definition?.workflow}
            </p>
            <p className="whitespace-pre-wrap">
              {selected.bot.definition?.output}
            </p>
          </div>
          <p className="text-xs leading-6 text-ink-3">
            恢复名称、头像、职责与文字边界，并创建新版本。工具授权、协作和状态保持当前设置。上方显示当前职责，可与此版本对照。
          </p>
          <Button
            disabled={disabled || selected.version === bot.version}
            loading={busy}
            onClick={async () => {
              setBusy(true);
              setError("");
              try {
                await onSaved(
                  await agentsAPI.restore(
                    bot.id,
                    selected.version,
                    bot.version,
                  ),
                );
              } catch (e) {
                setError(describeError(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            确认恢复此版本
          </Button>
        </div>
      )}
      {error && (
        <p role="alert" className="bot-error">
          {error}
        </p>
      )}
    </div>
  );
}
