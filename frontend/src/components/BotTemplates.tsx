import { useCallback, useEffect, useRef, useState } from "react";
import {
  Button,
  IconButton,
  Modal,
  SelectField,
  Spinner,
  TextAreaField,
  TextField,
} from "./ui";
import { IconClose } from "./icons";
import { TemplateCard } from "./TemplateCard";
import { actionLabel, avatarURL, type Bot } from "../services/agents";
import { describeError } from "../services/api";
import {
  templatesAPI,
  type PublishedTemplate,
  type TemplateConfig,
  type TemplatePreview,
} from "../services/templates";

export function TemplateContents({ config }: { config: TemplateConfig }) {
  const definition = config.definition;
  return (
    <div className="space-y-4 rounded-xl border border-line bg-surface-2 p-5 text-sm">
      <div className="flex items-center gap-3">
        <img src={avatarURL(config.avatar)} alt="" width={40} height={40} />
        <div>
          <p className="font-semibold text-ink-1">{config.name}</p>
          <p className="text-xs text-ink-3">
            {config.memoryEnabled ? "独立记忆" : "记忆关闭"} · 独立副本
          </p>
        </div>
      </div>
      {config.description && (
        <p className="whitespace-pre-wrap break-words text-ink-2">
          {config.description}
        </p>
      )}
      <div>
        <h3 className="mb-2 font-medium">职责</h3>
        <p className="whitespace-pre-wrap break-words leading-6 text-ink-2">
          {config.instructions}
        </p>
      </div>
      {(
        [
          ["负责事项", definition.canDo],
          ["不处理的事项", definition.cannotDo],
        ] as const
      ).map(([title, rows]) =>
        rows?.length ? (
          <div key={title}>
            <h3 className="mb-2 font-medium">{title}</h3>
            <ul className="list-disc space-y-1 pl-5 text-ink-2">
              {rows.map((row, i) => (
                <li className="break-words" key={i}>
                  {row}
                </li>
              ))}
            </ul>
          </div>
        ) : null,
      )}
      {(
        [
          ["工作方式", definition.workflow],
          ["输出标准", definition.output],
        ] as const
      ).map(([title, value]) =>
        value ? (
          <div key={title}>
            <h3 className="mb-2 font-medium">{title}</h3>
            <p className="whitespace-pre-wrap break-words text-ink-2">
              {value}
            </p>
          </div>
        ) : null,
      )}
      <div>
        <h3 className="mb-2 font-medium">所需能力</h3>
        <p className="break-words text-ink-2">
          {config.requiredCapabilities?.length
            ? config.requiredCapabilities.map(actionLabel).join("、")
            : "文字对话"}
          。导入后需自行连接并授权工具。
        </p>
      </div>
      {config.policyTemplate &&
        Object.keys(config.policyTemplate.actions).length > 0 && (
          <details>
            <summary className="cursor-pointer text-ink-2">
              查看模板动作规则
            </summary>
            <ul className="mt-2 space-y-1 text-xs text-ink-3">
              {Object.entries(config.policyTemplate.actions).map(
                ([key, value]) => (
                  <li key={key}>
                    {actionLabel(key)}：
                    {value === "deny"
                      ? "禁用"
                      : value === "confirm"
                        ? "每次确认"
                        : "允许（仍受应用审批约束）"}
                  </li>
                ),
              )}
            </ul>
          </details>
        )}
    </div>
  );
}

