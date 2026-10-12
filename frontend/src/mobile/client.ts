import nacl from "tweetnacl";
import { invoke } from "@tauri-apps/api/core";
export const native = "__TAURI_INTERNALS__" in window;
export const encode = (bytes: Uint8Array) => {
  let raw = "";
  for (let i = 0; i < bytes.length; i += 8192)
    raw += String.fromCharCode(...bytes.subarray(i, i + 8192));
  return btoa(raw).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
};
export const decode = (text: string) =>
  Uint8Array.from(atob(text.replace(/-/g, "+").replace(/_/g, "/")), (c) =>
    c.charCodeAt(0),
  );
export interface PairInfo {
  v: number;
  relay: string;
  host: string;
  publicKey: string;
  secret?: string;
  expiresAt?: number;
}
export interface Credential {
  pair: PairInfo;
  secretKey: string;
}
export interface WireRequest {
  id: string;
  deadline: number;
  kind: "pair" | "api";
  secret?: string;
  name?: string;
  method?: string;
  path?: string;
  botId?: string;
  body?: unknown;
}
const encoder = new TextEncoder(),
  decoder = new TextDecoder();
export function parsePair(text: string): PairInfo {
  const url = new URL(text);
  const part = new URLSearchParams(url.hash.slice(1)).get("pair");
  if (!part || part.length > 4096) throw new Error("不是有效的 Soren 配对码");
  const pair = JSON.parse(decoder.decode(decode(part))) as PairInfo;
  validatePair(pair);
  if (
    !pair.secret ||
    !/^[a-f0-9]{64}$/.test(pair.secret) ||
    !pair.expiresAt ||
    pair.expiresAt < Date.now() / 1000
  )
    throw new Error("二维码已过期，请在 Mac 刷新");
  return pair;
}
function validatePair(p: PairInfo) {
  const u = new URL(p.relay);
  const local =
    import.meta.env.DEV &&
    u.protocol === "http:" &&
    ["localhost", "127.0.0.1"].includes(u.hostname);
  if (
    p.v !== 1 ||
    (!local && u.protocol !== "https:") ||
    u.username ||
    u.password ||
    u.search ||
    u.hash ||
    (u.pathname !== "/" && u.pathname !== "") ||
    !/^[a-f0-9]{64}$/.test(p.host) ||
    decode(p.publicKey).length !== 32
  )
    throw new Error("配对地址无效");
  // Production apps pin the product-operated relay, never arbitrary QR origins.
  const expected = import.meta.env.VITE_SOREN_RELAY_URL as string | undefined;
  if (
    !import.meta.env.DEV &&
    (!expected || u.origin !== new URL(expected).origin)
  )
    throw new Error("当前安装包尚未配置此 Soren 连接服务");
}
export class MobileClient {
  constructor(public credential: Credential) {
    validatePair(credential.pair);
    if (decode(credential.secretKey).length !== 32)
      throw new Error("设备身份无效");
  }
  static create(pair: PairInfo) {
    return new MobileClient({
      pair,
      secretKey: encode(nacl.box.keyPair().secretKey),
    });
  }
  request(method: string, path: string, botId = "soren", body?: unknown) {
    return this.exchange({
      id: crypto.randomUUID(),
      deadline: Math.floor(Date.now() / 1000) + 120,
      kind: "api",
      method,
      path,
      botId,
      body,
    });
  }
  async pair(name: string) {
    return this.exchange({
      id: crypto.randomUUID(),
      deadline: Math.floor(Date.now() / 1000) + 120,
      kind: "pair",
      secret: this.credential.pair.secret,
      name,
    });
  }
  async confirmationCode() {
    const privateKey = decode(this.credential.secretKey),
      publicKey = nacl.box.keyPair.fromSecretKey(privateKey).publicKey;
    const input = new Uint8Array(64);
    input.set(decode(this.credential.pair.publicKey));
    input.set(publicKey, 32);
    const sum = new Uint8Array(await crypto.subtle.digest("SHA-256", input));
    return String(((sum[0] << 16) | (sum[1] << 8) | sum[2]) % 1000000).padStart(
      6,
      "0",
    );
  }
  async exchange<T = unknown>(request: WireRequest): Promise<T> {
    const { pair, secretKey } = this.credential;
    const secret = decode(secretKey),
      publicKey = nacl.box.keyPair.fromSecretKey(secret).publicKey,
      nonce = nacl.randomBytes(24);
    const plain = Uint8Array.from(encoder.encode(JSON.stringify(request)));
    if (plain.length > 96 * 1024) throw new Error("内容过长，请缩短后重试");
    const envelope = {
      key: encode(publicKey),
      nonce: encode(nonce),
      box: encode(nacl.box(plain, nonce, decode(pair.publicKey), secret)),
    };
    const res = await fetch(`${pair.relay}/v1/hosts/${pair.host}/exchange`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(envelope),
      signal: AbortSignal.timeout(85000),
      credentials: "omit",
      redirect: "error",
      cache: "no-store",
      referrerPolicy: "no-referrer",
    });
    if (!res.ok)
      throw new Error(
        res.status === 503
          ? "Mac 离线，请保持电脑运行并联网"
          : res.status === 429
            ? "连接繁忙，请稍后刷新"
            : "连接中断；操作可能已提交，请先刷新任务确认",
      );
    const text = await res.text();
    if (text.length > 2 * 1024 * 1024) throw new Error("响应过大");
    const wire = JSON.parse(text) as {
      key: string;
      nonce: string;
      box: string;
    };
    if (wire.key !== pair.publicKey) throw new Error("Mac 身份不匹配");
    const data = nacl.box.open(
      decode(wire.box),
      decode(wire.nonce),
      decode(pair.publicKey),
      secret,
    );
    if (!data) throw new Error("消息验证失败");
    const result = JSON.parse(decoder.decode(data)) as {
      id: string;
      status: number;
      body: T & { error?: string };
    };
    if (result.id !== request.id) throw new Error("响应编号不匹配");
    if (result.status >= 400) throw new Error(result.body?.error || "操作失败");
    return result.body;
  }
  async save() {
    const pair = { ...this.credential.pair };
    delete pair.secret;
    delete pair.expiresAt;
    this.credential = { ...this.credential, pair };
    if (native)
      await invoke("save_mobile_credential", {
        value: JSON.stringify(this.credential),
      });
  }
  static async restore() {
    if (!native) return null;
    const raw = await invoke<string | null>("load_mobile_credential");
    return raw ? new MobileClient(JSON.parse(raw) as Credential) : null;
  }
  static async forget() {
    if (native) await invoke("delete_mobile_credential");
  }
}
