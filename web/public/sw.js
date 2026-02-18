self.addEventListener('install', event => {
  event.waitUntil(self.skipWaiting())
})

self.addEventListener('activate', event => {
  event.waitUntil(self.clients.claim())
})

// Minimal fetch handler to keep the app installable as a PWA.
self.addEventListener('fetch', () => {})
