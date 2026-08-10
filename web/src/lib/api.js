// Thin wrapper around the backend JSON API. Kept framework-agnostic on
// purpose (plain fetch, no React import) — this is the exact same contract
// a future React Native app would talk to, so it's the natural seam to
// eventually extract into a shared package.

class ApiError extends Error {
  constructor(message, status, body) {
    super(message);
    this.status = status;
    // Some endpoints (login/signup when the account isn't verified yet)
    // return structured info alongside the error message — the caller
    // needs the whole body, not just a string, to react correctly.
    this.body = body;
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
    throw new ApiError(body.error || `Erro ${res.status}`, res.status, body);
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

export function streamURL(fileId, version = "") {
  const params = new URLSearchParams({ id: fileId });
  if (version) params.set("v", version);
  return `/api/stream?${params.toString()}`;
}

function browserVideoMime(item) {
  const extension = (item.name || "").split(".").pop()?.toLowerCase();
  if (extension === "mov") return "video/quicktime";
  if (extension === "mp4" || extension === "m4v") return "video/mp4";
  if (extension === "webm") return "video/webm";
  return item.mimeType?.startsWith("video/") ? item.mimeType : "";
}

export function watchPath(item) {
  const params = new URLSearchParams({
    id: item.id,
    title: item.name || "",
    v: item.modifiedTime || "",
		type: browserVideoMime(item),
  });
  return `/watch?${params.toString()}`;
}

// --- Auth ---
export function getMe() {
  return request("/api/auth/me");
}

// Resolves with either a full user (already the shape AuthContext wants)
// or, if the account needs confirmation first, {email, pendingVerification: true}.
export function signup(email, password, firstName, lastName) {
  return postJSON("/api/auth/signup", { email, password, firstName, lastName });
}

// Resolves with a full user on success. Throws ApiError with
// err.body.pendingVerification set when the credentials are correct but
// the email isn't confirmed yet — err.body.email is the account's email,
// for prefilling the "check your inbox" screen.
export function login(email, password) {
  return postJSON("/api/auth/login", { email, password });
}

export function logout() {
  return request("/api/auth/logout", { method: "POST" });
}

export function resendVerification(email) {
  return request("/api/auth/resend-verification", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
}

export function forgotPassword(email) {
  return request("/api/auth/forgot-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email }),
  });
}

export function resetPassword(token, password) {
  return request("/api/auth/reset-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ token, password }),
  });
}

export const AVATAR_URL = "/api/auth/avatar";

export function uploadAvatar(file) {
  return request(AVATAR_URL, {
    method: "POST",
    headers: { "Content-Type": file.type },
    body: file,
  });
}

export function updateProfile(firstName, lastName) {
  return request("/api/auth/profile", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ firstName, lastName }),
  });
}

export function changePassword(currentPassword, newPassword) {
  return request("/api/auth/change-password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ currentPassword, newPassword }),
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
