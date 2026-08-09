// Mirrors internal/auth/password.go's ValidatePassword rules, for live
// feedback as the user types. The Go side is still the source of truth —
// this is UX only, never trust it as the actual gate.
export const MIN_PASSWORD_LENGTH = 8;

const SPECIAL_CHARS = /[!@#$%^&*()_+\-=[\]{}|;:'",.<>/?~`\\]/;

function hasSequentialDigits(password) {
  const runs = password.match(/\d+/g) || [];
  for (const run of runs) {
    if (run.length < 4) continue;
    for (let i = 0; i <= run.length - 4; i++) {
      const slice = run.slice(i, i + 4);
      const codes = [...slice].map(Number);
      const ascending = codes.every((c, j) => j === 0 || c === codes[j - 1] + 1);
      const descending = codes.every((c, j) => j === 0 || c === codes[j - 1] - 1);
      const repeated = codes.every((c) => c === codes[0]);
      if (ascending || descending || repeated) return true;
    }
  }
  return false;
}

// Returns the checklist shown under the password field — each rule with
// whether it currently passes, in the same order the backend checks them.
export function passwordChecklist(password) {
  return [
    { key: "length", label: `Pelo menos ${MIN_PASSWORD_LENGTH} caracteres`, ok: password.length >= MIN_PASSWORD_LENGTH },
    { key: "upper", label: "Uma letra maiúscula", ok: /[A-Z]/.test(password) },
    { key: "lower", label: "Uma letra minúscula", ok: /[a-z]/.test(password) },
    { key: "digit", label: "Um número", ok: /\d/.test(password) },
    { key: "symbol", label: "Um caractere especial", ok: SPECIAL_CHARS.test(password) },
    { key: "sequential", label: "Sem sequência numérica óbvia (ex: 1234)", ok: password.length === 0 || !hasSequentialDigits(password) },
  ];
}

export function isPasswordValid(password) {
  return passwordChecklist(password).every((rule) => rule.ok);
}
