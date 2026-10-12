import { useEffect, useRef, useState } from "react";
import { MobileClient, native, parsePair } from "./client";
import { Button, TextField } from "../components/ui";
import { MarkdownView } from "../components/chat/MarkdownView";
import { avatarURL, type Bot, type Run } from "../services/agents";
import type { Artifact, Invocation } from "../services/connections";
import "../styles/index.css";
import "./mobile.css";

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));
const labels: Record<string, string> = {
  queued: "等待执行",
  running: "执行中",
  waiting_approval: "等待审批",
  completed: "已完成",
  succeeded: "已完成",
  failed: "失败",
  cancelled: "已取消",
  pending: "等待确认",
  approved: "已批准",
  denied: "已拒绝",
  unknown: "结果待核实",
};
const initialPair = location.hash.includes("pair=") ? location.href : "";
if (initialPair) history.replaceState(null, "", location.pathname);
export function MobileApp() {
  const [client, setClient] = useState<MobileClient | null>(null),
    [restoring, setRestoring] = useState(true),
    [pairClient, setPairClient] = useState<MobileClient | null>(null);
  const [link, setLink] = useState(initialPair),
    [name, setName] = useState("我的手机"),
    [code, setCode] = useState(""),
    [error, setError] = useState(""),
    [connectionError, setConnectionError] = useState(""),
    [busy, setBusy] = useState(false);
  const [bots, setBots] = useState<Bot[]>([]),
    [bot, setBot] = useState("soren"),
    [runs, setRuns] = useState<Run[]>([]),
    [selected, setSelected] = useState(""),
    [goal, setGoal] = useState("");
  const [tools, setTools] = useState<Invocation[]>([]),
    [artifacts, setArtifacts] = useState<Artifact[]>([]),
    [online, setOnline] = useState(false);
  const [tab, setTab] = useState<"tasks" | "memories">("tasks"),
    [memories, setMemories] = useState<{ id: number; content: string }[]>([]);
  const requestId = useRef(crypto.randomUUID()),
    generation = useRef(0);
  useEffect(() => {
    let active = true;
    void MobileClient.restore()
      .then((c) => {
        if (active) setClient(c);
      })
      .catch((e) => {
        if (active) setError(message(e));
      })
      .finally(() => {
        if (active) setRestoring(false);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    const media = matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      document.documentElement.classList.toggle("dark", media.matches);
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (!pairClient) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const result = (await pairClient.pair(name)) as {
          state: string;
          code?: string;
        };
        if (!active) return;
        if (result.state === "paired") {
          await pairClient.save();
          if (active) {
            setClient(pairClient);
            setPairClient(null);
            setLink("");
            setCode("");
            setError("");
          }
          return;
        }
        setCode(await pairClient.confirmationCode());
        if (active) timer = setTimeout(poll, 2000);
      } catch (e) {
        if (active) {
          setError(message(e));
          setPairClient(null);
          setCode("");
        }
      }
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [pairClient, name]);
  useEffect(() => {
    if (!client) return;
    const current = ++generation.current;
    let timer: ReturnType<typeof setTimeout>;
    let polling = false;
    const poll = async () => {
      if (polling || current !== generation.current) return;
      polling = true;
      try {
        const list = (await client.request("GET", "/api/bots")) as Bot[];
        const tasks = (await client.request("GET", "/api/runs", bot)) as Run[];
        if (current !== generation.current) return;
        setBots(list);
        setRuns(tasks);
        if (selected) {
          const calls = (await client.request(
            "GET",
            `/api/runs/${selected}/tools`,
            bot,
          )) as Invocation[];
          const files = (await client.request(
            "GET",
            `/api/runs/${selected}/artifacts`,
            bot,
          )) as Artifact[];
          if (current !== generation.current) return;
          setTools(calls);
          setArtifacts(files);
        }
        if (tab === "memories") {
          const data = (await client.request("GET", "/api/memories", bot)) as {
            items?: { id: number; content: string }[];
          };
          if (current === generation.current) setMemories(data.items ?? []);
        }
        if (current === generation.current) {
          setOnline(true);
          setConnectionError("");
        }
      } catch (e) {
        if (current === generation.current) {
          setOnline(false);
          setConnectionError(message(e));
        }
      }
      polling = false;
      if (current === generation.current) timer = setTimeout(poll, 3000);
    };
    void poll();
    const resume = () => {
      if (document.visibilityState === "visible") {
        clearTimeout(timer);
        /* Poll finishes and schedules its next refresh. */ timer = setTimeout(
          () => {
            if (current === generation.current) void poll();
          },
          300,
        );
      }
    };
    document.addEventListener("visibilitychange", resume);
    return () => {
      generation.current++;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", resume);
    };
  }, [client, bot, selected, tab]);
  const act = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  };
  const pair = (raw: string) =>
    act(async () => {
      if (!name.trim()) throw new Error("请填写设备名称");
      const next = MobileClient.create(parsePair(raw));
      setCode(await next.confirmationCode());
      setPairClient(next);
    });
  const scan = () =>
    act(async () => {
      const { scan, Format, requestPermissions } =
        await import("@tauri-apps/plugin-barcode-scanner");
      await requestPermissions();
      const result = await scan({ formats: [Format.QRCode], windowed: true });
      const next = MobileClient.create(parsePair(result.content));
      setCode(await next.confirmationCode());
      setPairClient(next);
    });
  const start = () =>
    act(async () => {
      const run = (await client!.request("POST", "/api/runs", bot, {
        goal,
        participants: [],
        requestId: requestId.current,
      })) as Run;
      requestId.current = crypto.randomUUID();
      setGoal("");
      setSelected(run.id);
      setTools([]);
      setArtifacts([]);
      setRuns((prev) => [run, ...prev.filter((x) => x.id !== run.id)]);
    });
  const unlink = () =>
    act(async () => {
      await MobileClient.forget();
      generation.current++;
      setClient(null);
      setConnectionError("");
      setBots([]);
      setRuns([]);
      setMemories([]);
      setTools([]);
      setArtifacts([]);
      setSelected("");
      setGoal("");
      setOnline(false);
    });
  const currentRun = runs.find((r) => r.id === selected);
  return (
    <main className="mobile-shell">
      <header className="mobile-header">
        <div className="flex items-center gap-3">
          <img src="/app-icon.svg" alt="" width="36" height="36" />
          <strong className="text-xl tracking-tight">
            Soren
            <span className="ml-2 text-xs font-normal text-ink-3">随身</span>
          </strong>
        </div>
        {client && (
          <button
            className="text-xs text-ink-3"
            onClick={() => void unlink()}
            disabled={busy}
          >
            断开本机
          </button>
        )}
      </header>
      {restoring ? (
        <p className="p-8 text-ink-3">正在读取设备身份…</p>
      ) : !client ? (
        <section className="mobile-pair">
          <div className="mobile-orb">
            <img src="/bot-avatars/robo.png" alt="Soren 伙伴" />
          </div>
          <p className="mobile-eyebrow">YOUR SPACE, WITH YOU</p>
          <h1>
            把你的 Soren
            <br />
            带在身边。
          </h1>
          <p className="mt-4 text-sm leading-7 text-ink-2">
            在 Mac 打开「设置 → 关联设备」，
            <br />
            扫码后，在电脑上确认。
          </p>
          {pairClient ? (
            <div className="mobile-confirm">
              <p className="text-sm text-ink-2">请核对 Mac 上的确认码</p>
              <strong>{code}</strong>
              <p className="text-xs text-ink-3">等待 Mac 允许关联</p>
              <Button
                className="mt-4"
                variant="ghost"
                onClick={() => {
                  setPairClient(null);
                  setCode("");
                }}
              >
                取消等待
              </Button>
            </div>
          ) : (
            <div className="mt-8 space-y-4 text-left">
              <TextField
                label="设备名称"
                value={name}
                maxLength={60}
                onChange={(e) => setName(e.target.value)}
              />
              {native && (
                <Button
                  variant="primary"
                  block
                  loading={busy}
                  onClick={() => void scan()}
                >
                  扫描 Mac 二维码
                </Button>
              )}
              <details open={!native}>
                <summary className="cursor-pointer text-center text-xs text-ink-3">
                  使用配对链接
                </summary>
                <div className="mt-4 space-y-3">
                  <TextField
                    label="配对链接"
                    value={link}
                    onChange={(e) => setLink(e.target.value)}
                    placeholder="粘贴 Mac 的配对链接"
                  />
                  <Button
                    variant="primary"
                    block
                    disabled={!link || busy}
                    onClick={() => void pair(link)}
                  >
                    关联 Mac
                  </Button>
                </div>
              </details>
              {!native && (
                <p className="text-xs leading-6 text-ink-3">
                  网页测试入口：身份仅保存在本次页面内，关闭后需重新配对。
                </p>
              )}
            </div>
          )}
        </section>
      ) : (
        <>
          <div className="mobile-connection">
            <span className={online ? "mobile-dot online" : "mobile-dot"} />
            {online ? "Mac 已连接" : "正在连接 / Mac 离线"}
            <span className="ml-auto text-xs text-ink-3">任务在 Mac 执行</span>
          </div>
          <nav className="mobile-bots" aria-label="选择 Bot">
            {bots.map((b) => (
              <button
                key={b.id}
                disabled={busy}
                aria-pressed={bot === b.id}
                className={bot === b.id ? "active" : ""}
                onClick={() => {
                  setBot(b.id);
                  setSelected("");
                  setRuns([]);
                  setTools([]);
                  setArtifacts([]);
                  setMemories([]);
                  requestId.current = crypto.randomUUID();
                }}
              >
                <img src={avatarURL(b.avatar)} alt="" />
                <span>{b.name}</span>
              </button>
            ))}
          </nav>
          <div className="mobile-tabs">
            <button
              aria-pressed={tab === "tasks"}
              onClick={() => setTab("tasks")}
            >
              任务
            </button>
            <button
              aria-pressed={tab === "memories"}
              onClick={() => setTab("memories")}
            >
              记忆
            </button>
          </div>
          {tab === "tasks" ? (
            <>
              <div className="mobile-composer">
                <textarea
                  aria-label="交给 Bot 的任务"
                  placeholder="有什么交给我？"
                  value={goal}
                  maxLength={8000}
                  onChange={(e) => setGoal(e.target.value)}
                />
                <div className="flex justify-end">
                  <Button
                    variant="primary"
                    disabled={!goal.trim() || !online}
                    loading={busy}
                    onClick={() => void start()}
                  >
                    交给 {bots.find((b) => b.id === bot)?.name || "Soren"}
                  </Button>
                </div>
              </div>
              <section className="space-y-3" aria-label="任务列表">
                {runs.map((run) => (
                  <article className="mobile-task" key={run.id}>
                    <button
                      className="mobile-task-title"
                      aria-expanded={selected === run.id}
                      onClick={() => {
                        setSelected(selected === run.id ? "" : run.id);
                        setTools([]);
                        setArtifacts([]);
                      }}
                    >
                      <span>{run.goal}</span>
                      <small>{labels[run.status] || run.status}</small>
                    </button>
                    {currentRun?.id === run.id && (
                      <div className="mt-4 space-y-4">
                        {run.result && <MarkdownView content={run.result} />}{" "}
                        {run.error && (
                          <p className="text-sm text-red-500">{run.error}</p>
                        )}
                        {run.steps?.map((step, i) => (
                          <details key={i}>
                            <summary className="text-xs text-ink-2">
                              {step.name} · {labels[step.status] || step.status}
                            </summary>
                            <MarkdownView content={step.output || ""} />
                          </details>
                        ))}
                        {tools.map((call) => (
                          <div key={call.id} className="mobile-tool">
                            <p className="break-all text-xs font-medium">
                              {call.tool} · {labels[call.status] || call.status}
                            </p>
                            {call.account && (
                              <p className="mt-1 break-all text-xs text-ink-3">
                                {call.account}
                              </p>
                            )}
                            <pre>{call.arguments}</pre>
                            {call.status === "pending" && (
                              <div className="flex gap-2">
                                {[false, true].map((approve) => (
                                  <Button
                                    key={String(approve)}
                                    size="sm"
                                    variant={approve ? "primary" : "ghost"}
                                    disabled={busy || !online}
                                    onClick={() =>
                                      void act(async () => {
                                        await client.request(
                                          "POST",
                                          `/api/tool-invocations/${call.id}/decision`,
                                          bot,
                                          { approve },
                                        );
                                        setTools((prev) =>
                                          prev.map((x) =>
                                            x.id === call.id
                                              ? {
                                                  ...x,
                                                  status: approve
                                                    ? "approved"
                                                    : "denied",
                                                }
                                              : x,
                                          ),
                                        );
                                      })
                                    }
                                  >
                                    {approve ? "批准此操作" : "拒绝"}
                                  </Button>
                                ))}
                              </div>
                            )}
                          </div>
                        ))}
                        {artifacts.map((a) => (
                          <details key={a.id}>
                            <summary className="text-sm font-medium">
                              {a.title}
                            </summary>
                            <MarkdownView content={a.content} />
                          </details>
                        ))}
                        <div className="flex gap-2">
                          {["waiting_approval", "paused"].includes(
                            run.status,
                          ) && (
                            <Button
                              size="sm"
                              disabled={busy || !online}
                              onClick={() =>
                                void act(async () => {
                                  await client.request(
                                    "POST",
                                    `/api/runs/${run.id}/resume`,
                                    bot,
                                    {},
                                  );
                                })
                              }
                            >
                              继续任务
                            </Button>
                          )}
                          {["queued", "running", "waiting_approval"].includes(
                            run.status,
                          ) && (
                            <Button
                              size="sm"
                              variant="ghost"
                              disabled={busy || !online}
                              onClick={() =>
                                void act(async () => {
                                  await client.request(
                                    "POST",
                                    `/api/runs/${run.id}/cancel`,
                                    bot,
                                    {},
                                  );
                                })
                              }
                            >
                              停止
                            </Button>
                          )}
                        </div>
                      </div>
                    )}
                  </article>
                ))}
              </section>
            </>
          ) : (
            <section className="space-y-3">
              {memories.map((m) => (
                <article
                  key={m.id}
                  className="mobile-task text-sm leading-7 text-ink-2"
                >
                  {m.content}
                </article>
              ))}
            </section>
          )}
        </>
      )}
      {(error || connectionError) && (
        <p role="alert" className="mobile-error">
          {error || connectionError}
        </p>
      )}
    </main>
  );
}
