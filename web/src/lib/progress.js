// Shared thresholds for "continuar assistindo" — used both when deciding
// what to show (skip anything barely started or already finished) and
// when deciding whether a resume-seek is worth doing. Progress itself now
// lives server-side (see lib/api.js fetchProgress/saveProgress/
// removeProgress) rather than localStorage, since it's tied to a real
// account now, not just "whoever's browser this is".
export const MIN_RESUMABLE_SECONDS = 10;
export const NEAR_END_RATIO = 0.95;
