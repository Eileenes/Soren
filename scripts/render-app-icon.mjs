#!/usr/bin/env node
// Use a browser renderer so SVG masks and soft shadows survive all icon exports.
import { chromium } from '../frontend/node_modules/playwright/index.mjs';
import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
const root = new URL('../', import.meta.url);
const svg = await readFile(new URL('assets/app-icon.svg', root), 'utf8');
const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: 1024, height: 1024 }, deviceScaleFactor: 1 });
  await page.setContent(`<style>html,body{margin:0;width:100%;height:100%;background:transparent}svg{display:block;width:100%;height:100%}</style>${svg}`);
  await page.locator('svg').screenshot({ path: fileURLToPath(new URL('assets/app-icon-1024.png', root)), omitBackground: true });
  for (const name of ['app-icon.svg', 'favicon.svg']) {
    await writeFile(new URL(`frontend/public/${name}`, root), svg);
  }
  for (const [name, size] of [['apple-touch-icon.png', 180], ['icon-192.png', 192], ['icon-512.png', 512]]) {
    await page.setViewportSize({ width: size, height: size });
    await page.locator('svg').screenshot({ path: fileURLToPath(new URL(`frontend/public/${name}`, root)), omitBackground: true });
  }
  const iconUrl = `data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}`;
  const avatar = await readFile(new URL('assets/logo.svg', root), 'utf8');
  const avatarUrl = `data:image/svg+xml;base64,${Buffer.from(avatar).toString('base64')}`;
  const previewStyle = '<style>html,body{margin:0;width:100%;height:100%;background:#F2F2F2;color:#191919;font-family:Arial,"PingFang SC",sans-serif}img{display:block}h1,p{margin:0}p{color:#666}figure{position:absolute;margin:0}figcaption{margin-top:20px;color:#666;font-size:18px}</style>';
  async function savePreview(name, width, height, content) {
    await page.setViewportSize({ width, height });
    await page.setContent(previewStyle + content);
    await page.locator('img').evaluateAll(images => Promise.all(images.map(image => image.decode())));
    await page.screenshot({ path: fileURLToPath(new URL(`assets/${name}`, root)) });
  }
  await savePreview('app-icon-preview.png', 1400, 900, `
    <img src="${iconUrl}" style="position:absolute;left:72px;top:137px;width:628px;height:628px"/>
    <h1 style="position:absolute;left:790px;top:150px;font-size:32px;font-weight:500">Soren</h1>
    <p style="position:absolute;left:790px;top:210px;font-size:18px">一体式轮廓 · 左厚右薄的黑色弧面</p>
    ${[[256,790,300],[128,1110,360],[64,790,654],[32,912,670],[16,1014,678]].map(([size,x,y]) => `<figure style="left:${x}px;top:${y}px"><img src="${iconUrl}" width="${size}" height="${size}"/><figcaption>${size} px</figcaption></figure>`).join('')}
  `);
  await savePreview('brand-preview.png', 1200, 720, `
    <h1 style="position:absolute;left:60px;top:50px;font-size:38px;font-weight:500">Soren / 黑白立体形象</h1>
    <p style="position:absolute;left:60px;top:114px;font-size:20px">灰白脸面、柔和黑色顶壳，与 Soren 的眨眼微笑。</p>
    <img src="${iconUrl}" style="position:absolute;left:60px;top:217px;width:380px;height:380px"/>
    <img src="${avatarUrl}" style="position:absolute;left:540px;top:285px;width:225px;height:225px"/>
    <div style="position:absolute;left:840px;top:250px;width:297px;height:297px;background:#191919;border-radius:40px;display:grid;place-items:center"><img src="${avatarUrl}" width="225" height="225"/></div>
    <p style="position:absolute;left:60px;top:630px;font-size:20px">应用图标</p>
    <p style="position:absolute;left:540px;top:630px;font-size:20px">Soren 头像</p>
    <p style="position:absolute;left:840px;top:630px;font-size:20px">深色背景</p>
  `);
  console.log('Rendered the master icon, web icons, and preview sheets.');
} finally {
  await browser.close();
}
