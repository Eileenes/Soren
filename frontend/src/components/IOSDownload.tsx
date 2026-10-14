import { useEffect, useState } from 'react';
import { Button } from './ui';
import { openExternal } from '../services/connections';
import { IOS_GUIDE_URL, RELEASES_URL, iosDistribution } from '../services/distribution';

export function IOSDownload() {
  const download = iosDistribution(import.meta.env.VITE_SOREN_IOS_DOWNLOAD_URL);
  const url = download?.url;
  const [qr, setQR] = useState('');
  const [error, setError] = useState('');
  useEffect(() => {
    let active = true;
    setQR('');
    if (url) {
      void import('qrcode')
        .then((qr) => qr.toDataURL(url, { width: 200, margin: 2, errorCorrectionLevel: 'M' }))
        .then((image) => { if (active) setQR(image); })
        .catch(() => { if (active) setError('安装二维码暂不可用，请打开下载页面。'); });
    }
    return () => { active = false; };
  }, [url]);
  const open = async (target: string) => {
    setError('');
    try { await openExternal(target); }
    catch { setError('未能打开浏览器，请稍后重试。'); }
  };
  return (
    <div className="rounded-2xl bg-surface-1 p-5">
      <div className="flex flex-col justify-between gap-5 sm:flex-row sm:items-center">
        <div className="min-w-0">
          <p className="text-[10px] font-semibold tracking-[.2em] text-ink-3">SOREN FOR iOS</p>
          <h3 className="mt-2 text-base font-semibold text-ink-1">把 Soren 带在身边</h3>
          <p className="mt-2 text-xs leading-6 text-ink-2">
            {download ? '用 iPhone 相机扫码安装，再用 Soren 扫描下方配对码。' : 'iOS 下载待开放，Mac 本地功能可正常使用。'}
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            <Button size="sm" variant="primary" disabled={!download} onClick={() => download && void open(download.url)}>
              {download ? `通过 ${download.channel} 安装` : 'iOS 即将开放'}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => void open(download ? RELEASES_URL : IOS_GUIDE_URL)}>
              {download ? '版本下载' : '查看发布进度'}
            </Button>
          </div>
        </div>
        {url && qr && (
          <figure className="shrink-0 self-center text-center">
            <img src={qr} alt="iOS 安装二维码，请用 iPhone 相机扫描" className="h-32 w-32 rounded-xl" />
            <figcaption className="mt-2 text-[11px] text-ink-3">安装二维码</figcaption>
          </figure>
        )}
      </div>
      {error && <p role="alert" className="mt-3 text-xs text-error">{error}</p>}
    </div>
  );
}
