// Thin wrapper around the backend JSON API. Kept framework-agnostic on
// purpose (plain fetch, no React import) — this is the exact same contract
// a future React Native app would talk to, so it's the natural seam to
// eventually extract into a shared package.

async function getJSON(url) {
  const res = await fetch(url, { headers: { Accept: "application/json" } });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Erro ${res.status}`);
  }
  return res.json();
}

export function fetchCatalog() {
  return getJSON("/api/catalog");
}

export function streamURL(fileId) {
  return `/api/stream?id=${encodeURIComponent(fileId)}`;
}

export function watchPath(item) {
  const params = new URLSearchParams({ id: item.id, title: item.name || "" });
  return `/watch?${params.toString()}`;
}
