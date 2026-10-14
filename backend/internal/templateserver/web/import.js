// Browsers may require permission or a user gesture to open the registered app.
// The visible anchor always provides that fallback; no Bot is imported here.
const openLink = document.getElementById("open-soren");
if (openLink instanceof HTMLAnchorElement) {
  const open = () => {
    try { window.location.assign(openLink.href); } catch { /* Use the visible link. */ }
  };
  if (document.visibilityState === "visible") {
    open();
  } else {
    const onVisible = () => {
      if (document.visibilityState !== "visible") return;
      document.removeEventListener("visibilitychange", onVisible);
      open();
    };
    document.addEventListener("visibilitychange", onVisible);
  }
}

const copy = document.getElementById("copy-link");
const url = document.getElementById("template-url");
const status = document.getElementById("copy-status");
copy?.addEventListener("click", async () => {
  if (!(url instanceof HTMLInputElement)) return;
  try {
    await navigator.clipboard.writeText(url.value);
    copy.setAttribute("aria-label", "已复制链接");
    copy.classList.add("copied");
    status.textContent = "链接已复制";
  } catch {
    url.focus();
    url.select();
    status.textContent = "请手动复制已选中的链接";
  }
});
url?.addEventListener("focus", () => url.select());
