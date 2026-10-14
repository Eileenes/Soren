import { useEffect, useRef, useState } from "react";
import { Button, Modal, SelectField, TextAreaField, TextField } from "./ui";
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
  const [copied, setCopied] = useState(false);
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
  return (
    <Modal
      open
      onClose={() => {
        if (!busy) onClose();
      }}
      title={
        published ? "模板已发布" : templateId ? "发布模板新版" : "创建 Bot 模板"
      }
      size="lg"
      footer={
        published ? (
          <Button onClick={onClose}>完成</Button>
        ) : (
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
        )
      }
    >
      <div className="space-y-5">
        {published ? (
          <>
            <p className="text-sm leading-6 text-ink-2">
              把链接分享给朋友，对方可以导入自己的 Soren。此链接固定为版本{" "}
              {published.revision}，不会随原 Bot 的编辑改变。
            </p>
            <TextField label="模板链接" readOnly value={published.url} />
            <Button
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(published.url);
                  setCopied(true);
                } catch {
                  setError("复制失败，请选中上方链接手动复制。");
                }
              }}
            >
              {copied ? "已复制链接" : "复制模板链接"}
            </Button>
          </>
        ) : config ? (
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
  const [notice, setNotice] = useState("");
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
        <p className="text-sm leading-6 text-ink-2">
          管理你在当前模板服务发布的内容。停用或删除会阻止后续下载，已经导入的副本仍然保留。
        </p>
        {rows.map((row) => (
          <article
            key={row.id}
            className="space-y-3 rounded-xl border border-line p-4"
          >
            <div className="flex items-center justify-between gap-3">
              <h3 className="font-medium">{row.config.name}</h3>
              <span className="text-xs text-ink-3">
                v{row.revision} · {row.disabled ? "已停用" : "分享中"}
              </span>
            </div>
            <p className="break-all text-xs text-ink-3">{row.url}</p>
            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                disabled={busy || row.disabled}
                onClick={async () => {
                  try {
                    await navigator.clipboard.writeText(row.url);
                    setNotice("模板链接已复制");
                  } catch {
                    setError("复制失败，请手动复制上方链接。");
                  }
                }}
              >
                复制链接
              </Button>
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
                <Button disabled={!source} onClick={() => setPublishing(true)}>
                  预览新版
                </Button>
              </div>
            )}
          </article>
        ))}
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
        {notice && (
          <p role="status" className="text-sm text-ink-2">
            {notice}
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
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const load = async () => {
    setBusy(true);
    setError("");
    setPreview(null);
    try {
      const result = await templatesAPI.preview(link);
      setPreview(result);
      setName(result.published.config.name);
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  // Receiving a link opens this dialog only. Fetching and importing remain explicit.
  const accept = async () => {
    if (!preview) return;
    setBusy(true);
    setError("");
    try {
      const bot = await templatesAPI.import(preview.previewId, name);
      onImported(bot);
      onClose();
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      open
      title="导入 Bot 模板"
      size="lg"
      onClose={() => {
        if (!busy) onClose();
      }}
      footer={
        <>
          <Button disabled={busy} variant="ghost" onClick={onClose}>
            取消
          </Button>
          {preview ? (
            <Button
              variant="primary"
              loading={busy}
              disabled={!name.trim()}
              onClick={() => void accept()}
            >
              {preview.existingBotId ? "再创建一份副本" : "确认导入 Bot"}
            </Button>
          ) : (
            <Button
              loading={busy}
              disabled={!link.trim()}
              onClick={() => void load()}
            >
              预览模板
            </Button>
          )}
        </>
      }
    >
      <div className="space-y-5">
        <TextField
          label="模板链接"
          value={link}
          disabled={busy}
          onChange={(e) => {
            setLink(e.target.value);
            setPreview(null);
            setError("");
          }}
          placeholder="粘贴朋友分享的模板链接"
        />
        {preview && (
          <>
            <p className="text-xs text-ink-3">
              由 {preview.published.publisher} 发布 · v
              {preview.published.revision}
            </p>
            {preview.existingBotId && (
              <p role="status" className="text-sm text-ink-2">
                你已导入过这个版本，继续会创建另一份独立副本。
              </p>
            )}
            <TemplateContents config={preview.published.config} />
            <TextField
              label="导入后的 Bot 名称"
              maxLength={40}
              value={name}
              disabled={busy}
              onChange={(e) => setName(e.target.value)}
            />
            <p className="text-xs leading-6 text-ink-3">
              使用你的默认模型，初始对话和记忆为空。协作与全部工具动作默认关闭，可在「配置
              Bot」中逐项开启。
            </p>
          </>
        )}
        {!preview && (
          <p className="text-sm leading-6 text-ink-2">
            请先在「设置 → Bot
            模板服务」配置与分享者相同的服务地址。我们会先展示配置，确认后才创建
            Bot。
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

export function TemplateImportHost({
  onImported,
}: {
  onImported: (bot: Bot) => void;
}) {
  const [pending, setPending] = useState<{ link: string; key: number } | null>(
    null,
  );
  const queue = useRef<string[]>([]);
  const active = useRef(false);
  useEffect(() => {
    const receive = (link: string) => {
      if (link.length > 2048) return;
      if (active.current) {
        if (!queue.current.includes(link) && queue.current.length < 10)
          queue.current.push(link);
        return;
      }
      active.current = true;
      setPending({ link, key: Date.now() });
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
        active.current = next !== undefined;
        setPending(next === undefined ? null : { link: next, key: Date.now() });
      }}
    />
  );
}
