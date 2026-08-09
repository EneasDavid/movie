// Playback progress persistence. This app has a single user and no
// accounts, so localStorage IS the entire "continue watching" data
// store — one JSON blob per video, keyed by file ID.
const PREFIX = "progress:";
export const MIN_RESUMABLE_SECONDS = 10;
export const NEAR_END_RATIO = 0.95;

export function readProgress(fileId) {
  try {
    const raw = localStorage.getItem(PREFIX + fileId);
    if (!raw) return null;
    const data = JSON.parse(raw);
    if (typeof data.time !== "number" || typeof data.duration !== "number") return null;
    return data;
  } catch {
    return null;
  }
}

// Idempotent by design: writing the same {time, duration} twice just
// overwrites the same key with equivalent data — safe to call from a
// throttled timeupdate handler without any extra guarding.
export function writeProgress(fileId, time, duration, title) {
  try {
    localStorage.setItem(
      PREFIX + fileId,
      JSON.stringify({ time, duration, title, updatedAt: Date.now() })
    );
  } catch {
    // Storage full/unavailable (private browsing etc.) — resuming just
    // won't work this session, not worth surfacing to the user.
  }
}

export function clearProgress(fileId) {
  try {
    localStorage.removeItem(PREFIX + fileId);
  } catch {
    /* ignore */
  }
}

export function allProgressEntries() {
  const entries = [];
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (!key || !key.startsWith(PREFIX)) continue;
    const fileId = key.slice(PREFIX.length);
    const data = readProgress(fileId);
    if (data) entries.push({ fileId, ...data });
  }
  entries.sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0));
  return entries;
}
