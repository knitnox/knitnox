// Knitnox service worker: makes the app installable and receives push messages.
// It never caches pages (chats must always be fresh). Static assets are
// network-first with a cache fallback, so a deploy is visible immediately and
// the app still opens on a flaky connection. Asset URLs carry ?v=<build hash>.
const CACHE = "knitnox-static-v2";

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))));
  self.clients.claim();
});
self.addEventListener("fetch", (e) => {
  const url = new URL(e.request.url);
  if (e.request.method !== "GET" || url.origin !== location.origin || !url.pathname.startsWith("/static/")) return;
  e.respondWith(caches.open(CACHE).then(async (c) => {
    try {
      const r = await fetch(e.request);
      if (r.ok) c.put(e.request, r.clone());
      return r;
    } catch {
      return (await c.match(e.request)) || Response.error();
    }
  }));
});

self.addEventListener("push", (e) => {
  let data = {};
  try { data = e.data ? e.data.json() : {}; } catch { data = { title: "Knitnox", body: e.data?.text() || "" }; }
  e.waitUntil(self.registration.showNotification(data.title || "Knitnox", {
    body: data.body || "", icon: "/static/icons/icon-192.png", badge: "/static/icons/favicon-32.png",
    tag: data.tag || "knitnox", data: { url: data.url || "/" }, renotify: true,
  }));
});
self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const url = new URL(e.notification.data?.url || "/", location.origin).href;
  e.waitUntil(self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
    for (const c of list) if (c.url === url && "focus" in c) return c.focus();
    for (const c of list) if ("navigate" in c) return c.navigate(url).then((w) => w?.focus());
    return self.clients.openWindow(url);
  }));
});
