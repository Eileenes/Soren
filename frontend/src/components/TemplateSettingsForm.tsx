import { useEffect, useState } from "react";
import { Button, PasswordField, Switch, TextField } from "./ui";
import { templatesAPI, type TemplateSettings } from "../services/templates";
import { describeError } from "../services/api";

export function TemplateSettingsForm() {
  const [saved, setSaved] = useState<TemplateSettings | null>(null);
  const [url, setURL] = useState("");
  const [token, setToken] = useState("");
  const [clear, setClear] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const load = async () => {
    setBusy(true);
    setError("");
    try {
      const value = await templatesAPI.settings();
      setSaved(value);
      setURL(value.url);
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  useEffect(() => {
    void load();
  }, []);
  const save = async () => {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const value = await templatesAPI.saveSettings(url, token, clear);
      setSaved(value);
      setURL(value.url);
      setToken("");
      setClear(false);
      setNotice("模板服务配置已保存。");
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="space-y-5">
      <p className="text-sm leading-6 text-ink-2">
        与朋友使用同一个模板服务，即可分享和导入
        Bot。导入只需服务地址；发布模板需要管理员分配给你的发布凭据。
      </p>
      <TextField
        label="模板服务地址"
        value={url}
        placeholder="https://templates.example.com"
        disabled={busy || !saved}
        onChange={(e) => {
          setURL(e.target.value);
          setNotice("");
        }}
        helper="填写独立模板服务的根地址。更换地址会清除旧发布凭据。"
      />
      <PasswordField
        label="发布凭据（可选）"
        value={token}
        disabled={busy || !saved}
        onChange={(e) => setToken(e.target.value)}
        placeholder={
          saved?.configured && url === saved.url
            ? "已配置 · 留空保留"
            : "仅发布者需要填写"
        }
        helper="凭据保存在系统钥匙串中，不会随模板分享。"
      />
      {saved?.configured && (
        <Switch
          label="移除已保存的发布凭据"
          checked={clear}
          onChange={setClear}
        />
      )}
      <div className="flex flex-wrap gap-3">
        <Button loading={busy} disabled={!saved} onClick={() => void save()}>
          保存模板服务
        </Button>
        {saved?.configured && (
          <Button
            variant="ghost"
            disabled={busy || url !== saved.url || Boolean(token) || clear}
            onClick={async () => {
              setBusy(true);
              setError("");
              setNotice("");
              try {
                const identity = await templatesAPI.identity();
                setNotice(`发布凭据有效 · ${identity.name}`);
              } catch (e) {
                setError(describeError(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            验证发布身份
          </Button>
        )}
        {!saved && (
          <Button disabled={busy} onClick={() => void load()}>
            重新读取
          </Button>
        )}
      </div>
      {notice && (
        <p role="status" className="text-sm text-ink-2">
          {notice}
        </p>
      )}
      {error && (
        <p role="alert" className="bot-error">
          {error}
        </p>
      )}
    </div>
  );
}
