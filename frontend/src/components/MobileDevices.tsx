import { IOSDownload } from "./IOSDownload";
import { useEffect, useState } from "react";
import { connectionRequest } from "../services/connections";
import { Button, Switch } from "./ui";
import { describeError } from "../services/api";

type Device = { key: string; name: string; addedAt: number };
type Status = {
  available: boolean;
  enabled: boolean;
  online: boolean;
  devices: Device[];
  pending: { device: Device; code: string; expiresAt: number } | null;
};
export function MobileDevices() {
  const [status, setStatus] = useState<Status | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [pair, setPair] = useState<{
    image: string;
    url: string;
    expiresAt: number;
  } | null>(null);
  useEffect(() => {
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const next = await connectionRequest<Status>("GET", "/mobile");
        if (active) {
          setStatus(next);
          setError("");
          if (!next.enabled) setPair(null);
        }
      } catch (e) {
        if (active) setError(describeError(e));
      }
      if (active) timer = setTimeout(poll, 2000);
    };
    void poll();
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, []);
  const act = async (action: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await action();
      setStatus(await connectionRequest<Status>("GET", "/mobile"));
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  const create = () =>
    act(async () => {
      const data = await connectionRequest<{ url: string; expiresAt: number }>(
        "POST",
        "/mobile/pairing",
        {},
      );
      const QR = await import("qrcode");
      setPair({
        url: data.url,
        image: await QR.toDataURL(data.url, {
          width: 300,
          margin: 2,
          errorCorrectionLevel: "M",
        }),
        expiresAt: data.expiresAt,
      });
    });
  return (
    <div className="space-y-5">
      <IOSDownload />
      <div className="rounded-xl border border-line p-5 text-sm leading-7 text-ink-2">
        <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold text-ink-1">同一个 Bot，接着聊</h3><span className="rounded-full bg-surface-3 px-3 py-1 text-xs">聊天同步 · 开发中</span></div>
        <p className="mt-3">计划让 Mac 和手机共用每个 Bot 的对话与记忆。在 Mac 上聊到哪里，手机就从哪里继续。</p>
        <ol className="mt-3 list-decimal space-y-1 pl-5"><li>安装手机 App，扫描 Mac 上的配对码。</li><li>在两端核对确认码，允许设备关联。</li><li>Mac 保持联网、唤醒，手机即可访问同一份数据。</li></ol>
        <p className="mt-3 text-xs leading-6 text-ink-3">目前已有设备配对、加密连接和任务／记忆访问能力；完整聊天同步仍在开发。首版数据保存在 Mac，手机离线时不能发送消息，Mac 休眠或关机时也无法处理请求。</p>
      </div>
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-sm font-medium text-ink-1">手机连接</p>
          <p className="mt-1 text-xs text-ink-3">
            {!status
              ? "正在读取…"
              : !status.available
                ? "远程连接服务尚未上线"
                : !status.enabled
                  ? "已关闭"
                  : status.online
                    ? "中继已连接"
                    : "正在连接中继…"}
          </p>
        </div>
        <Switch
          checked={status?.enabled ?? false}
          disabled={busy || !status?.available}
          onChange={(enabled) =>
            void act(async () => {
              await connectionRequest("POST", "/mobile/enabled", { enabled });
              if (!enabled) setPair(null);
            })
          }
          label="开启手机连接"
        />
      </div>
      {status?.enabled && (
        <Button disabled={busy || !status.online} onClick={() => void create()}>
          {pair ? "刷新二维码" : "关联手机"}
        </Button>
      )}
      {pair && !status?.pending && (
        <div className="flex flex-col items-center gap-3 rounded-2xl bg-surface-1 p-5">
          {pair.expiresAt > Date.now() / 1000 ? (
            <>
              <img
                src={pair.image}
                alt="使用 Soren 手机 App 扫描此二维码"
                className="w-60 max-w-full rounded-xl"
              />
              <p className="text-sm text-ink-2">
                用 Soren 手机 App 扫码 · 5 分钟有效
              </p>
              <Button
                size="sm"
                variant="ghost"
                onClick={() =>
                  void act(() => navigator.clipboard.writeText(pair.url))
                }
              >
                复制配对链接
              </Button>
            </>
          ) : (
            <p className="text-sm text-ink-2">二维码已过期，请刷新</p>
          )}
        </div>
      )}
      {status?.pending && (
        <div className="rounded-2xl border border-line bg-surface-1 p-5">
          <p className="text-sm text-ink-1">
            {status.pending.device.name} 请求关联
          </p>
          <p className="my-4 font-mono text-3xl tracking-[.25em] text-ink-1">
            {status.pending.code}
          </p>
          <p className="mb-4 text-xs leading-6 text-ink-2">
            核对手机上的确认码。允许后，这台手机可访问全部
            Bot、记忆和任务，并审批操作。
          </p>
          <div className="flex gap-2">
            {[false, true].map((approve) => (
              <Button
                key={String(approve)}
                variant={approve ? "primary" : "ghost"}
                disabled={busy}
                onClick={() =>
                  void act(async () => {
                    await connectionRequest("POST", "/mobile/decision", {
                      key: status.pending!.device.key,
                      code: status.pending!.code,
                      approve,
                    });
                    setPair(null);
                  })
                }
              >
                {approve ? "允许关联" : "拒绝"}
              </Button>
            ))}
          </div>
        </div>
      )}
      {status?.devices.map((device) => (
        <div
          key={device.key}
          className="flex items-center justify-between gap-3 border-t border-line pt-4"
        >
          <div className="min-w-0">
            <p className="truncate text-sm text-ink-1">{device.name}</p>
            <p className="mt-1 text-xs text-ink-3">
              {new Date(device.addedAt * 1000).toLocaleDateString()} 关联
            </p>
          </div>
          <Button
            size="sm"
            variant="ghost"
            disabled={busy}
            onClick={() =>
              void act(() =>
                connectionRequest("DELETE", `/mobile/devices/${device.key}`),
              )
            }
          >
            解除关联
          </Button>
        </div>
      ))}
      {error && (
        <p role="alert" className="text-sm text-red-500">
          {error}
        </p>
      )}
    </div>
  );
}
