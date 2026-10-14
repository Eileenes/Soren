import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  TemplateImportHost,
  TemplatePublisher,
} from "../components/BotTemplates";
import { TemplateSettingsForm } from "../components/TemplateSettingsForm";
import {
  templatesAPI,
  openTemplateImport,
  type TemplateConfig,
} from "../services/templates";
import { DEFAULT_BOT } from "../services/agents";

vi.mock("../services/templates", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../services/templates")>();
  return {
    ...actual,
    templatesAPI: {
      export: vi.fn(),
      publish: vi.fn(),
      preview: vi.fn(),
      import: vi.fn(),
      settings: vi.fn(),
      saveSettings: vi.fn(),
      identity: vi.fn(),
    },
  };
});
const config: TemplateConfig = {
  format: "soren.bot-template",
  schemaVersion: 1,
  name: "视频助手",
  description: "",
  avatar: "robo",
  instructions: "先写脚本",
  definition: {
    canDo: ["写分镜"],
    cannotDo: ["不发邮件"],
    workflow: "",
    output: "",
  },
  policyTemplate: { actions: { "video.generate": "confirm" } },
  memoryEnabled: true,
  requiredCapabilities: ["video.generate"],
};
const published = {
  id: "template",
  revision: 1,
  digest: "digest",
  publisher: "作者",
  createdAt: "",
  url: "https://templates.example/t/template/v/1",
  disabled: false,
  config,
};
beforeEach(() => {
  vi.clearAllMocks();
});
describe("Bot template flows", () => {
  it("opens a received link as a card and creates a Bot only after import", async () => {
    vi.mocked(templatesAPI.preview).mockResolvedValue({
      previewId: "preview",
      service: "https://templates.example",
      published,
      expiresAt: "",
    });
    vi.mocked(templatesAPI.import).mockResolvedValue({
      ...DEFAULT_BOT,
      id: "imported",
      name: config.name,
    });
    const onImported = vi.fn();
    render(<TemplateImportHost onImported={onImported} />);
    act(() => openTemplateImport(published.url));
    await screen.findByRole("button", { name: "导入到我的 Soren" });
    expect(templatesAPI.preview).toHaveBeenCalledWith(published.url);
    expect(
      screen.getByRole("heading", { name: config.name }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("模板链接")).toHaveValue(published.url);
    expect(screen.queryByText("不发邮件")).not.toBeInTheDocument();
    expect(templatesAPI.import).not.toHaveBeenCalled();
    act(() => openTemplateImport(published.url));
    fireEvent.click(screen.getByRole("button", { name: "导入到我的 Soren" }));
    await waitFor(() => expect(onImported).toHaveBeenCalledOnce());
    expect(templatesAPI.import).toHaveBeenCalledWith("preview", config.name);
    expect(templatesAPI.preview).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
  it("warns about existing copies without importing on close", async () => {
    vi.mocked(templatesAPI.preview).mockResolvedValue({
      previewId: "preview",
      service: "",
      published,
      expiresAt: "",
      existingBotId: "existing",
    });
    render(<TemplateImportHost onImported={vi.fn()} />);
    act(() => openTemplateImport(published.url));
    await screen.findByRole("button", { name: "再导入一份" });
    expect(screen.getByText(/已导入过这个模板/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(templatesAPI.import).not.toHaveBeenCalled();
  });
  it("allows manual paste and a retry after a failed preview", async () => {
    vi.mocked(templatesAPI.preview)
      .mockRejectedValueOnce(new Error("无法连接模板服务"))
      .mockResolvedValueOnce({
        previewId: "preview",
        service: "",
        published,
        expiresAt: "",
      });
    render(<TemplateImportHost onImported={vi.fn()} />);
    act(() => openTemplateImport());
    expect(templatesAPI.preview).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("模板链接"), {
      target: { value: published.url },
    });
    fireEvent.click(screen.getByRole("button", { name: "打开模板卡片" }));
    await screen.findByRole("alert");
    expect(screen.getByLabelText("模板链接")).toHaveValue(published.url);
    fireEvent.click(screen.getByRole("button", { name: "打开模板卡片" }));
    await screen.findByRole("button", { name: "导入到我的 Soren" });
    expect(templatesAPI.import).not.toHaveBeenCalled();
  });
  it("discards a late preview after closing and opening another link", async () => {
    let resolveFirst!: (
      value: Awaited<ReturnType<typeof templatesAPI.preview>>,
    ) => void;
    vi.mocked(templatesAPI.preview)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve;
          }),
      )
      .mockResolvedValueOnce({
        previewId: "second",
        service: "",
        published: { ...published, config: { ...config, name: "第二个 Bot" } },
        expiresAt: "",
      });
    render(<TemplateImportHost onImported={vi.fn()} />);
    act(() => openTemplateImport(published.url));
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    act(() => openTemplateImport(published.url + "2"));
    await screen.findByRole("heading", { name: "第二个 Bot" });
    await act(async () =>
      resolveFirst({
        previewId: "first",
        service: "",
        published,
        expiresAt: "",
      }),
    );
    expect(
      screen.queryByRole("heading", { name: config.name }),
    ).not.toBeInTheDocument();
  });
  it("previews only the exported configuration and retains the idempotency key on retry", async () => {
    vi.mocked(templatesAPI.export).mockResolvedValue(config);
    vi.mocked(templatesAPI.publish)
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce(published);
    render(<TemplatePublisher botId="original" onClose={vi.fn()} />);
    await screen.findByText("不发邮件");
    expect(templatesAPI.publish).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("确认发布模板"));
    await screen.findByRole("alert");
    const requestId = vi.mocked(templatesAPI.publish).mock.calls[0][1];
    fireEvent.click(screen.getByText("确认发布模板"));
    await screen.findByRole("dialog", { name: "模板已发布" });
    expect(screen.queryByText("不发邮件")).not.toBeInTheDocument();
    expect(vi.mocked(templatesAPI.publish).mock.calls[1][1]).toBe(requestId);
    expect(screen.getByLabelText("模板链接")).toHaveValue(published.url);
  });
  it("does not echo or persist publishing credentials in settings", async () => {
    vi.mocked(templatesAPI.settings).mockResolvedValue({
      url: "https://templates.example",
      configured: false,
    });
    vi.mocked(templatesAPI.saveSettings).mockResolvedValue({
      url: "https://templates.example",
      configured: true,
    });
    const storage = vi.spyOn(Storage.prototype, "setItem");
    render(<TemplateSettingsForm />);
    await waitFor(() =>
      expect(screen.getByLabelText("发布凭据（可选）")).toBeEnabled(),
    );
    fireEvent.change(screen.getByLabelText("发布凭据（可选）"), {
      target: { value: "test-private-token" },
    });
    fireEvent.click(screen.getByText("保存模板服务"));
    await screen.findByText("模板服务配置已保存。");
    expect(screen.getByLabelText("发布凭据（可选）")).toHaveValue("");
    expect(storage).not.toHaveBeenCalled();
    storage.mockRestore();
  });
});
