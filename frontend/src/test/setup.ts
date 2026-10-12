/**
 * 测试环境初始化
 *
 * jsdom 缺少若干浏览器 API，这里补齐应用实际用到的最小集合。
 */

import '@testing-library/jest-dom/vitest';
import { afterEach, vi } from 'vitest';
import { cleanup } from '@testing-library/react';

// React 18 的并发调度依赖 MessageChannel；jsdom 未实现它
if (typeof globalThis.MessageChannel === 'undefined') {
  class FakePort {
    onmessage: ((event: { data: unknown }) => void) | null = null;
    private other: FakePort | null = null;

    link(other: FakePort): void {
      this.other = other;
    }

    postMessage(data: unknown): void {
      const target = this.other;
      if (target && typeof target.onmessage === 'function') {
        setTimeout(() => target.onmessage?.({ data }), 0);
      }
    }

    addEventListener(type: string, handler: (event: { data: unknown }) => void): void {
      if (type === 'message') this.onmessage = handler;
    }

    removeEventListener(): void {}
    start(): void {}
    close(): void {}
  }

  class FakeMessageChannel {
    port1 = new FakePort();
    port2 = new FakePort();

    constructor() {
      this.port1.link(this.port2);
      this.port2.link(this.port1);
    }
  }

  // @ts-expect-error 补齐 jsdom 缺失的全局实现
  globalThis.MessageChannel = FakeMessageChannel;
}

// 滚动相关 API 在 jsdom 中为空实现
if (!HTMLElement.prototype.scrollTo) {
  HTMLElement.prototype.scrollTo = function scrollTo() {};
}
if (!HTMLElement.prototype.scrollIntoView) {
  HTMLElement.prototype.scrollIntoView = function scrollIntoView() {};
}

// matchMedia
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia;
}

afterEach(() => {
  cleanup();
});
