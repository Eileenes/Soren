import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IOSDownload } from '../components/IOSDownload';
import { iosDistribution, IOS_GUIDE_URL } from '../services/distribution';
const mocks = vi.hoisted(() => ({ open: vi.fn(), qr: vi.fn() }));
vi.mock('../services/connections', () => ({ openExternal: mocks.open }));
vi.mock('qrcode', () => ({ toDataURL: mocks.qr }));
afterEach(() => { vi.unstubAllEnvs(); vi.clearAllMocks(); });
describe('iOS installation entry', () => {
  it('does not offer an installer or QR code before distribution is available', () => {
    vi.stubEnv('VITE_SOREN_IOS_DOWNLOAD_URL', '');
    render(<IOSDownload />);
    expect(screen.getByRole('button', { name: 'iOS 即将开放' })).toBeDisabled();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '查看发布进度' }));
    expect(mocks.open).toHaveBeenCalledWith(IOS_GUIDE_URL);
  });
  it('uses the same verified TestFlight link for the button and installation QR', async () => {
    const url = 'https://testflight.apple.com/join/Abcd1234';
    vi.stubEnv('VITE_SOREN_IOS_DOWNLOAD_URL', url);
    mocks.qr.mockResolvedValue('data:image/png;base64,aGVsbG8=');
    render(<IOSDownload />);
    fireEvent.click(screen.getByRole('button', { name: '通过 TestFlight 安装' }));
    expect(mocks.open).toHaveBeenCalledWith(url);
    await waitFor(() => expect(screen.getByRole('img')).toHaveAttribute('src', 'data:image/png;base64,aGVsbG8='));
    expect(mocks.qr).toHaveBeenCalledWith(url, expect.any(Object));
  });
  it('reports browser failures without changing the installation state', async () => {
    vi.stubEnv('VITE_SOREN_IOS_DOWNLOAD_URL', '');
    mocks.open.mockRejectedValueOnce(new Error('browser unavailable'));
    render(<IOSDownload />);
    fireEvent.click(screen.getByRole('button', { name: '查看发布进度' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('未能打开浏览器');
  });
  it('accepts App Store pages and rejects deceptive or incomplete installation links', () => {
    expect(iosDistribution('https://apps.apple.com/us/app/soren/id123456789')?.channel).toBe('App Store');
    for (const url of [undefined, '', 'https://testflight.apple.com/', 'https://testflight.apple.com.evil.test/join/Abcd1234', 'https://user:secret@testflight.apple.com/join/Abcd1234', 'http://testflight.apple.com/join/Abcd1234', 'https://testflight.apple.com:444/join/Abcd1234', 'https://testflight.apple.com/join/Abcd1234?token=secret', 'file:///tmp/app.ipa', 'https://github.com/example/app.ipa']) {
      expect(iosDistribution(url)).toBeNull();
    }
  });
});
