import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import nacl from "tweetnacl";
import { MobileClient, encode, decode, parsePair } from "../mobile/client";
const host = nacl.box.keyPair();
const pair = {
  v: 1,
  relay: "http://127.0.0.1:8090",
  host: "a".repeat(64),
  publicKey: encode(host.publicKey),
  secret: "b".repeat(64),
  expiresAt: Math.floor(Date.now() / 1000) + 300,
};
const pairURL = (p: unknown) =>
  "http://localhost:1420/mobile.html#pair=" +
  encode(new TextEncoder().encode(JSON.stringify(p)));
beforeEach(() => vi.stubGlobal("fetch", vi.fn()));
afterEach(() => vi.unstubAllGlobals());
describe("mobile trust and transport", () => {
  it("validates expiry, endpoint and public key before connecting", () => {
    expect(parsePair(pairURL(pair)).host).toBe(pair.host);
    expect(() => parsePair(pairURL({ ...pair, expiresAt: 1 }))).toThrow("过期");
    expect(() =>
      parsePair(pairURL({ ...pair, relay: "http://remote.example" })),
    ).toThrow("地址");
    expect(() => parsePair(pairURL({ ...pair, publicKey: "abc" }))).toThrow();
  });
  it("sends only ciphertext and accepts an authenticated matching reply", async () => {
    const client = MobileClient.create(pair);
    vi.mocked(fetch).mockImplementation(async (_url, options) => {
      const wire = JSON.parse(options!.body as string);
      expect(options!.body).not.toContain("private task");
      expect(options!.body).not.toContain("api/runs");
      const plain = nacl.box.open(
        decode(wire.box),
        decode(wire.nonce),
        decode(wire.key),
        host.secretKey,
      )!;
      const req = JSON.parse(new TextDecoder().decode(plain));
      expect(req.body.goal).toBe("private task");
      const nonce = nacl.randomBytes(24);
      return new Response(
        JSON.stringify({
          key: pair.publicKey,
          nonce: encode(nonce),
          box: encode(
            nacl.box(
              Uint8Array.from(
                new TextEncoder().encode(
                  JSON.stringify({
                    id: req.id,
                    status: 200,
                    body: { id: "run-1" },
                  }),
                ),
              ),
              nonce,
              decode(wire.key),
              host.secretKey,
            ),
          ),
        }),
        { status: 200 },
      );
    });
    expect(
      await client.request("POST", "/api/runs", "soren", {
        goal: "private task",
      }),
    ).toEqual({ id: "run-1" });
  });
  it("rejects a substituted host and reports an offline Mac honestly", async () => {
    const client = MobileClient.create(pair);
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ key: "wrong" })),
    );
    await expect(client.request("GET", "/api/bots")).rejects.toThrow(
      "身份不匹配",
    );
    vi.mocked(fetch).mockResolvedValueOnce(new Response("", { status: 503 }));
    await expect(client.request("GET", "/api/bots")).rejects.toThrow(
      "Mac 离线",
    );
  });
  it("clears QR secrets when pairing is complete without persisting browser credentials", async () => {
    const client = MobileClient.create(pair);
    const spy = vi.spyOn(Storage.prototype, "setItem");
    await client.save();
    expect(client.credential.pair.secret).toBeUndefined();
    expect(client.credential.pair.expiresAt).toBeUndefined();
    expect(spy).not.toHaveBeenCalled();
    spy.mockRestore();
  });
});
