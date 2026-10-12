// Cache only the public application shell. Never cache API responses or credentials.
const CACHE = 'soren-shell-v2';
self.addEventListener('install', () => self.skipWaiting());
self.addEventListener('activate', event => event.waitUntil((async () => {
  for (const key of await caches.keys()) if (key.startsWith('soren-shell-') && key !== CACHE) await caches.delete(key);
  await self.clients.claim();
})()));
self.addEventListener('fetch', event => {
  const request = event.request;
  const url = new URL(request.url);
  if (request.method !== 'GET' || url.origin !== self.location.origin || url.search || url.pathname.startsWith('/api/')) return;
  const asset = /^\/(assets\/|bot-avatars\/)/.test(url.pathname) || /^\/(icon-\d+\.png|soren-mark\.svg|manifest\.webmanifest)$/.test(url.pathname);
  if (request.mode !== 'navigate' && !asset) return;
  event.respondWith((async () => {
    const cache = await caches.open(CACHE);
    const key = request.mode === 'navigate' ? '/' : request;
    try { const response = await fetch(request); if (response.ok && response.type === 'basic') await cache.put(key, response.clone()); return response; }
    catch (error) { const saved = await cache.match(key); if (saved) return saved; throw error; }
  })());
});
