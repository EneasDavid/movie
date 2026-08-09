import { useState } from "react";
import { AVATAR_URL } from "../../lib/api";

// Default icon when no photo was uploaded — a plain geometric nod to a
// certain helmeted movie character, not a redraw of the actual design:
// just a rounded dome + a T-shaped visor, abstract enough to read as "a
// helmet" without tracing anyone's copyrighted artwork.
function DefaultAvatarIcon({ size }) {
  return (
    <svg width={size} height={size} viewBox="0 0 48 48" aria-hidden="true">
      <circle cx="24" cy="24" r="24" fill="var(--bg-elevated-2)" />
      <path
        d="M24 8c-8 0-13 6-13 13 0 5 2 9 4 11.5.6.7 1.6 1 2.5.7 2-.7 4.2-1.2 6.5-1.2s4.5.5 6.5 1.2c.9.3 1.9 0 2.5-.7 2-2.5 4-6.5 4-11.5 0-7-5-13-13-13z"
        fill="var(--text-dim)"
      />
      <rect x="15" y="21" width="18" height="4" rx="1.5" fill="var(--bg-elevated-2)" />
      <rect x="22.5" y="15" width="3" height="16" rx="1.5" fill="var(--bg-elevated-2)" />
    </svg>
  );
}

export default function Avatar({ hasAvatar, size = 32, className = "" }) {
  const [errored, setErrored] = useState(false);
  const showPhoto = hasAvatar && !errored;

  return (
    <span className={`avatar ${className}`} style={{ width: size, height: size }}>
      {showPhoto ? (
        <img
          className="avatar-img"
          src={`${AVATAR_URL}?t=${hasAvatar}`}
          alt=""
          width={size}
          height={size}
          onError={() => setErrored(true)}
        />
      ) : (
        <DefaultAvatarIcon size={size} />
      )}
    </span>
  );
}
