import axios from "axios";
import { getApiBaseUrl, normalizeError } from "./api";
import type { Bot, BotDefinition, BotPolicy } from "./agents";

export interface TemplateConfig {
  format: "soren.bot-template";
  schemaVersion: 1;
  name: string;
  description: string;
  avatar: string;
  instructions: string;
  definition: BotDefinition;
  policyTemplate: BotPolicy | null;
  memoryEnabled: boolean;
  requiredCapabilities: string[];
}
export interface PublishedTemplate {
  id: string;
  revision: number;
  digest: string;
  publisher: string;
  createdAt: string;
  url: string;
  disabled: boolean;
  config: TemplateConfig;
}
export interface TemplatePreview {
  previewId: string;
  service: string;
  published: PublishedTemplate;
  expiresAt: string;
  existingBotId?: string;
}
export interface TemplateSettings {
  url: string;
  configured: boolean;
}
async function request<T>(
  method: string,
  path: string,
  data?: unknown,
): Promise<T> {
  try {
    return (
      await axios.request<T>({
        method,
        url: getApiBaseUrl() + "/api" + path,
        data,
        headers: { "X-Bot-ID": "soren" },
        timeout: 20000,
      })
    ).data;
  } catch (error) {
    throw normalizeError(error);
  }
}
export const templatesAPI = {
  settings: () => request<TemplateSettings>("GET", "/template-service"),
  saveSettings: (url: string, token: string, clearToken: boolean) =>
    request<TemplateSettings>("PUT", "/template-service", {
      url,
      token,
      clearToken,
    }),
  identity: () =>
    request<{ name: string }>("GET", "/template-service/identity"),
  export: (botId: string) =>
    request<TemplateConfig>("GET", `/bots/${botId}/template`),
  list: () => request<PublishedTemplate[]>("GET", "/bot-templates"),
  publish: (config: TemplateConfig, requestId: string, id?: string) =>
    request<PublishedTemplate>(
      "POST",
      id ? `/bot-templates/${id}/versions` : "/bot-templates",
      { config, requestId },
    ),
  disable: (id: string, disabled: boolean) =>
    request("PATCH", `/bot-templates/${id}`, { disabled }),
  remove: (id: string) => request("DELETE", `/bot-templates/${id}`),
  preview: (link: string) =>
    request<TemplatePreview>("POST", "/template-imports/preview", { link }),
  import: (previewId: string, name: string) =>
    request<Bot>("POST", "/template-imports/confirm", { previewId, name }),
};

export function openTemplateImport(link = "") {
  window.dispatchEvent(
    new CustomEvent("soren:import-template", { detail: link }),
  );
}
