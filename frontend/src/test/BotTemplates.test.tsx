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
  it("opens a received link without downloading or creating a Bot until explicit actions", async () => {
    vi.mocked(templatesAPI.preview).mockResolvedValue({
      previewId: "preview",
      service: "https://templates.example",
      published,
      expiresAt: "",
      existingBotId: "existing",
    });
    vi.mocked(templatesAPI.import).mockResolvedValue({
      ...DEFAULT_BOT,
      id: "imported",
      name: "我的视频助手",
    });
    const onImported = vi.fn();
    render(<TemplateImportHost onImported={onImported} />);
    act(() => openTemplateImport(published.url));
    expect(screen.getByLabelText("模板链接")).toHaveValue(published.url);
    expect(templatesAPI.preview).not.toHaveBeenCalled();
    expect(templatesAPI.import).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("预览模板"));
    await screen.findByText("不发邮件");
    expect(templatesAPI.import).not.toHaveBeenCalled();
    fireEvent.change(screen.getByLabelText("导入后的 Bot 名称"), {
      target: { value: "我的视频助手" },
    });
    fireEvent.click(screen.getByText("再创建一份副本"));
    await waitFor(() =>
      expect(templatesAPI.import).toHaveBeenCalledWith(
        "preview",
        "我的视频助手",
      ),
    );
    await waitFor(() => expect(onImported).toHaveBeenCalledOnce());
  });
  it("changing a link invalidates the preview and cancel does not import", async () => {
    vi.mocked(templatesAPI.preview).mockResolvedValue({
      previewId: "preview",
      service: "",
      published,
      expiresAt: "",
    });
    render(<TemplateImportHost onImported={vi.fn()} />);
    act(() => openTemplateImport(published.url));
    fireEvent.click(screen.getByText("预览模板"));
    await screen.findByText("确认导入 Bot");
    fireEvent.change(screen.getByLabelText("模板链接"), {
      target: { value: "https://other.example/t/123/v/2" },
    });
    expect(screen.queryByText("确认导入 Bot")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("取消"));
    expect(templatesAPI.import).not.toHaveBeenCalled();
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
    await screen.findByText("模板已发布");
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
