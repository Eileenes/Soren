export const RELEASES_URL = 'https://github.com/Eileenes/Soren/releases';
export const IOS_GUIDE_URL = 'https://github.com/Eileenes/Soren/blob/main/docs/IOS-DISTRIBUTION.md';

// Only official Apple installation pages can be encoded in download QR codes.
export function iosDistribution(raw: string | undefined): { url: string; channel: string } | null {
  if (!raw?.trim()) return null;
  try {
    const url = new URL(raw.trim());
    if (url.protocol !== 'https:' || url.username || url.password || url.port || url.search || url.hash) return null;
    if (url.hostname === 'testflight.apple.com' && /^\/join\/[a-zA-Z0-9]{8}\/?$/.test(url.pathname)) {
      return { url: url.href, channel: 'TestFlight' };
    }
    if (url.hostname === 'apps.apple.com' && /^\/(?:[a-z]{2}\/)?app\/(?:[^/]+\/)?id\d+\/?$/.test(url.pathname)) {
      return { url: url.href, channel: 'App Store' };
    }
  } catch { /* Invalid release configuration must not produce an install link. */ }
  return null;
}
