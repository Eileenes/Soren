import { useState, type ReactNode } from "react";
import { avatarURL } from "../services/agents";
import { IconCheck, IconCopy } from "./icons";
import { IconButton } from "./ui";

/** The same portable identity appears when sharing, managing and importing. */
export function TemplateCard({
  name,
  avatar,
  url,
  children,
  corner,
}: {
  name: string;
  avatar: string;
  url: string;
  children?: ReactNode;
  corner?: ReactNode;
}) {
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");
  return (
    <article className="template-card" aria-label={`${name} 模板卡片`}>
      {corner && <div className="template-card-corner">{corner}</div>}
      <div className="template-card-identity">
        <div className="template-card-avatar">
          <img src={avatarURL(avatar)} alt="" width={88} height={88} />
        </div>
        <h2>{name}</h2>
      </div>
      <div className="template-card-link">
        <input
          aria-label="模板链接"
          readOnly
          value={url}
          onFocus={(event) => event.target.select()}
        />
        <IconButton
          label={copied ? "已复制链接" : "复制模板链接"}
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(url);
              setCopied(true);
              setError("");
            } catch {
              setError("复制失败，请选中链接手动复制。");
            }
          }}
        >
          {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
        </IconButton>
      </div>
      <span className="sr-only" role="status">
        {copied ? "模板链接已复制" : ""}
      </span>
      {error && (
        <p role="alert" className="bot-error mt-3">
          {error}
        </p>
      )}
      {children && <div className="template-card-actions">{children}</div>}
    </article>
  );
}
