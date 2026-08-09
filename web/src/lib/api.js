// Thin wrapper around the backend JSON API. Kept framework-agnostic on
// purpose (plain fetch, no React import) — this is the exact same contract
// a future React Native app would talk to, so it's the natural seam to
// eventually extract into a shared package.

class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

async function request(url, options = {}) {
  const res = await fetch(url, {
    credentials: "same-origin",
    headers: { Accept: "application/json", ...options.headers },
    ...options,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new ApiError(body.error || `Erro ${res.status}`, res.status);
  }
  if (res.status === 204) return null;
  return res.json();
}

function postJSON(url, body) {
  return request(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function fetchCatalog() {
  return request("/api/catalog");
}

export function streamURL(fileId) {
  return `/api/stream?id=${encodeURIComponent(fileId)}`;
}

export function watchPath(item) {
  const params = new URLSearchParams({ id: item.id, title: item.name || "" });
  return `/watch?${params.toString()}`;
}

// --- Auth ---
export function getMe() {
  return request("/api/auth/me");
}

export function signup(email, password, firstName, lastName) {
  return postJSON("/api/auth/signup", { email, password, firstName, lastName });
}

export function login(email, password) {
  return postJSON("/api/auth/login", { email, password });
}

export function logout() {
  return request("/api/auth/logout", { method: "POST" });
}

export function resendVerification() {
  return request("/api/auth/resend-verification", { method: "POST" });
}

export const AVATAR_URL = "/api/auth/avatar";

export function uploadAvatar(file) {
  return request(AVATAR_URL, {
    method: "POST",
    headers: { "Content-Type": file.type },
    body: file,
  });
}

// --- Progress ("continuar assistindo") ---
export async function fetchProgress() {
  const data = await request("/api/progress");
  return data?.entries || [];
}

export function saveProgress(fileId, time, duration, title) {
  return request(`/api/progress?id=${encodeURIComponent(fileId)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ time, duration, title }),
  });
}

export function removeProgress(fileId) {
  return request(`/api/progress?id=${encodeURIComponent(fileId)}`, { method: "DELETE" });
}

export { ApiError };