export function TemplatePublisher({
  botId,
  templateId,
  onClose,
}: {
  botId: string;
  templateId?: string;
  onClose: () => void;
}) {
  const [config, setConfig] = useState<TemplateConfig | null>(null);
  const [published, setPublished] = useState<PublishedTemplate | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const requestId = useRef(crypto.randomUUID());
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      setConfig(await templatesAPI.export(botId));
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    void load();
  }, [botId]);
  const update = (patch: Partial<TemplateConfig>) => {
    setConfig((value) => (value ? { ...value, ...patch } : value));
    requestId.current = crypto.randomUUID();
  };
  const publish = async () => {
    if (!config) return;
    setBusy(true);
    setError("");
    try {
      setPublished(
        await templatesAPI.publish(config, requestId.current, templateId),
      );
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  if (published)
    return (
      <Modal open compact size="sm" ariaLabel="模板已发布" onClose={onClose}>
        <TemplateCard
          name={published.config.name}
          avatar={published.config.avatar}
          url={published.url}
          corner={
            <IconButton label="关闭" onClick={onClose}>
              <IconClose size={16} />
            </IconButton>
          }
        />
      </Modal>
    );
  return (
    <Modal
      open
      onClose={() => {
        if (!busy) onClose();
      }}
      title={templateId ? "发布模板新版" : "创建 Bot 模板"}
      size="lg"
      footer={
        <>
          <Button disabled={busy} variant="ghost" onClick={onClose}>
            取消
          </Button>
          <Button
            variant="primary"
            disabled={
              !config || !config.name.trim() || !config.instructions.trim()
            }
            loading={busy}
            onClick={() => void publish()}
          >
            确认发布模板
          </Button>
        </>
      }
    >
      <div className="space-y-5">
        {config ? (
          <>
            <TextField
              label="模板名称"
              value={config.name}
              maxLength={40}
              disabled={busy}
              onChange={(e) => update({ name: e.target.value })}
            />
            <TextAreaField
              label="模板简介"
              value={config.description}
              maxLength={1000}
              rows={2}
              disabled={busy}
              onChange={(e) => update({ description: e.target.value })}
              placeholder="告诉对方，这个 Bot 能帮上什么忙"
            />
            <p className="text-xs leading-6 text-ink-3">
              下方是将要公开的完整配置。请确认职责中没有手动填写的密钥或内部资料。系统凭据、私人记忆、对话和连接授权不会打包。
            </p>
            <TemplateContents config={config} />
          </>
        ) : (
          <p className="text-sm text-ink-3">
            {busy ? "正在读取 Bot 配置…" : "无法读取配置。"}{" "}
            {!busy && (
              <button className="underline" onClick={() => void load()}>
                重试
              </button>
            )}
          </p>
        )}
        {error && (
          <p role="alert" className="bot-error">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}

export function MyTemplates({
  bots,
  onClose,
}: {
  bots: Bot[];
  onClose: () => void;
}) {
  const [rows, setRows] = useState<PublishedTemplate[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [revision, setRevision] = useState<PublishedTemplate | null>(null);
  const [source, setSource] = useState("");
  const [publishing, setPublishing] = useState(false);
  const [deleting, setDeleting] = useState<PublishedTemplate | null>(null);
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      setRows(await templatesAPI.list());
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  if (publishing && revision && source)
    return (
      <TemplatePublisher
        botId={source}
        templateId={revision.id}
        onClose={() => {
          setPublishing(false);
          setRevision(null);
          void load();
        }}
      />
    );
  const change = async (row: PublishedTemplate, remove = false) => {
    setBusy(true);
    setError("");
    try {
      if (remove) await templatesAPI.remove(row.id);
      else await templatesAPI.disable(row.id, !row.disabled);
      setDeleting(null);
      await load();
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      open
      title="我的模板"
      size="lg"
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <Button disabled={busy} onClick={onClose}>
          关闭
        </Button>
      }
    >
      <div className="space-y-4">
        <div className="template-card-grid">
          {rows.map((row) => (
            <TemplateCard
              key={row.id}
              name={row.config.name}
              avatar={row.config.avatar}
              url={row.url}
              corner={
                row.disabled ? (
                  <span className="template-status">已停用</span>
                ) : undefined
              }
            >
              <details className="template-management">
                <summary>管理模板</summary>
                <p className="mt-3 mb-3 text-xs text-ink-3">
                  版本 {row.revision} ·{" "}
                  {row.disabled ? "链接已停用" : "链接可用"}
                </p>
                <div className="flex flex-wrap gap-2">
                  <Button
                    size="sm"
                    disabled={busy || row.disabled}
                    onClick={() => {
                      setRevision(row);
                      setSource("");
                      setDeleting(null);
                    }}
                  >
                    发布新版
                  </Button>
                  <Button
                    size="sm"
                    disabled={busy}
                    onClick={() => void change(row)}
                  >
                    {row.disabled ? "启用链接" : "停用链接"}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    disabled={busy}
                    onClick={() => {
                      setDeleting(row);
                      setRevision(null);
                    }}
                  >
                    删除模板
                  </Button>
                </div>
                {deleting?.id === row.id && (
                  <div className="space-y-3 border-t border-line pt-3">
                    <p className="text-sm text-ink-2">
                      永久删除此模板的所有发布版本？此操作无法恢复。
                    </p>
                    <Button
                      variant="danger"
                      loading={busy}
                      onClick={() => void change(row, true)}
                    >
                      确认删除模板
                    </Button>
                    <Button
                      variant="ghost"
                      disabled={busy}
                      onClick={() => setDeleting(null)}
                    >
                      取消
                    </Button>
                  </div>
                )}
                {revision?.id === row.id && (
                  <div className="space-y-3 border-t border-line pt-3">
                    <SelectField
                      label="新版使用哪个 Bot 的配置"
                      value={source}
                      onChange={setSource}
                      options={[
                        { value: "", label: "请选择来源 Bot" },
                        ...bots.map((b) => ({ value: b.id, label: b.name })),
                      ]}
                    />
                    <Button
                      disabled={!source}
                      onClick={() => setPublishing(true)}
                    >
                      预览新版
                    </Button>
                  </div>
                )}
              </details>
            </TemplateCard>
          ))}
        </div>
        {!rows.length && !busy && !error && (
          <p className="py-8 text-center text-sm text-ink-3">
            还没有发布模板。从任意 Bot 的「创建模板」开始。
          </p>
        )}
        {busy && (
          <p role="status" className="text-sm text-ink-3">
            正在加载…
          </p>
        )}
        {error && (
          <p role="alert" className="bot-error">
            {error}{" "}
            <button className="underline" onClick={() => void load()}>
              重试
            </button>
          </p>
        )}
      </div>
    </Modal>
  );
}

function ImportDialog({
  link: initialLink,
  onClose,
  onImported,
}: {
  link: string;
  onClose: () => void;
  onImported: (bot: Bot) => void;
}) {
  const [link, setLink] = useState(initialLink);
  const [preview, setPreview] = useState<TemplatePreview | null>(null);
  const [busy, setBusy] = useState(Boolean(initialLink));
  const [error, setError] = useState("");
  const generation = useRef(0);
  const importing = useRef(false);
  const load = useCallback(async (value: string) => {
    const current = ++generation.current;
    setBusy(true);
    setError("");
    setPreview(null);
    try {
      const result = await templatesAPI.preview(value);
      if (current === generation.current) setPreview(result);
    } catch (e) {
      if (current === generation.current) setError(describeError(e));
    } finally {
      if (current === generation.current) setBusy(false);
    }
  }, []);
  // Opening a link loads its card. Only the import button creates a local Bot.
  useEffect(() => {
    if (initialLink) void load(initialLink);
    return () => {
      generation.current++;
    };
  }, [initialLink, load]);
  const close = () => {
    if (!importing.current) onClose();
  };
  const accept = async () => {
    if (!preview || importing.current) return;
    importing.current = true;
    setBusy(true);
    setError("");
    try {
      const bot = await templatesAPI.import(
        preview.previewId,
        preview.published.config.name,
      );
      onImported(bot);
      onClose();
    } catch (e) {
      setError(describeError(e));
    } finally {
      importing.current = false;
      setBusy(false);
    }
  };
  return (
    <Modal open compact size="sm" ariaLabel="导入 Bot 模板" onClose={close}>
      {preview ? (
        <TemplateCard
          name={preview.published.config.name}
          avatar={preview.published.config.avatar}
          url={preview.published.url}
          corner={
            <IconButton label="关闭" disabled={busy} onClick={close}>
              <IconClose size={16} />
            </IconButton>
          }
        >
          {preview.existingBotId && (
            <p role="status" className="mb-4 text-xs leading-5 text-ink-3">
              已导入过这个模板，再次导入将创建独立副本。
            </p>
          )}
          <Button
            variant="primary"
            block
            loading={busy}
            onClick={() => void accept()}
          >
            {preview.existingBotId ? "再导入一份" : "导入到我的 Soren"}
          </Button>
          {error && (
            <p role="alert" className="bot-error mt-3">
              {error}
            </p>
          )}
        </TemplateCard>
      ) : (
        <div className="template-import-entry">
          <div className="template-import-heading">
            <h2>导入 Bot 模板</h2>
            <IconButton label="关闭" onClick={close}>
              <IconClose size={16} />
            </IconButton>
          </div>
          {busy ? (
            <div role="status" className="template-card-loading">
              <Spinner size={24} />
              <p>正在打开 Bot 卡片…</p>
            </div>
          ) : (
            <form
              onSubmit={(event) => {
                event.preventDefault();
                if (link.trim()) void load(link.trim());
              }}
              className="space-y-4"
            >
              <TextField
                label="模板链接"
                value={link}
                onChange={(event) => {
                  setLink(event.target.value);
                  setError("");
                }}
                placeholder="粘贴朋友分享的模板链接"
              />
              {error && (
                <p role="alert" className="bot-error">
                  {error}
                </p>
              )}
              <Button type="submit" block disabled={!link.trim()}>
                打开模板卡片
              </Button>
            </form>
          )}
        </div>
      )}
    </Modal>
  );
}

export function TemplateBotPicker({
  bots,
  onSelect,
  onClose,
}: {
  bots: Bot[];
  onSelect: (bot: Bot) => void;
  onClose: () => void;
}) {
  const available = bots.filter((bot) => bot.status !== "archived");
  return (
    <Modal
      open
      title="创建 Bot 模板"
      description="选择一个 Bot，将它的配置分享给朋友。"
      onClose={onClose}
    >
      <div className="template-bot-picker">
        {available.map((bot) => (
          <button
            key={bot.id}
            disabled={!bot.instructions.trim()}
            onClick={() => onSelect(bot)}
          >
            <img src={avatarURL(bot.avatar)} alt="" width={48} height={48} />
            <span>
              <strong>{bot.name}</strong>
              <small>
                {bot.instructions.trim() ? "创建模板" : "请先在配置中填写职责"}
              </small>
            </span>
            <span aria-hidden="true">↗</span>
          </button>
        ))}
        {!available.length && (
          <p className="py-8 text-sm text-ink-3">
            先新建一个 Bot，再创建分享模板。
          </p>
        )}
      </div>
    </Modal>
  );
}

export function TemplateImportHost({
  onImported,
}: {
  onImported: (bot: Bot) => void;
}) {
  const [pending, setPending] = useState<{ link: string; key: number } | null>(
    null,
  );
  const queue = useRef<string[]>([]);
  const active = useRef<string | null>(null);
  const sequence = useRef(0);
  useEffect(() => {
    const receive = (link: string) => {
      if (link.length > 2048) return;
      if (active.current !== null) {
        if (
          active.current !== link &&
          !queue.current.includes(link) &&
          queue.current.length < 10
        )
          queue.current.push(link);
        return;
      }
      active.current = link;
      setPending({ link, key: ++sequence.current });
    };
    const custom = (event: Event) =>
      receive(
        typeof (event as CustomEvent).detail === "string"
          ? (event as CustomEvent<string>).detail
          : "",
      );
    window.addEventListener("soren:import-template", custom);
    let dispose: (() => void) | undefined;
    let stopped = false;
    if ("__TAURI_INTERNALS__" in window) {
      void Promise.all([
        import("@tauri-apps/api/event"),
        import("@tauri-apps/api/core"),
      ])
        .then(async ([{ listen }, { invoke }]) => {
          const drain = async () => {
            const links = await invoke<string[]>("take_template_links");
            if (!stopped) links.forEach(receive);
          };
          const unlisten = await listen("soren-template-link", () => {
            void drain().catch(() => {});
          });
          if (stopped) {
            unlisten();
            return;
          }
          dispose = unlisten;
          await drain();
        })
        .catch(() => {
          /* Manual paste remains available when native events cannot be read. */
        });
    }
    return () => {
      stopped = true;
      dispose?.();
      window.removeEventListener("soren:import-template", custom);
    };
  }, []);
  if (!pending) return null;
  return (
    <ImportDialog
      key={pending.key}
      link={pending.link}
      onImported={onImported}
      onClose={() => {
        const next = queue.current.shift();
        active.current = next ?? null;
        setPending(
          next === undefined ? null : { link: next, key: ++sequence.current },
        );
      }}
    />
  );
}
