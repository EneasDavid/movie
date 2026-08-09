import { useState } from "react";
import { Link } from "react-router-dom";
import { watchPath } from "../../lib/api";
import { NEAR_END_RATIO } from "../../lib/progress";
import { formatTitle } from "../../lib/titleFormat";

export default function Card({ item, index, progress, onRemove }) {
  const [loaded, setLoaded] = useState(false);

  let progressPct = null;
  if (progress && progress.duration > 0) {
    const ratio = Math.min(1, progress.time / progress.duration);
    if (ratio > 0.02 && ratio < NEAR_END_RATIO) progressPct = Math.round(ratio * 100);
  }

  return (
    <Link className="card" to={watchPath(item)} style={{ "--i": index % 12 }} title={formatTitle(item.name)}>
      <div className="card-thumb-wrap">
        {item.thumbnailUrl && (
          <img
            className={`card-thumb${loaded ? " loaded" : ""}`}
            src={item.thumbnailUrl}
            alt=""
            loading="lazy"
            onLoad={() => setLoaded(true)}
          />
        )}
        {progressPct !== null && (
          <div className="card-progress">
            <span className="card-progress-fill" style={{ width: `${progressPct}%` }} />
          </div>
        )}
        <div className="card-hover-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" width="28" height="28"><path fill="currentColor" d="M8 5v14l11-7z" /></svg>
        </div>
        {onRemove && (
          <button
            className="card-remove"
            type="button"
            aria-label={`Remover "${formatTitle(item.name)}" de continuar assistindo`}
            title="Remover de continuar assistindo"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              onRemove(item.id);
            }}
          >
            <svg viewBox="0 0 24 24" width="16" height="16"><path fill="currentColor" d="M19 6.41 17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" /></svg>
          </button>
        )}
      </div>
      <div className="card-title">{formatTitle(item.name)}</div>
    </Link>
  );
}
