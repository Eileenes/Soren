import { act, cleanup, render } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BotAvatar } from '../components/BotAvatar';

describe('Bot avatar task motion', () => {
  let reduced = false;
  let hidden = false;
  let motionChange: () => void;
  const animations: { cancel: ReturnType<typeof vi.fn>; onfinish: (() => void) | null }[] = [];
  const animate = vi.fn(() => {
    const animation = { cancel: vi.fn(), onfinish: null };
    animations.push(animation);
    return animation;
  });

  beforeEach(() => {
    vi.useFakeTimers();
    animations.length = 0; animate.mockClear(); reduced = false; hidden = false;
    vi.spyOn(Math, 'random').mockReturnValue(.5);
    vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden);
    vi.spyOn(window, 'matchMedia').mockImplementation(query => ({
      media: query, get matches() { return reduced; },
      addEventListener: (_type: string, listener: () => void) => { motionChange = listener; },
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList));
    Object.defineProperty(Element.prototype, 'animate', { configurable: true, value: animate });
  });

  afterEach(() => {
    cleanup(); vi.restoreAllMocks(); vi.useRealTimers();
    Reflect.deleteProperty(Element.prototype, 'animate');
  });

  it('only moves running avatars and cancels all work when the task stops', () => {
    const { rerender } = render(<BotAvatar avatar="robo" />);
    act(() => vi.advanceTimersByTime(5000));
    expect(animate).not.toHaveBeenCalled();
    rerender(<BotAvatar avatar="robo" running />);
    act(() => vi.advanceTimersByTime(1200));
    expect(animate).toHaveBeenCalledTimes(2);
    rerender(<BotAvatar avatar="robo" />);
    expect(animations.every(animation => animation.cancel.mock.calls.length === 1)).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('responds to reduced motion changes and cleans up on unmount', () => {
    reduced = true;
    const { unmount } = render(<BotAvatar avatar="soren" running />);
    expect(vi.getTimerCount()).toBe(0);
    act(() => { reduced = false; motionChange(); vi.advanceTimersByTime(1200); });
    expect(animate).toHaveBeenCalledTimes(2);
    act(() => { reduced = true; motionChange(); });
    expect(animations.every(animation => animation.cancel.mock.calls.length === 1)).toBe(true);
    expect(vi.getTimerCount()).toBe(0);
    act(() => { reduced = false; motionChange(); });
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it('suspends motion when the page is hidden and resumes on return', () => {
    render(<BotAvatar avatar="pudin" running />);
    act(() => { vi.advanceTimersByTime(1200); hidden = true; document.dispatchEvent(new Event('visibilitychange')); });
    expect(vi.getTimerCount()).toBe(0);
    expect(animations.every(animation => animation.cancel.mock.calls.length === 1)).toBe(true);
    act(() => { hidden = false; document.dispatchEvent(new Event('visibilitychange')); vi.advanceTimersByTime(1200); });
    expect(animate).toHaveBeenCalledTimes(4);
  });
});
